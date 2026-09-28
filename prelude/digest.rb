# prelude/digest.rb
# rbs_inline: enabled
#
# Digest::MD5/SHA1/SHA256/SHA384/SHA512 on crypto/*. Always defined.
# `new` returns a Digest::Base holding the bytes fed to it; the sum is
# taken when asked for.

module Digest
  # @go_type struct { algo string; buf []byte }
  class Base < Object
    #: (String) -> Base
    def self.__for(algo) = %x{ return &Digest_Base{algo: string(algo)} }

    #: (String) -> Base
    def update(s) = %x{
      self.buf = append(self.buf, s...)
      return self
    }

    #: (String) -> Base
    def <<(s) = update(s)

    #: () -> Base
    def reset = %x{
      self.buf = self.buf[:0]
      return self
    }

    #: () -> String
    def digest = %x{ String(rbDigestSum(self.algo, self.buf)) }

    #: () -> String
    def hexdigest = %x{ String(hex.EncodeToString(rbDigestSum(self.algo, self.buf))) }

    #: () -> String
    def base64digest = %x{ String(base64.StdEncoding.EncodeToString(rbDigestSum(self.algo, self.buf))) }

    #: () -> String
    def to_s = hexdigest

    #: () -> String
    def name = %x{ String(self.algo) }

    #: () -> String
    def inspect = "#<Digest::#{name}: #{hexdigest}>"
  end

  class MD5 < Object
    #: () -> Base
    def self.new = Base.__for("MD5")
    #: (String) -> String
    def self.digest(s) = Base.__for("MD5").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("MD5").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("MD5").update(s).base64digest
  end

  class SHA1 < Object
    #: () -> Base
    def self.new = Base.__for("SHA1")
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA1").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA1").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA1").update(s).base64digest
  end

  class SHA256 < Object
    #: () -> Base
    def self.new = Base.__for("SHA256")
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA256").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA256").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA256").update(s).base64digest
  end

  class SHA384 < Object
    #: () -> Base
    def self.new = Base.__for("SHA384")
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA384").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA384").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA384").update(s).base64digest
  end

  class SHA512 < Object
    #: () -> Base
    def self.new = Base.__for("SHA512")
    #: (String) -> String
    def self.digest(s) = Base.__for("SHA512").update(s).digest
    #: (String) -> String
    def self.hexdigest(s) = Base.__for("SHA512").update(s).hexdigest
    #: (String) -> String
    def self.base64digest(s) = Base.__for("SHA512").update(s).base64digest
  end
end
