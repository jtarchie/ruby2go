# rbs_inline: enabled
# nil_check: runtime
# trace: roundhouse's runtime/ and its shims are on $LOAD_PATH, so they compile as a gem does: only what the blog reaches (decision 296)
# load_path: shims
# load_path: blog/runtime

# Roundhouse's Blog (a Rails app, transpiled by roundhouse --target spinel into blog/, unmodified), served by shims/tep_shim.rb and driven by Net::HTTP (README.md).
$LOADED_FEATURES << File.expand_path("blog/runtime/tep/tep.rb", __dir__)
$LOADED_FEATURES << File.expand_path("blog/runtime/db.rb", __dir__)
$LOAD_PATH.unshift(File.expand_path("shims", __dir__), File.expand_path("blog/runtime", __dir__))
require "ffi_shim"
require "db_shim"
require "tep_shim"
require "net/http"
require "socket"
require "tmpdir"

ENV["BLOG_DB"] = File.join(Dir.mktmpdir, "blog.sqlite3")

Tep.on_listen(lambda do |port|
  File.foreach(File.join(__dir__, "blog/db/seed.sql")) { |line| Db.exec(line) unless line.start_with?("--") || line.strip.empty? }
  http = Net::HTTP.new("127.0.0.1", port)
  show = lambda do |label, res|
    puts "#{label} #{res.code} #{res["location"]} #{res.body.to_s.bytesize}"
  end
  %w[/ /articles /articles/1 /articles/new /articles/1/edit /articles.json /articles/1.json].each { |path| show.call("GET #{path}", http.get(path)) }
  form = { "Content-Type" => "application/x-www-form-urlencoded" }
  show.call("POST /articles (too short)", http.post("/articles", "article[title]=Hi&article[body]=short", form))
  show.call("POST /articles", http.post("/articles", "article[title]=Compiled&article[body]=A+body+long+enough+to+validate.", form))
  show.call("POST /articles/4/comments", http.post("/articles/4/comments", "comment[commenter]=Ann&comment[body]=Nice", form))
  show.call("GET /articles/4", http.get("/articles/4"))
  show.call("PATCH /articles/4", http.post("/articles/4", "_method=patch&article[title]=Renamed", form))
  show.call("DELETE /articles/4/comments/4", http.post("/articles/4/comments/4", "_method=delete", form))
  show.call("DELETE /articles/4", http.post("/articles/4", "_method=delete", form))
  show.call("GET /articles", http.get("/articles"))
end)

require_relative "blog/runtime/importmap" # the importmap before the routes, as Rails loads it: blog/boot.rb loads the views (and their hoisted
require_relative "blog/config/importmap"  # importmap tags) first, which only MRI evaluates at that point
require_relative "blog/main"
