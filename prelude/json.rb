# prelude/json.rb
# rbs_inline: enabled

#
# JSON, matching the json gem (2.x): `require "json"` gives every object
# #to_json, and JSON.parse/.load reads a document back (decision 25).

module Kernel
  # json's Object#to_json: the JSON string of to_s.
  #: (*untyped) -> String
  def to_json(*state) = %x{ return rbToS(self).ToJson(rest_...) }
end

class String
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONString(string(self), rbJSONStateOf(rest_)) }
end

class Symbol
  #: (*untyped) -> String
  def to_json(*state) = %x{ self.ToS().ToJson(rest_...) }
end

class Integer
  #: (*untyped) -> String
  def to_json(*_state) = to_s
end

class Float
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONFloat(float64(self), rbJSONStateOf(rest_)) }
end

class Boolean
  #: (*untyped) -> String
  def to_json(*_state) = to_s
end

class Array
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONArray(self.s, rest_) }
end

class Hash
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONHash(self, rest_) }
end

module JSON
  class GeneratorError < StandardError; end
  class ParserError < StandardError; end
  class NestingError < ParserError; end

  #: (untyped, *untyped) -> String
  def self.generate(obj, *state) = %x{ rbJSONGenerate(obj, rbJSONStateOf(rest_)) }

  # PRETTY_STATE_PROTOTYPE: two-space indent, one value per line.
  # opts: nil (Homebrew's Tab#to_json(options = nil) passes it on); ponytail: a Hash of options raises, merge it into the pretty state when a program needs one.
  #: (untyped, ?untyped) -> String
  def self.pretty_generate(obj, opts = nil) = %x{
    if opts != nil {
      panic(NewNotImplementedError(Ref(String("JSON.pretty_generate with options is not supported"))))
    }
    st := &rbJSONState{indent: "  ", space: " ", objectNl: "\\n", arrayNl: "\\n", maxNesting: 100}
    return rbJSONGenerate(obj, st)
  }

  #: (untyped) -> String
  def self.dump(obj) = %x{ rbJSONGenerate(obj, rbJSONStateOfMax(nil, 0, true)) }

  # dump(obj, io) writes to io and returns it; nil or options give the String; an Integer limit raises NotImplementedError.
  #: (untyped, untyped) -> untyped
  # @dynamic
  def self.__dump_2(obj, arg)
    return __dump_state(obj, arg) if arg.nil? || arg.is_a?(Hash)
    raise NotImplementedError, "JSON.dump limit is not supported" if arg.is_a?(Integer)
    raise TypeError, "no implicit conversion of #{arg.class} into Integer" unless arg.respond_to?(:write)

    arg.write(dump(obj))
    arg
  end

  #: (untyped, untyped) -> String
  def self.__dump_state(obj, arg) = %x{ rbJSONGenerate(obj, rbJSONStateOfMax([]any{arg}, 0, true)) }

  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.parse(str, opts = {}) = %x{ rbJSONParse(string(str), opts) }

  # MRI's .load is .parse with unsafe defaults meant for Marshal-like data (max_nesting stays 100); rugaroo's closed, untyped-only world has nothing unsafe to opt out of.
  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.load(str, opts = {}) = parse(str, opts)
end
