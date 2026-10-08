//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"fmt"
	"io"
	"iter"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
)

// rbRackHandler serves every path and method through app, as rackup's WEBrick::Server#service bypasses the mount table.
func rbRackHandler(app func(*Hash[String, any], String) any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		input, _ := io.ReadAll(r.Body)
		var res any
		func() {
			defer func() {
				if p := recover(); p != nil {
					p = rbWrapPanic(p)
					res = nil
					fmt.Fprintln(os.Stderr, "ERROR", rbToS(p), "("+rbClassName(p)+")")
				}
			}()
			res = app(rbRackEnv(r), String(input))
		}()
		rbRackWrite(w, r, res)
	}
}

// rbRackEnv is the Rack 3 env in WEBrick#meta_vars' order; Go's Header map has lost arrival order, so HTTP_* keys are sorted.
func rbRackEnv(r *http.Request) *Hash[String, any] {
	env := NewHash[String, any]()
	set := func(k, v string) { Hash_Op_idxSet[String, any](env, String(k), String(v)) }
	if r.ContentLength > 0 {
		set("CONTENT_LENGTH", strconv.FormatInt(r.ContentLength, 10))
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		set("CONTENT_TYPE", ct)
	}
	path, query, _ := strings.Cut(r.RequestURI, "?")
	set("PATH_INFO", path)
	set("QUERY_STRING", query)
	if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		set("REMOTE_ADDR", ip)
	}
	set("REQUEST_METHOD", r.Method)
	set("REQUEST_URI", r.RequestURI)
	set("SCRIPT_NAME", "")
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host, port = r.Host, "80"
	}
	set("SERVER_NAME", host)
	set("SERVER_PORT", port)
	set("SERVER_PROTOCOL", r.Proto)
	set("HTTP_HOST", r.Host) // Go moves Host out of r.Header
	for _, k := range slices.Sorted(maps.Keys(r.Header)) {
		if k != "Content-Type" && k != "Content-Length" {
			set("HTTP_"+strings.ToUpper(strings.ReplaceAll(k, "-", "_")), strings.Join(r.Header.Values(k), ", "))
		}
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	set("rack.url_scheme", scheme)
	set("REQUEST_PATH", path)
	return env
}

// rbRackProc is a lambda app as the one Proc type a Rack app can have; nil for an object, called through dynamic dispatch.
func rbRackProc(app any) **func(*Hash[String, any]) any {
	if f, ok := app.(*func(*Hash[String, any]) any); ok {
		return &f
	}
	return nil
}

// rbRackChunks is a Rack body's parts: an Array's elements, or what an each typed `() { (String) -> void } -> void` yields.
func rbRackChunks(body any) *Array[String] {
	out := &Array[String]{}
	switch b := body.(type) {
	case Array_Any:
		for _, x := range b._ToAny().s {
			out.s = append(out.s, rbToS(x))
		}
	case interface{ Each() iter.Seq[String] }:
		for x := range b.Each() {
			out.s = append(out.s, x)
		}
	case interface{ Each(blk func(String)) }:
		b.Each(func(x String) { out.s = append(out.s, x) })
	default:
		panic(NewNoMethodError(Ref(String("undefined method 'each' for an instance of " + rbClassName(body)))))
	}
	return out
}

// rbRackWrite sends [status, headers, chunks] as rackup's handler does: set-cookie one line per value, others joined, Content-Length always.
func rbRackWrite(w http.ResponseWriter, r *http.Request, res any) {
	var parts []any
	if a, ok := res.(Array_Any); ok {
		parts = a._ToAny().s
	}
	var status Integer
	if len(parts) == 3 {
		status, _ = rbUnbox(parts[0]).(Integer)
	}
	if status < 100 || status > 999 {
		if res != nil {
			fmt.Fprintln(os.Stderr, "ERROR", "invalid Rack response", rbInspect(res))
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	h := w.Header()
	if hs, ok := parts[1].(Hash_Any); ok {
		hh := hs._ToAny()
		for _, k := range hh.keys {
			key := string(rbToS(k))
			if strings.HasPrefix(key, "rack.") {
				continue
			}
			var vals []string
			if a, ok := hh.vals[k].(Array_Any); ok {
				for _, v := range a._ToAny().s {
					vals = append(vals, string(rbToS(v)))
				}
			} else {
				vals = []string{string(rbToS(hh.vals[k]))}
			}
			if strings.EqualFold(key, "set-cookie") {
				for _, v := range vals {
					h.Add(key, v)
				}
			} else {
				h.Set(key, strings.Join(vals, ", "))
			}
		}
	}
	if loc := h.Get("Location"); loc != "" { // WEBrick makes it absolute
		if u, err := url.Parse(loc); err == nil && !u.IsAbs() {
			h.Set("Location", (&url.URL{Scheme: "http", Host: r.Host, Path: r.URL.Path}).ResolveReference(u).String())
		}
	}
	var body strings.Builder
	if a, ok := parts[2].(Array_Any); ok {
		for _, x := range a._ToAny().s {
			body.WriteString(string(rbToS(x)))
		}
	}
	if _, ok := h["Content-Type"]; !ok {
		h["Content-Type"] = nil // WEBrick sends none; stop Go sniffing one
	}
	if h.Get("Content-Length") == "" {
		h.Set("Content-Length", strconv.Itoa(body.Len()))
	}
	w.WriteHeader(int(status))
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, body.String())
	}
}
