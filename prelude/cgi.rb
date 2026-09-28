# prelude/cgi.rb
# rbs_inline: enabled
#
# cgi/escape: CGI's URL and HTML escaping (all that remains of CGI in Ruby 4).

module CGI
  # Space becomes +; alphanumerics and _.-~ stay; everything else is %XX.
  #: (String) -> String
  def self.escape(s) = %x{ String(rbCGIEscape(string(s), "+")) }

  #: (String) -> String
  def self.escapeURIComponent(s) = %x{ String(rbCGIEscape(string(s), "%20")) }

  # + becomes space; malformed %-escapes are kept as written.
  #: (String) -> String
  def self.unescape(s) = %x{ String(rbCGIUnescape(string(s), true)) }

  #: (String) -> String
  def self.unescapeURIComponent(s) = %x{ String(rbCGIUnescape(string(s), false)) }

  #: (String) -> String
  def self.escapeHTML(s) = %x{ String(rbHTMLEscaper.Replace(string(s))) }

  # Only the entities MRI decodes: amp quot gt lt apos and numeric references.
  #: (String) -> String
  def self.unescapeHTML(s) = %x{ String(rbHTMLUnescape(string(s))) }

  #: (String) -> String
  def self.escape_html(s) = escapeHTML(s)

  #: (String) -> String
  def self.unescape_html(s) = unescapeHTML(s)

  #: (String) -> String
  def self.h(s) = escapeHTML(s)
end
