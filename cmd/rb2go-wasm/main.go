//go:build js && wasm

// Command rb2go-wasm is rb2go's compiler for a browser (the static `rb2go web`, decision 102): it defines rb2goCompile(src) and stays alive for calls.
package main

import (
	"context"
	"fmt"
	"runtime/debug"
	"syscall/js"

	"github.com/jtarchie/ruby2go"
)

const mainName = "main.rb"

func main() {
	// a compile allocates ~130 MB and lives well under a second: collecting half as often is 25% faster (0.63 s vs 0.83 s), and the limit keeps the tab's memory bounded
	debug.SetGCPercent(200)
	debug.SetMemoryLimit(512 << 20)
	js.Global().Set("rb2goCompile", js.FuncOf(func(_ js.Value, args []js.Value) any {
		return compile(args[0].String())
	}))
	select {}
}

// compile is /compile's JSON shape as a JS object; a compiler crash becomes its error, since a panic would end the instance for every later call.
func compile(src string) (res map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			res = map[string]any{"go": "", "user": "", "warnings": []any{}, "error": fmt.Sprintf("rb2go: internal error: %v", r)}
		}
	}()
	code, warnings, err := rb2go.Compile(context.Background(), mainName, []byte(src))
	ws := make([]any, len(warnings))
	for i, w := range warnings {
		ws[i] = w
	}
	res = map[string]any{"go": string(code), "user": rb2go.UserCode(code, mainName), "warnings": ws}
	if err != nil {
		res["error"] = err.Error()
	}
	return res
}
