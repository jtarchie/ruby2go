# rbs_inline: enabled

# Spinel binds C functions with `ffi_func`; here that is a no-op and the functions the blog declares are Ruby (README.md).
require "openssl"
require "base64"
require "securerandom"

class Module
  #: (*untyped) -> nil
  def ffi_func(*) = nil
end

module SpCrypto
  #: (String, String) -> String
  def self.sp_crypto_hmac_sha1_hex(key, msg) = OpenSSL::HMAC.hexdigest("SHA1", key, msg)

  #: (String, String) -> String
  def self.sp_crypto_hmac_sha256_hex(key, msg) = OpenSSL::HMAC.hexdigest("SHA256", key, msg)

  #: (String, String, Integer, Integer) -> String
  def self.sp_crypto_pbkdf2_sha256_b64url_len(secret, salt, iters, dklen)
    Base64.urlsafe_encode64(OpenSSL::PKCS5.pbkdf2_hmac(secret, salt, iters, dklen, "SHA256"), padding: false)
  end

  #: (String) -> String
  def self.sp_crypto_b64url_decode(s) = Base64.urlsafe_decode64(s)
end

module CsrfRand
  #: (Integer) -> String
  def self.sp_crypto_random_b64url(n) = SecureRandom.urlsafe_base64(n)
end
