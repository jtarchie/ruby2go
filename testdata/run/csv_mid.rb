# rbs_inline: enabled
require "tmpdir"
require "csv"

# CSV.read/foreach/open on real files (decision 53), plus row_sep/skip_blanks/force_quotes.
Dir.mktmpdir do |dir|
  path = File.join(dir, "a.csv")
  File.write(path, "sku,qty\na1,4\n\na2,7\n")

  p CSV.read(path)

  rows = [] #: Array[Array[String?]]
  CSV.foreach(path) { |row| rows << row }
  p rows

  p CSV.foreach(path).to_a

  empty = File.join(dir, "empty.csv")
  File.write(empty, "")
  p CSV.read(empty)

  out = File.join(dir, "out.csv")
  CSV.open(out, "w") do |csv|
    csv << ["sku", "qty"]
    csv << ["b1", 2]
    csv << ["b2", nil]
  end
  puts File.read(out)
  p CSV.read(out)

  CSV.open(out, "a") { |csv| csv << ["b3", 9] }
  p CSV.read(out).last

  forced = File.join(dir, "forced.csv")
  CSV.open(forced, "w", force_quotes: true) { |csv| csv << ["a", nil, "b,c"] }
  puts File.read(forced)

  sep = File.join(dir, "sep.csv")
  File.write(sep, "a;b;c")
  p CSV.read(sep, row_sep: ";")

  p CSV.parse("a,b\n\n\nc,d\n", skip_blanks: true)

  p CSV.generate_line(["a", nil], force_quotes: true)
end
