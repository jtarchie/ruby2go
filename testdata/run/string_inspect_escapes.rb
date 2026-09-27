# rbs_inline: enabled

# Non-printable non-ASCII characters are \uXXXX (\u{X} above U+FFFF);
# format (Cf) and private-use characters print raw, like MRI.
puts "\u0085| | | |­|​|".inspect
puts "\u{10FFFF}|\u{E0001}|\u{1F600}|�|é".inspect
# Invalid bytes are \xNN, one per byte; a surrogate encoding is invalid too.
puts "\xed\xa0\x80|\xe1\x80|\xc3".inspect
# Named escapes and the #{ #$ #@ guard still apply around them.
puts "\e\a\b\f\v\u0085\#{x}\#$y\#@z#a".inspect
puts [" ", "\xff"].inspect
# ASCII-only symbols are US-ASCII in MRI, so controls stay \xNN.
puts :"a\x00b".inspect
puts({"a\x7f": 1}.inspect)
