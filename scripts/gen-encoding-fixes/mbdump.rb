# ruby mbdump.rb ENC FORM: MRI's view of a multibyte encoding for gen-encoding-fixes. Status lines "s HEXSEQ RESULT" for
# every 1- and 2-byte input, every 0x8F-led 3-byte one (FORM 3, EUC-JP) or every [81-FE][30-39][81-FE][30-39] one
# (FORM 4, GB18030); RESULT is "ok CPS", "undef", "inv ERR AGAIN" or "inc ERR". Then "e CP BYTES" for every code point
# it encodes, and "n NAME" with the converter name its encode errors give.
e, form = ARGV
def hex(s) = s.bytes.map { |x| x.to_s(16) }.join(".")
def st(s, e)
  u = s.encode("UTF-8", e)
  "ok " + u.codepoints.map { |c| c.to_s(16) }.join(".")
rescue Encoding::UndefinedConversionError
  "undef"
rescue Encoding::InvalidByteSequenceError => ex
  (ex.incomplete_input? ? "inc " : "inv ") + hex(ex.error_bytes) + " " + hex(ex.readagain_bytes || "")
end
out = $stdout
256.times do |a|
  s = [a].pack("C")
  out.puts "s #{hex(s)} #{st(s, e)}"
  next if a < 0x80
  256.times { |b| s = [a, b].pack("CC"); out.puts "s #{hex(s)} #{st(s, e)}" }
end
if form == "3"
  256.times { |b| 256.times { |c| s = [0x8f, b, c].pack("CCC"); out.puts "s #{hex(s)} #{st(s, e)}" } }
elsif form == "4"
  (0x81..0xfe).each { |a| (0x30..0x39).each { |b| (0x81..0xfe).each { |c| (0x30..0x39).each { |d| s = [a, b, c, d].pack("CCCC"); out.puts "s #{hex(s)} #{st(s, e)}" } } } }
  (0x81..0xfe).each { |a| (0x30..0x39).each { |b| [0x20, 0x7f, 0xff].each { |c| s = [a, b, c].pack("CCC"); out.puts "s #{hex(s)} #{st(s, e)}" } } }
end
(0..0x10FFFF).each do |cp|
  next if cp.between?(0xD800, 0xDFFF)
  begin
    out.puts "e #{cp.to_s(16)} #{hex([cp].pack("U").encode(e))}"
  rescue EncodingError
  end
end
begin
  "\u{10FFFF}".encode(e)
rescue Encoding::UndefinedConversionError => ex
  out.puts "n #{ex.message[/ to (\S+)\z/, 1]}"
end
