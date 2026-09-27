# prelude/dynamic.rb
# rbs_inline: enabled
#
# Runtime support for calls on untyped values. The transpiler generates, per
# method name called that way, a `DynName(args ...any) any` wrapper on each
# class having the method (`_DynName` if it is private), and an `rbDynName`
# dispatcher: the wrapper, else method_missing, else NoMethodError. These
# helpers check arity and convert arguments the way a typed call would have.

