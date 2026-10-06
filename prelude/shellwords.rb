# prelude/shellwords.rb
# rbs_inline: enabled
#
# Shellwords: Bourne-shell word splitting and escaping, as MRI's. Always defined.

module Shellwords
  #: (String) -> Array[String]
  def self.shellsplit(line) = %x{
    out := &Array[String]{}
    for _, w := range rbShellSplit(string(line)) {
      out.s = append(out.s, String(w))
    }
    return out
  }

  #: (String) -> Array[String]
  def self.split(line) = shellsplit(line)

  #: (String) -> Array[String]
  def self.shellwords(line) = shellsplit(line)

  #: (untyped) -> String
  def self.shellescape(str) = %x{
    s := string(rbToS(str))
    if s == "" {
      return "''"
    }
    return String(strings.ReplaceAll(rbShellUnsafe.ReplaceAllString(s, `\\$0`), "\\n", "'\\n'"))
  }

  #: (untyped) -> String
  def self.escape(str) = shellescape(str)

  #: (Array[untyped]) -> String
  def self.shelljoin(words) = words.map { |w| shellescape(w) }.join(" ")

  #: (Array[untyped]) -> String
  def self.join(words) = shelljoin(words)
end

class String
  #: () -> Array[String]
  def shellsplit = Shellwords.shellsplit(self)

  #: () -> String
  def shellescape = Shellwords.shellescape(self)
end

class Array
  #: () -> String
  def shelljoin = Shellwords.shelljoin(self)
end
