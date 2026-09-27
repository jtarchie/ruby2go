# rbs_inline: enabled

#: (Integer) -> void
def joined(n)
  y = "a"
  y = nil if n > 5
  puts y.inspect
  if n > 0
    label = "pos"
  else
    label = nil
  end
  puts label.inspect
end
joined(1)
joined(9)
joined(-1)
