# rbs_inline: enabled

require_relative "date"

# `require "time"`: Time's parsers (decision 52). MRI's time.rb loads date.
class Time
  # `require "time"`'s parsers; each tries a fixed list of layouts
  # (rbTimeLayouts), not Date._parse's heuristics.
  #: (String) -> Time
  def self.parse(s) = %x{ return rbTimeParseOr(string(s), rbTimeLayouts, "no time information in "+string(rbStringInspect(string(s)))) }

  #: (String) -> Time
  def self.iso8601(s) = %x{ return rbTimeParseOr(string(s), rbTimeISOLayouts, "invalid xmlschema format: "+string(rbStringInspect(string(s)))) }

  #: (String) -> Time
  def self.xmlschema(s) = iso8601(s)

  #: (String) -> Time
  def self.httpdate(s) = %x{ return rbTimeParseOr(string(s), []string{"Mon, 02 Jan 2006 15:04:05 MST"}, "not RFC 2616 compliant date: "+string(rbStringInspect(string(s)))) }

  #: (String) -> Time
  def self.rfc2822(s) = %x{ return rbTimeParseOr(string(s), rbTimeRFC2822Layouts, "not RFC 2822 compliant date: "+string(rbStringInspect(string(s)))) }

  #: (String) -> Time
  def self.rfc822(s) = rfc2822(s)

  #: (String, String) -> Time
  def self.strptime(s, fmt) = %x{ return rbTimeStrptime(string(s), string(fmt)) }
end
