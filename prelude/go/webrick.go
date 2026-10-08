//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

func rbWEBrickListen(config *Hash[Symbol, any]) *WEBrick_HTTPServer_Handle__ {
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
	Hash_Op_idxSet[Symbol, any](config, Symbol("Port"), Integer(ln.Addr().(*net.TCPAddr).Port))
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return &WEBrick_HTTPServer_Handle__{srv: srv, ln: ln, mux: mux}
}

// rbWEBrickParseCookies is WEBrick::HTTPRequest#cookies: the Cookie header split on ";", trimmed, name=value pairs kept in order.
func rbWEBrickParseCookies(raw string) *Array[WEBrick_CookieI] {
	out := NewArray[WEBrick_CookieI]()
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		Array_Push[WEBrick_CookieI](out, NewWEBrick_Cookie(String(strings.TrimSpace(k)), String(v)))
	}
	return out
}

// rbWEBrickServlet is a mounted handler; the mux only looks it up (rbWEBrickRoute), HTTPServer#service runs it.
type rbWEBrickServlet func(*WEBrick_HTTPRequest, *WEBrick_HTTPResponse)

func (rbWEBrickServlet) ServeHTTP(http.ResponseWriter, *http.Request) {}

// rbWEBrickMount registers h for dir and everything below it.
func rbWEBrickMount(mux *http.ServeMux, dir string, h func(*WEBrick_HTTPRequest, *WEBrick_HTTPResponse)) {
	dir = "/" + strings.Trim(dir, "/")
	if dir == "/" {
		mux.Handle("/", rbWEBrickServlet(h))
		return
	}
	mux.Handle(dir, rbWEBrickServlet(h))
	mux.Handle(dir+"/", rbWEBrickServlet(h))
}

// rbWEBrickRoute is HTTPServer#service: the servlet mounted for the path, else NotFound as WEBrick raises.
func rbWEBrickRoute(mux *http.ServeMux, req *WEBrick_HTTPRequest, res *WEBrick_HTTPResponse) {
	h, _ := mux.Handler(req.r)
	s, ok := h.(rbWEBrickServlet)
	if !ok {
		panic(NewWEBrick_HTTPStatus_NotFound(Ref(String("`" + req.r.URL.Path + "' not found."))))
	}
	s(req, res)
}

// rbWEBrickURI is WEBrick's request_uri: absolute, from the Host header.
func rbWEBrickURI(r *http.Request) string {
	return "http://" + r.Host + r.RequestURI
}

// rbWEBrickMetaVars is HTTPRequest#meta_vars in WEBrick's key order; Go's Header map has lost arrival order, so HTTP_* keys are sorted.
func rbWEBrickMetaVars(r *http.Request) *Hash[String, any] {
	env := NewHash[String, any]()
	set := func(k, v string) { Hash_Op_idxSet[String, any](env, String(k), String(v)) }
	if r.ContentLength > 0 {
		set("CONTENT_LENGTH", strconv.FormatInt(r.ContentLength, 10))
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		set("CONTENT_TYPE", ct)
	}
	set("GATEWAY_INTERFACE", "CGI/1.1")
	path, query, _ := strings.Cut(r.RequestURI, "?")
	if p, err := url.PathUnescape(path); err == nil {
		path = p
	}
	set("PATH_INFO", path)
	set("QUERY_STRING", query)
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		set("REMOTE_ADDR", ip)
		set("REMOTE_HOST", ip) // WEBrick's DoNotReverseLookup
	}
	Hash_Op_idxSet[String, any](env, "REMOTE_USER", nil)
	set("REQUEST_METHOD", r.Method)
	set("REQUEST_URI", rbWEBrickURI(r))
	set("SCRIPT_NAME", "")
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host, port = r.Host, "80"
	}
	set("SERVER_NAME", host)
	set("SERVER_PORT", port)
	set("SERVER_PROTOCOL", "HTTP/1.1") // the server's HTTPVersion, as WEBrick's
	set("HTTP_HOST", r.Host)           // Go moves Host out of r.Header
	for _, k := range slices.Sorted(maps.Keys(r.Header)) {
		if k != "Content-Type" && k != "Content-Length" {
			set("HTTP_"+strings.ToUpper(strings.ReplaceAll(k, "-", "_")), strings.Join(r.Header.Values(k), ", "))
		}
	}
	return env
}

func rbParseQuery(h *Hash[String, String], q string) {
	for _, part := range strings.FieldsFunc(q, func(c rune) bool { return c == '&' || c == ';' }) {
		k, v, _ := strings.Cut(part, "=")
		k, _ = url.QueryUnescape(k)
		v, _ = url.QueryUnescape(v)
		if _, seen := h.vals[String(k)]; !seen {
			Hash_Op_idxSet(h, String(k), String(v))
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

// rbWEBrickServe runs a handler and writes its response like WEBrick; a raised HTTPStatus keeps the body/header already set (set_redirect relies on this).
func rbWEBrickServe(w http.ResponseWriter, r *http.Request, handle func(*WEBrick_HTTPRequest, *WEBrick_HTTPResponse)) {
	req := rbWEBrickRequest(r)
	res := &WEBrick_HTTPResponse{status: 200, header: http.Header{}, cookies: NewArray[any]()}
	func() {
		defer func() {
			if p := recover(); p != nil {
				p = rbWrapPanic(p)
				if st, ok := p.(WEBrick_HTTPStatus_StatusI); ok {
					res.status = st.Code()
				} else {
					res.status, res.body, res.header = 500, "", http.Header{}
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
	for _, ck := range res.cookies.s {
		h.Add("Set-Cookie", string(rbToS(ck)))
	}
	if _, ok := h["Content-Type"]; !ok {
		h["Content-Type"] = nil // WEBrick sends none; stop Go sniffing one
	}
	w.WriteHeader(int(res.status))
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, string(res.body))
	}
}
