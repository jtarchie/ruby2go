# rbs_inline: enabled

# The name and reference patterns of rexml 3.4.4's xmltokens.rb and parsers/baseparser.rb, verbatim, for programs
# that build their own regexps from them (kramdown); rb2go's REXML parses in Go (decision 114), so the parser itself is not here.
module REXML
  module XMLTokens
    # rexml builds these from local Arrays of ranges; their values, as literals
    NAME_START_CHAR = "[:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\u{10000}-\\u{EFFFF}]"
    NAME_CHAR = "[:A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\u{10000}-\\u{EFFFF}\\-\\.0-9\\u00B7\\u0300-\\u036F\\u203F-\\u2040]"
    NAMECHAR = NAME_CHAR # deprecated. Use NAME_CHAR instead.

    NCNAME_STR = "[A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\u{10000}-\\u{EFFFF}][A-Z_a-z\\u00C0-\\u00D6\\u00D8-\\u00F6\\u00F8-\\u02FF\\u0370-\\u037D\\u037F-\\u1FFF\\u200C-\\u200D\\u2070-\\u218F\\u2C00-\\u2FEF\\u3001-\\uD7FF\\uF900-\\uFDCF\\uFDF0-\\uFFFD\\u{10000}-\\u{EFFFF}\\-\\.0-9\\u00B7\\u0300-\\u036F\\u203F-\\u2040]*"
    NAME_STR = "(?:#{NCNAME_STR}:)?#{NCNAME_STR}"

    NAME = "(#{NAME_START_CHAR}#{NAME_CHAR}*)"
    NMTOKEN = "(?:#{NAME_CHAR})+"
    NMTOKENS = "#{NMTOKEN}(\\s+#{NMTOKEN})*"
    REFERENCE = "(?:&#{NAME};|&#\\d+;|&#x[0-9a-fA-F]+;)"
  end

  module Parsers
    class BaseParser
      LETTER = "[:alpha:]"
      DIGIT = "[:digit:]"

      COMBININGCHAR = "" # TODO
      EXTENDER = ""      # TODO

      NCNAME_STR = "[#{LETTER}_][-[:alnum:]._#{COMBININGCHAR}#{EXTENDER}]*"
      QNAME_STR = "(?:(#{NCNAME_STR}):)?(#{NCNAME_STR})"
      QNAME = /(#{QNAME_STR})/

      # Just for backward compatibility. For example, kramdown uses this.
      UNAME_STR = "(?:#{NCNAME_STR}:)?#{NCNAME_STR}"

      NAMECHAR = '[\-\w\.:]'
      NAME = "([\\w:]#{NAMECHAR}*)"
      NMTOKEN = "(?:#{NAMECHAR})+"
      NMTOKENS = "#{NMTOKEN}(\\s+#{NMTOKEN})*"
      REFERENCE = "&(?:#{NAME};|#\\d+;|#x[0-9a-fA-F]+;)"
      REFERENCE_RE = /#{REFERENCE}/
    end
  end
end
