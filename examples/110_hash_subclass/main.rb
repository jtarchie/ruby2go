# rbs_inline: enabled

# A Hash subclass that downcases its keys, like Rack::Headers (#86).
# `super` reaches the embedded Hash; keys not overridden promote to it.

# @rbs generic V
# @rbs inherits Hash[untyped, V]
class Downcaser < Hash
  def [](key)
    super(key.to_s.downcase)
  end

  def []=(key, value)
    super(key.to_s.downcase, value)
  end
end

headers = Downcaser.new
headers["Content-Type"] = "text/html"
headers["X-Trace"] = "abc"

puts headers["content-type"]
puts headers["X-TRACE"]
puts headers.keys.inspect
puts headers.size
puts headers.key?("x-trace")
