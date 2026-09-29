# prelude/abbrev.rb - rbs_inline: enabled - Abbrev: unambiguous word abbreviations, as MRI's lib/abbrev.rb. Always defined.

module Abbrev
  # A prefix seen twice is ambiguous and dropped; MRI stops shortening further once that happens (`case`'s `else`/`break`).

  #: (Array[String], ?untyped) -> Hash[String, String]
  def self.abbrev(words, pattern = nil)
    table = {} #: Hash[String, String]
    seen = {} #: Hash[String, Integer]

    words.each do |word|
      next if word.empty?

      word.size.downto(1) do |len|
        ab = word[0, len] || ""
        next unless __abbrev_match?(ab, pattern)

        seen[ab] = (seen[ab] || 0) + 1
        case seen[ab]
        when 1
          table[ab] = word
        when 2
          table.delete(ab)
        else
          break
        end
      end
    end

    words.each do |word|
      next unless __abbrev_match?(word, pattern)

      table[word] = word
    end

    table
  end

  # A String pattern is a prefix, not a literal Regexp (rb2go has no runtime Regexp.new).

  #: (String, untyped) -> bool
  def self.__abbrev_match?(candidate, pattern)
    return true if pattern.nil?
    return candidate.start_with?(pattern) if pattern.is_a?(String)

    pattern.match?(candidate)
  end
end

class Array
  # `self` is generic Array[E]; Go can't pass it where Array[String] is wanted, so normalize first (a no-op for E = String, since String#to_s is self).

  #: (?untyped) -> Hash[String, String]
  def abbrev(pattern = nil) = Abbrev.abbrev(map { |w| w.to_s }, pattern)
end
