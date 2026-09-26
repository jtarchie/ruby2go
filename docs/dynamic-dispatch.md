# Plan: constants reflection and dynamic dispatch

Goal: compile jtarchie/resty's own library code, which relies on
`constantize`, `const_get`, `constants` and `method_missing`. Everything is
generated from the closed world (one program, every class known); the output
never imports `reflect`.

Policy: reflective dispatch is supported only on `untyped` receivers, is
generated statically, and costs nothing unless used. Every dynamic call site
gets a transpiler warning with `file:line`.

Out of scope: `eval`, `define_method`, `instance_variable_get`,
`Object#methods`, blocks across a dynamic call, `class << self`.

1. **Class objects for every class and module.** Prelude `Module` and
   `Class < Module`; metaclasses inherit from the parent's metaclass, else
   from `Class` (modules: `Module`). `name`, `to_s`, `inspect`, `==`,
   `x.class` for primitives and for class objects.
2. **Constant tables.** Per class object, a generated table in definition
   order (own and inherited constants); a generated path lookup for
   `Object.const_get("A::B")` and `String#constantize`. `NameError` on a
   miss. Typing: `M.const_get(x)` is the join of the constant types of `M`
   and its descendants; `untyped` when they share none. Constants initialize
   in `main` in source order, as MRI runs them.
3. **`extend`, `&block`, `camelize`.** `extend M` includes M in the
   metaclass; `&block` parameters forward into iterators and closures;
   `String#camelize`.
4. **`Struct.new(:a, :b) do ... end`** with member types from a trailing
   `#: [A, B]`.
5. **Static `method_missing`**: on a typed receiver whose class defines it,
   an unknown method calls `MethodMissing(:name, args)`. `respond_to?` folds
   to a constant or calls `respond_to_missing?`.
6. **Generated dynamic send** on `untyped` receivers: per method name a
   `Dyn_name(args ...any) any` wrapper on every class that has the method,
   an interface per name, and call sites that type-switch to the wrapper,
   then `method_missing`, then `NoMethodError`. `send`/`public_send`.
7. **resty as written**: resty's library code with only Rack, ActiveRecord
   and the inflector swapped, producing the same transcript as example 25.

Risks: wrapper count grows with dynamic names x classes (bounded by the
closed world); `const_get` joins may fall to `untyped`; MRI's `constants`
order is its symbol-table order, ours is definition order, so examples must
not print it unsorted.
