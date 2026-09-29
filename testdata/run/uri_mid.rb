# rbs_inline: enabled
require "uri"

u = URI.parse("http://user:pass@host:8080/path?q=1#frag")
puts u.class, u.scheme, u.host, u.port, u.path, u.query, u.fragment, u.userinfo, u.user, u.password
puts u.to_s, u.request_uri
puts u.is_a?(URI::HTTP), u.is_a?(URI::HTTPS)

s = URI.parse("https://example.com/a")
puts s.class, s.port, s.is_a?(URI::HTTP)

h = URI.parse("http://example.com/a")
puts h.port, URI.parse("http://example.com:80/a").to_s

bare = URI("http://example.com")
puts bare.class, bare.path, bare.request_uri

# URI.join: trailing slash, "..", absolute path, absolute override, multiple relatives
puts URI.join("http://x.com/a/b", "c").to_s
puts URI.join("http://x.com/a/b/", "c").to_s
puts URI.join("http://x.com/a/b", "../c").to_s
puts URI.join("http://x.com/a/b", "/c").to_s
puts URI.join("http://x.com/a/b", "http://y.com/z").to_s
puts URI.join("http://x.com/a/", "b", "c").to_s
puts (URI.parse("http://x.com/a/b") + "c").to_s

# URI.decode_www_form is the inverse of URI.encode_www_form
form = { "a" => "1", "b" => "hello world", "c" => "x&y=z" } #: Hash[String, String]
encoded = URI.encode_www_form(form)
puts encoded
puts URI.decode_www_form(encoded).inspect
puts URI.decode_www_form("").inspect
puts URI.decode_www_form("a&b=2").inspect

begin
  URI.parse("http://exa mple.com")
rescue URI::InvalidURIError => e
  puts e.class, e.message
end
