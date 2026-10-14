package main

import (
	"strings"
	"testing"
)

func TestRunTargetsRecoversPanic(t *testing.T) {
	targets := []target{{name: "core/a"}, {name: "core/boom"}, {name: "core/c"}}
	results := runTargets(targets, 2, func(tg target) *result {
		if tg.name == "core/boom" {
			panic("kaboom")
		}
		return &result{name: tg.name, pass: 1}
	}, false)
	if len(results) != 3 || results[0].pass != 1 || results[2].pass != 1 {
		t.Fatalf("the other programs' results were lost: %+v", results)
	}
	if r := results[1]; r.name != "core/boom" || !strings.Contains(r.fatal, "kaboom") {
		t.Errorf("panicking target = %+v, want a FATAL result naming the panic", r)
	}
	if exitCode(results) != 1 {
		t.Error("a fatal program must exit non-zero")
	}
}

func TestExitCode(t *testing.T) {
	if got := exitCode([]*result{{name: "a", fail: 3, errored: 2}}); got != 0 {
		t.Errorf("failing examples are reported, not fatal: exit %d", got)
	}
	if got := exitCode([]*result{{name: "a"}, {name: "b", fatal: "go build: x"}}); got != 1 {
		t.Errorf("one fatal program: exit %d, want 1", got)
	}
}
