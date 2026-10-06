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

  #: (untyped, *untyped) -> String
  def self.generate(obj, *state) = %x{ rbToJson(obj, rbJSONStateOf(rest_)) }

  # PRETTY_STATE_PROTOTYPE: two-space indent, one value per line.
  #: (untyped) -> String
  def self.pretty_generate(obj) = %x{
    st := &rbJSONState{indent: "  ", space: " ", objectNl: "\\n", arrayNl: "\\n"}
    return rbToJson(obj, st)
  }

  #: (untyped, *untyped) -> String
  def self.dump(obj, *state) = %x{ rbToJson(obj, rbJSONStateOf(rest_)) }

  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.parse(str, opts = {}) = %x{ rbJSONParse(string(str), rbJSONSymbolizeNames(opts)) }

  # MRI's .load is .parse with unsafe defaults meant for Marshal-like data; rb2go's closed, untyped-only world has nothing unsafe to opt out of.
  #: (String, ?Hash[Symbol, untyped]) -> untyped
  def self.load(str, opts = {}) = parse(str, opts)
end
