# rbs_inline: enabled

require "minitest/autorun"
require "net/http"
require "open-uri"
require "openssl"
require "rackup"
require "tmpdir"
require "webrick"

module NetTests
  # Ports differ between MRI and rb2go runs, so messages holding one go through net_unport.
  class OpenURITest < Minitest::Test
    #: (String, Integer) -> String
    def net_unport(s, port) = s.sub(":#{port}", ":PORT")

    def net_server
      server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
      server.mount_proc("/") do |req, res|
        case req.path
        when "/hello"
          res["Content-Type"] = "text/plain"
          res.body = "hello\nworld\n"
        when "/echo"
          res["Content-Type"] = "text/plain; charset=UTF-8"
          res.body = "#{req["User-Agent"]}|#{req["X-Token"]}"
        when "/meta"
          res["Content-Type"] = "Text/HTML; Charset=\"ISO-8859-1\""
          res["Last-Modified"] = "Sat, 01 Jan 2022 10:20:30 GMT"
          res["Content-Encoding"] = "X-Custom"
          res["X-Thing"] = "a"
          res.cookies << WEBrick::Cookie.new("a", "1")
          res.cookies << WEBrick::Cookie.new("b", "2")
          res.body = "meta"
        when "/bin"
          res.body = "\x00\x01"
        when "/bare"
          res["Content-Type"] = "text"
          res.body = "bare"
        when "/auth"
          if req["Authorization"] == "Basic YWxpY2U6c2VjcmV0"
            res.body = "welcome"
          else
            res.status = 401
            res.body = "nope"
          end
        when "/redirect"
          res.set_redirect(WEBrick::HTTPStatus::Found, "/hello")
        when "/moved"
          res.set_redirect(WEBrick::HTTPStatus::MovedPermanently, "/redirect")
        when "/auth-redirect"
          res.set_redirect(WEBrick::HTTPStatus::Found, "/auth")
        when "/loop/a"
          res.set_redirect(WEBrick::HTTPStatus::Found, "/loop/b")
        when "/loop/b"
          res.set_redirect(WEBrick::HTTPStatus::Found, "/loop/a")
        when "/forbidden"
          res.set_redirect(WEBrick::HTTPStatus::Found, "file:///etc/hosts")
        when "/boom"
          res.status = 500
          res.body = "boom"
        else
          res.status = 404
          res.body = "no such page"
        end
      end
      server
    end

    def test_block_and_blockless
      server = net_server
      thread = Thread.new { server.start }
      port = server.config[:Port]
      base = "http://127.0.0.1:#{port}"

      lines = [] #: Array[String]
      got = URI.open("#{base}/hello") do |f|
        assert_equal ["200", "OK"], f.status
        assert_equal "text/plain", f.content_type
        assert_equal "utf-8", f.charset
        assert_equal "/hello", f.base_uri&.path
        assert_equal "hello\n", f.gets
        f.each_line { |l| lines << l }
        assert_equal true, f.eof?
        f.rewind
        assert_equal ["hello\n", "world\n"], f.readlines
        "done"
      end
      assert_equal "done", got
      assert_equal ["world\n"], lines

      io = URI.open("#{base}/hello")
      assert_equal "hello\nworld\n", io.read
      assert_equal StringIO, io.class
      assert_equal true, io.is_a?(StringIO)
      io.close

      assert_equal "hello\nworld\n", URI.open("#{base}/hello", "r", &:read)
      assert_equal "hello\nworld\n", URI.open("#{base}/hello", "rb").read
      server.shutdown
      thread.join
    end

    def test_headers_and_options
      server = net_server
      thread = Thread.new { server.start }
      port = server.config[:Port]
      base = "http://127.0.0.1:#{port}"

      assert_equal "Ruby|", URI.open("#{base}/echo").read
      assert_equal "rb2go|t1", URI.open("#{base}/echo", "User-Agent" => "rb2go", "X-Token" => "t1").read
      assert_equal "rb2go|", URI.open("#{base}/echo", "r", "User-Agent" => "rb2go", read_timeout: 5, open_timeout: 5).read
      headers = { "X-Token" => "t2" }
      assert_equal "Ruby|t2", URI.open("#{base}/echo", headers).read
      assert_equal "Ruby|t4", URI.open("#{base}/echo", { "X-Token" => "t4", redirect: true }).read
      assert_equal "Ruby|", URI.open("#{base}/echo", ssl_verify_mode: OpenSSL::SSL::VERIFY_NONE).read

      assert_equal "welcome", URI.open("#{base}/auth", http_basic_authentication: ["alice", "secret"]).read
      # dropped on redirect, as MRI does
      begin
        URI.open("#{base}/auth-redirect", http_basic_authentication: ["alice", "secret"])
        flunk "no error"
      rescue OpenURI::HTTPError => e
        assert_equal "401 Unauthorized", e.message
      end

      lengths = [] #: Array[Integer?]
      sizes = [] #: Array[Integer]
      URI.open("#{base}/hello",
               content_length_proc: ->(n) { lengths << n },
               progress_proc: ->(n) { sizes << n }) { |f| f.read }
      assert_equal [12], lengths
      assert_equal [12], sizes
      server.shutdown
      thread.join
    end

    def test_meta
      server = net_server
      thread = Thread.new { server.start }
      port = server.config[:Port]
      base = "http://127.0.0.1:#{port}"

      URI.open("#{base}/meta") do |f|
        assert_equal "text/html", f.content_type
        assert_equal "iso-8859-1", f.charset
        assert_equal ["x-custom"], f.content_encoding
        assert_equal "2022-01-01 10:20:30 UTC", f.last_modified.to_s
        assert_equal "a", f.meta["x-thing"]
        assert_equal "a=1, b=2", f.meta["set-cookie"]
        assert_equal ["a=1", "b=2"], f.metas["set-cookie"]
        assert_equal ["a"], f.metas["x-thing"]
        assert_equal "Text/HTML; Charset=\"ISO-8859-1\"", f.meta["content-type"]
      end
      URI.open("#{base}/bin") do |f|
        assert_equal "application/octet-stream", f.content_type
        assert_nil f.charset
        assert_equal "fallback", f.charset { "fallback" }
        assert_equal [], f.content_encoding
        assert_nil f.last_modified
        assert_equal "\x00\x01", f.read
      end
      URI.open("#{base}/bare") do |f|
        assert_equal "application/octet-stream", f.content_type
        assert_nil f.charset
      end
      server.shutdown
      thread.join
    end

    def test_errors_and_redirects
      server = net_server
      thread = Thread.new { server.start }
      port = server.config[:Port]
      base = "http://127.0.0.1:#{port}"

      begin
        URI.open("#{base}/missing")
        flunk "no error"
      rescue OpenURI::HTTPError => e
        assert_equal "404 Not Found", e.message
        assert_equal ["404", "Not Found"], e.io.status
        assert_equal "no such page", e.io.read
        assert_nil e.io.base_uri
      end

      begin
        URI.open("#{base}/boom")
        flunk "no error"
      rescue OpenURI::HTTPError => boom
        assert_equal "500 Internal Server Error", boom.message
        assert_equal true, boom.is_a?(StandardError)
      end

      URI.open("#{base}/moved") do |f|
        assert_equal "/hello", f.base_uri&.path
        assert_equal ["200", "OK"], f.status
        assert_equal "hello\nworld\n", f.read
      end

      begin
        URI.open("#{base}/redirect", redirect: false)
        flunk "no error"
      rescue OpenURI::HTTPRedirect => redir
        assert_equal "302 Found", redir.message
        assert_equal "/hello", redir.uri.path
        assert_equal ["302", "Found"], redir.io.status
        assert_equal true, redir.is_a?(OpenURI::HTTPError)
      end

      begin
        URI.open("#{base}/moved", max_redirects: 1)
        flunk "no error"
      rescue OpenURI::TooManyRedirects => many
        assert_equal "Too many redirects", many.message
        assert_equal ["302", "Found"], many.io.status
      end

      begin
        URI.open("#{base}/loop/a")
        flunk "no error"
      rescue RuntimeError => loop_err
        assert_equal "HTTP redirection loop: http://127.0.0.1:PORT/loop/b", net_unport(loop_err.message, port)
      end

      begin
        URI.open("#{base}/forbidden")
        flunk "no error"
      rescue RuntimeError => forbidden
        assert_equal "redirection forbidden: http://127.0.0.1:PORT/forbidden -> file:///etc/hosts", net_unport(forbidden.message, port)
      end
      server.shutdown
      thread.join
    end

    def test_uri_open_and_read
      server = net_server
      thread = Thread.new { server.start }
      port = server.config[:Port]

      uri = URI("http://127.0.0.1:#{port}/echo")
      assert_equal "Ruby|", uri.read
      assert_equal "x|", uri.read("User-Agent" => "x")
      assert_equal "Ruby|", uri.open(&:read)
      io = uri.open("X-Token" => "t3")
      assert_equal "Ruby|t3", io.read
      assert_equal "/echo", io.base_uri&.path
      assert_equal "Ruby|", URI.open(uri).read
      server.shutdown
      thread.join
    end

    def test_local_file_and_mode
      Dir.mktmpdir do |dir|
        path = File.join(dir, "local.txt")
        File.write(path, "one\ntwo\n")
        assert_equal "one\ntwo\n", URI.open(path, &:read)
        f = URI.open(path)
        assert_equal "one\n", f.gets
        f.close
      end

      begin
        StringIO.new("x").last_modified
        flunk "no error"
      rescue NoMethodError => nm
        assert_equal "undefined method 'last_modified' for an instance of StringIO", nm.message
      end

      # the redirect check prints the target, so an empty authority must survive to_s
      assert_equal "", URI("file:///etc/hosts").host
      assert_equal "file:///etc/hosts", URI("file:///etc/hosts").to_s

      mode = "w"
      begin
        URI.open("http://127.0.0.1:1/x", mode)
        flunk "no error"
      rescue ArgumentError => e
        assert_equal "invalid access mode w (URI::HTTP resource is read only.)", e.message
      end
    end
  end

  # Rackup::Handler::WEBrick (decision 174): what a Rack app sees in env, and how its response goes out.
  class RackTest < Minitest::Test
    #: (untyped) { (Net::HTTP, Integer) -> void } -> void
    def net_rack(app)
      ready = Queue.new #: Queue[WEBrick::HTTPServer]
      thread = Thread.new { Rackup::Handler::WEBrick.run(app, Host: "127.0.0.1", Port: 0) { |s| ready << s } }
      server = ready.pop || raise("the server did not start")
      port = server.config[:Port] #: Integer
      yield Net::HTTP.new("127.0.0.1", port), port
      Rackup::Handler::WEBrick.shutdown
      thread.join
    end

    def test_env
      keys = %w[REQUEST_METHOD SCRIPT_NAME PATH_INFO QUERY_STRING SERVER_NAME SERVER_PROTOCOL CONTENT_TYPE CONTENT_LENGTH HTTP_X_TOKEN rack.url_scheme]
      app = ->(env) { [200, { "content-type" => "text/plain" }, [keys.map { |k| env[k].inspect }.join(" "), " #{env["SERVER_PORT"]} #{env["HTTP_HOST"]} #{env.fetch("rack.input").read.inspect} #{env["rack.errors"] == $stderr}"]] } #: ^(Hash[String, untyped]) -> untyped
      net_rack(app) do |http, port|
        body = http.get("/a%20b/c").body.to_s.gsub(port.to_s, "PORT")
        assert_equal "\"GET\" \"\" \"/a%20b/c\" \"\" \"127.0.0.1\" \"HTTP/1.1\" nil nil nil \"http\" PORT 127.0.0.1:PORT \"\" true", body
        res = http.post("/f?x=1&y", "k=v w", { "Content-Type" => "application/x-www-form-urlencoded", "X-Token" => "t" })
        body = res.body.to_s.gsub(port.to_s, "PORT")
        assert_equal "\"POST\" \"\" \"/f\" \"x=1&y\" \"127.0.0.1\" \"HTTP/1.1\" \"application/x-www-form-urlencoded\" \"5\" \"t\" \"http\" PORT 127.0.0.1:PORT \"k=v w\" true", body
      end
    end

    def test_response
      app = lambda do |env|
        case env["PATH_INFO"]
        when "/boom" then raise "boom"
        when "/rack" then [204, { "rack.hidden" => "x", "x-multi" => ["a", "b"] }, []]
        else [418, { "set-cookie" => ["a=1", "b=2"], "content-length" => "2" }, ["hi"]]
        end
      end #: ^(Hash[String, untyped]) -> untyped
      net_rack(app) do |http, _|
        res = http.get("/")
        assert_equal ["418", "hi", ["a=1", "b=2"], "2", nil], [res.code, res.body, res.get_fields("set-cookie"), res["content-length"], res["content-type"]]
        res = http.head("/")
        assert_equal ["418", "", "2"], [res.code, res.body.to_s, res["content-length"]] # MRI's body is nil, rb2go's Net::HTTP ""
        res = http.get("/rack")
        assert_equal ["204", "", nil, "a, b"], [res.code, res.body.to_s, res["rack.hidden"], res["x-multi"]]
        assert_equal "500", http.get("/boom").code
      end
    end
  end
end
