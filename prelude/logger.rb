# rbs_inline: enabled

# No shared IO base (decision 48), so the log device is untyped and dispatched dynamically; see decision 63.
class Logger < Object
  DEBUG = 0
  INFO = 1
  WARN = 2
  ERROR = 3
  FATAL = 4
  UNKNOWN = 5

  SEV_LABEL = ["DEBUG", "INFO", "WARN", "ERROR", "FATAL", "ANY"] #: Array[String]
  LEVELS = {"debug" => 0, "info" => 1, "warn" => 2, "error" => 3, "fatal" => 4, "unknown" => 5} #: Hash[String, Integer]
  DEFAULT_DATETIME_FORMAT = "%Y-%m-%dT%H:%M:%S.%6N" #: String

  attr_accessor :progname #: String?
  attr_accessor :formatter #: (^(String, Time, String?, untyped) -> String)?
  attr_accessor :datetime_format #: String?

  #: (untyped) -> void
  def initialize(logdev)
    @level = DEBUG
    @progname = nil
    @formatter = nil
    @datetime_format = nil
    @io = logdev.is_a?(String) ? File.new(logdev, "a") : logdev
  end

  #: () -> Integer
  def level = @level

  #: (untyped) -> Integer
  def level=(severity)
    if severity.is_a?(Integer)
      @level = severity
    else
      val = LEVELS[severity.to_s.downcase]
      raise ArgumentError, "invalid log level: #{severity}" if val.nil?
      @level = val
    end
    @level
  end

  #: (?untyped) -> bool
  def debug(msg = nil) = add(DEBUG, msg)

  #: () { () -> untyped } -> bool
  def __debug_block
    return true if @io.nil? || DEBUG < @level
    __write_entry(DEBUG, yield)
    true
  end

  #: (?untyped) -> bool
  def info(msg = nil) = add(INFO, msg)

  #: () { () -> untyped } -> bool
  def __info_block
    return true if @io.nil? || INFO < @level
    __write_entry(INFO, yield)
    true
  end

  #: (?untyped) -> bool
  def warn(msg = nil) = add(WARN, msg)

  #: () { () -> untyped } -> bool
  def __warn_block
    return true if @io.nil? || WARN < @level
    __write_entry(WARN, yield)
    true
  end

  #: (?untyped) -> bool
  def error(msg = nil) = add(ERROR, msg)

  #: () { () -> untyped } -> bool
  def __error_block
    return true if @io.nil? || ERROR < @level
    __write_entry(ERROR, yield)
    true
  end

  #: (?untyped) -> bool
  def fatal(msg = nil) = add(FATAL, msg)

  #: () { () -> untyped } -> bool
  def __fatal_block
    return true if @io.nil? || FATAL < @level
    __write_entry(FATAL, yield)
    true
  end

  #: (?untyped) -> bool
  def unknown(msg = nil) = add(UNKNOWN, msg)

  #: () { () -> untyped } -> bool
  def __unknown_block
    return true if @io.nil? || UNKNOWN < @level
    __write_entry(UNKNOWN, yield)
    true
  end

  #: (Integer, ?untyped) -> bool
  def add(severity, msg = nil)
    return true if @io.nil? || severity < @level
    __write_entry(severity, msg)
    true
  end

  private

  #: (Integer, untyped) -> void
  def __write_entry(severity, msg)
    label = SEV_LABEL[severity] || "ANY"
    now = Time.now
    fmtr = @formatter
    line = fmtr ? fmtr.call(label, now, @progname, msg) : __default_format(label, now, msg)
    io = @io
    return unless io
    io.write(line)
    io.flush if io.respond_to?(:flush)
  end

  #: (String, Time, untyped) -> String
  def __default_format(label, now, msg)
    ts = now.strftime(@datetime_format || DEFAULT_DATETIME_FORMAT)
    format("%.1s, [%s #%d] %5s -- %s: %s\n", label, ts, __pid, label, @progname || "", __msg2str(msg))
  end

  #: (untyped) -> String
  def __msg2str(msg)
    case msg
    when String then msg
    when Exception then "#{msg.message} (#{msg.class})"
    else msg.inspect
    end
  end

  #: () -> Integer
  def __pid = %x{ Integer(os.Getpid()) }
end
