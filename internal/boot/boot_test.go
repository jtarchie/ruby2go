package boot

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
