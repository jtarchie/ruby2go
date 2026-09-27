# rbs_inline: enabled

class Ctr
  #: () -> void
  def initialize
    @n = 0
  end

  #: () -> String
  def nxt
    @n += 1
    puts "nxt called"
    @n.to_s
  end
end

c = Ctr.new
re = /x#{c.nxt}/
puts re.source, re.match?("x1"), c.nxt
