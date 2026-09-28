# prelude/securerandom.rb
# rbs_inline: enabled
#
# SecureRandom on crypto/rand. Always defined, `require "securerandom"` or not.

module SecureRandom
  #: (?Integer) -> String
  def self.random_bytes(n = 16) = %x{
    b := make([]byte, n)
    _, _ = rand.Read(b) // crypto/rand.Read never fails (Go 1.24+)
    return String(b)
  }

  #: (?Integer) -> String
  def self.hex(n = 16) = %x{
    b := make([]byte, n)
    _, _ = rand.Read(b)
    return String(hex.EncodeToString(b))
  }

  #: (?Integer) -> String
  def self.base64(n = 16) = Base64.strict_encode64(random_bytes(n))

  #: (?Integer, ?bool) -> String
  def self.urlsafe_base64(n = 16, padding = false) = Base64.urlsafe_encode64(random_bytes(n), padding: padding)

  #: () -> String
  def self.uuid = %x{ String(rbUUID4()) }

  #: (?Integer) -> String
  def self.alphanumeric(n = 16) = %x{
    const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, n)
    for i := range b {
      v, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
      b[i] = chars[v.Int64()]
    }
    return String(b)
  }

  # A uniform Integer in 0...n.
  #: (Integer) -> Integer
  def self.random_number(n) = %x{
    if n <= 0 {
      return Integer(0)
    }
    v, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
    return Integer(v.Int64())
  }

  # A Float in [0, 1): 53 random bits, as MRI's.
  #: () -> Float
  def self.__random_number_0 = %x{
    v, _ := rand.Int(rand.Reader, big.NewInt(1<<53))
    return Float(float64(v.Int64()) / (1 << 53))
  }
end
