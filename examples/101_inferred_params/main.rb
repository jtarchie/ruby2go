# rbs_inline: enabled

# No `#:` annotations: rb2go types each parameter from the calls the
# program makes, as it types return values from method bodies (example 34).

# A playlist of track lengths, in seconds.
class Playlist
  # `*lengths` is Array[Integer]: every Playlist.new passes Integers.
  def initialize(name, *lengths)
    @name = name
    @lengths = lengths
  end

  attr_reader :name #: String

  # The block's parameters are what each yield passes: Integer and Integer.
  def each_track
    @lengths.each_with_index { |s, i| yield i + 1, s }
  end

  def total = @lengths.sum
  def longer_than(limit) = @lengths.select { |s| s > limit }
end

# `seconds` is Integer, `pad` is String: its default and the one call agree.
def clock(seconds, pad = "0")
  "#{seconds / 60}:#{(seconds % 60).to_s.rjust(2, pad)}"
end

mix = Playlist.new("road trip", 185, 242, 97, 310)
puts "#{mix.name}: #{clock(mix.total)} total"
mix.each_track { |n, s| puts "  #{n}. #{clock(s)}" }
puts "over four minutes: #{mix.longer_than(240).map { |s| clock(s) }.join(", ")}"
puts clock(65, " ")

# A lambda's parameter types come from its calls.
fmt = ->(label, value) { "#{label.ljust(8)}#{value}" }
puts fmt.call("tracks", 4)
puts fmt.("longest", 310)
