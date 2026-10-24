package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWeb(t *testing.T) {
	w := newWeb("127.0.0.1", 3*time.Second)
	do := func(method, path, body string, edit func(*http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequestWithContext(t.Context(), method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if edit != nil {
			edit(r)
		}
		rec := httptest.NewRecorder()
		w.ServeHTTP(rec, r)
		return rec
	}
	decode := func(rec *httptest.ResponseRecorder, v any) {
		t.Helper()
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d: %s", rec.Code, rec.Body)
		}
		err := json.Unmarshal(rec.Body.Bytes(), v)
		if err != nil {
			t.Fatal(err)
		}
	}

	var c compileResult
	decode(do("POST", "/compile", `{"src":"puts 1 + 2\n"}`, nil), &c)
	if c.Error != "" || !strings.Contains(c.Go, "\npackage main\n") {
		t.Errorf("compile: error %q, go %.200q", c.Error, c.Go)
	}
	// the prelude-hidden view: the class's struct, method and generated plumbing, the top-level statement, none of the prelude
	c = compileResult{}
	decode(do("POST", "/compile", `{"src":"class Pt\n  #: (Integer x) -> void\n  def initialize(x) = @x = x\nend\nputs Pt.new(3).inspect\n"}`, nil), &c)
	for _, want := range []string{"//line main.rb:1\ntype Pt struct {", "//line main.rb:3\nfunc Pt_Initialize(self *Pt", "func main() {\n//line main.rb:5\n", "func NewPt(x Integer) *Pt {"} {
		if !strings.Contains(c.User, want) {
			t.Errorf("user view lacks %q:\n%s", want, c.User)
		}
	}
	if strings.Contains(c.User, "prelude/") || strings.Contains(c.User, "type Object struct") {
		t.Errorf("user view has prelude code:\n%s", c.User)
	}
	c = compileResult{}
	decode(do("POST", "/compile", `{"src":"def (\n"}`, nil), &c)
	if !strings.Contains(c.Error, "main.rb:1") {
		t.Errorf("compile error lacks main.rb:1: %q", c.Error)
	}

	// what a hostile page could send: a rebound Host, a foreign Origin, a form POST (no preflight)
	for name, edit := range map[string]func(*http.Request){
		"host":   func(r *http.Request) { r.Host = "evil.example:8080" },
		"origin": func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") },
		"form":   func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
	} {
		if rec := do("POST", "/run", `{"src":"puts 1\n"}`, edit); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", name, rec.Code)
		}
	}
	if rec := do("POST", "/compile", `{}`, func(r *http.Request) { r.Header.Set("Origin", "http://127.0.0.1:8080") }); rec.Code != http.StatusOK {
		t.Errorf("same origin: status %d", rec.Code)
	}

	if rec := do("GET", "/highlight.min.js", "", nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Highlight.js") {
		t.Errorf("highlight.min.js: status %d", rec.Code)
	}

	var ex []example
	decode(do("GET", "/examples", "", nil), &ex)
	if len(ex) == 0 || ex[0].Src == "" {
		t.Errorf("examples: %d", len(ex))
	}

	var r runResult
	decode(do("POST", "/run", `{"src":"puts \"hi\"\n$stderr.puts \"oops\"\nexit 3\n"}`, nil), &r)
	if r.Stdout != "hi\n" || r.Stderr != "oops\n" || r.Exit != 3 || r.Error != "" {
		t.Errorf("run: %+v", r)
	}
	r = runResult{}
	decode(do("POST", "/run", `{"src":"while true\nend\n"}`, nil), &r)
	if r.Exit != -1 || !strings.Contains(r.Error, "killed after") {
		t.Errorf("run timeout: %+v", r)
	}
}

// TestWebRunEnv: submitted code sees PATH and its own temp dir as HOME/TMPDIR, not the developer's environment (tokens).
func TestWebRunEnv(t *testing.T) {
	const sentinel = "hunter2-rb2go-test-sentinel"
	t.Setenv("RB2GO_TEST_SECRET", sentinel)
	w := newWeb("127.0.0.1", 30*time.Second)
	src, err := json.Marshal(map[string]string{"src": "puts ENV[\"HOME\"]\nputs ENV[\"TMPDIR\"]\nputs ENV.keys.sort.inspect\nputs ENV.fetch(\"RB2GO_TEST_SECRET\", \"unset\")\n"})
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequestWithContext(t.Context(), "POST", "http://127.0.0.1:8080/run", strings.NewReader(string(src)))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	w.ServeHTTP(rec, r)
	var res runResult
	err = json.Unmarshal(rec.Body.Bytes(), &res)
	if err != nil || res.Error != "" || res.Exit != 0 {
		t.Fatalf("run: %v %+v", err, res)
	}
	lines := strings.Split(strings.TrimSuffix(res.Stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("stdout %q", res.Stdout)
	}
	home, tmp, keys, leaked := lines[0], lines[1], lines[2], lines[3]
	if home == "" || home != tmp || !strings.HasPrefix(filepath.Base(home), "rb2go-") {
		t.Errorf("HOME %q, TMPDIR %q: want the program's own temp dir", home, tmp)
	}
	if keys != `["HOME", "PATH", "TMPDIR"]` {
		t.Errorf("ENV keys %s, want only HOME, PATH, TMPDIR", keys)
	}
	if leaked != "unset" || strings.Contains(res.Stdout+res.Stderr, sentinel) {
		t.Errorf("the server's environment leaked: %q", res.Stdout)
	}
}

// A panic in a handler is a JSON error the page shows, not a dropped connection.
func TestWebPanicIsJSON(t *testing.T) {
	w := newWeb("127.0.0.1", time.Second)
	w.mux.HandleFunc("POST /boom", func(http.ResponseWriter, *http.Request) { panic("kaboom") })
	r := httptest.NewRequestWithContext(t.Context(), "POST", "http://127.0.0.1:8080/boom", strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	w.ServeHTTP(rec, r)
	var res runResult
	err := json.Unmarshal(rec.Body.Bytes(), &res)
	if err != nil || !strings.Contains(res.Error, "kaboom") || res.Exit != -1 {
		t.Errorf("status %d, body %q, err %v", rec.Code, rec.Body, err)
	}
}

func TestWebStatic(t *testing.T) {
	dir := t.TempDir()
	err := writeStatic(dir, "https://assets.example/abc/")
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(dir, "index.html")) //nolint:gosec // our temp dir
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`window.rb2goAssets = "https://assets.example/abc/";`, "<script>/*!\n  Highlight.js"} {
		if !strings.Contains(string(page), want) {
			t.Errorf("index.html lacks %q", want)
		}
	}
	_, err = os.Stat(filepath.Join(dir, "highlight.min.js"))
	if err == nil {
		t.Error("highlight.min.js written beside index.html; it's inlined")
	}
	var ex []example
	raw, err := os.ReadFile(filepath.Join(dir, "examples.json")) //nolint:gosec // our temp dir
	if err == nil {
		err = json.Unmarshal(raw, &ex)
	}
	if err != nil || len(ex) == 0 {
		t.Errorf("examples.json: %d examples, %v", len(ex), err)
	}
	for _, bad := range []string{"", "/rel/", "https://no-slash", "javascript:alert(1)/", "https://x/</script>/"} {
		if writeStatic(t.TempDir(), bad) == nil {
			t.Errorf("-assets %q accepted", bad)
		}
	}
}
