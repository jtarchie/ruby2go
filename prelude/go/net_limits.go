//go:build rugaroo_prelude

// Package prelude is concatenated verbatim into the output (loadPreludeGo). `go vet -tags rugaroo_prelude ./prelude/go` type-checks it against generated stubs for the types generated code declares (0_stubs.go, decision 154).
package prelude

import (
	"errors"
	"io"
)

// Peer-controlled buffering limits (#109): a Go out-of-memory is fatal and `rescue` cannot catch it; the first two mirror webrick 1.9.2's httprequest.rb (414 / 413).
const rbWEBrickMaxURI = 2083

const rbWEBrickMaxHeader = 112 * 1024

// WEBrick's RequestTimeout default (config.rb): the longest it waits for the request, per read there and for the whole request here.
const rbWEBrickRequestTimeout = 30

// WEBrick has no cap on a request body (MRI buffers whatever Content-Length says), so this is a generous one: no real request is this big, and the body is held in memory (then copied into a String). Past it the request is answered 413 and the handler is not run.
const rbWEBrickMaxBody = 256 << 20

// Net::HTTP has no cap on a response body either. A quarter of rbAllocMax: the body is read into a []byte and copied into a String, so a 1 GiB response peaks at 2 GiB. Past it the call raises NoMemoryError.
const rbHTTPMaxBody = rbAllocMax / 4

var errRBBodyTooLarge = errors.New("body too large")

// rbReadAllLimit is io.ReadAll that fails with errRBBodyTooLarge once r holds more than limit bytes. It reads one byte past the limit to tell a body that fits exactly from one that does not.
func rbReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err == nil && int64(len(data)) > limit {
		return nil, errRBBodyTooLarge
	}
	return data, err
}
