# rbs_inline: enabled

# trace: kramdown requires its parser and converters by computed name, so rugaroo compiles what an MRI run loads (decision 296)

# The unmodified kramdown gem: parse a Markdown document and convert it to HTML, many times over.
require "kramdown"

SECTION = <<~MD.freeze
  ## Section

  Some *emphasis*, **strong**, `code`, a [link](https://example.com "title") and an ![image](a.png).
  "Smart quotes" -- dashes... and &copy; entities.

  1. first item
  2. second item with `code`
     - nested
     - list

  > A quote with a [reference link][ref].

  | Name | Value |
  |------|------:|
  | one  | 1     |
  | two  | 2     |

      indented code block

  Term
  : Definition

  A footnote[^1].

  [ref]: https://example.org
  [^1]: The footnote text.
MD

DOC = "# Benchmark {#top}\n\n#{SECTION * 20}"

bytes = 0
html = ""
30.times do
  html = Kramdown::Document.new(DOC).to_html
  bytes += html.bytesize
end
puts bytes
puts html.lines.first(3).join
