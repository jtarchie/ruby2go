# rbs_inline: enabled

# The unmodified cuba gem, on the unmodified rack gem, served by rackup's WEBrick handler and driven by Net::HTTP in the same process.
require "cuba"
require "net/http"
require "rackup"
require "webrick"

Cuba.define do
  on get, root do
    res.write "<h1>home</h1>"
  end

  on get, "users/:id" do |id|
    res.text "user #{id}"
  end

  on get, "search", param("q") do |q|
    res.html "<p>#{Rack::Utils.escape_html(q)}, page #{req.params["page"] || 1}</p>"
  end

  on post, "items", param("name") do |name|
    res.status = 201
    res["location"] = "/items/#{Rack::Utils.escape_path(name)}"
    res.text "created #{name} x#{req.POST["qty"]}"
  end
end

ready = Queue.new #: Queue[WEBrick::HTTPServer]
thread = Thread.new do
  Rackup::Handler::WEBrick.run(Cuba, Host: "127.0.0.1", Port: 0) { |server| ready << server }
end
server = ready.pop || raise("the server did not start")

http = Net::HTTP.new("127.0.0.1", server.config[:Port])
form = { "Content-Type" => "application/x-www-form-urlencoded" }
[
  http.get("/"),
  http.get("/users/42"),
  http.get("/search?q=%3Cb%3Erb2go%3C%2Fb%3E&page=2"),
  http.get("/search"),
  http.post("/items", URI.encode_www_form("name" => "a b", "qty" => "3"), form),
  http.get("/missing")
].each do |res|
  puts "#{res.code} #{res["content-type"].inspect} #{res["content-length"].inspect} #{res["location"]&.sub(/:\d+/, ":PORT").inspect}"
  puts "  #{res.body.inspect}"
end

server.shutdown
thread.join
puts "stopped"
