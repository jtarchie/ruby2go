# A `.rbs` method may list several signatures (overloads, decision 12).
# The method body compiles once; each call takes the type of the first
# signature its arguments fit: by count, then class, then the value of a
# literal argument, which may come through a constant (`Settings::PATH`).

require_relative "settings"

env = Settings.new({ "PATH_INFO" => "/users/7", "QUERY_STRING" => "a=1", "SERVER_PORT" => 8080 })

puts env[Settings::PATH].upcase
puts env["QUERY_STRING"].split("=").last
puts env[Settings::PORT] + 1
missing = env["HTTP_HOST"]
puts missing.nil? ? "no host" : missing.to_s

puts env.double(21) + 1
puts env.double("ab").upcase

puts env.greet
puts env.greet("rb2go")
puts env.greet("rb2go", "?")
