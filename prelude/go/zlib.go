//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbZlibErr is the exception MRI raises for a failed inflate: zlib's own
// messages ("incorrect header check", "incorrect data check"), and a
// BufError for input that ends early.
func rbZlibErr(err error) any {
	switch {
	case errors.Is(err, zlib.ErrHeader):
		return NewZlib_DataError(Ref(String("incorrect header check")))
	case errors.Is(err, zlib.ErrChecksum):
		return NewZlib_DataError(Ref(String("incorrect data check")))
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return NewZlib_BufError(Ref(String("buffer error")))
	}
	return NewZlib_DataError(Ref(String(err.Error())))
}

// rbGzipErr is Zlib::GzipFile::Error as MRI words it for a bad gzip stream.
func rbGzipErr(err error) any {
	switch {
	case errors.Is(err, gzip.ErrHeader):
		return NewZlib_GzipFile_Error(Ref(String("not in gzip format")))
	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return NewZlib_GzipFile_Error(Ref(String("unexpected end of string")))
	}
	return NewZlib_GzipFile_Error(Ref(String(err.Error())))
}
