# rbs_inline: enabled

#: (Integer) -> Integer
def dbl(n) = n * 2

class Animal
  #: () -> String
  def name = "animal"
end

#: (Animal) -> String
def greet(a) = "hi #{a.name}"

x = 5 #: Integer?
if x.is_a?(Integer)
  puts dbl(x)
end
y = nil #: Integer?
puts(y.is_a?(Integer) ? dbl(y) : -1)
pet = Animal.new #: Animal?
puts greet(pet) if pet.is_a?(Animal)
