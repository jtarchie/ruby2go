//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
	Hash_Op_idxSet[Symbol, any](config, Symbol("Port"), Integer(ln.Addr().(*net.TCPAddr).Port))
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return &WEBrick_HTTPServer{srv: srv, ln: ln, mux: mux, config: config}
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
	res := &WEBrick_HTTPResponse{status: 200, header: http.Header{}, cookies: NewArray[WEBrick_CookieI]()}
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
	for _, ck := range *res.cookies {
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
