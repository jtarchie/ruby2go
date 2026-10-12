package compiler

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
)

// sendSpec is a type-switch dispatcher for a struct class's method that some subclass overrides (#96 phase 3b).
type sendSpec struct {
	owner *Class
	m     *Method
}

// sendName names the dispatcher of owner's method m.
func sendName(owner *Class, m *Method) string { return "rbSend_" + owner.Name + "_" + m.GoName }

// dispatchable reports whether a call of owner's method m on a receiver of static type owner can bypass Go method
// dispatch: a direct call to the free func when nothing below owner overrides it, else an rbSend_ type switch over the
// classes that do. Then no class needs a Go forwarder for the name and owner's interface does not declare it. Only the
// plain case qualifies: a struct class (not a module, metaclass or universal owner), a plainSelf public method
// (decision 210) nothing reaches through a Go method set (the std library, prelude Go's interface assertions, a
// universal protocol name, an including module's self call), and every override below with the same Go signature.
func (c *Compiler) dispatchable(owner *Class, m *Method) bool {
	if v, ok := c.dispatchMemo[m]; ok {
		return v
	}
	ok := c.dispatchableUncached(owner, m)
	if c.dispatchMemo == nil {
		c.dispatchMemo = map[*Method]bool{}
	}
	c.dispatchMemo[m] = ok
	return ok
}

func (c *Compiler) dispatchableUncached(owner *Class, m *Method) bool {
	ok, why := c.dispatchableWhy(owner, m)
	if os.Getenv("RB2GO_DISPATCH_DEBUG") != "" && owner != nil && owner.universal && !ok {
		fmt.Fprintf(os.Stderr, "dispatch-debug %s#%s: %s\n", owner.Name, m.Name, why)
	}
	return ok
}

func (c *Compiler) dispatchableWhy(owner *Class, m *Method) (bool, string) {
	if why := c.dispatchableOwner(owner, m); why != "" {
		return false, why
	}
	ps, ret := c.sig(m, map[string]Type{})
	for _, k := range c.classList {
		if !k.isStruct() || !k.isSubclassOf(owner) {
			continue
		}
		if !owner.universal && c.includerCalls(k, m.Name) {
			return false, "includerCalls" // a module body reaches it through its constraint interface (a universal name leaves the constraint instead)
		}
		impl := k.lookup(m.Name)
		if impl == nil || impl.M == m {
			continue
		}
		if !impl.Owner.isStruct() || !c.plainMethod(impl.M) || !c.plainSelf(impl.M) {
			return false, "override in " + impl.Owner.Name + " not plain"
		}
		if p2, r2 := c.sig(impl.M, map[string]Type{}); p2 != ps || r2 != ret {
			return false, "override in " + impl.Owner.Name + " has another signature" // an adapter answers it (shadowed)
		}
	}
	return true, ""
}

// dispatchableAll reports whether every ancestor of cls (cls included) that defines name dispatches it, so no interface
// above cls declares it and cls needs no Go method for it; one ancestor that keeps the Go method keeps it on every
// descendant (its interface still declares the name).
func (c *Compiler) dispatchableAll(cls *Class, name string) bool {
	found := false
	for k := cls; k != nil; k = k.Super {
		if e := k.lookup(name); e != nil {
			found = true
			if !c.dispatchable(e.Owner, e.M) {
				return false // a module's or an ancestor's kept method: its interface declares the name
			}
		}
	}
	return found
}

// dispatchableOwner checks the owner and the method itself; "" when they qualify, else why not.
func (c *Compiler) dispatchableOwner(owner *Class, m *Method) string {
	switch {
	case owner == nil || m.Owner != owner || owner.hashBase || len(owner.TypeParams) > 0:
		return "owner"
	case !owner.isStruct() && !owner.universal:
		return "owner is not a struct class"
	case !c.plainMethod(m):
		return "not plain method"
	case !c.plainSelf(m):
		return "not plainSelf"
	case stdMethodNames[m.GoName] || genericProtocol[m.Name] || c.goCalled()[m.GoName] || !owner.universal && c.universalHas(m.Name):
		return "protocol/goCalled"
	}
	return ""
}

// universalDispatch reports whether name resolves on every object to a universal owner's dispatchable method, so no
// module constraint declares it and no class carries a forwarder for it.
func (c *Compiler) universalDispatch(name string) bool {
	e := c.classes["Object"].lookup(name)
	return e != nil && e.Owner.universal && c.dispatchable(e.Owner, e.M)
}

