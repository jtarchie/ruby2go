# skip: an untyped non-String passed to a typed Regexp method fails its Go type assertion and surfaces as StandardError; MRI raises TypeError (the dynamic-dispatch path already does)

# rbs_inline: enabled

#: () -> untyped
def five = 5

begin
  puts(/a/.match?(five))
rescue TypeError => e
  puts "match? #{e.class}"
end
begin
  puts(/a/ =~ five)
rescue TypeError => e
  puts "=~ #{e.class}"
end
begin
  puts "abc".match?(five)
rescue TypeError => e
  puts "String#match? #{e.class}"
end
