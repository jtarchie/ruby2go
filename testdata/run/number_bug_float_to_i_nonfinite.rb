# skip: Float#to_i/floor/ceil/round on Infinity or NaN return a garbage Integer; MRI raises FloatDomainError (a RangeError)
# rbs_inline: enabled

z = 0.0 #: Float
inf = 1.0 / z #: Float
nan = z / z #: Float
[inf, -inf, nan].each do |f|
  begin
    puts f.to_i.inspect
  rescue RangeError => e
    puts "to_i #{e.message}"
  end
  begin
    puts f.floor.inspect
  rescue RangeError => e
    puts "floor #{e.message}"
  end
  begin
    puts f.round.inspect
  rescue RangeError => e
    puts "round #{e.message}"
  end
end
puts "uncaught next"
puts inf.ceil.inspect
