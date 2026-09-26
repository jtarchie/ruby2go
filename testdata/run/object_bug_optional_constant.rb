# skip: a constant annotated `#: T?` with a non-nil initializer emits `NAME = *String(Ref[String]("n"))`, which Go parses as a dereference: go build fails "cannot convert Ref[String]("n") (value of type *String) to type String"

# rbs_inline: enabled

NAME = "n" #: String?
COUNT = 3 #: Integer?

module Settings
  LABEL = "l" #: String?
end

puts NAME.inspect, COUNT.inspect, Settings::LABEL.inspect
puts NAME.upcase if NAME
puts (COUNT || 0) + 1
