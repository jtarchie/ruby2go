package compiler

import "slices"

// Instance variables in a module (decision 147). A module's methods compile
// to Go functions generic over Self, with no struct of their own, so a
// module with `# @rbs @x: T` declarations gets a state struct
// (`M_Ivars`), each struct class that includes it a field of that type,
// and an accessor `_M() *M_Ivars` that the module's constraint requires:
// `@x` in the module's methods is then `self._M().x`, as a class's own
// ivar is `self._Foo().x`. Per-object state, no side table and no lock.

// ivarModules lists the modules with instance variables in cls's ancestors.
func ivarModules(cls *Class) []*Class {
	if cls == nil {
		return nil
	}
	var out []*Class
	for _, a := range cls.allAncestors() {
		if a.IsModule && len(a.IvarList) > 0 && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

// ownIvarModules are those a struct class holds itself: not reached
// through its struct superclass, whose embedded field already has them.
func ownIvarModules(cls *Class) []*Class {
	var inherited []*Class
	if cls.Super != nil && !cls.Super.universal {
		inherited = ivarModules(cls.Super)
	}
	var out []*Class
	for _, m := range ivarModules(cls) {
		if !slices.Contains(inherited, m) {
			out = append(out, m)
		}
	}
	return out
}

func ivarsType(mod *Class) string { return mod.Name + "_Ivars" }

func ivarsField(mod *Class) string { return "rbIv_" + mod.Name }

// checkIvarModules rejects a module with instance variables where it has
// no struct to live in: a @go_type class, or Object (main's include).
func (c *Compiler) checkIvarModules() {
	for _, cls := range c.classList {
		if cls.IsModule || cls.isStruct() && !cls.universal {
			continue
		}
		for _, m := range ivarModules(cls) {
			c.errorf(cls.File, nil, "%s:%d: %s has instance variables, so only a class whose objects are structs can include it, not %s (decision 147)", cls.File.Name, cls.Line, m.RubyName, cls.RubyName)
		}
	}
	for _, cls := range c.classList {
		if cls.IsModule && len(cls.IvarList) > 0 && len(cls.TypeParams) > 0 {
			c.errorf(cls.File, nil, "%s:%d: generic module %s cannot have instance variables (decision 147)", cls.File.Name, cls.Line, cls.RubyName)
		}
	}
}
