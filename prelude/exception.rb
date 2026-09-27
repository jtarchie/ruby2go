# prelude/exception.rb
# rbs_inline: enabled
#
# The exception hierarchy.

class Exception < Object
  #: (?String?) -> void
  def initialize(message = nil)
    @message = message
  end

  # MRI order: message calls to_s, so overriding to_s changes message too.

  #: () -> String
  def to_s
    m = @message
    return m if m
    __class_name
  end

  #: () -> String
  def message = to_s

  #: () -> String
  def inspect
    s = to_s
    return __class_name if s.empty?
    return "#<#{__class_name}:#{s.inspect}>" if s.include?("\n")
    "#<#{__class_name}: #{s}>"
  end

  #: () -> String?
  def backtrace = nil
end

class StandardError < Exception; end
class IOError < StandardError; end

class RuntimeError < StandardError; end

class ArgumentError < StandardError; end

class TypeError < StandardError; end

class NameError < StandardError; end

class NoMethodError < NameError; end

class IndexError < StandardError; end

class KeyError < IndexError; end

class StopIteration < IndexError; end

class RangeError < StandardError; end

class FloatDomainError < RangeError; end

class ZeroDivisionError < StandardError; end

class ScriptError < Exception; end

class NotImplementedError < ScriptError; end
