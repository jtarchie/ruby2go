# rbs_inline: enabled

ak = {} #: Hash[Array[Integer], String]
ak[[1, 2]] = "one-two"
puts ak[[1, 2]].inspect, ak.key?([1, 2]).inspect
ak[[1, 2]] = "again"
puts ak.size, ak.inspect
puts [[1], [1], [2]].tally.inspect
puts [1, 2, 3, 4].group_by { |n| [n % 2] }.inspect
hk = {} #: Hash[Hash[String, Integer], Integer]
hk[{ "a" => 1 }] = 1
puts hk[{ "a" => 1 }].inspect, hk.key?({ "a" => 1 }).inspect
# Equal hashes as array elements: uniq/tally/group_by key them the same way.
a = { "a" => 1 } #: Hash[String, Integer]
b = { "a" => 1 } #: Hash[String, Integer]
puts [a, b].uniq.size, [a, b].tally.size, [a, b].group_by { |x| x }.size
