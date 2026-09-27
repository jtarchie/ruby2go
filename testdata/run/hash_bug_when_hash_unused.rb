# rbs_inline: enabled

#: (untyped) -> String
def kind(x)
  case x
  when Hash then "hash"
  else "other"
  end
end

puts kind({ "a" => 1 }), kind(1)
