# rbs_inline: enabled

require "tmpdir"

# Binary Strings: `.b` and File.binread give Strings whose characters are bytes, the way URL encoders and file-format
# sniffers read data.

#: (String) -> String
def percent_encode(text)
  text.b.gsub(/[^A-Za-z0-9\-._~]/) { |byte| "%%%02X" % byte.ord }
end

#: (String) -> String
def percent_decode(text)
  text.b.gsub(/%[0-9A-F]{2}/) { |esc| esc.delete("%").to_i(16).chr }.force_encoding("UTF-8")
end

word = "naïve café"
encoded = percent_encode(word)
puts encoded
puts percent_decode(encoded)
puts "#{word.size} characters, #{word.b.size} bytes"

raw = word.b
p raw[0, 4]
puts raw.chars.first(4).map(&:ord).inspect
puts raw.encoding

path = File.join(Dir.tmpdir, "rugaroo_example_125_#{Process.pid}.jpg")
File.write(path, "\xFF\xD8\xFF\xE0fake jpeg".b)
data = File.binread(path)
File.delete(path)
kind = case data
       when /\A\xFF\xD8\xFF/n then "JPEG"
       when /\A\x89PNG/n then "PNG"
       else "unknown"
       end
puts "#{kind}, #{data.size} bytes, header #{data.unpack1("H8")}"
