# rbs_inline: enabled

# A hand-written Rack app served by rackup's WEBrick handler, driven by Net::HTTP in the same process.
require "net/http"
require "rackup"
require "webrick"

# A streamed body: Rack calls each for the chunks, then close.
class Countdown
  #: (Integer) -> void
  def initialize(n)
    @n = n
  end

  #: () { (String) -> void } -> void
  def each
    @n.downto(1) { |i| yield "#{i} " }
    yield "liftoff"
  end

  #: () -> void
  def close
    puts "  (countdown closed)"
  end
end

class App
  #: (Hash[String, untyped]) -> [Integer, Hash[String, untyped], untyped]
  def call(env)
    method = env["REQUEST_METHOD"].to_s
    path = env["PATH_INFO"].to_s
    text = { "content-type" => "text/plain" } #: Hash[String, untyped]
    if method == "GET" && path == "/hello"
      params = URI.decode_www_form(env["QUERY_STRING"].to_s).to_h
      headers = text.merge("x-agent" => env["HTTP_X_AGENT"].to_s, "set-cookie" => ["a=1", "b=2"])
      [200, headers, ["hello #{params["name"]} ", "(#{params.size} params, #{env["SCRIPT_NAME"].inspect}, #{env["rack.url_scheme"]})"]]
    elsif method == "POST" && path == "/items"
      form = URI.decode_www_form(env.fetch("rack.input").read.to_s).to_h
      [201, text.merge("location" => "/items/1"), ["created #{form["item[name]"]} x#{form["qty"]} as #{env["CONTENT_TYPE"]}"]]
    elsif path == "/countdown"
      [200, text, Countdown.new(3)]
    else
      [404, text.merge("x-list" => ["a", "b"]), ["no route for #{method} #{path}"]]
    end
  end
end

ready = Queue.new #: Queue[WEBrick::HTTPServer]
thread = Thread.new do
  Rackup::Handler::WEBrick.run(App.new, Host: "127.0.0.1", Port: 0) { |server| ready << server }
end
server = ready.pop || raise("the server did not start")

port = server.config[:Port]
http = Net::HTTP.new("127.0.0.1", port)
form = { "Content-Type" => "application/x-www-form-urlencoded" }
[
  http.get("/hello?name=world&x=1", { "X-Agent" => "rb2go" }),
  http.post("/items", URI.encode_www_form("item[name]" => "a b", "qty" => "2"), form),
  http.get("/countdown"),
  http.delete("/nope")
].each do |res|
  puts "#{res.code} #{res["content-type"].inspect} #{res["x-agent"].inspect} #{res["location"]&.sub(port.to_s, "PORT").inspect} #{res["x-list"].inspect}"
  puts "  cookies: #{res.get_fields("set-cookie").inspect}"
  puts "  #{res.body}"
end

server.shutdown
thread.join
puts "stopped"
