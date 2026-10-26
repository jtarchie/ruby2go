# rbs_inline: enabled

# Each buffer is a fresh local nothing else holds until it is returned, so `<<` compiles (decision 382).

Row = Struct.new(:name, :qty, :price)

ROWS = [
  Row.new("apple", 3, 0.5),
  Row.new("pear, green", 1, 0.75),
  Row.new("fig", 12, 0.2)
].freeze

#: (String) -> String
def escape_csv(field)
  return field unless field.include?(",") || field.include?("\"")

  "\"#{field.gsub("\"", "\"\"")}\""
end

#: (Array[Row]) -> String
def csv(rows)
  out = +"name,qty,total\n"
  rows.each do |r|
    out << escape_csv(r.name) << "," << r.qty.to_s << ","
    out << format("%.2f", r.qty * r.price) << "\n"
  end
  out
end

#: (Array[Row]) -> String
def html(rows)
  out = String.new
  out << "<table>\n"
  rows.each_with_index do |r, i|
    cls = i.even? ? "even" : "odd"
    out.concat("  <tr class=\"", cls, "\">")
    out << "<td>" << r.name << "</td><td>" << r.qty.to_s << "</td></tr>\n"
  end
  out << "</table>"
end

#: (Integer) -> String
def ruler(width)
  line = +""
  i = 0
  while i < width
    line << ((i % 10).zero? ? "|" : "-")
    i += 1
  end
  line
end

puts csv(ROWS)
puts html(ROWS)
puts ruler(31)
big = ruler(100_000)
puts big.size
puts big.count("|")
