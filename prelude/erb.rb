# rbs_inline: enabled

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
  end
end
