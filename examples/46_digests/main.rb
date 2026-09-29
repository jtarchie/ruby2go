# rbs_inline: enabled
# args: --seed 1

require "base64"
require "digest"
require "zlib"
require "securerandom"
require "cgi/escape"
require "minitest/autorun"

# A content-addressed store: blobs keyed by SHA-256, checked by CRC32 on read.
class BlobStore
  #: () -> void
  def initialize
    @blobs = {} #: Hash[String, String]
    @crcs = {} #: Hash[String, Integer]
  end

  #: (String) -> String
  def put(data)
    key = Digest::SHA256.hexdigest(data)
    @blobs[key] = Base64.strict_encode64(data)
    @crcs[key] = Zlib.crc32(data)
    key
  end

  #: (String) -> String?
  def get(key)
    enc = @blobs[key]
    return nil if enc.nil?
    data = Base64.strict_decode64(enc)
    raise "corrupt #{key}" unless Zlib.crc32(data) == @crcs[key]
    data
  end
end

FOX = "The quick brown fox jumps over the lazy dog"

class BlobStoreTest < Minitest::Test
  def test_put_keys_by_sha256_and_get_round_trips
    store = BlobStore.new
    k = store.put("hello, world")
    assert_equal "09ca7e4eaa6e8ae9c7d261167129184883644d07dfba7cbfbc4c8a2e08360d5b", k
    assert_equal "hello, world", store.get(k)
    assert_nil store.get("nope")
  end
end

class DigestTest < Minitest::Test
  def test_hexdigests
    assert_equal "9e107d9d372bb6826bd81d3542a419d6", Digest::MD5.hexdigest(FOX)
    assert_equal "2fd4e1c67a2d28fced849ee1bb76e7391b93eb12", Digest::SHA1.hexdigest(FOX)
    assert_equal "ca737f1014a48f4c0b6dd43cb177b0afd9e5169367544c494011e3317dbf9a509cb1e5dc1e85a941bbee3d7f2afbc9b1",
                 Digest::SHA384.hexdigest(FOX)
    assert_equal "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e",
                 Digest::SHA512.hexdigest("")
  end

  def test_raw_and_base64_digests
    assert_equal 32, Digest::SHA256.digest(FOX).bytesize
    assert_equal "nhB9nTcrtoJr2B01QqQZ1g==", Digest::MD5.base64digest(FOX)
  end

  # An incremental digest matches the one-shot digest.
  def test_update_and_append
    chunked = Digest::SHA256.new.update("The quick ").update("brown fox jumps over the lazy dog")
    assert_equal Digest::SHA256.hexdigest(FOX), chunked.hexdigest
    d = Digest::SHA1.new
    d << "abc"
    assert_equal "a9993e364706816aba3e25717850c26c9cd0d89d", d.hexdigest
    assert_equal d.hexdigest, d.to_s
  end
end

class Base64Test < Minitest::Test
  # encode64 wraps at 60 characters and ends with a newline.
  def test_encode64_wraps_lines
    assert_equal "QUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFB\nQUFBQUFBQUFBQUFBQUFB\n",
                 Base64.encode64("A" * 60)
    assert_equal "aGk=\n", Base64.encode64("hi")
  end

  def test_urlsafe_variants
    assert_equal "__4_Pg==", Base64.urlsafe_encode64("\xff\xfe?>")
    assert_equal "YWI", Base64.urlsafe_encode64("ab", padding: false)
    assert_equal [0xff, 0xfe, 0x3f, 0x3e], Base64.urlsafe_decode64("__4_Pg").bytes
    assert_equal "hello", Base64.decode64("aGVsbG8=\n")
  end

  def test_strict_decode_rejects_bad_input
    e = assert_raises(ArgumentError) { Base64.strict_decode64("not base64!") }
    assert_equal "invalid base64", e.message
  end
end

class ZlibTest < Minitest::Test
  def test_checksums
    assert_equal 0, Zlib.crc32("")
    assert_equal 891_568_578, Zlib.crc32("abc")
    assert_equal 300_286_872, Zlib.adler32("Wikipedia")
    # A running CRC continues from a previous one.
    assert_equal Zlib.crc32("abcdef"), Zlib.crc32("def", Zlib.crc32("abc"))
  end
end

# SecureRandom values differ every run, so only their shape is checked.
class SecureRandomTest < Minitest::Test
  def test_hex_uuid_and_bytes_have_the_right_shape
    assert_equal 32, SecureRandom.hex.size
    assert_match(/\A[0-9a-f]{8}\z/, SecureRandom.hex(4))
    assert_match(/\A\h{8}-\h{4}-4\h{3}-[89ab]\h{3}-\h{12}\z/, SecureRandom.uuid)
    assert_equal 10, SecureRandom.random_bytes(10).bytesize
    assert_equal 12, SecureRandom.base64(9).size
    assert_match(/\A[\w-]+\z/, SecureRandom.urlsafe_base64)
    assert_match(/\A[A-Za-z0-9]{12}\z/, SecureRandom.alphanumeric(12))
  end

  def test_random_numbers_stay_in_range
    n = SecureRandom.random_number(10)
    assert_equal true, n >= 0 && n < 10
    f = SecureRandom.random_number
    assert_equal true, f >= 0.0 && f < 1.0
  end
end

class CGITest < Minitest::Test
  def test_form_escaping
    assert_equal "a+b%26c%3Dd%2F%C3%A9~", CGI.escape("a b&c=d/é~")
    assert_equal "a b/é", CGI.unescape("a+b%2F%C3%A9")
  end

  def test_html_escaping
    assert_equal "&lt;a href=&quot;x&quot;&gt;&#39;&amp;&lt;/a&gt;", CGI.escapeHTML(%q(<a href="x">'&</a>))
    assert_equal "<&'\">AA", CGI.unescapeHTML("&lt;&amp;&#39;&quot;&gt;&#x41;&#65;")
  end

  def test_uri_component_escaping
    assert_equal "a%20b%2Bc", CGI.escapeURIComponent("a b+c")
    assert_equal "a b+c", CGI.unescapeURIComponent("a%20b+c")
  end
end
