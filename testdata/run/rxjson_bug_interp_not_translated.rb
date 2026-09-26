# skip: interpolated values skip the Ruby-to-RE2 translation, so a \h arriving by interpolation raises RegexpError (MRI matches)

# rbs_inline: enabled

h = "\\h+"
puts(/#{h}/.match("zzF0").inspect)
