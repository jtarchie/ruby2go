# rbs_inline: enabled
require "logger"
require "stringio"
require "tmpdir"

deterministic = proc { |severity, _time, progname, msg|
  tag = progname ? "[#{progname}] " : ""
  "#{severity}: #{tag}#{msg}\n"
} #: ^(String, Time, String?, untyped) -> String

logger = Logger.new($stdout)
logger.formatter = deterministic

logger.debug("starting up")
logger.info("listening on :8080")

logger.level = Logger::WARN
logger.debug("suppressed")
logger.info("also suppressed")
logger.warn("disk 90% full")

calls = 0
logger.debug { calls += 1; "expensive: #{calls}" }
logger.error { calls += 1; "expensive: #{calls}" }
puts calls

logger.progname = "worker"
logger.level = Logger::DEBUG
logger.unknown("mystery")
logger.fatal("crash")

logger.level = "warn"
puts logger.level == Logger::WARN
logger.level = :fatal
puts logger.level == Logger::FATAL
begin
  logger.level = "nonsense"
rescue ArgumentError => e
  puts e.message
end

plain = StringIO.new
buffered = Logger.new(plain)
buffered.datetime_format = "%Y-%m-%d"
buffered.info("to buffer")
line = plain.string
puts line.start_with?("I, [#{Time.now.strftime('%Y-%m-%d')} #")
puts line.end_with?("INFO -- : to buffer\n")

Dir.mktmpdir do |dir|
  path = File.join(dir, "app.log")
  file_logger = Logger.new(path)
  file_logger.formatter = deterministic
  file_logger.info("wrote to disk")
  puts File.read(path).include?("INFO: wrote to disk")
end

disabled = Logger.new(nil)
disabled.formatter = deterministic
ran = false
disabled.error { ran = true; "unreachable" }
puts ran
