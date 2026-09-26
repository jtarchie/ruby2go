# skip: `case n when Integer` with n a typed primitive (Integer, String, ...) emits a Go type switch on a non-interface value: go build fails "n is not an interface"; MRI matches Integer === n

# rbs_inline: enabled

#: (Integer) -> String
def typed_int(n)
  case n
  when Float then "float"
  when Integer then "int #{n + 1}"
  else "?"
  end
end

#: (String) -> String
def typed_str(s)
  case s
  when Symbol then "symbol"
  when String then "string #{s.size}"
  else "?"
  end
end

puts typed_int(4), typed_str("abc")
