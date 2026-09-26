# skip: to_i saturates at 2**63-1; MRI returns a Bignum

# rbs_inline: enabled

puts "99999999999999999999".to_i
