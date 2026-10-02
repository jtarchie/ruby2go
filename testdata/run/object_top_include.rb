# rbs_inline: enabled
# standalone: a top-level include is Object's, which changes every class's ancestors

# a top-level include (Object's, as in MRI) was a run-time NoMethodError
module TopInclude
  #: () -> Integer
  def top_five = 5
end
include TopInclude

class TopIncluder
  #: () -> Integer
  def six = top_five + 1
end

p top_five, TopIncluder.new.six, 3.top_five
p Object.ancestors.include?(TopInclude)
