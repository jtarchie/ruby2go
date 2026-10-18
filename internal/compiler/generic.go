package compiler

import (
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"slices"

	"github.com/jtarchie/ruby2go/internal/rbs"
)

// Generic struct classes are monomorphized (decision 308): a template is never emitted, each closed instantiation is a cloned struct class.

// errNeedInstances: a compile named an instantiation it did not create before link; compile runs again with it.
var errNeedInstances = errors.New("generic instances to create")

// needInstances is the panic behind errNeedInstances.
type needInstances struct{}

// maxInstanceNesting: deeper type arguments are taken for polymorphic recursion (`Box[T]` naming `Box[Array[T]]`), which needs instances without end.
const maxInstanceNesting = 8

// isTemplate reports a generic struct class: instantiated per type arguments, never emitted itself.
func (c *Class) isTemplate() bool {
	return c.template == nil && len(c.TypeParams) > 0 && c.isStruct() && !c.universal && !c.hashBase && c.metaOf == nil && c.superRef == nil
}

// instKey names an instantiation as RBS: the key instances are memoized and requested by.
func instKey(tmpl *Class, args []Type) string { return tmpl.RubyName + "[" + joinTypes(args) + "]" }

// instOf is tmpl at closed args; one requested after makeInstances is nil, and the compile restarts with it (instMissing).
func (c *Compiler) instOf(tmpl *Class, args []Type, at srcPos) *Class {
	key := instKey(tmpl, args)
	if inst := c.insts[key]; inst != nil {
		return inst
	}
	if typeDepth(TClass{C: tmpl, Args: args}) > maxInstanceNesting {
		c.errorAt(at, "%s instantiates %s without end (polymorphic recursion): a generic class's methods may not name it at type arguments built from its own", key, tmpl.RubyName)
	}
	if !c.instOpen { // typed as the template until the restart, so one pass finds every sibling instance
		if !slices.Contains(*c.instReqs, key) {
			*c.instReqs = append(*c.instReqs, key)
		}
		c.instMissing = true
		return nil
	}
	inst := c.cloneTemplate(tmpl, args)
	c.insts[key] = inst
	return inst
}

// maxInstanceRestarts bounds the compiles one program may take to find its instances: each finds every instance its pass names.
const maxInstanceRestarts = 32

// typeDepth is how deeply t nests type arguments.
func typeDepth(t Type) int {
	d := 0
	switch t := t.(type) {
	case TClass:
		for _, a := range t.Args {
			d = max(d, typeDepth(a))
		}
		for _, a := range t.C.instArgs {
			d = max(d, typeDepth(a))
		}
		if len(t.Args)+len(t.C.instArgs) > 0 {
			d++
		}
	case TOpt:
		d = typeDepth(t.Elem)
	case TTuple:
		for _, e := range t.Elems {
			d = max(d, typeDepth(e))
		}
		d++
	case TUnion:
		for _, e := range t.Members {
			d = max(d, typeDepth(e))
		}
	case TFunc:
		for _, p := range t.Params {
			d = max(d, typeDepth(p))
		}
		d = max(d, typeDepth(t.Ret)) + 1
	case TAny, TNil, TVar, TVoid, nil:
	}
	return d
}

// makeInstances creates the instances an earlier pass of this compile requested, before link.
func (c *Compiler) makeInstances(reqs *[]string) {
	c.insts, c.instReqs, c.instOpen = map[string]*Class{}, reqs, true
	for i := 0; i < len(*reqs); i++ { //nolint:intrange // creating one may request its arguments' instances, which append
		t, err := rbs.ParseType("::" + (*reqs)[i])
		if err != nil {
			c.errorf(nil, nil, "generic instance %s: %v", (*reqs)[i], err)
		}
		_ = c.resolveType(t, typeScope{})
		if c.instMissing {
			c.errorf(nil, nil, "generic instance %s: not created", (*reqs)[i])
		}
	}
	c.instOpen = false
}

// cloneTemplate copies tmpl's collected declarations into a new class bound to args: link resolves the copies'
// signatures and ivars with tmpl's parameters bound (resolveNameType), as for any class.
func (c *Compiler) cloneTemplate(tmpl *Class, args []Type) *Class {
	key := instKey(tmpl, args)
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	inst := &Class{Name: fmt.Sprintf("%s_G%08x", tmpl.Name, h.Sum32()), RubyName: tmpl.RubyName, template: tmpl, instArgs: args,
		Methods: map[string]*Method{}, Ivars: map[string]*Ivar{}, srcPos: tmpl.srcPos, superRef: tmpl.superRef,
		ivarDecls: tmpl.ivarDecls, privateNew: tmpl.privateNew, undefs: tmpl.undefs, delegations: tmpl.delegations}
	for _, inc := range tmpl.Includes {
		inc.Mod, inc.Args = nil, nil
		inst.Includes = append(inst.Includes, inc)
	}
	for _, m := range tmpl.MethodList {
		mm := *m
		mm.Owner = inst
		mm.pendingIdx = maps.Clone(m.pendingIdx)
		inst.MethodList = append(inst.MethodList, &mm)
		inst.Methods[mm.Name] = &mm
	}
	for _, iv := range tmpl.IvarList {
		cp := *iv
		cp.Owner = inst
		inst.IvarList = append(inst.IvarList, &cp)
		inst.Ivars[cp.Name] = &cp
	}
	c.classList = append(c.classList, inst)
	tmpl.instances = append(tmpl.instances, inst)
	return inst
}
