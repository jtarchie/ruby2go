# rbs_inline: enabled

# trace: kramdown requires its parser and converters by computed name (`require "kramdown/converter/#{name}"`), so rugaroo compiles what an MRI run loads (decision 296)

# The unmodified kramdown gem: Markdown in, HTML (and normalized kramdown) out.
require "kramdown"

SOURCE = <<~MD
  # Release notes {#notes}

  rugaroo compiles *unmodified* gems. This page came from **kramdown**.

  ## Features

  1. Typed Ruby to Go
  2. Gems: `cuba`, `rack`, `sqlite3`
     - nested item
     - another

  > A quote with a [link](https://example.com "Example").

  ```ruby
  puts "hello"
  ```

  | Name  | Value |
  |-------|------:|
  | one   | 1     |
  | two   | 2     |

  Term
  : Definition here

  A footnote[^1], <span>inline html</span> & entities &copy;.
  "Smart quotes" -- dashes... and an ![image](a.png).

  [^1]: The footnote.
MD

doc = Kramdown::Document.new(SOURCE)
puts doc.to_html
p doc.warnings

# Options: shift every header down one level and leave out the generated ids.
small = Kramdown::Document.new("# Title\n\nBody *text*.\n", header_offset: 1, auto_ids: false)
puts small.to_html

# The kramdown converter writes the parsed tree back out as normalized kramdown.
puts Kramdown::Document.new("Some  *emphasis*   and a [link]( https://example.com ).\n\n* a\n* b\n").to_kramdown
