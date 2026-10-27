# rbs_inline: enabled

# Stands in for the blog's runtime/tep/ (Spinel's HTTP server): the slice of Tep the blog calls, on TCPServer (README.md).
require "socket"
require_relative "../blog/runtime/tep/url"
require_relative "../blog/runtime/tep/parser"

module Sock
  #: (String) -> Integer
  def self.sphttp_filesize(path) = File.exist?(path) ? File.size(path) : -1
end

module Tep
  #: () -> Hash[String, String]
  def self.str_hash = Hash.new("")

  class Request
    attr_accessor :verb #: String
    attr_accessor :path #: String
    attr_accessor :raw_path #: String
    attr_accessor :http_version #: String
    attr_accessor :req_params #: Hash[String, String]
    attr_accessor :query #: Hash[String, String]
    attr_accessor :req_headers #: Hash[String, String]
    attr_accessor :cookies #: Hash[String, String]
    attr_accessor :raw_body #: String
    attr_accessor :remote_host #: String
    attr_accessor :uploads #: Hash[String, untyped]

    #: () -> void
    def initialize
      @verb = ""
      @path = ""
      @raw_path = ""
      @http_version = "HTTP/1.1"
      @req_params = Tep.str_hash
      @query = Tep.str_hash
      @req_headers = Tep.str_hash
      @cookies = Tep.str_hash
      @raw_body = ""
      @remote_host = ""
      @uploads = {}
    end

    #: () -> String?
    def host = @req_headers["host"]

    #: () -> String?
    def user_agent = @req_headers["user-agent"]

    #: () -> String?
    def referer = @req_headers["referer"]

    #: () -> String?
    def accept = @req_headers["accept"]

    #: () -> String?
    def content_type = @req_headers["content-type"]
  end

  class Response
    attr_accessor :status #: Integer
    attr_accessor :headers #: Hash[String, String]
    attr_accessor :body #: String
    attr_accessor :file_path #: String
    attr_accessor :set_cookies #: Array[String]
    attr_accessor :upgrading_ws #: bool

    #: () -> void
    def initialize
      @status = 200
      @headers = Tep.str_hash
      @body = ""
      @file_path = ""
      @set_cookies = []
      @upgrading_ws = false
    end

    #: (String, String, Hash[String, String]) -> void
    def set_cookie(name, value, opts)
      line = name + "=" + Url.escape(value)
      opts.each { |k, v| line += v.empty? ? "; " + k : "; " + k + "=" + v }
      @set_cookies.push(line)
    end

    #: (String) -> void
    def send_file(path)
      @file_path = path
      @body = ""
    end

    #: (String, WebSocket::Driver) -> void
    def start_websocket(accept_key, driver)
      @upgrading_ws = true
    end
  end

  # Action Cable's half of Tep, without a WebSocket transport yet: every upgrade is refused, so nothing subscribes.
  module WebSocket
    class Event
      attr_accessor :data #: String
      attr_accessor :code #: Integer
      attr_accessor :reason #: String

      #: () -> void
      def initialize
        @data = ""
        @code = 0
        @reason = ""
      end
    end

    class Handler
      attr_accessor :req #: Request

      #: () -> void
      def initialize
        @req = Request.new
      end

      #: (Event) -> Integer
      def handle_event(event) = 0
    end

    class Driver
      attr_accessor :fd #: Integer
      attr_accessor :subprotocol #: String
      attr_accessor :h_open #: Handler
      attr_accessor :h_message #: Handler
      attr_accessor :h_close #: Handler

      #: (Integer) -> void
      def initialize(fd)
        @fd = fd
        @subprotocol = ""
        @h_open = Handler.new
        @h_message = Handler.new
        @h_close = Handler.new
      end

      #: (String) -> void
      def set_subprotocol(name) = @subprotocol = name

      #: (Handler) -> void
      def set_on_open(h) = @h_open = h

      #: (Handler) -> void
      def set_on_message(h) = @h_message = h

      #: (Handler) -> void
      def set_on_close(h) = @h_close = h

      #: (String) -> Integer
      def text(s) = 0
    end

    class Handshake
      class Result
        attr_accessor :valid #: bool
        attr_accessor :reason #: String
        attr_accessor :accept_key #: String
        attr_accessor :protocols #: Array[String]

        #: () -> void
        def initialize
          @valid = false
          @reason = "WebSocket is not served by the rugaroo Tep shim yet"
          @accept_key = ""
          @protocols = []
        end
      end

      #: (Request) -> Result
      def self.check(req) = Result.new
    end
  end

  module Broadcast
    #: (String, String) -> Integer
    def self.publish(topic, payload) = 0

    #: (String, WebSocket::Driver) -> Integer
    def self.subscribe_ws(topic, ws) = 0
  end

  class App
    attr_accessor :name #: String
    attr_accessor :cable_identifiers #: Hash[String, String]
    attr_accessor :cable_lock #: Mutex
    attr_accessor :cable_conns #: Array[WebSocket::Driver]
    attr_accessor :cable_conns_lock #: Mutex
    attr_accessor :cable_heartbeat #: Integer

    #: () -> void
    def initialize
      @name = ""
      @cable_identifiers = Tep.str_hash
      @cable_lock = Mutex.new
      @cable_conns = []
      @cable_conns_lock = Mutex.new
      @cable_heartbeat = 0
    end

    #: (Request, Response) -> void
    def dispatch(req, res)
      Db.with_connection { Main.dispatch(req, res) }
    end
  end

  APP = App.new

  LISTEN_HOOKS = [] #: Array[^(Integer) -> void]

  #: (^(Integer) -> void) -> void
  def self.on_listen(hook) # the server then binds a free port, runs the hook with it and stops: an in-process client
    LISTEN_HOOKS << hook
  end

  REASONS = { 200 => "OK", 201 => "Created", 204 => "No Content", 301 => "Moved Permanently", 302 => "Found",
              303 => "See Other", 304 => "Not Modified", 400 => "Bad Request", 401 => "Unauthorized", 403 => "Forbidden",
              404 => "Not Found", 422 => "Unprocessable Content", 500 => "Internal Server Error", 501 => "Not Implemented" } #: Hash[Integer, String]

  #: (TCPSocket) -> Request?
  def self.read_request(io)
    blob = ""
    while blob.index("\r\n\r\n").nil?
      blob += io.readpartial(65_536).force_encoding("UTF-8")
    end
    req = Parser.parse(blob)
    return nil if req.nil?
    length = req.req_headers.fetch("content-length", "0").to_i
    req.raw_body += io.readpartial(length - req.raw_body.bytesize).force_encoding("UTF-8") while req.raw_body.bytesize < length
    req.remote_host = io.peeraddr[3].to_s
    type = req.req_headers.fetch("content-type", "")
    if type.start_with?("application/x-www-form-urlencoded")
      Url.parse_query(req.raw_body).each { |k, v| req.req_params[k] = v }
    elsif type.start_with?("multipart/form-data")
      form = ActionDispatch::Http::Multipart.parse(req.raw_body, type)
      form.fields.each { |k, v| req.req_params[k] = v }
      form.files.each { |k, v| req.uploads[k] = v }
    end
    req
  rescue EOFError
    nil
  end

  #: (TCPSocket, Response, bool) -> void
  def self.write_response(io, res, keep_alive)
    if res.upgrading_ws
      res.status = 501
      res.body = "WebSocket is not served by the rugaroo Tep shim yet"
    end
    body = res.file_path.empty? ? res.body : File.binread(res.file_path)
    res.headers["Content-Type"] = "text/html; charset=utf-8" if !body.empty? && !res.headers.key?("Content-Type")
    head = "HTTP/1.1 " + res.status.to_s + " " + REASONS.fetch(res.status, "") + "\r\n"
    res.headers.each { |k, v| head += k + ": " + v + "\r\n" }
    res.set_cookies.each { |line| head += "Set-Cookie: " + line + "\r\n" }
    io.write(head + "Content-Length: " + body.bytesize.to_s + "\r\nConnection: " + (keep_alive ? "keep-alive" : "close") + "\r\n\r\n")
    io.write(body)
  end

  #: (TCPSocket) -> void
  def self.serve_connection(io) # HTTP/1.1 keeps the connection for the next request, as tep does, unless a side asks to close
    loop do
      req = read_request(io)
      break if req.nil?
      keep = req.http_version == "HTTP/1.1" && req.req_headers.fetch("connection", "").downcase != "close"
      res = Response.new
      APP.dispatch(req, res)
      write_response(io, res, keep)
      break unless keep
    end
  ensure
    io.close
  end

  #: (TCPServer) -> void
  def self.accept_loop(server)
    loop { Thread.new(server.accept) { |io| serve_connection(io) } }
  rescue IOError
    nil
  end

  module Server
    class Threaded
      #: (untyped) -> void
      def initialize(app)
        @app = app
      end

      #: (Integer, Integer, bool) -> void
      def run(port, workers, reuse)
        hook = LISTEN_HOOKS.first
        if hook.nil?
          Tep.accept_loop(TCPServer.new("0.0.0.0", port))
          return
        end
        server = TCPServer.new("127.0.0.1", 0)
        thread = Thread.new { Tep.accept_loop(server) }
        begin
          hook.call(server.addr[1].to_i)
        ensure
          server.close
          thread.join
        end
      end
    end

    class Scheduled < Threaded
    end
  end
end
