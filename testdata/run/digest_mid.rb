# rbs_inline: enabled

require "digest"

File.write("digest_mid_tmp.txt", "hello world")
d = Digest::MD5.file("digest_mid_tmp.txt")
puts d.hexdigest, d.hexdigest == Digest::MD5.hexdigest("hello world")
File.delete("digest_mid_tmp.txt")

a = Digest::SHA256.new
b = Digest::SHA256.new
puts a == b
a.update("abc")
puts a == b
b.update("abc")
puts a == b
puts a == "not a digest"

puts Digest::SHA2.new(256).hexdigest == Digest::SHA256.hexdigest("")
puts Digest::SHA2.new(384).hexdigest == Digest::SHA384.hexdigest("")
puts Digest::SHA2.new(512).hexdigest == Digest::SHA512.hexdigest("")
puts Digest::SHA2.new.hexdigest == Digest::SHA256.hexdigest("")
begin
  Digest::SHA2.new(123)
rescue ArgumentError => e
  puts e.message
end

puts Digest.hexencode("abc")
