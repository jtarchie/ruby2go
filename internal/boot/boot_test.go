package boot

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func needRuby(t *testing.T) {
	t.Helper()
	_, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH")
	}
}

// writeEntry writes src as name in a fresh temp dir and returns the dir.
func writeEntry(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// shrink sets a package limit for one test.
func shrink[T any](t *testing.T, p *T, v T) {
	t.Helper()
	old := *p
	*p = v
	t.Cleanup(func() { *p = old })
}

// bounded fails the test if f does not return in a generous watchdog time (a guard against a hang, not a timing assertion).
func bounded(t *testing.T, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { defer close(done); f() }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("call did not return: the timeout or WaitDelay did not bound it")
	}
}

func TestCaptureTimeout(t *testing.T) {
	needRuby(t)
	shrink(t, &bootTimeout, 300*time.Millisecond)
	shrink(t, &waitDelay, 300*time.Millisecond)
	dir := writeEntry(t, "entry.rb", "sleep 60\n")
	bounded(t, func() {
		_, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("err = %v, want a timed out error", err)
		}
	})
}

// A grandchild keeps the stderr pipe open after ruby is killed: WaitDelay must release Wait.
func TestCaptureTimeoutGrandchild(t *testing.T) {
	needRuby(t)
	shrink(t, &bootTimeout, 300*time.Millisecond)
	shrink(t, &waitDelay, 300*time.Millisecond)
	dir := writeEntry(t, "entry.rb", "spawn('sleep 25')\nsleep 60\n")
	bounded(t, func() {
		_, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("err = %v, want a timed out error", err)
		}
	})
}

func TestFindGemsTimeout(t *testing.T) {
	needRuby(t)
	shrink(t, &bootTimeout, time.Nanosecond)
	bounded(t, func() {
		_, err := FindGems(context.Background(), t.TempDir(), []string{"rake"})
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("err = %v, want a timed out error", err)
		}
	})
}

func TestCaptureDashEntry(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "-x.rb", "module DashEntry; end\n")
	t.Chdir(dir)
	m, err := Capture(context.Background(), "-x.rb")
	if err != nil {
		t.Fatal(err)
	}
	if m.Class("DashEntry") == nil {
		t.Errorf("entry -x.rb was not run as a script; classes %v", classNames(m))
	}
}

func TestCaptureStderrCapped(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "entry.rb", "$stderr.write('x' * (1 << 20))\nexit 1\n")
	_, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err == nil {
		t.Fatal("want the failed boot's error")
	}
	if len(err.Error()) > 80<<10 || !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error is %d bytes, want <= 80 KiB and a truncation note", len(err.Error()))
	}
}

func TestCaptureManifestTooBig(t *testing.T) {
	needRuby(t)
	shrink(t, &maxBytes, 10)
	dir := writeEntry(t, "entry.rb", "class Big; end\n")
	_, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("err = %v, want a size limit error", err)
	}
}

func TestFindGemsOutputTooBig(t *testing.T) {
	needRuby(t)
	shrink(t, &maxBytes, 4)
	_, err := FindGems(context.Background(), t.TempDir(), []string{"rake"})
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("err = %v, want a size limit error", err)
	}
}

func TestRubyMissing(t *testing.T) {
	t.Setenv("PATH", "/nonexistent")
	dir := writeEntry(t, "entry.rb", "\n")
	_, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err == nil || !strings.Contains(err.Error(), "needs ruby on PATH") {
		t.Errorf("Capture err = %v, want the ruby-on-PATH message", err)
	}
	_, err = FindGems(context.Background(), dir, []string{"rake"})
	if err == nil || !strings.Contains(err.Error(), "needs ruby on PATH") {
		t.Errorf("FindGems err = %v, want the ruby-on-PATH message", err)
	}
}

func TestCapture(t *testing.T) {
	if _, err := exec.LookPath("ruby"); err != nil { //nolint:noinlineerr // a skip guard, not error handling
		t.Skip("ruby not on PATH")
	}
	m, err := Capture(context.Background(), "testdata/entry.rb", "testdata/lib")
	if err != nil {
		t.Fatal(err)
	}

	thing := m.Class("Demo::Thing")
	if thing == nil {
		t.Fatalf("Demo::Thing not recorded; got %v", classNames(m))
	}
	if thing.Super != "Demo::Base" {
		t.Errorf("Demo::Thing super = %q, want Demo::Base", thing.Super)
	}
	if !contains(thing.Ancestors, "Demo::Base") {
		t.Errorf("ancestors = %v, want Demo::Base", thing.Ancestors)
	}
	if m.Class("Demo::Lazy") == nil {
		t.Errorf("autoloaded Demo::Lazy not recorded; got %v", classNames(m))
	}

	al := findAutoload(m, "Demo", "Lazy")
	if al == nil {
		t.Fatalf("no autoload for Demo::Lazy; got %+v", m.Autoloads)
	}
	if !al.Resolved {
		t.Errorf("Demo::Lazy autoload not marked resolved")
	}
	if !strings.HasSuffix(al.Loaded, "demo/lazy.rb") {
		t.Errorf("Demo::Lazy loaded = %q, want a demo/lazy.rb path", al.Loaded)
	}

	methods := map[string]MethodInfo{}
	for _, d := range m.DefinedMethods("Demo::Thing") {
		methods[d.Name] = d
	}
	if d, ok := methods["shout"]; !ok || !strings.HasSuffix(d.File, "demo.rb") || d.Line == 0 {
		t.Errorf("define_method :shout not located in demo.rb; got %+v", methods["shout"])
	}
	if _, ok := methods["from_eval"]; !ok {
		t.Errorf("define_method :from_eval through eval not captured; got %+v", methods)
	}

	if len(m.Evals) == 0 {
		t.Errorf("no class_eval string captured")
	} else if !strings.Contains(m.Evals[0].Source, "define_method") {
		t.Errorf("eval source = %q", m.Evals[0].Source)
	}

	state := map[string]ClassStateInfo{}
	for _, s := range m.ClassState {
		state[s.Owner+"#"+s.Ivar] = s
	}
	if reg, ok := state["Demo#@registry"]; !ok || !reg.Recreate || !strings.HasSuffix(reg.Kind, "Mutex") {
		t.Errorf("Demo @registry = %+v, want a recreated Mutex", state["Demo#@registry"])
	}
	if c, ok := state["Demo#@cache"]; !ok || c.Kind != "Hash" || !strings.Contains(c.Desc, "default=true") {
		t.Errorf("Demo @cache = %+v, want a defaulted Hash", state["Demo#@cache"])
	}

	loaded := strings.Join(m.LoadedFeatures, "\n")
	if !strings.Contains(loaded, "demo.rb") || !strings.Contains(loaded, filepath.Join("demo", "lazy.rb")) {
		t.Errorf("loaded_features = %v, want demo.rb and demo/lazy.rb", m.LoadedFeatures)
	}
}

