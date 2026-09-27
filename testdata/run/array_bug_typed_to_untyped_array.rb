# rbs_inline: enabled
#: (Array[untyped]) -> String
def show(xs) = xs.map { |x| x.inspect }.join(" ")

nums = [1, 2] #: Array[Integer]
puts show(nums)
puts ([] + nums).inspect
