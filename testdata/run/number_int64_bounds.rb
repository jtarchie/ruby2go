# rbs_inline: enabled
# Results at the edges of Integer's 64 bits (README decision 35): none of
# these may trip the overflow checks.

m = 0x7fff_ffff_ffff_ffff #: Integer
n = -9_223_372_036_854_775_808 #: Integer
t = -2 #: Integer
o = -1 #: Integer
puts m + 0, n + 0, m - 0, (n + 1) - 1, m + n, n - -m, 0 - m
puts 3_037_000_499 * 3_037_000_499, -3_037_000_499 * 3_037_000_499
puts(-4_611_686_018_427_387_904 * 2, 4_611_686_018_427_387_903 * 2)
puts n * 1, m * -1, -1 * m, 2_147_483_648 * 2_147_483_647, -2_147_483_648 * -2_147_483_648
puts 1_099_511_627_776 * 4_194_304, -1_099_511_627_776 * 8_388_608
puts 2 ** 62, t ** 63, 3 ** 39, 10 ** 18, (t - 1) ** 3, 7 ** 0
puts 1 ** 1_000_000_000_000, o ** 1_000_000_000_001, o ** -4, o ** -3, 1 ** -5
puts (-m).abs, m.pred.succ, n / 1, n / 2, n % -1, -m / -1
puts 9.2e18.to_i, -9.2e18.floor, "-9223372036854775808".to_i, "9223372036854775807".to_i
