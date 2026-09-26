# skip: `case x when Hash` on an untyped local rebinds x through _to_any into a temp that gets no `_ = t`, so an arm that never reads x fails go build "declared and not used" (when Array too)

# rbs_inline: enabled

#: (untyped) -> String
def kind(x)
  case x
  when Hash then "hash"
  else "other"
  end
end

puts kind({ "a" => 1 }), kind(1)
