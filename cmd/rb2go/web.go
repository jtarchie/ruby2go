package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/jtarchie/ruby2go"
	"github.com/jtarchie/ruby2go/examples"
)

//go:embed web.html
var webPage []byte

// highlight.js 11.11.1 (BSD-3-Clause, notice in its banner), from cdnjs; its common bundle has Ruby and Go.
//
//go:embed highlight.min.js
var highlightJS []byte

const (
	webFile    = "main.rb"
	runTimeout = 10 * time.Second
	outputCap  = 1 << 20 // per stream
	maxSource  = 1 << 20
)

// webCmd serves a two-pane playground: Ruby in, generated Go out, plus a Run button (decision 102).
func webCmd(args []string) int {
	fset := flag.NewFlagSet("web", flag.ExitOnError)
	addr := fset.String("addr", "127.0.0.1:8080", "listen address; /run executes code, so keep it on loopback")
	_ = fset.Parse(args) // ExitOnError
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		fmt.Fprintln(os.Stderr, "warning: rb2go web runs submitted code; anyone who can reach", *addr, "can run programs as you")
	}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", *addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "rb2go web: http://%s/\n", ln.Addr())
	srv := &http.Server{Handler: newWeb(host, runTimeout), ReadHeaderTimeout: 10 * time.Second}
	err = srv.Serve(ln)
	fmt.Fprintln(os.Stderr, err)
	return 1
}

type web struct {
	hosts   []string      // Host header names accepted: loopback plus -addr's, so a DNS-rebound name is refused
	timeout time.Duration // wall clock per /run
	mu      sync.Mutex    // ponytail: one Compile at a time (a local tool has one user); a pool if it's ever hosted
	mux     *http.ServeMux
}

func newWeb(host string, timeout time.Duration) *web {
	w := &web{hosts: []string{"localhost", "127.0.0.1", "::1", host}, timeout: timeout, mux: http.NewServeMux()}
	w.mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = rw.Write(webPage)
	})
	w.mux.HandleFunc("GET /highlight.min.js", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = rw.Write(highlightJS)
	})
	w.mux.HandleFunc("GET /examples", w.examples)
	w.mux.HandleFunc("POST /compile", w.compile)
	w.mux.HandleFunc("POST /run", w.run)
	return w
}

// ServeHTTP refuses what a hostile page in the same browser could send: a foreign Host (DNS rebinding), a foreign Origin, or a non-JSON POST (a "simple" cross-site request skips the CORS preflight).
func (w *web) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	origin := r.Header.Get("Origin")
	if !slices.Contains(w.hosts, host) || origin != "" && origin != "http://"+r.Host || r.Method == http.MethodPost && ct != "application/json" {
		http.Error(rw, "forbidden", http.StatusForbidden)
		return
	}
	w.mux.ServeHTTP(rw, r)
}

type example struct {
	Name string `json:"name"`
	Src  string `json:"src"`
}

func (w *web) examples(rw http.ResponseWriter, _ *http.Request) {
	paths, _ := fs.Glob(examples.FS, "*/main.rb") // the pattern is constant, so no ErrBadPattern
	out := make([]example, 0, len(paths))
	for _, p := range paths {
		src, err := fs.ReadFile(examples.FS, p)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, example{Name: path.Base(path.Dir(p)), Src: string(src)})
	}
	writeJSON(rw, out)
}

type compileResult struct {
	Go       string   `json:"go"`
	User     string   `json:"user"` // Go from main.rb only: the prelude-hidden view
	Warnings []string `json:"warnings"`
	Error    string   `json:"error,omitempty"`
}

func (w *web) compile(rw http.ResponseWriter, r *http.Request) {
	src, ok := readSource(rw, r)
	if !ok {
		return
	}
	code, warnings, err := w.transpile(r.Context(), src)
	res := compileResult{Go: string(code), User: rb2go.UserCode(code, webFile), Warnings: warnings}
	if err != nil {
		res.Error = err.Error()
	}
	writeJSON(rw, res)
}

func (w *web) transpile(ctx context.Context, src []byte) ([]byte, []string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	err := ctx.Err()
	if err != nil {
		return nil, nil, fmt.Errorf("rb2go web: %w", err) // superseded by a newer edit while queued
	}
	return rb2go.Compile(ctx, webFile, src) //nolint:wrapcheck // already rb2go-prefixed
}

type runResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Exit   int    `json:"exit"`
	Error  string `json:"error,omitempty"` // compile/build failure or timeout; Stderr then holds go build's output
}

func (w *web) run(rw http.ResponseWriter, r *http.Request) {
	src, ok := readSource(rw, r)
	if !ok {
		return
	}
	code, warnings, err := w.transpile(r.Context(), src)
	var diag, stdout, stderr capped
	f, e := false, ""
	dir, err := buildModule(r.Context(), code, warnings, err, "", buildOpts{race: &f, gcflags: &e, work: &f}, &diag)
	if dir != "" {
		defer func() { _ = os.RemoveAll(dir) }()
	}
	if err != nil {
		writeJSON(rw, runResult{Stderr: diag.String(), Exit: -1, Error: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), w.timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(dir, "prog")) //nolint:gosec // the binary we just built
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	cmd.WaitDelay = time.Second // a grandchild holding the pipes can't hang the request
	// ponytail: the timeout kills prog only; a grandchild (system("sleep 99")) outlives it. Setpgid + kill(-pid) in cmd.Cancel, unix-only.
	err = cmd.Run()
	res := runResult{Stdout: stdout.String(), Stderr: diag.String() + stderr.String()}
	var exit *exec.ExitError
	switch {
	case ctx.Err() != nil:
		res.Exit, res.Error = -1, fmt.Sprintf("killed after %v", w.timeout)
	case errors.As(err, &exit):
		res.Exit = exitStatus(exit)
	case err != nil:
		res.Exit, res.Error = -1, err.Error()
	}
	writeJSON(rw, res)
}

func readSource(rw http.ResponseWriter, r *http.Request) ([]byte, bool) {
	var req struct {
		Src string `json:"src"`
	}
	err := json.NewDecoder(http.MaxBytesReader(rw, r.Body, maxSource)).Decode(&req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusBadRequest)
		return nil, false
	}
	return []byte(req.Src), true
}

func writeJSON(rw http.ResponseWriter, v any) {
	rw.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(rw).Encode(v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rb2go web:", err) // the client went away mid-response
	}
}

// capped keeps the first outputCap bytes and swallows the rest, so a chatty program neither blocks nor fills memory.
type capped struct {
	buf bytes.Buffer
	cut bool
}

func (c *capped) Write(p []byte) (int, error) {
	room := outputCap - c.buf.Len()
	c.cut = c.cut || len(p) > room
	c.buf.Write(p[:max(0, min(len(p), room))])
	return len(p), nil
}

func (c *capped) String() string {
	if c.cut {
		return c.buf.String() + "\n[output truncated at 1 MB]\n"
	}
	return c.buf.String()
}
