# skip: is_a? on a non-local receiver whose static type decides the answer leaves an unused temp: go build fails "declared and not used"

# rbs_inline: enabled

class Vehicle
end

class Car < Vehicle
end

puts :s.is_a?(Symbol), [1].is_a?(Array), { "a" => 1 }.is_a?(Hash), Car.new.is_a?(Vehicle), Car.new.kind_of?(String)
