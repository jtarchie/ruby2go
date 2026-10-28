# int_overflow: wrap

# FNV-style string hash: its multiply is meant to wrap at 64 bits.
#: (String) -> Integer
def checksum(s)
  h = 1_469_598_103_934_665_603
  s.each_byte do |b|
    h ^= b
    h *= 1_099_511_628_211
  end
  h
end
