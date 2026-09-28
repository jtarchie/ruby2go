# rbs_inline: enabled
require "tmpdir"
require "stringio"
require "zlib"

data = "the quick brown fox jumps over the lazy dog\n" * 20

deflated = Zlib::Deflate.deflate(data)
puts "deflate round-trip: #{Zlib::Inflate.inflate(deflated) == data}"
puts "deflate smaller: #{deflated.bytesize < data.bytesize}"
puts "Zlib.deflate/inflate: #{Zlib.inflate(Zlib.deflate(data)) == data}"

gz = Zlib.gzip(data)
puts "gzip round-trip: #{Zlib.gunzip(gz) == data}"
puts "gzip magic: #{gz.bytes[0]} #{gz.bytes[1]}"

Dir.mktmpdir do |dir|
  path = File.join(dir, "out.txt.gz")
  File.open(path, "w") do |f|
    w = Zlib::GzipWriter.new(f)
    w.write(data)
    w.close
  end

  File.open(path) do |f|
    r = Zlib::GzipReader.new(f)
    puts "gzipwriter/reader over File round-trip: #{r.read == data}"
    r.close
  end
end

sio = StringIO.new
w2 = Zlib::GzipWriter.new(sio)
w2.write(data)
w2.finish
puts "gzipwriter over StringIO: #{Zlib.gunzip(sio.string) == data}"

sio2 = StringIO.new(sio.string)
r2 = Zlib::GzipReader.new(sio2)
puts "gzipreader over StringIO: #{r2.read == data}"

known = "the quick brown fox jumps over the lazy dog\n"
deflated_literal = "\x78\xDA\x2B\xC9\x48\x55\x28\x2C\xCD\x4C\xCE\x56\x48\x2A\xCA\x2F\xCF\x53\x48\xCB\xAF\x50\xC8\x2A\xCD\x2D\x28\x56\xC8\x2F\x4B\x2D\x52\x28\x01\x4A\xE7\x24\x56\x55\x2A\xA4\xE4\xA7\x73\x01\x00\x71\x40\x10\x04"
puts "MRI-produced deflate blob: #{Zlib::Inflate.inflate(deflated_literal) == known}"

gzipped_literal = "\x1F\x8B\x08\x00\xF9\xA0\xBA\x6A\x00\x03\x2B\xC9\x48\x55\x28\x2C\xCD\x4C\xCE\x56\x48\x2A\xCA\x2F\xCF\x53\x48\xCB\xAF\x50\xC8\x2A\xCD\x2D\x28\x56\xC8\x2F\x4B\x2D\x52\x28\x01\x4A\xE7\x24\x56\x55\x2A\xA4\xE4\xA7\x73\x01\x00\xBF\xDE\xC3\x28\x2C\x00\x00\x00"
puts "MRI-produced gzip blob: #{Zlib.gunzip(gzipped_literal) == known}"

begin
  Zlib::Inflate.inflate("not valid zlib data")
rescue => e
  puts "corrupt deflate: #{e.class}"
end

begin
  Zlib.gunzip("not valid gzip data")
rescue => e
  puts "corrupt gzip: #{e.class}"
end

puts "levels: #{Zlib::NO_COMPRESSION} #{Zlib::BEST_SPEED} #{Zlib::BEST_COMPRESSION} #{Zlib::DEFAULT_COMPRESSION}"
puts "order: #{Zlib::NO_COMPRESSION < Zlib::BEST_SPEED && Zlib::BEST_SPEED < Zlib::BEST_COMPRESSION && Zlib::DEFAULT_COMPRESSION < Zlib::NO_COMPRESSION}"
