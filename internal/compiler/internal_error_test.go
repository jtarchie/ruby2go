package compiler

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

// A compiler bug's panic is a returned ErrInternal naming the file, not a raw Go trace (decision 263).
func TestNonCompilePanicIsInternalError(t *testing.T) {
	t.Setenv("RB2GO_ERR_STACK", "")
	opts := options{testHook: func() { panic("boom") }}
	_, err := compile(context.Background(), os.DirFS("../.."), []Source{{Name: "main.rb", Src: []byte("puts 1\n")}}, &opts)
	if !errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want ErrInternal", err)
	}
	if want := "internal error: boom while compiling main.rb"; err.Error() != want {
		t.Fatalf("err = %q, want %q", err, want)
	}
	var ce compileError
	if errors.As(err, &ce) || strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("must not look like a compile error or carry the stack: %v", err)
	}
}

// A compile error is not an internal error, so the fuzzers keep tolerating it.
func TestCompileErrorIsNotInternal(t *testing.T) {
	_, _, err := CompileWithWarnings(context.Background(), os.DirFS("../.."), "main.rb", []byte("def f(\n"))
	if err == nil || errors.Is(err, ErrInternal) {
		t.Fatalf("err = %v, want a plain compile error", err)
	}
}
