# rbs_inline: enabled

#: (Integer) -> Integer
def risky(n)
  raise ArgumentError, "neg" if n < 0
  n
end

risky(-1) rescue puts("statement fallback")
risky(1) rescue puts("not printed")
log = [] #: Array[String]
risky(-2) rescue log.clear
puts "done"
