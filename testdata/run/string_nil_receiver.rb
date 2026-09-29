# rbs_inline: enabled

# Decision 20: an uncaught NoMethodError on a nil String? flushes stdout so far and exits 1, like MRI.
# (The rest of this file's checks moved to testdata/test/string_test.rb.)
s = "abc"
puts "before crash"
puts s[9].size
puts "unreachable"
