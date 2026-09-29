# rbs_inline: enabled

# IPAddr: IPv4/IPv6 + CIDR arithmetic over net/netip.Addr, masked to the network on construction like MRI (decision 69).

# @go_type struct { addr netip.Addr; bits int }
class IPAddr < Object
  include Comparable

  class Error < ArgumentError; end
  class AddressFamilyError < Error; end
  class InvalidAddressError < Error; end
  class InvalidPrefixError < InvalidAddressError; end

  #: (String) -> IPAddr
  def self.new(addr) = %x{
    a, bits, exc := rbIPAddrParse(string(addr))
    if exc != nil {
      panic(exc)
    }
    return &IPAddr{addr: a, bits: bits}
  }

  #: (untyped) -> IPAddr
  def mask(prefixlen) = %x{
    bits, exc := rbIPAddrMaskSpec(self.addr, prefixlen)
    if exc != nil {
      panic(exc)
    }
    return &IPAddr{addr: netip.PrefixFrom(self.addr, bits).Masked().Addr(), bits: bits}
  }

  #: (untyped) -> bool
  def include?(other) = %x{
    o, bits, ok, exc := rbIPAddrCoerce(other)
    if exc != nil {
      panic(exc)
    }
    if !ok || self.addr.Is4() != o.Is4() {
      return Boolean(false)
    }
    return Boolean(bits >= self.bits && netip.PrefixFrom(o, self.bits).Masked().Addr() == self.addr)
  }

  #: () -> IPAddr
  def succ = %x{
    next, overflow, ok := rbIPAddrSucc(self.addr)
    if !ok {
      panic(NewIPAddr_InvalidAddressError(Ref(String("invalid address: " + overflow))))
    }
    return &IPAddr{addr: next, bits: self.bits}
  }

  #: () -> IPAddr
  def network = %x{ return &IPAddr{addr: self.addr, bits: self.addr.BitLen()} }

  #: () -> IPAddr
  def broadcast = %x{ return &IPAddr{addr: rbIPAddrBroadcast(self.addr, self.bits), bits: self.addr.BitLen()} }

  #: () -> Range[IPAddr]
  def to_range = network..broadcast

  #: (IPAddr) -> Integer?
  def <=>(other) = %x{
    if self.addr.Is4() != other.addr.Is4() {
      return nil
    }
    return Ref(Integer(self.addr.Compare(other.addr)))
  }

  #: (untyped) -> bool
  def ==(other) = %x{
    o, _, ok, _ := rbIPAddrCoerce(other)
    return Boolean(ok && self.addr.Is4() == o.Is4() && self.addr.Compare(o) == 0)
  }

  #: (untyped) -> bool
  def eql?(other) = %x{
    o, ok := other.(*IPAddr)
    return Boolean(ok && o.addr == self.addr && o.bits == self.bits)
  }

  #: () -> Integer
  def hash = %x{ Integer(rbKeyHash(self.addr.String() + "/" + strconv.Itoa(self.bits))) }

  #: () -> String
  def to_s = %x{ String(self.addr.String()) }

  #: () -> String
  def inspect = %x{
    family := "IPv4"
    if self.addr.Is6() {
      family = "IPv6"
    }
    return String("#<IPAddr: " + family + ":" + rbIPAddrInspectAddr(self.addr) + "/" + rbIPAddrNetmask(self.addr, self.bits) + ">")
  }

  #: () -> bool
  def ipv4? = %x{ Boolean(self.addr.Is4()) }

  #: () -> bool
  def ipv6? = %x{ Boolean(self.addr.Is6()) }
end
