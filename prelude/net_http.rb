# prelude/net_http.rb
# rbs_inline: enabled
#
# Net::HTTP's client on Go's net/http: no redirects, every status returned; open_timeout dials, read_timeout bounds the rest of the round trip.

module Timeout
  class Error < RuntimeError; end
end

module Net
  class OpenTimeout < Timeout::Error; end
  class ReadTimeout < Timeout::Error; end

  # @go_type struct { address String; port Integer; useSSL Boolean; openTimeout Float; readTimeout Float }
  class HTTP < Object
    #: (String, ?Integer) -> HTTP
    def self.new(address, port = 80) = %x{ return &Net_HTTP{address: address, port: port, openTimeout: 60, readTimeout: 60} }

    #: (String, Integer) { (HTTP) -> void } -> void
    def self.start(address, port)
      yield new(address, port)
    end

    #: (URI::Generic) -> String
    def self.get(uri) = get_response(uri).body

    #: (URI::Generic) -> HTTPResponse
    def self.get_response(uri) = __for(uri).get(uri.request_uri)

    #: (URI::Generic, Hash[String, String]) -> HTTPResponse
    def self.post_form(uri, params)
      form = { "Content-Type" => "application/x-www-form-urlencoded" }
      __for(uri).post(uri.request_uri, URI.encode_www_form(params), form)
    end

    #: (URI::Generic) -> HTTP
    def self.__for(uri)
      http = new(uri.host || "", uri.port || 80)
      http.use_ssl = uri.scheme == "https"
      http
    end

    #: () -> String
    def address = %x{ self.address }

    #: () -> Integer
    def port = %x{ self.port }

    #: () -> bool
    def use_ssl? = %x{ self.useSSL }

    #: (bool) -> void
    def use_ssl=(v)
      %x{ self.useSSL = v }
    end

    #: () -> Float
    def open_timeout = %x{ self.openTimeout }

    #: (Float) -> void
    def open_timeout=(v)
      %x{ self.openTimeout = v }
    end

    #: () -> Float
    def read_timeout = %x{ self.readTimeout }

    #: (Float) -> void
    def read_timeout=(v)
      %x{ self.readTimeout = v }
    end

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

    # Sends a pre-built request object (Net::HTTP::Get.new(path) and friends).
    #: (HTTPGenericRequest) -> HTTPResponse
    def request(req) = __request(req.method, req.path, req.body, req.header)

    #: () -> String
    def inspect = "#<Net::HTTP #{address}:#{port} open=false>"

    private

    #: (String, String, String?, Hash[String, String]) -> HTTPResponse
    def __request(method, path, body, headers) = %x{ rbHTTPRequest(self, string(method), string(path), body, headers) }

    # A request built by hand (Net::HTTP::Get.new(path), req["H"] = "v",
    # req.body = ..., http.request(req)) instead of the get/post/etc.
    # convenience methods.
    class HTTPGenericRequest < Object
      #: (String, ?Hash[String, String]?) -> void
      def initialize(path, initheader = nil)
        @path = path
        @header = {}
        initheader&.each { |k, v| @header[k] = v }
        @body = nil
      end

      attr_reader :path #: String
      attr_reader :header #: Hash[String, String]
      attr_reader :body #: String?

      #: () -> String
      def method = ""

      #: (String) -> String?
      def [](name) = %x{
        if v, ok := self.Header().vals[String(rbHTTPCanonKey(string(name)))]; ok {
          return Ref(v)
        }
        return nil
      }

      #: (String, String) -> void
      def []=(name, value)
        %x{ self.Header().Op_idxSet(String(rbHTTPCanonKey(string(name))), value) }
      end

      #: (String) -> void
      def body=(data)
        @body = data
      end

      #: (String, String) -> void
      def basic_auth(user, pass)
        self["Authorization"] = "Basic #{Base64.strict_encode64("#{user}:#{pass}")}"
      end

      #: (Hash[String, String]) -> void
      def set_form_data(params)
        self["Content-Type"] = "application/x-www-form-urlencoded"
        self.body = URI.encode_www_form(params)
      end
    end

    class Get < HTTPGenericRequest
      #: () -> String
      def method = "GET"
    end

    class Head < HTTPGenericRequest
      #: () -> String
      def method = "HEAD"
    end

    class Post < HTTPGenericRequest
      #: () -> String
      def method = "POST"
    end

    class Put < HTTPGenericRequest
      #: () -> String
      def method = "PUT"
    end

    class Patch < HTTPGenericRequest
      #: () -> String
      def method = "PATCH"
    end

    class Delete < HTTPGenericRequest
      #: () -> String
      def method = "DELETE"
    end

    class Options < HTTPGenericRequest
      #: () -> String
      def method = "OPTIONS"
    end
  end

  # By status: MRI's Net::HTTPResponse::CODE_TO_OBJ, checked against
  # `ruby -rnet/http -e 'p Net::HTTPResponse::CODE_TO_OBJ'`. `case res when
  # Net::HTTPSuccess` etc. works because these are real subclasses.
  class HTTPResponse < Object
    #: (String, String, String, Hash[String, String]) -> void
    def initialize(code, message, body, header)
      @code = code
      @message = message
      @body = body
      @header = header
    end

    attr_reader :code #: String
    attr_reader :message #: String
    attr_reader :body #: String
    attr_reader :header #: Hash[String, String]

    # Case-insensitive; repeated headers are joined with ", " as Ruby does.
    #: (String) -> String?
    def [](name) = %x{
      if v, ok := self.Header().vals[String(rbHTTPCanonKey(string(name)))]; ok {
        return Ref(v)
      }
      return nil
    }

    #: (String) -> bool
    def key?(name) = %x{
      _, ok := self.Header().vals[String(rbHTTPCanonKey(string(name)))]
      return Boolean(ok)
    }

    #: () -> String
    def inspect = "#<#{__class_name} #{code} #{message} readbody=true>"
  end

  class HTTPUnknownResponse < HTTPResponse; end
  class HTTPInformation < HTTPResponse; end
  class HTTPSuccess < HTTPResponse; end
  class HTTPRedirection < HTTPResponse; end
  class HTTPClientError < HTTPResponse; end
  class HTTPServerError < HTTPResponse; end

  class HTTPContinue < HTTPInformation; end
  class HTTPSwitchProtocol < HTTPInformation; end
  class HTTPProcessing < HTTPInformation; end
  class HTTPEarlyHints < HTTPInformation; end

  class HTTPOK < HTTPSuccess; end
  class HTTPCreated < HTTPSuccess; end
  class HTTPAccepted < HTTPSuccess; end
  class HTTPNonAuthoritativeInformation < HTTPSuccess; end
  class HTTPNoContent < HTTPSuccess; end
  class HTTPResetContent < HTTPSuccess; end
  class HTTPPartialContent < HTTPSuccess; end
  class HTTPMultiStatus < HTTPSuccess; end
  class HTTPAlreadyReported < HTTPSuccess; end
  class HTTPIMUsed < HTTPSuccess; end

  class HTTPMultipleChoices < HTTPRedirection; end
  class HTTPMovedPermanently < HTTPRedirection; end
  class HTTPFound < HTTPRedirection; end
  class HTTPSeeOther < HTTPRedirection; end
  class HTTPNotModified < HTTPRedirection; end
  class HTTPUseProxy < HTTPRedirection; end
  class HTTPTemporaryRedirect < HTTPRedirection; end
  class HTTPPermanentRedirect < HTTPRedirection; end

  class HTTPBadRequest < HTTPClientError; end
  class HTTPUnauthorized < HTTPClientError; end
  class HTTPPaymentRequired < HTTPClientError; end
  class HTTPForbidden < HTTPClientError; end
  class HTTPNotFound < HTTPClientError; end
  class HTTPMethodNotAllowed < HTTPClientError; end
  class HTTPNotAcceptable < HTTPClientError; end
  class HTTPProxyAuthenticationRequired < HTTPClientError; end
  class HTTPRequestTimeout < HTTPClientError; end
  class HTTPConflict < HTTPClientError; end
  class HTTPGone < HTTPClientError; end
  class HTTPLengthRequired < HTTPClientError; end
  class HTTPPreconditionFailed < HTTPClientError; end
  class HTTPPayloadTooLarge < HTTPClientError; end
  class HTTPURITooLong < HTTPClientError; end
  class HTTPUnsupportedMediaType < HTTPClientError; end
  class HTTPRangeNotSatisfiable < HTTPClientError; end
  class HTTPExpectationFailed < HTTPClientError; end
  class HTTPMisdirectedRequest < HTTPClientError; end
  class HTTPUnprocessableEntity < HTTPClientError; end
  class HTTPLocked < HTTPClientError; end
  class HTTPFailedDependency < HTTPClientError; end
  class HTTPUpgradeRequired < HTTPClientError; end
  class HTTPPreconditionRequired < HTTPClientError; end
  class HTTPTooManyRequests < HTTPClientError; end
  class HTTPRequestHeaderFieldsTooLarge < HTTPClientError; end
  class HTTPUnavailableForLegalReasons < HTTPClientError; end

  class HTTPInternalServerError < HTTPServerError; end
  class HTTPNotImplemented < HTTPServerError; end
  class HTTPBadGateway < HTTPServerError; end
  class HTTPServiceUnavailable < HTTPServerError; end
  class HTTPGatewayTimeout < HTTPServerError; end
  class HTTPVersionNotSupported < HTTPServerError; end
  class HTTPVariantAlsoNegotiates < HTTPServerError; end
  class HTTPInsufficientStorage < HTTPServerError; end
  class HTTPLoopDetected < HTTPServerError; end
  class HTTPNotExtended < HTTPServerError; end
  class HTTPNetworkAuthenticationRequired < HTTPServerError; end
end
