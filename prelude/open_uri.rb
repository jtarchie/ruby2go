# rbs_inline: enabled

# Entry points for internal/compiler/openuri.go, which splits MRI's mixed options Hash at compile time (decision 134).

class StringIO
  #: (Array[String], Hash[String, Array[String]]) -> void
  def __oum_init(status, metas)
    %x{ self.oum = &rbOpenURIMeta{status: status, metas: metas} }
  end

  #: (String) -> void
  def __oum_base(uri)
    %x{ self.rbOUM("base_uri").base = &uri }
  end

  #: () -> Array[String]
  def status = %x{ self.rbOUM("status").status }

  #: () -> URI::Generic?
  def base_uri
    s = __oum_base_s
    s ? URI.parse(s) : nil
  end

  #: () -> String?
  def __oum_base_s = %x{ self.rbOUM("base_uri").base }

  #: () -> Hash[String, String]
  def meta = %x{ rbOpenURIJoined(self.rbOUM("meta").metas) }

  #: () -> Hash[String, Array[String]]
  def metas = %x{ self.rbOUM("metas").metas }

  #: () -> Time?
  def last_modified
    v = __oum_field("last_modified", "last-modified")
    v ? Time.httpdate(v) : nil
  end

  #: (String, String) -> String?
  def __oum_field(meth, name) = %x{
    if v, ok := rbOpenURIField(self.rbOUM(string(meth)).metas, name); ok {
      return Ref(String(v))
    }
    return nil
  }

  #: () -> String
  def content_type = %x{
    if v, ok := rbOpenURIField(self.rbOUM("content_type").metas, "content-type"); ok {
      if typ, _, _, ok := rbOpenURIMediaType(v); ok {
        return String(typ)
      }
    }
    return "application/octet-stream"
  }

  #: () ?{ () -> String? } -> String?
  def charset(&blk)
    cs = __charset
    return cs if cs
    return yield if block_given?
    content_type.start_with?("text/") && __content_type_parsed? ? "utf-8" : nil
  end

  #: () -> String?
  def __charset = %x{
    if v, ok := rbOpenURIField(self.rbOUM("charset").metas, "content-type"); ok {
      if _, cs, has, ok := rbOpenURIMediaType(v); ok && has {
        return Ref(String(cs))
      }
    }
    return nil
  }

  #: () -> bool
  def __content_type_parsed? = %x{
    v, ok := rbOpenURIField(self.oum.metas, "content-type")
    if !ok {
      return false
    }
    _, _, _, ok = rbOpenURIMediaType(v)
    return Boolean(ok)
  }

  #: () -> Array[String]
  def content_encoding = %x{
    v, ok := rbOpenURIField(self.rbOUM("content_encoding").metas, "content-encoding")
    if !ok {
      return NewArray[String]()
    }
    return rbOpenURICodings(v)
  }
end

