# prelude/webrick.rb
# rbs_inline: enabled
#
# WEBrick's server on Go's net/http: one goroutine per request instead of
# one thread. Behaviour kept from WEBrick because programs observe it: no
# Content-Type unless set (Go would sniff one), form bodies parsed into
# `query` only for form content types, mount_proc answering only GET, HEAD,
# POST and PUT, relative Location headers made absolute, and HTTPStatus
# exceptions becoming their status code.

%x{
  func rbWEBrickNew(config *Hash[Symbol, any]) *WEBrick_HTTPServer {
    port := 0
    if p, ok := config.vals[Symbol("Port")]; ok && p != nil {
      port = int(p.(Integer))
    }
    bind := ""
    if b, ok := config.vals[Symbol("BindAddress")]; ok && b != nil {
      bind = string(b.(String))
    }
    ln, err := net.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(port)))
    if err != nil {
      panic(NewIOError(Ref(String(err.Error()))))
    }
    config.IdxSet(Symbol("Port"), Integer(ln.Addr().(*net.TCPAddr).Port))
    mux := http.NewServeMux()
    srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
    return &WEBrick_HTTPServer{srv: srv, ln: ln, mux: mux, config: config}
  }

  // rbWEBrickMount registers h for dir and everything below it.
  func rbWEBrickMount(mux *http.ServeMux, dir string, h http.HandlerFunc) {
    dir = "/" + strings.Trim(dir, "/")
    if dir == "/" {
      mux.HandleFunc("/", h)
      return
    }
    mux.HandleFunc(dir, h)
    mux.HandleFunc(dir+"/", h)
  }

  func rbParseQuery(h *Hash[String, String], q string) {
    for _, part := range strings.FieldsFunc(q, func(c rune) bool { return c == '&' || c == ';' }) {
      k, v, _ := strings.Cut(part, "=")
      k, _ = url.QueryUnescape(k)
      v, _ = url.QueryUnescape(v)
      if _, seen := h.vals[String(k)]; !seen {
        h.IdxSet(String(k), String(v))
      }
    }
  }

  func rbWEBrickRequest(r *http.Request) *WEBrick_HTTPRequest {
    var body *String
    if data, err := io.ReadAll(r.Body); err == nil && len(data) > 0 {
      s := String(data)
      body = &s
    }
    query := NewHash[String, String]()
    switch {
    case r.Method == http.MethodGet || r.Method == http.MethodHead:
      rbParseQuery(query, r.URL.RawQuery)
    case body != nil && strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded"):
      rbParseQuery(query, string(*body))
    }
    return &WEBrick_HTTPRequest{r: r, query: query, body: body}
  }

  // rbWEBrickServe runs a handler and writes its response like WEBrick.
  func rbWEBrickServe(w http.ResponseWriter, r *http.Request, handle func(*WEBrick_HTTPRequest, *WEBrick_HTTPResponse)) {
    req := rbWEBrickRequest(r)
    res := &WEBrick_HTTPResponse{status: 200, header: http.Header{}}
    func() {
      defer func() {
        if p := recover(); p != nil {
          p = rbWrapPanic(p)
          res.status, res.body, res.header = 500, "", http.Header{}
          if st, ok := p.(WEBrick_HTTPStatus_StatusI); ok {
            res.status = st.Code()
          } else {
            fmt.Fprintln(os.Stderr, "ERROR", rbToS(p), "("+rbClassName(p)+")")
          }
        }
      }()
      handle(req, res)
    }()
    if loc := res.header.Get("Location"); loc != "" {
      if u, err := url.Parse(loc); err == nil && !u.IsAbs() {
        base := &url.URL{Scheme: "http", Host: r.Host, Path: r.URL.Path}
        res.header.Set("Location", base.ResolveReference(u).String())
      }
    }
    h := w.Header()
    for k, v := range res.header {
      h[k] = v
    }
    if _, ok := h["Content-Type"]; !ok {
      h["Content-Type"] = nil // WEBrick sends none; stop Go sniffing one
    }
    w.WriteHeader(int(res.status))
    if r.Method != http.MethodHead {
      _, _ = io.WriteString(w, string(res.body))
    }
  }
}

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
