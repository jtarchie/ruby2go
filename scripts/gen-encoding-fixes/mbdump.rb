# ruby mbdump.rb ENC FORM: MRI's view of a multibyte encoding for gen-encoding-fixes. Status lines "s HEXSEQ RESULT" for
# every 1- and 2-byte input, every 0x8F-led 3-byte one (FORM 3, EUC-JP) or every [81-FE][30-39][81-FE][30-39] one
# (FORM 4, GB18030); RESULT is "ok CPS", "undef", "inv ERR AGAIN" or "inc ERR". Then "e CP BYTES" for every code point
# it encodes, and "n NAME" with the converter name its encode errors give.
e, form = ARGV
def hex(s) = s.bytes.map { |x| x.to_s(16) }.join('.')

def st(s, e)
  u = s.encode('UTF-8', e)
  'ok ' + u.codepoints.map { |c| c.to_s(16) }.join('.')
rescue Encoding::UndefinedConversionError
  'undef'
rescue Encoding::InvalidByteSequenceError => e
  (e.incomplete_input? ? 'inc ' : 'inv ') + hex(e.error_bytes) + ' ' + hex(e.readagain_bytes || '')
end
out = $stdout
256.times do |a|
  s = [a].pack('C')
  out.puts "s #{hex(s)} #{st(s, e)}"
  next if a < 0x80

  256.times do |b|
    s = [a, b].pack('CC')
    out.puts "s #{hex(s)} #{st(s, e)}"
  end
end
if form == '3'
  256.times do |b|
    256.times do |c|
      s = [0x8f, b, c].pack('CCC')
      out.puts "s #{hex(s)} #{st(s, e)}"
    end
  end
elsif form == '4'
  (0x81..0xfe).each do |a|
    (0x30..0x39).each do |b|
      (0x81..0xfe).each do |c|
        (0x30..0x39).each do |d|
          s = [a, b, c, d].pack('CCCC')
          out.puts "s #{hex(s)} #{st(s, e)}"
        end
      end
    end
  end
  (0x81..0xfe).each do |a|
    (0x30..0x39).each do |b|
      [0x20, 0x7f, 0xff].each do |c|
        s = [a, b, c].pack('CCC')
        out.puts "s #{hex(s)} #{st(s, e)}"
      end
    end
  end
end
(0..0x10FFFF).each do |cp|
  next if cp.between?(0xD800, 0xDFFF)

  begin
    out.puts "e #{cp.to_s(16)} #{hex([cp].pack('U').encode(e))}"
  rescue EncodingError
  end
end
begin
  "\u{10FFFF}".encode(e)
rescue Encoding::UndefinedConversionError => e
  out.puts "n #{e.message[/ to (\S+)\z/, 1]}"
end
