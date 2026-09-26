# skip: a top-level def with an RBS type parameter is emitted without Go type params (undefined: T)

# rbs_inline: enabled
#: [T] (Array[T]) -> T?
def second(arr) = arr[1]

puts second([1, 2, 3]).inspect, second(["a"]).inspect
