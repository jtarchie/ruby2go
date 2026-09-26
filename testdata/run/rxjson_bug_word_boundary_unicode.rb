# skip: \b treats non-ASCII letters as word characters in MRI ("café" !~ /caf\b/); RE2's \b is ASCII-only

# rbs_inline: enabled

puts "café".match(/caf\b/).inspect, "éb".match(/\bb/).inspect, "日本 x".match(/\b本/).inspect