// A forked child that exits normally runs the inherited at_exit: it must not dump over the parent's manifest.
func TestCaptureForkedChildDoesNotDump(t *testing.T) {
	needRuby(t)
	// The child blocks on the pipe until the parent has exited (and dumped), then defines ForkChild and exits through the handler.
	src := "class Parent; end\nr, w = IO.pipe\nfork { w.close; r.read; class ForkChild; end }\nr.close\n"
	dir := writeEntry(t, "entry.rb", src)
	m, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Class("Parent") == nil {
		t.Errorf("parent's manifest was overwritten; classes %v", classNames(m))
	}
	if m.Class("ForkChild") != nil {
		t.Errorf("the forked child dumped the manifest; classes %v", classNames(m))
	}
}

// exit! skips at_exit, so no manifest: the error says why and carries the program's stderr.
func TestCaptureExitBang(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "entry.rb", "$stderr.puts 'marker-from-program'\nexit!(0)\n")
	_, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err == nil {
		t.Fatal("want an error for the missing manifest")
	}
	for _, want := range []string{"marker-from-program", "manifest", "exit!"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to mention %q", err, want)
		}
	}
}

// A binary String held in class state (not valid UTF-8) must not make the dump raise.
func TestCaptureBinaryClassState(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "entry.rb", "module Bin\n  @blob = \"\\xff\\xfe\".b\n  @bad = \"\\xff\"\nend\n")
	m, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for _, s := range m.ClassState {
		if s.Owner == "Bin" {
			n++
		}
	}
	if n != 2 {
		t.Errorf("class state of Bin = %+v, want both ivars", m.ClassState)
	}
}

// A program may define its own Boot; the hook's state must not collide with it or scan it as class state.
func TestCaptureUserBoot(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "entry.rb", "class Boot\n  @mine = {}\n  def self.dump(*) = raise('user Boot.dump called')\nend\n")
	m, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Class("Boot") == nil {
		t.Errorf("user class Boot not recorded; classes %v", classNames(m))
	}
}

// A project-local json.rb on the load path must not stand in for the stdlib's in the hook.
func TestCaptureLocalJSONShadow(t *testing.T) {
	needRuby(t)
	lib := writeEntry(t, "json.rb", "raise 'shadowed json'\n")
	dir := writeEntry(t, "entry.rb", "class Plain; end\n")
	m, err := Capture(context.Background(), filepath.Join(dir, "entry.rb"), lib)
	if err != nil {
		t.Fatal(err)
	}
	if m.Class("Plain") == nil {
		t.Errorf("classes %v", classNames(m))
	}
}

// A load-list line over the scanner's buffer is an error, not a silently shortened list.
func TestTraceOverlongLine(t *testing.T) {
	needRuby(t)
	dir := writeEntry(t, "entry.rb", "$LOAD_PATH << 'x' * (2 << 20)\n")
	_, err := Trace(context.Background(), filepath.Join(dir, "entry.rb"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "load list") {
		t.Errorf("err = %v, want a load list error", err)
	}
}

// The load list is capped like the manifest.
func TestTraceListTooBig(t *testing.T) {
	needRuby(t)
	shrink(t, &maxBytes, 10)
	dir := writeEntry(t, "entry.rb", "\n")
	_, err := Trace(context.Background(), filepath.Join(dir, "entry.rb"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("err = %v, want a size limit error", err)
	}
}

func TestManifestLookups(t *testing.T) {
	m := &Manifest{Classes: []ClassInfo{{Name: "A"}}}
	if m.Class("A") == nil || m.Class("B") != nil {
		t.Errorf("Class lookup wrong")
	}
}

func classNames(m *Manifest) []string {
	out := make([]string, len(m.Classes))
	for i, c := range m.Classes {
		out[i] = c.Name
	}
	return out
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func findAutoload(m *Manifest, owner, cst string) *AutoloadInfo {
	for i := range m.Autoloads {
		if m.Autoloads[i].Owner == owner && m.Autoloads[i].Const == cst {
			return &m.Autoloads[i]
		}
	}
	return nil
}
