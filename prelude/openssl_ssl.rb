# rbs_inline: enabled

# Only what verify_mode=/open-uri's ssl_verify_mode: take, shared by net/http and openssl so net/http stays small.
module OpenSSL
  module SSL
    VERIFY_NONE = 0
    VERIFY_PEER = 1
  end
end
