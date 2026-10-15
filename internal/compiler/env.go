package compiler

import "os"

// envConfig is the RB2GO_* environment, read once per compile (not per process) so a test's t.Setenv is seen; flags are "non-empty".
type envConfig struct {
	NoWasmCache   bool   // RB2GO_NO_WASM_CACHE: no wazero compilation cache for prism.wasm
	InferDebug    bool   // RB2GO_INFER_DEBUG: what each inference round typed and could not compile
	Timing        bool   // RB2GO_TIMING: per-phase durations on stderr
	ErrStack      bool   // RB2GO_ERR_STACK: the Go stack of whatever pass raised an error
	AllLibs       bool   // RB2GO_ALL_LIBS: load every prelude lib, required or not
	GemWarnings   bool   // RB2GO_GEM_WARNINGS: print gem files' warnings too
	PruneDebug    bool   // RB2GO_PRUNE_DEBUG: why gem pruning reached each method name
	NoPrune       bool   // RB2GO_NO_PRUNE: keep every declaration
	PruneWhy      bool   // RB2GO_PRUNE_WHY: the keep chains of the pruner
	KeepGoing     bool   // RB2GO_KEEP_GOING: record a collect error and go on
	DispatchDebug string // RB2GO_DISPATCH_DEBUG: "all", or any other non-empty value for Kernel/Object/module owners
	GopoisonDump  string // RB2GO_GOPOISON_DUMP: path prefix for the Go before rejected gem methods become stubs
	UntypedWhy    bool   // RB2GO_UNTYPED_WHY: each dynamic-call warning names where its untyped receiver came from
}

func loadEnv() envConfig {
	set := func(name string) bool { return os.Getenv(name) != "" }
	return envConfig{
		NoWasmCache:   set("RB2GO_NO_WASM_CACHE"),
		InferDebug:    set("RB2GO_INFER_DEBUG"),
		Timing:        set("RB2GO_TIMING"),
		ErrStack:      set("RB2GO_ERR_STACK"),
		AllLibs:       set("RB2GO_ALL_LIBS"),
		GemWarnings:   set("RB2GO_GEM_WARNINGS"),
		PruneDebug:    set("RB2GO_PRUNE_DEBUG"),
		NoPrune:       set("RB2GO_NO_PRUNE"),
		PruneWhy:      set("RB2GO_PRUNE_WHY"),
		KeepGoing:     set("RB2GO_KEEP_GOING"),
		DispatchDebug: os.Getenv("RB2GO_DISPATCH_DEBUG"),
		GopoisonDump:  os.Getenv("RB2GO_GOPOISON_DUMP"),
		UntypedWhy:    set("RB2GO_UNTYPED_WHY"),
	}
}
