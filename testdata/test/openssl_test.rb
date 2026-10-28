# rbs_inline: enabled

require "minitest/autorun"
require "openssl"
require "base64"

module OpensslTests
  PEM = <<~KEY.freeze
    -----BEGIN PUBLIC KEY-----
    MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAo+sLhF4tO7J5uqLDk7Gx
    5f/G4pPCe5iwXi9ReJxjTIxzKq+wiDslgo6avU6PXdRYB35Rb/x22b6kA9i6YIK6
    8XWM4ZmzkE17P4HCbh+N0V7G3x3bl59lrvwYcGMU8QhKo7ZHixYQpi7FEqxzz47N
    WhNjHgWoOJjRixgwBBnCFVHtmJguTrIMizWTbZc699Z5EzMqcBe4tHgEXSc4jmr/
    zpR5n0ZRwsY3I17aa/A+juSg+YhWYnOHJzO1sos2YlmbWinhnHPka3OoddA++WUF
    BdfmbOris5sPd/9Di+ZUK/4jN/cFSAs3bf0p0I0YbdFxleHO/Ao9N185dEcAbXp+
    NwIDAQAB
    -----END PUBLIC KEY-----
  KEY
  PSS_SIG = Base64.strict_decode64("iTC0G6jFe+ZNXOurh5dW/W15NY55+YttKV4FzEFwiseJgwWEbdzDsWyUlyZTFfIim02w33ObNW2mCoNzvFVK0B/5IRg7GFMQ7aTRQ8ofCdamzpnXNg1jGKCJATT60dTcur9DEZhhg92o8HzLfp6UdQckkk+HFZm6IDi0eiu/NKR/1pqA9W5aAn3XRrxF/6aGWbxJVVHuVtysSmu6LJ9MvJkseJz6iG5ydfdy06orM4GS7hi3vP0seHbJoMN2yKjgAV9xeAl4jIU+aYYbN4yNxy+DsC5Ei7GotbyLUq6gWwXoEQpazHsGuXarYnHea0cbefUmSHjKUDjzeRZhBZYm+g==")
  V15_SIG = Base64.strict_decode64("EdLJfxb6xW7qv2CR0ejWVQvZzEnz9+ANL7fAV0GDSrg5Rs/brbYr0s+UFBYd6nqpVLBB3lH0DRItg81EpLhFyEEMBIE9vvo87xwzZkO0enGwN8djLBZBFbAq+g5p8SKgtRE8ypgYLRk5lPGhse65DVNO1zslf2N+UPD+8NDldwymgn10EHOzGIqzVZg3Podpaql/mnbAPdbO+eVugEjyx5ohRLXW5VybxPD2reCf0WZHmdum2M48RMRQ1Xsm7eatSJT0DBq7+GnTnEDQv3ixZH9G1hr1lxrafG4OWBB70DrgZuLuYu031k7VpbNmyjqzptza+mjTT8hAzafzlcnIIQ==")

  class RSATest < Minitest::Test
    def test_public_key
      key = OpenSSL::PKey::RSA.new(PEM)
      assert key.public?
      refute key.private?
    end

    def test_verify_pss
      key = OpenSSL::PKey::RSA.new(PEM)
      assert key.verify_pss("SHA512", PSS_SIG, "hdr.payload", salt_length: :digest, mgf1_hash: "SHA512")
      assert key.verify_pss("SHA512", PSS_SIG, "hdr.payload", salt_length: :auto, mgf1_hash: "SHA512")
      refute key.verify_pss("SHA512", PSS_SIG, "hdr.payloadx", salt_length: :digest, mgf1_hash: "SHA512")
      refute key.verify_pss("SHA512", PSS_SIG, "hdr.payload", salt_length: :digest, mgf1_hash: "SHA256")
    end

    def test_verify_pkcs1
      key = OpenSSL::PKey::RSA.new(PEM)
      assert key.verify("SHA256", V15_SIG, "hdr.payload")
      refute key.verify("SHA256", V15_SIG, "hdr.payloadx")
      refute key.verify("SHA256", "short", "hdr.payload")
    end

    def test_bad_pem
      err = begin
        OpenSSL::PKey::RSA.new("junk")
        nil
      rescue OpenSSL::PKey::RSAError => e
        e
      end
      assert_kind_of OpenSSL::OpenSSLError, err
      assert_equal OpenSSL::PKey::PKeyError, OpenSSL::PKey::RSAError
    end
  end

  class DigestTest < Minitest::Test
    def test_hmac
      assert_equal "2d93cbc1be167bcb1637a4a23cbff01a7878f0c50ee833954ea5221bb1b8c628", OpenSSL::HMAC.hexdigest("SHA256", "key", "msg")
      assert_equal "102900b72b7bf1031eec76b4804b66052376896b", OpenSSL::HMAC.hexdigest("SHA1", "key", "msg")
      assert_equal 32, OpenSSL::HMAC.digest("SHA256", "k", "m").bytesize
    end

    def test_pbkdf2
      key = OpenSSL::PKCS5.pbkdf2_hmac("secret", "salt", 1000, 32, "SHA256")
      assert_equal "a8df899f3c4f204d967e0ad63c092987c10055ebb017b3d9d28add218d4f7aad", key.unpack1("H*")
    end
  end
end
