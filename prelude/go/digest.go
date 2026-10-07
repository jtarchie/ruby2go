//go:build rb2go_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rb2go_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
)

// rbDigestSum is Digest::<algo>.digest of b.
func rbDigestSum(algo string, b []byte) []byte {
	switch algo {
	case "MD5":
		s := md5.Sum(b) //nolint:gosec // Digest::MD5 is what the program asked for
		return s[:]
	case "SHA1":
		s := sha1.Sum(b) //nolint:gosec // Digest::SHA1 is what the program asked for
		return s[:]
	case "SHA384":
		s := sha512.Sum384(b)
		return s[:]
	case "SHA512":
		s := sha512.Sum512(b)
		return s[:]
	}
	s := sha256.Sum256(b)
	return s[:]
}
