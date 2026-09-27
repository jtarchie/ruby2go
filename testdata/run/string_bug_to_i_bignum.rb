# skip: wontfix: "99999999999999999999".to_i is a Bignum in MRI; rb2go raises RangeError (README decision 35)

# rbs_inline: enabled

puts "99999999999999999999".to_i
