package rugaroo

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// netLimitsProgram exercises the DoS caps of prelude/go/net_limits.go. MRI's WEBrick and Net::HTTP have no body cap, so this cannot be a testdata/test file (MRI must pass those): the test lowers the caps in the generated main.go instead of adding a runtime knob.
const netLimitsProgram = `
require "net/http"
require "socket"
require "webrick"

server = WEBrick::HTTPServer.new(Port: 0, BindAddress: "127.0.0.1")
server.mount_proc("/") { |req, res| res.body = "len=#{req.body.to_s.bytesize}" }
server.mount_proc("/big") { |_req, res| res.body = "y" * 100 }
Thread.new { server.start }
port = server.config[:Port]
http = Net::HTTP.new("127.0.0.1", port)

[10, 64, 65, 500].each do |n|
  r = http.post("/", "x" * n)
  puts "post #{n}: #{r.code} #{r.body}".strip
end

# an unknown-length (chunked) body is cut off at the cap too
s = TCPSocket.new("127.0.0.1", port)
s.write "POST / HTTP/1.1\r\nHost: x\r\nTransfer-Encoding: chunked\r\nConnection: close\r\n\r\n41\r\n#{"z" * 65}\r\n0\r\n\r\n"
puts "chunked 65: #{s.gets.to_s.split(" ")[1]}"
s.close

# WEBrick's MAX_URI_LENGTH (414) and MAX_HEADER_LENGTH (413)
s = TCPSocket.new("127.0.0.1", port)
s.write "GET /#{"a" * 2100} HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"
puts "long uri: #{s.gets.to_s.split(" ")[1]}"
s.close
s = TCPSocket.new("127.0.0.1", port)
s.write "GET / HTTP/1.1\r\nHost: x\r\nX-Big: #{"a" * 130_000}\r\nConnection: close\r\n\r\n"
puts "big header: #{s.gets.to_s.split(" ")[1]}"
s.close

# the client refuses a response body past its cap rather than exhausting memory
begin
  http.get("/big")
  puts "get /big: no error"
rescue NoMemoryError => e
  puts "get /big: #{e.class}: #{e.message}"
end
r = http.get("/?ok=1")
puts "get /: #{r.code} #{r.body}"
server.shutdown
`

const netLimitsWant = `post 10: 200 len=10
post 64: 200 len=64
post 65: 413
post 500: 413
chunked 65: 413
long uri: 414
big header: 413
get /big: NoMemoryError: Net::HTTP response body too large
get /: 200 len=0
`

func TestNetLimits(t *testing.T) {
	requireToolchain(t)
	t.Parallel()
	gen := transpile(t, "net_limits.rb", []byte(netLimitsProgram))
	path := filepath.Join(gen, "main.go")
	code, err := os.ReadFile(path) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"rbWEBrickMaxBody", "rbHTTPMaxBody"} {
		re := regexp.MustCompile(`(?m)^(const ` + name + ` = )[^\n]+`)
		if !re.Match(code) {
			t.Fatalf("generated program has no `const %s = ...` to lower", name)
		}
		code = re.ReplaceAll(code, []byte("${1}64"))
	}
	err = os.WriteFile(path, code, 0o600) //nolint:gosec // under t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	bin := goBuild(t, gen, "-l")
	got := binOutput(t, gen, progIO{}, bin, nil)
	if strings.TrimSpace(got.Stdout) != strings.TrimSpace(netLimitsWant) {
		t.Errorf("stdout differs\n--- want ---\n%s\n--- got ---\n%s\nstderr:\n%s", netLimitsWant, got.Stdout, got.Stderr)
	}
	if got.Code != 0 {
		t.Errorf("exit code %d; stderr:\n%s", got.Code, got.Stderr)
	}
}
