# skip: passing a T? (e.g. Integer?) to an `untyped?` parameter fails go build (*Integer is not *any); decision 20 says untyped? is untyped

# rbs_inline: enabled

#: (Integer) -> Integer?
def maybe(n) = n.positive? ? n : nil

#: (untyped?) -> String
def show(v) = v.inspect

puts show(maybe(-1)), show(maybe(4))
