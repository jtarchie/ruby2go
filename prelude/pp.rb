# rbs_inline: enabled

# pp and PrettyPrint (decision 113). PP is the printer a class's own
# `pretty_print(q)` receives: text, breakable, group, nest, pp.
# @go_type struct { q *rbPrettyPrint }
class PP < Object
  # Prints obj laid out to width columns (PP.width_for($stdout) by default) to out ($stdout, or appended to a String, which is returned).
  #: (untyped, ?untyped, ?Integer?) -> untyped
  def self.pp(obj, out = nil, width = nil) = %x{
    var f *os.File
    switch o := rbUnbox(out).(type) {
    case nil:
      f = os.Stdout
    case *IO:
      f = o.rbOSFile()
    }
    w := rbPPWidthFor(f)
    if width != nil {
      w = int(*width)
    }
    s := rbPrettyInspect(obj, w)
    switch o := rbUnbox(out).(type) {
    case nil:
      rbWriteOut(s)
      return nil
    case String:
      return o + String(s)
    case interface{ Write(any) Integer }:
      o.Write(String(s))
      return out
    }
    panic(NewTypeError(Ref(String("PP.pp: out must be an IO or a String"))))
  }

  #: () -> Integer
  def self.width_for_stdout = %x{ return Integer(rbPPWidthFor(os.Stdout)) }

  #: (String) -> void
  def text(s) = %x{ self.q.text(string(s)) }

  #: (?String) -> void
  def breakable(sep = " ") = %x{ self.q.breakable(string(sep)) }

  #: () -> void
  def comma_breakable = %x{ self.q.commaBreakable() }

  #: (?Integer, ?String, ?String) { () -> void } -> void
  def group(indent = 0, open = "", close = "") = %x{ self.q.group(int(indent), string(open), string(close), blk) }

  #: (Integer) { () -> void } -> void
  def nest(indent) = %x{ self.q.nest(int(indent), blk) }

  #: (untyped) -> void
  def pp(obj) = %x{ self.q.pp(obj) }

  #: (untyped) { () -> void } -> void
  def object_group(obj) = %x{ self.q.group(1, "#<"+rbClassName(rbUnbox(obj)), ">", blk) }

  # @rbs [E] (Array[E]) { (E) -> void } -> void
  def seplist(list) = %x{
    items := list.s
    self.q.seplist(len(items), nil, func(i int) { blk(items[i]) })
  }
end

module Kernel
  # PP.pp(self, +""): laid out to $COLUMNS - 1 (79 when unset), a String having no terminal width.
  #: () -> String
  def pretty_inspect = %x{ return String(rbPrettyInspect(any(self), rbPPWidthFor(nil))) }

  private

  # Pretty-prints each object on its own line; returns its argument, or the arguments as an Array.
  #: (*untyped) -> untyped
  def pp(*objs) = %x{
    w := rbPPWidthFor(os.Stdout)
    for _, o := range rest_ {
      rbWriteOut(rbPrettyInspect(o, w))
    }
    switch len(rest_) {
    case 0:
      return nil
    case 1:
      return rest_[0]
    }
    return &Array[any]{s: rest_}
  }
end
