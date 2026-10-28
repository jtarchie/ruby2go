# rbs_inline: enabled

# Working with `untyped`: is_a? checks narrow a local, `||` and truthiness.

#: (untyped) -> String
def render(resource)
  return "none" unless resource

  case resource
  when Array
    "[" + resource.map { |r| render(r) }.join(",") + "]"
  when Integer
    "int:#{resource + 1}"
  when String
    "str:#{resource.upcase}"
  else
    "<#{resource}>"
  end
end

puts render(nil), render(false), render(5), render("s"), render([1, "x", [2]]), render(2.5)
value = nil #: untyped
puts value || "fallback", (value || 3).inspect
puts 5.is_a?(Integer), 5.is_a?(Comparable), "s".is_a?(Integer), 5.is_a?(Object)
