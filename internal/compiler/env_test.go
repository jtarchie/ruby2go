package compiler

import (
	"context"
	"os"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	flags := map[string]func(envConfig) bool{
		"RB2GO_NO_WASM_CACHE": func(e envConfig) bool { return e.NoWasmCache },
		"RB2GO_INFER_DEBUG":   func(e envConfig) bool { return e.InferDebug },
		"RB2GO_TIMING":        func(e envConfig) bool { return e.Timing },
		"RB2GO_ERR_STACK":     func(e envConfig) bool { return e.ErrStack },
		"RB2GO_ALL_LIBS":      func(e envConfig) bool { return e.AllLibs },
		"RB2GO_GEM_WARNINGS":  func(e envConfig) bool { return e.GemWarnings },
		"RB2GO_PRUNE_DEBUG":   func(e envConfig) bool { return e.PruneDebug },
		"RB2GO_NO_PRUNE":      func(e envConfig) bool { return e.NoPrune },
		"RB2GO_PRUNE_WHY":     func(e envConfig) bool { return e.PruneWhy },
		"RB2GO_KEEP_GOING":    func(e envConfig) bool { return e.KeepGoing },
	}
	values := map[string]func(envConfig) string{
		"RB2GO_DISPATCH_DEBUG": func(e envConfig) string { return e.DispatchDebug },
		"RB2GO_GOPOISON_DUMP":  func(e envConfig) string { return e.GopoisonDump },
	}
	for name := range flags {
		t.Setenv(name, "")
	}
	for name := range values {
		t.Setenv(name, "")
	}
	if got := loadEnv(); got != (envConfig{}) {
		t.Fatalf("empty environment: %+v", got)
	}
	for name, field := range flags {
		t.Setenv(name, "1")
		if !field(loadEnv()) {
			t.Errorf("%s set but not seen", name)
		}
		t.Setenv(name, "")
	}
	for name, field := range values {
		t.Setenv(name, "all")
		if got := field(loadEnv()); got != "all" {
			t.Errorf("%s = %q, want all", name, got)
		}
		t.Setenv(name, "")
	}
}

// A compile reads the environment when it starts: RB2GO_NO_PRUNE set between two compiles keeps the whole prelude.
func TestCompileSeesNoPruneEnv(t *testing.T) {
	t.Setenv("RB2GO_NO_PRUNE", "")
	fsys := os.DirFS("../..")
	src := []Source{{Name: "main.rb", Src: []byte("puts 1\n")}}
	pruned, err := compile(context.Background(), fsys, src, &options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RB2GO_NO_PRUNE", "1")
	whole, err := compile(context.Background(), fsys, src, &options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(whole) < 5*len(pruned) {
		t.Fatalf("RB2GO_NO_PRUNE=1 gave %d bytes, pruned is %d", len(whole), len(pruned))
	}
}
