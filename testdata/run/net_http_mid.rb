# rbs_inline: enabled
require "net/http"
require "webrick"
require "uri"
require "base64"

server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
server.mount_proc("/") do |req, res|
  case req.path
  when "/ok"
    res.body = "ok"
  when "/created"
    res.status = 201
    res.body = "created"
  when "/badreq"
    raise WEBrick::HTTPStatus::BadRequest, "bad"
  when "/auth"
    if req["Authorization"] == "Basic #{Base64.strict_encode64("alice:secret")}"
      res.body = "welcome"
    else
      res.status = 401
      res.body = "nope"
    end
  when "/form"
    res.body = req.query.inspect
  when "/headers"
    pairs = []
    req.each { |k, v| pairs << "#{k}=#{v}" if k.start_with?("x-") }
    res.body = pairs.sort.join(",")
  when "/redirect"
    res.set_redirect(WEBrick::HTTPStatus::Found, "/ok")
  when "/setcookie"
    res.cookies << WEBrick::Cookie.new("sid", "abc123")
    res.body = "cookie set"
  when "/readcookie"
    res.body = req.cookies.map { |c| "#{c.name}=#{c.value}" }.join(",")
  else
    res.status = 404
    res.body = "missing"
  end
end
thread = Thread.new { server.start }
port = server.config[:Port]

http = Net::HTTP.new("127.0.0.1", port)

# response classes: case/when dispatch, is_a? against the category
["/ok", "/created", "/missing", "/badreq"].each do |path|
  res = http.get(path)
  kind = case res
         when Net::HTTPOK then "OK"
         when Net::HTTPCreated then "Created"
         when Net::HTTPNotFound then "NotFound"
         when Net::HTTPBadRequest then "BadRequest"
         else "Other"
         end
  puts "#{res.code} #{kind} success=#{res.is_a?(Net::HTTPSuccess)}"
end

# request objects: custom headers, basic auth, form data
req = Net::HTTP::Get.new("/headers")
req["X-Foo"] = "1"
req["X-Bar"] = "2"
puts http.request(req).body

req2 = Net::HTTP::Get.new("/auth")
req2.basic_auth("alice", "secret")
res2 = http.request(req2)
puts res2.code, res2.body

req3 = Net::HTTP::Get.new("/auth")
res3 = http.request(req3)
puts res3.code, res3.body

form_req = Net::HTTP::Post.new("/form")
form_req.set_form_data({ "a" => "1", "b" => "two words" })
puts http.request(form_req).body

# redirect: body/Location set before the raise, status from the HTTPStatus class
res4 = http.get("/redirect")
puts res4.code, (res4["Location"] || "").end_with?("/ok"), res4.is_a?(Net::HTTPRedirection)

# WEBrick cookies: set on the response, read back on the request
res5 = http.get("/setcookie")
puts res5["Set-Cookie"]

req6 = Net::HTTP::Get.new("/readcookie")
req6["Cookie"] = "sid=abc123; theme=dark"
puts http.request(req6).body

# Net::HTTP class-level convenience over a URI
uri = URI("http://127.0.0.1:#{port}/ok")
puts Net::HTTP.get(uri)
puts Net::HTTP.get_response(uri).body

form_uri = URI("http://127.0.0.1:#{port}/form")
puts Net::HTTP.post_form(form_uri, { "x" => "y" }).body

Net::HTTP.start("127.0.0.1", port) do |http2|
  puts http2.get("/ok").body
end

# use_ssl/open_timeout/read_timeout accessors
http.open_timeout = 5.0
http.read_timeout = 5.0
puts http.open_timeout, http.read_timeout
http.use_ssl = false
puts http.use_ssl?

server.shutdown
thread.join
puts "stopped"
