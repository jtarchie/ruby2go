# prelude/uri.rb
# rbs_inline: enabled
#
# The form-encoding corner of URI.

module URI
  #: (Hash[String, String]) -> String
  def self.encode_www_form(form) = %x{
    parts := make([]string, 0, len(form.keys))
    for _, k := range form.keys {
      parts = append(parts, rbFormEscape(string(k))+"="+rbFormEscape(string(form.vals[k])))
    }
    return String(strings.Join(parts, "&"))
  }

  #: (String) -> String
  def self.encode_www_form_component(s) = %x{ String(rbFormEscape(string(s))) }

  #: (String) -> String
  def self.decode_www_form_component(s) = %x{
    out, err := url.QueryUnescape(string(s))
    if err != nil {
      panic(NewArgumentError(Ref(String("invalid %-encoding (" + string(s) + ")"))))
    }
    return String(out)
  }
end
