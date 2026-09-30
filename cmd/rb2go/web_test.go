package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
