package compiler

import (
	"fmt"

	"golang.org/x/tools/imports"
)

func formatGo(src []byte) ([]byte, error) {
	out, err := imports.Process("main.go", src, &imports.Options{Comments: true, TabIndent: true, TabWidth: 8})
	if err != nil {
		return nil, fmt.Errorf("gofmt: %w", err)
	}
	return out, nil
}
