# prelude/webrick.rb
# rbs_inline: enabled
#
# WEBrick's server on Go's net/http: one goroutine per request instead of
# one thread. Behaviour kept from WEBrick because programs observe it: no
# Content-Type unless set (Go would sniff one), form bodies parsed into
# `query` only for form content types, mount_proc answering only GET, HEAD,
# POST and PUT, relative Location headers made absolute, and HTTPStatus
# exceptions becoming their status code.

module WEBrick
  # @go_type struct { r *http.Request; query *Hash[String, String]; body *String }
  class HTTPRequest < Object
    #: () -> String
    def request_method = %x{ String(self.r.Method) }

    #: () -> String
    def path = %x{ String(self.r.URL.Path) }

    #: () -> String?
    def query_string = %x{
      if self.r.URL.RawQuery == "" {
        return nil
      }
      return Ref(String(self.r.URL.RawQuery))
    }

    #: () -> Hash[String, String]
    def query = %x{ self.query }

    #: () -> String?
    def body = %x{ self.body }

    #: (String) -> String?
    def [](name) = %x{
      values := self.r.Header.Values(string(name))
      if len(values) == 0 {
        return nil
      }
      return Ref(String(strings.Join(values, ", ")))
    }
  end

  # @go_type struct { status Integer; body String; header http.Header }
  class HTTPResponse < Object
    #: () -> Integer
    def status = %x{ self.status }

    #: (Integer) -> void
    def status=(status)
      %x{ self.status = status }
    end

    #: () -> String
    def body = %x{ self.body }

    #: (String) -> void
    def body=(body)
      %x{ self.body = body }
    end

    #: (String) -> String?
    def [](name) = %x{
      values := self.header.Values(string(name))
      if len(values) == 0 {
        return nil
      }
      return Ref(String(strings.Join(values, ", ")))
    }

    #: (String, String) -> void
    def []=(name, value)
      %x{ self.header.Set(string(name), string(value)) }
    end

    #: (String) -> void
    def content_type=(value)
      self["Content-Type"] = value
    end

    #: () -> String?
    def content_type = self["Content-Type"]
  end

  # @go_type struct { srv *http.Server; ln net.Listener; mux *http.ServeMux; config *Hash[Symbol, any] }
  class HTTPServer < Object
    # Listens immediately, like WEBrick; with Port: 0 the chosen port is
    # written back into config[:Port].
    #: (?Hash[Symbol, untyped]) -> HTTPServer
    def self.new(config = {}) = %x{ return rbWEBrickNew(config) }

    #: () -> Hash[Symbol, untyped]
    def config = %x{ self.config }

    #: (Symbol) -> untyped
    def [](key) = config[key]

    #: (String) { (HTTPRequest, HTTPResponse) -> void } -> void
    def mount_proc(dir) = %x{
      rbWEBrickMount(self.mux, string(dir), func(w http.ResponseWriter, r *http.Request) {
        rbWEBrickServe(w, r, func(req *WEBrick_HTTPRequest, res *WEBrick_HTTPResponse) {
          switch r.Method {
          case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut:
            blk(req, res)
          default:
            panic(NewWEBrick_HTTPStatus_MethodNotAllowed(Ref(String("unsupported method '" + r.Method + "'."))))
          }
        })
      })
    }

    # A new servlet instance per request, as WEBrick does.
    #: (String, singleton(HTTPServlet::AbstractServlet)) -> void
    def mount(dir, servlet) = %x{
      rbWEBrickMount(self.mux, string(dir), func(w http.ResponseWriter, r *http.Request) {
        rbWEBrickServe(w, r, func(req *WEBrick_HTTPRequest, res *WEBrick_HTTPResponse) {
          inst := any(servlet).(interface {
            New(*WEBrick_HTTPServer) WEBrick_HTTPServlet_AbstractServletI
          }).New(self)
          inst.Service(req, res)
        })
      })
    }

    # Serves until shutdown.
    #: () -> void
    def start = %x{
      if err := self.srv.Serve(self.ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
        panic(NewIOError(Ref(String(err.Error()))))
      }
    }

    #: () -> void
    def shutdown = %x{ _ = self.srv.Close() }
  end

  module HTTPStatus
    class Status < StandardError
      #: () -> Integer
      def code = 500
    end

    class NotFound < Status
      def code = 404
    end

    class MethodNotAllowed < Status
      def code = 405
    end
  end

  module HTTPServlet
    class AbstractServlet
      attr_reader :server #: HTTPServer

      #: (HTTPServer) -> void
      def initialize(server)
        @server = server
      end

      #: (HTTPRequest, HTTPResponse) -> void
      def service(req, res)
        case req.request_method
        when "GET" then do_GET(req, res)
        when "HEAD" then do_HEAD(req, res)
        when "POST" then do_POST(req, res)
        when "PUT" then do_PUT(req, res)
        when "DELETE" then do_DELETE(req, res)
        when "PATCH" then do_PATCH(req, res)
        when "OPTIONS" then do_OPTIONS(req, res)
        else unsupported(req)
        end
      end

      #: (HTTPRequest, HTTPResponse) -> void
      def do_GET(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_HEAD(req, res) = do_GET(req, res)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_POST(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_PUT(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_DELETE(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_PATCH(req, res) = unsupported(req)

      #: (HTTPRequest, HTTPResponse) -> void
      def do_OPTIONS(req, res) = unsupported(req)

      private

      #: (HTTPRequest) -> void
      def unsupported(req) = raise(HTTPStatus::MethodNotAllowed, "unsupported method '#{req.request_method}'.")
    end
  end
end
