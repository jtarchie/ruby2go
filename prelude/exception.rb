# prelude/exception.rb
# rbs_inline: enabled
#
# The exception hierarchy.

class Exception < Object
  #: (?String?) -> void
  def initialize(message = nil)
    @message = message
  end

  #: () -> String
  def message
    m = @message
    return m if m
    __class_name
  end

  #: () -> String
  def to_s = message

  #: () -> String
  def inspect = "#<#{__class_name}: #{message}>"

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

class ZeroDivisionError < StandardError; end

class ScriptError < Exception; end

class NotImplementedError < ScriptError; end
