# rbs_inline: enabled

require_relative "openssl_ssl"

# Only RSA public-key verify (Homebrew's JWS check), HMAC, PBKDF2 and SSL verify modes: no signing, keygen or TLS (decisions 281, 404).
module OpenSSL
  class OpenSSLError < StandardError; end

  module HMAC
    # A digest is named by a String ("SHA256"); OpenSSL::Digest objects are not modelled.
    #: (String, String, String) -> String
    def self.digest(digest, key, data) = %x{ String(rbHMAC(string(digest), string(key), string(data))) }

    #: (String, String, String) -> String
    def self.hexdigest(digest, key, data) = %x{ String(hex.EncodeToString([]byte(rbHMAC(string(digest), string(key), string(data))))) }
  end

  module PKCS5
    #: (String, String, Integer, Integer, String) -> String
    def self.pbkdf2_hmac(pass, salt, iter, keylen, digest) = %x{ String(rbPBKDF2(string(pass), string(salt), int(iter), int(keylen), string(digest))) }
  end

  module PKey
    class PKeyError < OpenSSLError; end
    RSAError = PKeyError

    # @go_type struct { pub *rsa.PublicKey; priv *rsa.PrivateKey }
    class RSA < Object
      #: (String) -> RSA
      def self.new(pem) = %x{ return rbRSAParse(string(pem)) }

      #: () -> bool
      def public? = true

      #: () -> bool
      def private? = %x{ Boolean(self.priv != nil) }

      #: (String, String, String) -> bool
      def verify(digest, signature, data) = %x{
        h, sum := rbRSAHash(string(digest), string(data))
        return Boolean(rsa.VerifyPKCS1v15(self.pub, h, sum, []byte(signature)) == nil)
      }

      # ponytail: Go's MGF1 always uses the message digest, so another mgf1_hash returns false; upgrade is a hand-rolled EMSA-PSS decode.
      #: (String, String, String, ?salt_length: untyped, ?mgf1_hash: String?) -> bool
      def verify_pss(digest, signature, data, salt_length: :auto, mgf1_hash: nil) = %x{
        h, sum := rbRSAHash(string(digest), string(data))
        if mgf1_hash != nil && rbRSAHashName(string(*mgf1_hash)) != rbRSAHashName(string(digest)) {
          return false
        }
        return Boolean(rsa.VerifyPSS(self.pub, h, sum, []byte(signature), &rsa.PSSOptions{SaltLength: rbRSASaltLength(salt_length, h)}) == nil)
      }
    end
  end
end
