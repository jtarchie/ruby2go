package compiler

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// a library the compiler answers cannot ship without its docs/support.md row (decision 373)
func TestSupportDoc(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "support.md"))
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, libs := range []map[string]string{preludeLibs, vendoredLibs} {
		for name := range libs {
			if !strings.HasPrefix(name, "__") && !strings.Contains(string(doc), "`"+name+"`") {
				missing = append(missing, name)
			}
		}
	}
	slices.Sort(missing)
	if len(missing) > 0 {
		t.Errorf("docs/support.md has no `name` for these require names (decision 373): %s", strings.Join(missing, ", "))
	}
}

// mriLib answers by whole name or first path segment: net/protocol is the stdlib's, net/ftp a bundled gem to look up (decision 409).
func TestMRILib(t *testing.T) {
	for name, want := range map[string]bool{"pathname": true, "io/console": true, "net/protocol": true, "net/protocol.rb": true, "net/ftp": false, "net/smtp": false, "net": false, "matrix": false} {
		if got := mriLib(name); got != want {
			t.Errorf("mriLib(%q) = %v, want %v", name, got, want)
		}
	}
}
