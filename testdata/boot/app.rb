# load_path: lib
require "demo"
require "demo/text"

puts Demo::Thing.new("hi").greet
puts Demo::Lazy.new.go
p Demo::Text.forwarded_values("for=192.0.2.60;proto=http;by=203.0.113.43")
p Demo::Text.forwarded_values(%q{for="_gazonk", For="[2001:db8:cafe::17]:4711"})
p Demo::Text.forwarded_values(nil)
p Demo::Text.first_param(+"for=x")
