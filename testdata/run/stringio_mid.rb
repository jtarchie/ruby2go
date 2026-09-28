# rbs_inline: enabled

require "stringio"

seek_io = StringIO.new("hello world")
seek_io.seek(2)
puts seek_io.tell
seek_io.seek(2, IO::SEEK_CUR)
puts seek_io.tell
seek_io.seek(-2, IO::SEEK_END)
puts seek_io.tell
puts seek_io.read.inspect
begin
  seek_io.seek(-100, IO::SEEK_SET)
rescue => e
  puts "#{e.class}: #{e.message}"
end

closable = StringIO.new("abc")
puts closable.closed?
closable.close
puts closable.closed?
begin
  closable.read
rescue => e
  puts "#{e.class}: #{e.message}"
end
begin
  closable.write("x")
rescue => e
  puts "#{e.class}: #{e.message}"
end
begin
  closable.seek(0)
rescue => e
  puts "#{e.class}: #{e.message}"
end

half_closed = StringIO.new("abc", "r+")
half_closed.close_write
puts half_closed.closed?
puts half_closed.read
begin
  half_closed.write("z")
rescue => e
  puts "#{e.class}: #{e.message}"
end
begin
  StringIO.new("abc", "r").close_write
rescue => e
  puts "#{e.class}: #{e.message}"
end

byte_io = StringIO.new("héllo")
byte_io.each_char { |c| print c, "|" }
puts
byte_io.rewind
byte_io.each_byte { |b| print b, "," }
puts
byte_io.rewind
puts byte_io.getbyte.inspect
puts byte_io.getbyte.inspect
empty_io = StringIO.new("")
puts empty_io.getbyte.inspect

unget_io = StringIO.new("abc")
unget_io.getc
unget_io.ungetc("X")
puts unget_io.string.inspect, unget_io.pos
unget_io.rewind
unget_io.ungetc("XYZ")
puts unget_io.string.inspect, unget_io.pos
eof_unget = StringIO.new("abc")
eof_unget.read
eof_unget.ungetc("Z")
puts eof_unget.string.inspect, eof_unget.pos

trunc_io = StringIO.new("hello world")
trunc_io.pos = 100
trunc_io.truncate(3)
puts trunc_io.string.inspect, trunc_io.pos
grow_io = StringIO.new("hi")
grow_io.truncate(5)
puts grow_io.string.bytesize, grow_io.string.bytes.last(3).inspect
begin
  trunc_io.truncate(-1)
rescue => e
  puts "#{e.class}: #{e.message}"
end

read_only = StringIO.new("abc", "r")
begin
  read_only.write("x")
rescue => e
  puts "#{e.class}: #{e.message}"
end
puts read_only.read

write_only = StringIO.new("abc", "w")
puts write_only.string.inspect
begin
  write_only.read
rescue => e
  puts "#{e.class}: #{e.message}"
end
write_only.write("XY")
puts write_only.string.inspect

append_io = StringIO.new("abc", "a")
append_io.pos = 0
append_io.write("Z")
puts append_io.string.inspect, append_io.pos

begin
  StringIO.new("abc", "nope")
rescue => e
  puts "#{e.class}: #{e.message}"
end
