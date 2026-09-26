# skip: an Array[Integer] where Array[untyped] is expected (an Array[untyped] param, `[] + nums`) is passed unconverted and go build fails (*Array[Integer] vs *Array[any])

# rbs_inline: enabled
#: (Array[untyped]) -> String
def show(xs) = xs.map { |x| x.inspect }.join(" ")

nums = [1, 2] #: Array[Integer]
puts show(nums)
puts ([] + nums).inspect
