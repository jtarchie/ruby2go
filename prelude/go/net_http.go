//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

var rbHTTPClient = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

func rbHTTPRequest(h *Net_HTTP, method, path string, body *String, headers *Hash[String, String]) *Net_HTTPResponse {
	var rd io.Reader
	if body != nil {
		rd = strings.NewReader(string(*body))
	}
	target := "http://" + net.JoinHostPort(string(h.address), strconv.Itoa(int(h.port))) + path
	req, err := http.NewRequestWithContext(context.Background(), method, target, rd)
	if err != nil {
		panic(NewArgumentError(Ref(String(err.Error()))))
	}
	req.Header.Set("User-Agent", "Ruby")
	for _, k := range headers.keys {
		req.Header.Set(string(k), string(headers.vals[k]))
	}
	resp, err := rbHTTPClient.Do(req)
	if err != nil {
		panic(NewIOError(Ref(String(err.Error()))))
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(NewIOError(Ref(String(err.Error()))))
	}
	return &Net_HTTPResponse{
		code:    String(strconv.Itoa(resp.StatusCode)),
		message: String(http.StatusText(resp.StatusCode)),
		body:    String(data),
		header:  resp.Header,
	}
}
