# skip: /i uses Unicode multi-character folding in MRI ("straße" =~ /STRASSE/i, "ﬀ" =~ /FF/i); RE2 folds single runes only

# rbs_inline: enabled

puts "straße".match?(/STRASSE/i), "SS".match?(/ß/i), "ﬀ".match?(/FF/i)
