package boot

import (
	"context"
	"os"
	"testing"
)

func TestCubaProbe(t *testing.T) {
	g := os.Getenv("RB2GO_BOOT_CUBA_DIR")
	if g == "" {
		t.Skip("set RB2GO_BOOT_CUBA_DIR to the dir holding cuba-*/lib and rack-*/lib")
	}
	entry := os.Getenv("RB2GO_BOOT_ENTRY")
	m, err := Capture(context.Background(), entry,
		g+"/cuba-4.0.3/lib", g+"/rack-3.2.7/lib")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("classes: %d, define_methods: %d, evals: %d, autoloads: %d, state: %d, loaded: %d",
		len(m.Classes), len(m.DefineMethods), len(m.Evals), len(m.Autoloads), len(m.ClassState), len(m.LoadedFeatures))
	for _, c := range m.Classes {
		t.Logf("class %s < %s (%s:%d)", c.Name, c.Super, c.Path, c.Line)
	}
	for _, d := range m.DefineMethods {
		t.Logf("define_method %s#%s (%s:%d)", d.Owner, d.Name, d.File, d.Line)
	}
	for _, e := range m.Evals {
		t.Logf("eval caller=%s file=%s line=%d src=%.60q", e.Caller, e.File, e.Line, e.Source)
	}
	for _, a := range m.Autoloads {
		t.Logf("autoload %s::%s path=%s resolved=%v loaded=%s", a.Owner, a.Const, a.Path, a.Resolved, a.Loaded)
	}
	for _, s := range m.ClassState {
		t.Logf("state %s%s kind=%s recreate=%v desc=%.60q", s.Owner, s.Ivar, s.Kind, s.Recreate, s.Desc)
	}
}
