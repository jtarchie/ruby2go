# rbs_inline: enabled
# args: --seed 1
require "ipaddr"
require "minitest/autorun"

class IPv4Test < Minitest::Test
  NET = IPAddr.new("192.168.1.0/24")

  def test_to_s_and_inspect
    assert_equal "192.168.1.0", NET.to_s
    assert_equal "#<IPAddr: IPv4:192.168.1.0/255.255.255.0>", NET.inspect
  end

  def test_include_takes_strings_and_subnets
    assert_equal true, NET.include?("192.168.1.42")
    assert_equal false, NET.include?("192.168.2.1")
    assert_equal true, NET.include?(IPAddr.new("192.168.1.128/25"))
    assert_equal false, NET.include?(IPAddr.new("192.168.0.0/16"))
  end

  def test_mask_by_prefix_or_netmask
    assert_equal "192.168.0.0", NET.mask(16).to_s
    assert_equal "192.168.0.0", NET.mask("255.255.0.0").to_s
  end

  def test_to_range_and_succ
    range = NET.to_range
    assert_equal "192.168.1.0", range.first.to_s
    assert_equal "192.168.1.255", range.last.to_s
    assert_equal "192.168.1.1", range.first.succ.to_s
  end

  def test_host_bits_are_masked_off_before_comparing
    other = IPAddr.new("192.168.1.5/24")
    assert_equal true, NET == other
    assert_equal 0, NET <=> other
    assert_equal true, NET.eql?(other)
    assert_equal true, NET.ipv4?
    assert_equal false, NET.ipv6?
  end
end

class IPv6Test < Minitest::Test
  V6 = IPAddr.new("2001:db8::/32")

  def test_to_s_is_compressed_and_inspect_is_expanded
    assert_equal "2001:db8::", V6.to_s
    assert_equal "#<IPAddr: IPv6:2001:0db8:0000:0000:0000:0000:0000:0000/ffff:ffff:0000:0000:0000:0000:0000:0000>", V6.inspect
  end

  def test_family_include_and_succ
    assert_equal false, V6.ipv4?
    assert_equal true, V6.ipv6?
    assert_equal true, V6.include?("2001:db8:1::1")
    assert_equal false, V6.include?("2001:db9::1")
    assert_equal "2001:db8::1", V6.succ.to_s
  end

  # An IPv4-mapped address stays IPv6 and doesn't compare with plain IPv4.
  def test_ipv4_mapped_address
    mapped = IPAddr.new("::ffff:192.168.1.1")
    assert_equal "::ffff:192.168.1.1", mapped.to_s
    assert_equal false, mapped.ipv4?
    assert_equal true, mapped.ipv6?
    assert_nil(mapped <=> IPAddr.new("192.168.1.1"))
  end
end

class IPAddrErrorTest < Minitest::Test
  def test_succ_past_the_last_address
    e = assert_raises(IPAddr::InvalidAddressError) { IPAddr.new("255.255.255.255").succ }
    assert_equal "invalid address: 4294967296", e.message
  end

  def test_bad_address
    e = assert_raises(IPAddr::InvalidAddressError) { IPAddr.new("999.1.1.1") }
    assert_equal "invalid address: 999.1.1.1", e.message
  end

  def test_bad_prefix
    e = assert_raises(IPAddr::InvalidPrefixError) { IPAddr.new("1.2.3.4/33") }
    assert_equal "invalid length", e.message
  end
end
