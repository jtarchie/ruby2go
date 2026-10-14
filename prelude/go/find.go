//go:build rb2go_prelude

package prelude

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// rbFindActive maps a goroutine id to the prune flag of the innermost Find.find that goroutine runs (decision 66): only that goroutine touches the flag, so threads cannot prune each other.
var rbFindActive sync.Map

// rbFindPrune is Find.prune: it marks the running Find.find's current entry; outside one MRI's throw :prune has no catch, so it raises the same UncaughtThrowError.
func rbFindPrune() {
	p, ok := rbFindActive.Load(rbGoID())
	if !ok {
		panic(NewUncaughtThrowError("uncaught throw :prune", Symbol("prune"), nil))
	}
	*p.(*bool) = true
}

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
	id, pruned := rbGoID(), new(bool)
	outer, nested := rbFindActive.Load(id)
	rbFindActive.Store(id, pruned)
	defer func() { // a Find.find in the block resumes the outer one's flag
		if nested {
			rbFindActive.Store(id, outer)
		} else {
			rbFindActive.Delete(id)
		}
	}()
	stop := false
	for _, root := range paths {
		if stop {
			return
		}
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil {
				*pruned = false
				if !yield(String(path)) {
					stop = true
					return filepath.SkipAll
				}
				if *pruned && d.IsDir() {
					return filepath.SkipDir
				}
			}
			return nil
		})
	}
}
