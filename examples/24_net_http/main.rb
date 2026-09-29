# rbs_inline: enabled
# args: --seed 1
# A WEBrick server and a Net::HTTP client in one process.
require "net/http"
require "webrick"
require "minitest/autorun"

class NetHTTPTest < Minitest::Test
  FORM = { "Content-Type" => "application/x-www-form-urlencoded" }

  #: () -> void
  def test_round_trips
    server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
    server.mount_proc("/") do |req, res|
      if req.path == "/missing"
        res.status = 404
        res.body = "no such page"
      else
        res["Content-Type"] = "text/plain"
        res["X-Method"] = req.request_method
        res.body = "#{req.request_method} #{req.path} #{req.query.inspect}"
      end
    end
    thread = Thread.new { server.start }

    http = Net::HTTP.new("127.0.0.1", server.config[:Port])
    got = [
      http.get("/hello?name=world&x=1"),
      http.get("/missing"),
      http.post("/items", URI.encode_www_form("item[name]" => "a b", "qty" => "2"), FORM),
      http.put("/items/1", "qty=3", FORM)
    ].map do |res|
      # Header lookup is case-insensitive; a missing header is nil.
      [res.code, res["Content-Type"], res["x-method"], res.body]
    end

    server.shutdown
    thread.join

    assert_equal [
      ["200", "text/plain", "GET", 'GET /hello {"name" => "world", "x" => "1"}'],
      ["404", nil, nil, "no such page"],
      ["200", "text/plain", "POST", 'POST /items {"item[name]" => "a b", "qty" => "2"}'],
      ["200", "text/plain", "PUT", 'PUT /items/1 {"qty" => "3"}'],
    ], got
  end
end
