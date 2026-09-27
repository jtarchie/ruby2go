# rbs_inline: enabled
#: (*String) -> Array[String]
def collect(*parts) = parts

words = ["p", "q"] #: Array[String]
got = collect(*words)
got[0] = "z"
puts words.inspect, got.inspect