// plainMethod: a public def with one Go signature and no adapters.
func (c *Compiler) plainMethod(m *Method) bool {
	return (m.Kind == kindDef || m.Kind == kindPrimitive) && !m.Private && !m.generic() && m.Name != "initialize" && !m.seqAdapter && len(m.shadowed) == 0 && m.SelfType == nil
}

// sendArms lists the classes below owner whose own resolution of m is not owner's body: the dispatcher's cases.
func (c *Compiler) sendArms(owner *Class, m *Method) []*Class {
	var out []*Class
	for _, k := range c.sortedClasses() {
		if k == owner || !k.isStruct() || !k.isSubclassOf(owner) {
			continue
		}
		if impl := k.lookup(m.Name); impl != nil && impl.M != m {
			out = append(out, k)
		}
	}
	return out
}

// sendCall renders a dispatchable call: the free func when nothing overrides m below the receiver's static class,
// else its rbSend_ dispatcher, registered for emitSends.
func (c *Compiler) sendCall(e *entry, recv, args string) string {
	if len(c.sendArms(e.Owner, e.M)) == 0 {
		return freeFuncName(e.M) + "(" + recv + comma(args) + ")"
	}
	name := sendName(e.Owner, e.M)
	if c.sends == nil {
		c.sends = map[string]sendSpec{}
	}
	c.sends[name] = sendSpec{e.Owner, e.M}
	return name + "(" + recv + comma(args) + ")"
}

// emitSends emits the dispatchers kept code reaches, one round of emitNext.
func (c *Compiler) emitSends(reached func(string) bool) {
	for _, name := range slices.Sorted(maps.Keys(c.sends)) {
		if c.sendOut[name] || !c.dynEvery && !reached(name) {
			continue
		}
		if c.sendOut == nil {
			c.sendOut = map[string]bool{}
		}
		c.sendOut[name] = true
		s := c.sends[name]
		ps, ret := c.sig(s.m, map[string]Type{})
		args := c.argNames(s.m)
		call := func(recv string, m *Method) string {
			code := freeFuncName(m) + "(" + recv + comma(args) + ")"
			if ret == "" {
				return code + "\n\t\treturn"
			}
			return "return " + code
		}
		arms := c.sendArms(s.owner, s.m)
		c.labels[name] = s.owner.RubyName + "#" + s.m.Name
		c.w("func %s(self %s%s) %s {\n", name, c.plainSelfType(s.owner), comma(ps), ret)
		if len(arms) == 1 { // gocritic rejects a one-case switch
			k := arms[0]
			c.w("\tif v, ok := self.(*%s); ok {\n\t\t%s\n\t}\n", k.Name, call("v", k.lookup(s.m.Name).M))
		} else {
			c.w("\tswitch v := self.(type) {\n")
			for _, k := range arms {
				c.w("\tcase *%s:\n\t\t%s\n", k.Name, call("v", k.lookup(s.m.Name).M))
			}
			c.w("\t}\n")
		}
		dflt := freeFuncName(s.m) + "(self" + comma(args) + ")"
		if ret == "" {
			c.w("\t%s\n}\n\n", dflt)
		} else {
			c.w("\treturn %s\n}\n\n", dflt)
		}
	}
}

var goSelectorCall = regexp.MustCompile(`\.([A-Z_]\w*)\(`)

// goCalled is every capitalized method name the prelude's Go text (`%x{}` bodies, prelude/go helpers) calls by selector:
// Go code reaches those through a method set, so they stay Go methods. Ruby method names are lowercase, so the scan over
// the .rb sources only sees their Go escapes.
func (c *Compiler) goCalled() map[string]bool {
	if c.goCalledNames != nil {
		return c.goCalledNames
	}
	c.goCalledNames = map[string]bool{}
	for _, pattern := range []string{"prelude.rb", "prelude/*.rb", "prelude/go/*.go"} {
		names, _ := fs.Glob(c.preludeFS, pattern)
		for _, name := range names {
			src, err := fs.ReadFile(c.preludeFS, name)
			if err != nil {
				panic(compileError{msg: fmt.Sprintf("prelude: %v", err)})
			}
			for _, m := range goSelectorCall.FindAllSubmatch(src, -1) {
				c.goCalledNames[string(m[1])] = true
			}
		}
	}
	return c.goCalledNames
}
