# rbs_inline: enabled

# TestGemCorpus: every testcase of the installed kramdown gem converted to HTML with default options, as rugaroo compiles
# it from a traced run; an exception prints its class, so one case cannot hide the rest.
require "kramdown"

dir = ARGV[0] || ""
Dir.glob(File.join(dir, "**", "*.text")).sort.each do |path|
  text = File.read(path)
  out = begin
    Kramdown::Document.new(text).to_html
  rescue StandardError, NotImplementedError => e
    "ERROR #{e.class}"
  end
  puts "== #{path.delete_prefix(dir)}"
  puts out
end
