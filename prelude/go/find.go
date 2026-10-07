//go:build rb2go_prelude

package prelude

import (
	"io/fs"
	"os"
	"path/filepath"
)

// findPruneRequested is Find.prune's signal back to the yield call that ran it; reset before each yield so nesting self-corrects (decision 63).
var findPruneRequested bool

// rbFind is MRI's Find.find: pre-order, sorted siblings, all roots checked to exist upfront, deeper errors swallowed (ignore_error: true).
func rbFind(roots []String, yield func(String) bool) {
	paths := make([]string, len(roots))
	for i, r := range roots {
		p := string(r)
		if _, err := os.Stat(p); err != nil {
			panic(NewErrno_ENOENT(Ref(String("No such file or directory - " + p))))
		}
		paths[i] = p
	}
	stop := false
	for _, root := range paths {
		if stop {
			return
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil {
				findPruneRequested = false
				if !yield(String(path)) {
					stop = true
					return filepath.SkipAll
				}
				if findPruneRequested && d.IsDir() {
					return filepath.SkipDir
				}
			}
			return nil
		})
	}
}
