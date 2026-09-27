# prelude/net_http.rb
# rbs_inline: enabled
#
# Net::HTTP's client on Go's net/http. Like Ruby's, it never follows
# redirects and returns every status as a response (no raise on 4xx/5xx).

module Net
  # @go_type struct { address String; port Integer }
  class HTTP < Object
    #: (String, ?Integer) -> HTTP
    def self.new(address, port = 80) = %x{ return &Net_HTTP{address: address, port: port} }

    #: () -> String
    def address = %x{ self.address }

    #: () -> Integer
    def port = %x{ self.port }

    #: (String, ?Hash[String, String]) -> HTTPResponse
    def get(path, headers = {}) = __request("GET", path, nil, headers)

    #: (String, ?Hash[String, String]) -> HTTPResponse
    def head(path, headers = {}) = __request("HEAD", path, nil, headers)

    #: (String, String, ?Hash[String, String]) -> HTTPResponse
    def post(path, data, headers = {}) = __request("POST", path, data, headers)

    #: (String, String, ?Hash[String, String]) -> HTTPResponse
    def put(path, data, headers = {}) = __request("PUT", path, data, headers)

    #: (String, String, ?Hash[String, String]) -> HTTPResponse
    def patch(path, data, headers = {}) = __request("PATCH", path, data, headers)

    #: (String, ?Hash[String, String]) -> HTTPResponse
    def delete(path, headers = {}) = __request("DELETE", path, nil, headers)

    #: () -> String
    def inspect = "#<Net::HTTP #{address}:#{port} open=false>"

    private

    #: (String, String, String?, Hash[String, String]) -> HTTPResponse
    def __request(method, path, body, headers) = %x{ rbHTTPRequest(self, string(method), string(path), body, headers) }
  end

  # @go_type struct { code String; message String; body String; header http.Header }
  class HTTPResponse < Object
    #: () -> String
    def code = %x{ self.code }

    #: () -> String
    def message = %x{ self.message }

    #: () -> String
    def body = %x{ self.body }

    # Case-insensitive; repeated headers are joined with ", " as Ruby does.
    #: (String) -> String?
    def [](name) = %x{
      values := self.header.Values(string(name))
      if len(values) == 0 {
        return nil
      }
      return Ref(String(strings.Join(values, ", ")))
    }

    #: (String) -> bool
    def key?(name) = %x{ Boolean(len(self.header.Values(string(name))) > 0) }

    #: () -> String
    def inspect = "#<Net::HTTPResponse #{code} #{message} readbody=true>"
  end
end
