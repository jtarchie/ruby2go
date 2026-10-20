# prelude/exception.rb
# rbs_inline: enabled
#
# The exception hierarchy.

class Exception < Object
  # @rbs @cause: Exception?
  # @rbs @__pcs: untyped
  # @rbs @__bt: Array[String]?
  # @rbs @__seen: bool

  #: (?String?) -> void
  def initialize(message = nil)
    @message = message
  end

  # The exception being handled when this one was raised: `raise` inside a
  # rescue clause sets it (lexically; a raise in a method the clause calls
  # does not, decision 103).
  #: () -> Exception?
  def cause = @cause

  #: (Exception) -> void
  def __set_cause(c)
    @cause = c unless @cause
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

  # Same class, message and backtrace, as MRI's exc_equal (so a dup is equal).
  #: (untyped) -> bool
  def ==(other)
    return true if equal?(other)
    return false unless other.is_a?(Exception) && other.class == self.class
    message == other.message && backtrace == other.backtrace
  end

  #: (?String?) -> self
  def exception(msg = nil)
    return self if msg.nil?
    e = dup
    e.__set_message(msg)
    e
  end

  #: (String) -> void
  def __set_message(msg)
    @message = msg
  end

  # MRI's rb_decorate_message: the class name after the first line (a lone trailing newline dropped), bold and underlined with highlight.
  #: (?highlight: bool) -> String
  def detailed_message(highlight: false)
    m = message
    return __bare_name(highlight) if m.empty?
    i = m.index("\n")
    first = i ? m[0, i] || "" : m
    head = highlight ? "\e[1m#{first} (\e[1;4m#{__class_name}\e[m\e[1m)\e[m" : "#{first} (#{__class_name})"
    return head unless i
    return head if i == m.size - 1
    rest = m[i + 1, m.size] || ""
    rest = rest.split("\n", -1).map { |l| l.empty? ? l : "\e[1m#{l}\e[m" }.join("\n") if highlight
    "#{head}\n#{rest}"
  end

  # MRI's rb_error_write: the error line, the "from" lines and the causes', no error_highlight snippet (decision 139).
  #: (?highlight: bool?, ?order: Symbol?) -> String
  def full_message(highlight: nil, order: nil)
    hl = highlight.nil? ? $stderr.tty? : highlight
    bottom = order == :bottom
    raise ArgumentError, "expected :top or :bottom as order: #{order.inspect}" unless order.nil? || bottom || order == :top
    shown = [] #: Array[Exception]
    return "#{hl ? "\e[1mTraceback\e[m" : "Traceback"} (most recent call last):\n#{__report(hl, true, shown)}" if bottom
    __report(hl, false, shown)
  end

  # Each cause once (MRI's show_cause), so a cycle ends.
  #: (bool, bool, Array[Exception]) -> String
  def __report(hl, bottom, shown)
    c = cause
    rest = ""
    if c && !shown.any? { |x| x.equal?(c) }
      shown << c
      rest = c.__report(hl, bottom, shown)
    end
    bottom ? "#{rest}#{__from_lines(true)}#{__errinfo(hl)}" : "#{__errinfo(hl)}#{__from_lines(false)}#{rest}"
  end

  #: (bool) -> String
  def __errinfo(hl)
    top = backtrace&.first
    m = detailed_message(highlight: hl)
    m = __bare_name(hl) if m.empty?
    "#{top ? "#{top}: " : __error_pos}#{m}\n"
  end

  #: (bool) -> String
  def __bare_name(hl)
    name = instance_of?(RuntimeError) ? "unhandled exception" : __class_name
    hl ? "\e[1;4m#{name}\e[m" : name
  end

  #: (bool) -> String
  def __from_lines(bottom)
    bt = backtrace || []
    n = bt.size
    return "" if n < 2
    width = (n - 1).to_s.size
    (1...n).map { |i| bottom ? "\t#{(n - i).to_s.rjust(width)}: from #{bt[n - i]}\n" : "\tfrom #{bt[i]}\n" }.join
  end

  # MRI's error_pos: where the program is now, for an exception with no backtrace.
  #: () -> String
  def __error_pos = %x{ return rbErrorPos("full_message") }

  #: () -> String
  def inspect
    s = to_s
    return __class_name if s.empty?
    return "#<#{__class_name}:#{s.inspect}>" if s.include?("\n")
    "#<#{__class_name}: #{s}>"
  end

  # The frames where this was raised, once a rescue bound it (decision 106); nil before, as MRI's is for an exception never raised.
  #: () -> Array[String]?
  def backtrace = %x{ return rbBacktrace(self) }

  #: (Array[String]?) -> Array[String]?
  def set_backtrace(bt)
    @__bt = bt
    bt
  end
end

class StandardError < Exception; end
class IOError < StandardError; end
class EOFError < IOError; end

class SystemCallError < StandardError
  # The C errno for this class (Errno::ENOENT's is 2); nil for SystemCallError itself.
  #: () -> Integer?
  def errno = %x{ return rbErrnoOf(self) }
end

module Errno
  class EINVAL < SystemCallError
    Errno = ::Errno::EINVAL.new.errno.to_i #: Integer
  end
  class ENOENT < SystemCallError
    Errno = ::Errno::ENOENT.new.errno.to_i #: Integer
  end
  class EBADF < SystemCallError
    Errno = ::Errno::EBADF.new.errno.to_i #: Integer
  end
  class ECHILD < SystemCallError
    Errno = ::Errno::ECHILD.new.errno.to_i #: Integer
  end
  class EEXIST < SystemCallError
    Errno = ::Errno::EEXIST.new.errno.to_i #: Integer
  end
  class EISDIR < SystemCallError
    Errno = ::Errno::EISDIR.new.errno.to_i #: Integer
  end
  class ENOTDIR < SystemCallError
    Errno = ::Errno::ENOTDIR.new.errno.to_i #: Integer
  end
  class EACCES < SystemCallError
    Errno = ::Errno::EACCES.new.errno.to_i #: Integer
  end
  class ENOTEMPTY < SystemCallError
    Errno = ::Errno::ENOTEMPTY.new.errno.to_i #: Integer
  end
  class EBUSY < SystemCallError
    Errno = ::Errno::EBUSY.new.errno.to_i #: Integer
  end
  class EROFS < SystemCallError
    Errno = ::Errno::EROFS.new.errno.to_i #: Integer
  end
  class ELOOP < SystemCallError
    Errno = ::Errno::ELOOP.new.errno.to_i #: Integer
  end
  class EAGAIN < SystemCallError
    Errno = ::Errno::EAGAIN.new.errno.to_i #: Integer
  end
  class ECONNREFUSED < SystemCallError
    Errno = ::Errno::ECONNREFUSED.new.errno.to_i #: Integer
  end
  class EADDRINUSE < SystemCallError
    Errno = ::Errno::EADDRINUSE.new.errno.to_i #: Integer
  end
  class EADDRNOTAVAIL < SystemCallError
    Errno = ::Errno::EADDRNOTAVAIL.new.errno.to_i #: Integer
  end
  class EPIPE < SystemCallError
    Errno = ::Errno::EPIPE.new.errno.to_i #: Integer
  end
  class ESPIPE < SystemCallError
    Errno = ::Errno::ESPIPE.new.errno.to_i #: Integer
  end
  class ECONNRESET < SystemCallError
    Errno = ::Errno::ECONNRESET.new.errno.to_i #: Integer
  end
  class ECONNABORTED < SystemCallError
    Errno = ::Errno::ECONNABORTED.new.errno.to_i #: Integer
  end
  class ENOTCONN < SystemCallError
    Errno = ::Errno::ENOTCONN.new.errno.to_i #: Integer
  end
  class EISCONN < SystemCallError
    Errno = ::Errno::EISCONN.new.errno.to_i #: Integer
  end
  class EDESTADDRREQ < SystemCallError
    Errno = ::Errno::EDESTADDRREQ.new.errno.to_i #: Integer
  end
  class ETIMEDOUT < SystemCallError
    Errno = ::Errno::ETIMEDOUT.new.errno.to_i #: Integer
  end
  class EHOSTUNREACH < SystemCallError
    Errno = ::Errno::EHOSTUNREACH.new.errno.to_i #: Integer
  end
  class ENETUNREACH < SystemCallError
    Errno = ::Errno::ENETUNREACH.new.errno.to_i #: Integer
  end
  class EAFNOSUPPORT < SystemCallError
    Errno = ::Errno::EAFNOSUPPORT.new.errno.to_i #: Integer
  end
  class EMFILE < SystemCallError
    Errno = ::Errno::EMFILE.new.errno.to_i #: Integer
  end
  class ENOTTY < SystemCallError
    Errno = ::Errno::ENOTTY.new.errno.to_i #: Integer
  end
  class ENODEV < SystemCallError
    Errno = ::Errno::ENODEV.new.errno.to_i #: Integer
  end
  class ERANGE < SystemCallError
    Errno = ::Errno::ERANGE.new.errno.to_i #: Integer
  end
  class ENOSYS < SystemCallError
    Errno = ::Errno::ENOSYS.new.errno.to_i #: Integer
  end
  class EOPNOTSUPP < SystemCallError
    Errno = ::Errno::EOPNOTSUPP.new.errno.to_i #: Integer
  end
end

class RuntimeError < StandardError; end

class FrozenError < RuntimeError; end

class ArgumentError < StandardError; end

class TypeError < StandardError; end

# name and receiver are the call that failed, when rb2go raised it (a dynamic call on an untyped value).
class NameError < StandardError
  # @rbs @name: Symbol?
  # @rbs @receiver: untyped
  # @rbs @has_receiver: bool

  #: () -> Symbol?
  def name = @name

  #: () -> untyped
  def receiver
    raise ArgumentError, "no receiver is available" unless @has_receiver

    @receiver
  end

  #: (Symbol, untyped) -> void
  def __set_call(name, receiver)
    @name = name
    @receiver = receiver
    @has_receiver = true
  end
end

class NoMethodError < NameError; end

class IndexError < StandardError; end

# key and receiver are the Hash#fetch (or ENV.fetch) that failed, as MRI's.
class KeyError < IndexError
  # @rbs @key: untyped
  # @rbs @receiver: untyped
  # @rbs @has_key: bool

  #: () -> untyped
  def key
    raise ArgumentError, "no key is available" unless @has_key

    @key
  end

  #: () -> untyped
  def receiver
    raise ArgumentError, "no receiver is available" unless @has_key

    @receiver
  end

  #: (String, untyped, untyped) -> KeyError
  def self.__for(message, receiver, key)
    e = new(message)
    e.__set(receiver, key)
    e
  end

  #: (untyped, untyped) -> void
  def __set(receiver, key)
    @receiver = receiver
    @key = key
    @has_key = true
  end
end

class StopIteration < IndexError
  # @rbs @result: untyped

  # What the finished iteration returned (`[1].each` → [1]); next raises it (decision 140).
  #: () -> untyped
  def result = @result

  #: (untyped) -> void
  def __set_result(r)
    @result = r
  end
end

# Raised when no pattern of a case/in (without else) or `=>` matches (decision 143).
class NoMatchingPatternError < StandardError; end

# A hash pattern's missing key, and the Hash it was looked up in, as MRI's.
class NoMatchingPatternKeyError < NoMatchingPatternError
  # @rbs @key: untyped
  # @rbs @matchee: untyped
  # @rbs @has_key: bool

  #: () -> untyped
  def key
    raise ArgumentError, "no key is available" unless @has_key

    @key
  end

  #: () -> untyped
  def matchee
    raise ArgumentError, "no matchee is available" unless @has_key

    @matchee
  end

  #: (String, untyped, untyped) -> NoMatchingPatternKeyError
  def self.__for(message, matchee, key)
    e = new(message)
    e.__set(matchee, key)
    e
  end

  #: (untyped, untyped) -> void
  def __set(matchee, key)
    @matchee = matchee
    @key = key
    @has_key = true
  end
end

# Kernel#throw raises it when no active catch has the tag (decision 91).
class UncaughtThrowError < ArgumentError
  #: (String, untyped, untyped) -> void
  def initialize(message, tag, value)
    @message = message
    @tag = tag
    @value = value
  end

  #: () -> untyped
  def tag = @tag

  #: () -> untyped
  def value = @value
end

class RangeError < StandardError; end

class FloatDomainError < RangeError; end

class ZeroDivisionError < StandardError; end

class ScriptError < Exception; end

# Raised only for a require in a method that names a file outside the closed world (decision 197); otherwise an unknown
# library is a compile error (decision 68).
class LoadError < ScriptError; end

#: (String) -> bot
def __rb2go_load_error(name) = raise(LoadError, "cannot load such file -- #{name}")

# Never raised by rb2go (there is no eval); MRI's default message.
class SyntaxError < ScriptError
  #: (?String?) -> void
  def initialize(message = nil)
    @message = message || "compile error"
  end
end

# Raised by a yield whose optional block (?{ }) is missing (decision 132).
class LocalJumpError < StandardError; end

# Never raised: Go's stack overflow is fatal and recover cannot catch it (decision 128).
class SystemStackError < Exception; end

class SecurityError < Exception; end

class EncodingError < StandardError; end

class NoMemoryError < Exception; end

# A signal received: `signo`, and "SIGTERM" as the message. An untrapped
# SIGINT/SIGTERM is raised (Interrupt/SignalException) only where the main
# thread blocks (sleep, join, Queue#pop); a computing program dies by the signal (decision 60).
class SignalException < Exception
  # @rbs @signo: Integer

  #: (untyped) -> void
  def initialize(sig)
    @signo = __signo(sig)
    super("SIG#{__signame(@signo)}")
  end

  #: (untyped) -> Integer
  def __signo(sig) = %x{ return Integer(rbSignalArg(sig)) }

  #: (Integer) -> String
  def __signame(n) = %x{ return String(rbSignalName(syscall.Signal(n))) }

  #: () -> Integer
  def signo = @signo

  #: () -> String
  def signm = message
end

class Interrupt < SignalException
  #: (?String?) -> void
  def initialize(message = nil)
    super("INT")
    @message = message || "Interrupt"
  end
end

class NotImplementedError < ScriptError; end

# Kernel#exit raises it, so ensure blocks and `rescue Exception` run first;
# rbTopRecover exits with its status and prints nothing.
class SystemExit < Exception
  #: (Integer) -> void
  def initialize(status)
    @message = "exit"
    @status = status
  end

  #: () -> Integer
  def status = @status

  #: () -> bool
  def success? = @status == 0
end
