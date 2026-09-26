# skip: a value typed as a module (Array[Printable]) is Go `any`, and calling the module's methods on it instantiates Printable_ToS[any], which go build rejects (any does not satisfy Printable_Self)

# rbs_inline: enabled

module Printable
  #: () -> String
  def title = raise(NotImplementedError)

  #: () -> String
  def to_s = "P(#{title})"
end

class Doc
  include Printable

  #: () -> String
  def title = "doc"
end

class Memo
  include Printable

  #: () -> String
  def title = "memo"
end

items = [Doc.new, Memo.new] #: Array[Printable]
puts items.map { |i| i.title }.inspect
puts items.map(&:to_s).inspect
