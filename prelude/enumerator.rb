# prelude/enumerator.rb
# rbs_inline: enabled
#
# Blockless map/select/reject return these, so `.with_index { … }` maps or
# filters as MRI's Enumerator does. Other blockless iterators return an
# Array standing in for the Enumerator (the `__<name>_enum` overloads).

module Enumerator
  # @rbs generic E
  # @go_type struct { items *Array[E] }
  class Map < Object
    #: [X] (Array[X]) -> Map[X]
    def self.new(items) = %x{ return &Enumerator_Map[X]{items: items} }

    #: [U] (?Integer) { (E, Integer) -> U } -> Array[U]
    def with_index(offset = 0) = %x{
      out := &Array[U]{}
      for i, x := range *self.items {
        *out = append(*out, blk(x, Integer(i)+offset))
      }
      return out
    }

    #: [U] () { (E, Integer) -> U } -> Array[U]
    def each_with_index = %x{
      out := &Array[U]{}
      for i, x := range *self.items {
        *out = append(*out, blk(x, Integer(i)))
      }
      return out
    }

    #: () -> Array[E]
    def to_a = %x{ self.items }

    #: () -> Map[untyped]
    def _to_any = %x{ return &Enumerator_Map[any]{items: self.items._ToAny()} }

    #: () -> String
    def inspect = "#<Enumerator: #{to_a.inspect}:map>"

    #: () -> String
    def to_s = inspect
  end

  # @rbs generic E
  # @go_type struct { items *Array[E]; negate bool; name string }
  class Select < Object
    #: [X] (Array[X], bool, String) -> Select[X]
    def self.new(items, negate, name) = %x{ return &Enumerator_Select[X]{items: items, negate: bool(negate), name: string(name)} }

    #: (?Integer) { (E, Integer) -> bool } -> Array[E]
    def with_index(offset = 0) = %x{
      out := &Array[E]{}
      for i, x := range *self.items {
        if bool(blk(x, Integer(i)+offset)) != self.negate {
          *out = append(*out, x)
        }
      }
      return out
    }

    #: () { (E, Integer) -> bool } -> Array[E]
    def each_with_index = %x{
      out := &Array[E]{}
      for i, x := range *self.items {
        if bool(blk(x, Integer(i))) != self.negate {
          *out = append(*out, x)
        }
      }
      return out
    }

    #: () -> Array[E]
    def to_a = %x{ self.items }

    #: () -> Select[untyped]
    def _to_any = %x{ return &Enumerator_Select[any]{items: self.items._ToAny(), negate: self.negate, name: self.name} }

    #: () -> String
    def inspect = "#<Enumerator: #{to_a.inspect}:#{__name}>"

    #: () -> String
    def to_s = inspect

    #: () -> String
    def __name = %x{ String(self.name) }
  end
end
