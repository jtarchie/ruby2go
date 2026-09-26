# skip: a literal that is the only witness for a method type parameter ([T] (T)) is passed as a Go untyped constant, so T becomes Go string/int (to_s is "#<string>", inspect panics), and nil cannot infer T (go build)

# rbs_inline: enabled

class Fmt
  # @rbs [T] (T) -> String
  def show(x) = "<#{x}>|#{x.inspect}"
end

f = Fmt.new
puts f.show("s"), f.show(12), f.show(2.5), f.show(:sym), f.show("a" + "b")
puts f.show(nil)
