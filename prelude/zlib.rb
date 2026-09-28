# prelude/zlib.rb
# rbs_inline: enabled
#
# Zlib's checksums on hash/crc32 and hash/adler32. Always defined.

module Zlib
  #: (?String, ?Integer) -> Integer
  def self.crc32(str = "", crc = 0) = %x{ Integer(crc32.Update(uint32(crc), crc32.IEEETable, []byte(str))) }

  #: (?String, ?Integer) -> Integer
  def self.adler32(str = "", adler = 1) = %x{
    // hash/adler32 cannot resume from a value; this is its update loop
    s1, s2 := uint32(adler)&0xffff, uint32(adler)>>16
    for i := range len(str) {
      s1 = (s1 + uint32(str[i])) % 65521
      s2 = (s2 + s1) % 65521
    }
    return Integer(s2<<16 | s1)
  }
end
