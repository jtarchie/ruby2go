# HTTP/1.x request parser. Produces a Tep::Request from the raw
# byte blob the C helper read off the wire (headers, possibly a
# prefix of the body).
module Tep
  class Parser
    # Returns a fully-populated Request, or nil if the blob is malformed.
    def self.parse(blob)
      # String#index returns nil (not -1) when not found — matches CRuby.
      # See matz/spinel#532; spinel 0210389 fixed the prior -1 sentinel.
      end_of_headers = blob.index("\r\n\r\n")
      return nil if end_of_headers.nil?

      headers_blob = blob[0, end_of_headers]
      lines = headers_blob.split("\r\n")
      return nil if lines.length.zero?

      first = lines[0]
      first_parts = first.split(" ")
      return nil if first_parts.length < 3

      req = Request.new
      req.verb         = first_parts[0]
      req.raw_path     = first_parts[1]
      req.http_version = first_parts[2]

      qmark = req.raw_path.index("?")
      if qmark.nil?
        req.path = req.raw_path
      else
        req.path  = req.raw_path[0, qmark]
        qstring   = req.raw_path[qmark + 1, req.raw_path.length - qmark - 1]
        req.query = Url.parse_query(qstring)
      end

      i = 1
      while i < lines.length
        line = lines[i]
        colon = line.index(":")
        unless colon.nil?
          name  = line[0, colon].downcase
          value = line[colon + 1, line.length - colon - 1].strip
          req.req_headers[name] = value
        end
        i += 1
      end

      # Pre-merge query into params; path captures will be folded in
      # by the router on a successful match.
      req.query.each do |k, v|
        req.req_params[k] = v
      end

      # Parse Cookie header into req.cookies. Format: "k=v; k2=v2; ...".
      # Whitespace around `;` is allowed and stripped.
      cookie_blob = req.req_headers["cookie"]
      if cookie_blob.length.positive?
        cookie_blob.split(";").each do |pair|
          eq = pair.index("=")
          next unless !eq.nil? && eq.positive?

          cname  = pair[0, eq].strip
          cvalue = pair[eq + 1, pair.length - eq - 1].strip
          req.cookies[cname] = Url.unescape(cvalue)
        end
      end

      # Carry over any body bytes already in the blob (the C helper
      # may have read more than just the headers in one recv()).
      body_start = end_of_headers + 4
      req.raw_body = blob[body_start, blob.length - body_start] if body_start < blob.length

      req
    end
  end
end
