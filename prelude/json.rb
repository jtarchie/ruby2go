# prelude/json.rb
# rbs_inline: enabled
#
# JSON generation, matching the json gem (2.x): `require "json"` gives every
# object #to_json. Parsing is not supported.

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
  def to_json(*state) = %x{ rbJSONArray(*self, rest_) }
end

class Hash
  #: (*untyped) -> String
  def to_json(*state) = %x{ rbJSONHash(self, rest_) }
end

module JSON
  class GeneratorError < StandardError; end

  #: (untyped) -> String
  def self.generate(obj) = %x{ rbToJson(obj, rbJSONStateOf(nil)) }
end
