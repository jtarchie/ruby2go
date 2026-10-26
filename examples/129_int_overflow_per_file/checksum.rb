# int_overflow: wrap

# FNV-style string hash: its multiply is meant to wrap at 64 bits.
#: (String) -> Integer
def checksum(s)
  h = 1469598103934665603
  s.each_byte do |b|
    h ^= b
    h *= 1099511628211
  end
  h
end
