//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

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
