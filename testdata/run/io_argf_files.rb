# rbs_inline: enabled

require "tmpdir"

# ARGF reads the files named in ARGV in turn, shifting each out as it opens.
Dir.mktmpdir do |dir|
  a = File.join(dir, "a.txt")
  b = File.join(dir, "b.txt")
  File.write(a, "a1\na2\n")
  File.write(b, "b1\nb2")
  ARGV << a
  ARGV << b
  p File.basename(ARGF.filename)
  p gets
  p ARGV.map { |f| File.basename(f) }
  p gets, gets
  p File.basename(ARGF.filename)
  p ARGF.eof?
  p ARGF.read
  p gets
  begin
    ARGF.eof?
  rescue IOError => e
    p e.message
  end
end

p $stdout.sync, STDERR.sync, $stdin.sync
$stdout.sync = true
p $stdout.sync
STDERR.puts "after sync"
puts "ordered"
