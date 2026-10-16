# rbs_inline: enabled

require_relative "cgi"
require_relative "strscan"

# ERB (decision 111). A template that is a string literal is compiled with
# the program: the compiler turns `ERB.new(literal)` into Ruby code, as
# ERB::Compiler does, and splices it where `result`, `result_with_hash` or
# `run` is called, so it runs in the caller's scope. The object itself
# only marks the template.
class ERB < Object
  #: () -> void
  def initialize = nil

  module Util
    #: (untyped) -> String
    def self.html_escape(s) = CGI.escapeHTML(s.to_s)

    #: (untyped) -> String
    def self.h(s) = CGI.escapeHTML(s.to_s)

    #: (untyped) -> String
    def html_escape(s) = CGI.escapeHTML(s.to_s)

    #: (untyped) -> String
    def h(s) = CGI.escapeHTML(s.to_s)

    # Every byte but A-Z a-z 0-9 _ - . ~ as %XX (uppercase), as MRI's.
    #: (untyped) -> String
    def self.url_encode(s) = %x{
      in := string(rbToS(s))
      var b strings.Builder
      for i := 0; i < len(in); i++ {
        c := in[i]
        if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.' || c == '~' {
          b.WriteByte(c)
        } else {
          fmt.Fprintf(&b, "%%%02X", c)
        }
      }
      return String(b.String())
    }

    #: (untyped) -> String
    def self.u(s) = url_encode(s)

    #: (untyped) -> String
    def url_encode(s) = ERB::Util.url_encode(s)

    #: (untyped) -> String
    def u(s) = ERB::Util.url_encode(s)
  end
end
