package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"regexp"
	"strings"
)

// chaReport: per Go method name, distinct implementations across receivers.
func chaReport(fset *token.FileSet, f *ast.File) {
	impls := map[string]map[string]bool{}
	fwdByName := map[string]int{}
	dynByName := map[string]int{}
	dynBodies := map[string]map[string]bool{}
	recvRe := regexp.MustCompile(`\b[A-Za-z0-9_]+(_Meta)?I?\b`)
	_ = recvRe
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv == nil {
			continue
		}
		name, r := fd.Name.Name, recvName(fd)
		var key string
		body := fd.Body.List
		if strings.HasPrefix(name, "Dyn") || strings.HasPrefix(name, "_Dyn") {
			var b bytes.Buffer
			printer.Fprint(&b, fset, fd.Body)
			s := b.String()
			s = strings.ReplaceAll(s, r+"_MetaI", "R_MI")
			s = strings.ReplaceAll(s, r+"I", "RI")
			s = strings.ReplaceAll(s, r, "R")
			// strip generic Self instantiations: X[Anything](self -> X[*](self
			s = regexp.MustCompile(`\[[A-Za-z0-9_*]+\]\(self`).ReplaceAllString(s, "[S](self")
			if dynBodies[name] == nil {
				dynBodies[name] = map[string]bool{}
			}
			dynBodies[name][s] = true
			dynByName[name]++
			continue
		}
		fwdTargets := map[string]int{}
		if len(body) == 1 && isFwd(body[0], fwdTargets) {
			for t := range fwdTargets {
				key = "fwd:" + t
			}
			fwdByName[name]++
		} else {
			key = "own:" + r
		}
		if impls[name] == nil {
			impls[name] = map[string]bool{}
		}
		impls[name][key] = true
	}
	mono, monoNames, poly := 0, 0, 0
	for n, c := range fwdByName {
		if len(impls[n]) == 1 {
			mono += c
			monoNames++
		} else {
			poly += c
		}
	}
	fmt.Printf("forwarders on monomorphic names (one impl program-wide): %d across %d names; on polymorphic names: %d across %d names\n", mono, monoNames, poly, len(fwdByName)-monoNames)
	tot, distinct := 0, 0
	for n, c := range dynByName {
		tot += c
		distinct += len(dynBodies[n])
	}
	fmt.Printf("Dyn wrappers: %d over %d names; distinct bodies after normalizing receiver: %d\n", tot, len(dynByName), distinct)
}