module OpenURI
  class HTTPError < StandardError
    #: (String, StringIO) -> void
    def initialize(message, io)
      super(message)
      @io = io
    end

    attr_reader :io #: StringIO
  end

  class HTTPRedirect < HTTPError
    #: (String, StringIO, URI::Generic) -> void
    def initialize(message, io, uri)
      super(message, io)
      @uri = uri
    end

    attr_reader :uri #: URI::Generic
  end

  class TooManyRedirects < HTTPError; end

  # MRI's OpenURI::Options keys rb2go acts on; any other key is a compile error.
  class OpenOptions < Object
    #: (?read_timeout: Float?, ?open_timeout: Float?, ?redirect: bool, ?max_redirects: Integer?, ?http_basic_authentication: Array[String]?, ?progress_proc: (^(Integer) -> void)?, ?content_length_proc: (^(Integer?) -> void)?, ?ssl_verify_mode: Integer?) -> void
    def initialize(read_timeout: nil, open_timeout: nil, redirect: true, max_redirects: nil, http_basic_authentication: nil, progress_proc: nil, content_length_proc: nil, ssl_verify_mode: nil)
      @read_timeout = read_timeout
      @open_timeout = open_timeout
      @redirect = redirect
      @max_redirects = max_redirects
      @http_basic_authentication = http_basic_authentication
      @progress_proc = progress_proc
      @content_length_proc = content_length_proc
      @ssl_verify_mode = ssl_verify_mode
    end

    attr_reader :read_timeout #: Float?
    attr_reader :open_timeout #: Float?
    attr_reader :redirect #: bool
    attr_reader :max_redirects #: Integer?
    attr_reader :http_basic_authentication #: Array[String]?
    attr_reader :progress_proc #: (^(Integer) -> void)?
    attr_reader :content_length_proc #: (^(Integer?) -> void)?
    attr_reader :ssl_verify_mode #: Integer?
  end

  # A local file comes back as a StringIO, not MRI's File, so URI.open has one return type.
  #: (String, Hash[String, String], OpenOptions, ?String) -> StringIO
  def self.__open_name(name, headers, opts, mode = "r")
    if name.match?(%r{\A[A-Za-z][A-Za-z0-9+\-.]*://})
      uri = URI.parse(name)
      return __open_uri(uri, headers, opts, mode) if uri.is_a?(URI::HTTP)
      raise NotImplementedError, "open-uri: #{uri.scheme} is not supported" if (uri.scheme || "").downcase == "ftp"
    end
    unless __read_mode?(mode)
      raise NotImplementedError, "URI.open of a local file in mode #{mode} is not supported; use File.open"
    end
    StringIO.new(File.read(name))
  end

  #: [T] (String, Hash[String, String], OpenOptions, ?String) { (StringIO) -> T } -> T
  def self.__open_name_block(name, headers, opts, mode = "r")
    io = __open_name(name, headers, opts, mode)
    begin
      yield io
    ensure
      io.close unless io.closed?
    end
  end

  #: (String) -> bool
  def self.__read_mode?(mode) = mode.match?(/\Arb?(?:\n?\z|:[^:])/)

  # MRI's OpenURI.open_uri and open_loop.
  #: (URI::Generic, Hash[String, String], OpenOptions, ?String) -> StringIO
  def self.__open_uri(uri, headers, opts, mode = "r")
    raise ArgumentError, "invalid access mode #{mode} (#{uri.class} resource is read only.)" unless __read_mode?(mode)
    uri_set = {} #: Hash[String, bool]
    max = opts.max_redirects || 64
    auth = opts.http_basic_authentication
    cur = uri
    while true
      raise NotImplementedError, "open-uri: #{cur.scheme} is not supported" unless cur.is_a?(URI::HTTP)
      res, io = __open_http(cur, headers, opts, auth)
      status = io.status.join(" ")
      case res
      when Net::HTTPSuccess
        io.__oum_base(cur.to_s)
        break
      when Net::HTTPMovedPermanently, Net::HTTPFound, Net::HTTPSeeOther, Net::HTTPTemporaryRedirect, Net::HTTPPermanentRedirect
        loc = res["location"]
        raise HTTPError.new("#{status} (Invalid Location URI)", io) unless loc
        begin
          target = URI.parse(loc)
        rescue URI::InvalidURIError
          raise HTTPError.new("#{status} (Invalid Location URI)", io)
        end
        target = cur.merge(loc) unless target.scheme
        raise HTTPRedirect.new(status, io, target) unless opts.redirect
        raise "redirection forbidden: #{cur} -> #{target}" unless __redirectable?(cur, target)
        auth = nil
        cur = target
        raise "HTTP redirection loop: #{cur}" if uri_set.key?(cur.to_s)
        uri_set[cur.to_s] = true
        raise TooManyRedirects.new("Too many redirects", io) if uri_set.size > max
      else
        raise HTTPError.new(status, io)
      end
    end
    io
  end

  #: (URI::Generic, URI::Generic) -> bool
  def self.__redirectable?(from, to)
    a = (from.scheme || "").downcase
    b = (to.scheme || "").downcase
    a == b || (a == "http" || a == "ftp") && (b == "http" || b == "https" || b == "ftp")
  end

  # MRI's OpenURI.open_http, minus proxies.
  #: (URI::HTTP, Hash[String, String], OpenOptions, Array[String]?) -> [Net::HTTPResponse, StringIO]
  def self.__open_http(uri, headers, opts, auth)
    raise ArgumentError, "userinfo not supported.  [RFC3986]" if uri.userinfo
    http = Net::HTTP.new(uri.host || "", uri.port || 80)
    if uri.is_a?(URI::HTTPS)
      http.use_ssl = true
      http.verify_mode = opts.ssl_verify_mode || OpenSSL::SSL::VERIFY_PEER
    end
    rt = opts.read_timeout
    http.read_timeout = rt if rt
    ot = opts.open_timeout
    http.open_timeout = ot if ot
    req = Net::HTTP::Get.new(uri.request_uri, headers)
    req.basic_auth(auth[0] || "", auth[1] || "") if auth
    res = http.request(req)
    if res.is_a?(Net::HTTPSuccess)
      clp = opts.content_length_proc
      if clp
        len = res["content-length"]
        clp.call(len ? len.to_i : nil)
      end
      pp = opts.progress_proc
      pp.call(res.body.bytesize) if pp && !res.body.empty?
    end
    io = StringIO.new(res.body)
    io.__oum_init([res.code, res.message], res.to_hash)
    [res, io]
  end
end

module URI
  class Generic
    #: (Hash[String, String], OpenURI::OpenOptions, ?String) -> StringIO
    def __open(headers, opts, mode = "r")
      raise NoMethodError, "undefined method 'open' for an instance of #{self.class}" unless is_a?(URI::HTTP)
      OpenURI.__open_uri(self, headers, opts, mode)
    end

    #: [T] (Hash[String, String], OpenURI::OpenOptions, ?String) { (StringIO) -> T } -> T
    def __open_block(headers, opts, mode = "r")
      io = __open(headers, opts, mode)
      begin
        yield io
      ensure
        io.close unless io.closed?
      end
    end

    #: (Hash[String, String], OpenURI::OpenOptions) -> String
    def __read(headers, opts)
      raise NoMethodError, "undefined method 'read' for an instance of #{self.class}" unless is_a?(URI::HTTP)
      io = OpenURI.__open_uri(self, headers, opts)
      s = io.read
      io.close
      s
    end
  end
end
