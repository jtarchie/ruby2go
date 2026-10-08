# rbs_inline: enabled

# A local assigned in a class body is visible to the body's later statements
# and constant initializers, as in MRI; methods do not see it. A constant that
# reads one needs its type written down.

class Limits
  env_int = lambda do |key, fallback|
    value = ENV[key]
    value ? Integer(value, 10) : fallback
  end #: ^(String, Integer) -> Integer

  BYTES = env_int.call("LIMITS_BYTES", 4096) #: Integer
  PARAMS = env_int.call("LIMITS_PARAMS", 128) #: Integer

  #: () -> String
  def self.describe = "bytes=#{BYTES} params=#{PARAMS}"
end

puts Limits.describe
