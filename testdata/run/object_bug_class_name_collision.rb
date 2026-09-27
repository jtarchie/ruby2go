# rbs_inline: enabled

class Opt
  #: () -> String
  def to_s = "opt"
end

class Ref
  #: () -> String
  def to_s = "ref"
end

puts Opt.new, Ref.new
