# prelude/base64.rb
# rbs_inline: enabled
#
# Base64 on encoding/base64. Always defined, `require "base64"` or not.
# Keyword options arrive as a Hash (decision 23).

module Base64
  # RFC 2045: a newline after every 60 characters and at the end.
  #: (String) -> String
  def self.encode64(bin) = %x{
    s := base64.StdEncoding.EncodeToString([]byte(bin))
    var b strings.Builder
    for len(s) > 60 {
      b.WriteString(s[:60] + "\\n")
      s = s[60:]
    }
    if s != "" {
      b.WriteString(s + "\\n")
    }
    return String(b.String())
  }

  #: (String) -> String
  def self.decode64(str) = %x{
    out, _ := rbUnpackB64(string(str))
    return String(out)
  }

  #: (String) -> String
  def self.strict_encode64(bin) = %x{ String(base64.StdEncoding.EncodeToString([]byte(bin))) }

  #: (String) -> String
  def self.strict_decode64(str) = %x{
    out, err := base64.StdEncoding.Strict().DecodeString(string(str))
    if err != nil {
      panic(NewArgumentError(Ref(String("invalid base64"))))
    }
    return String(out)
  }

  #: (String, ?Hash[Symbol, bool]) -> String
  def self.urlsafe_encode64(bin, opts = {})
    s = strict_encode64(bin).tr("+/", "-_")
    opts[:padding] == false ? s.delete("=") : s
  end

  #: (String) -> String
  def self.urlsafe_decode64(str)
    str = str.ljust((str.size + 3) / 4 * 4, "=") if !str.end_with?("=") && str.size % 4 != 0
    strict_decode64(str.tr("-_", "+/"))
  end
end
