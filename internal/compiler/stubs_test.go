package compiler

import (
	"context"
	"os"
	"testing"
)

// PreludeWhole keeps every declaration without touching the environment, so a compile running beside it still prunes.
func TestPreludeWholeNoEnv(t *testing.T) {
	t.Setenv("RB2GO_NO_PRUNE", "")
	fsys := os.DirFS("../..")
	src := []Source{{Name: "main.rb", Src: []byte("puts 1\n")}}
	pruned, err := compile(context.Background(), fsys, src, &options{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan []byte)
	go func() {
		whole, _ := PreludeWhole(context.Background(), fsys)
		done <- whole
	}()
	for range 3 {
		out, err := compile(context.Background(), fsys, src, &options{})
		if err != nil || len(out) != len(pruned) {
			t.Fatalf("a compile beside PreludeWhole changed: err %v, %d bytes, want %d", err, len(out), len(pruned))
		}
	}
	if whole := <-done; len(whole) < 20*len(pruned) {
		t.Fatalf("PreludeWhole is %d bytes, pruned hello is %d: not whole", len(whole), len(pruned))
	}
	if os.Getenv("RB2GO_NO_PRUNE") != "" {
		t.Fatal("PreludeWhole set RB2GO_NO_PRUNE")
	}
}
