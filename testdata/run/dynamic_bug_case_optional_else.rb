# skip: in `case x when C ... else <uses x> end` with x a T? local, the Go type switch shadows x as `any` but the else arm still treats it as *T: go build fails "cannot indirect x"; MRI runs it

# rbs_inline: enabled

class Animal
  #: () -> String
  def name = "animal"
end

class Dog < Animal
  #: () -> String
  def bark = "woof"
end

#: (Animal?) -> String
def describe(a)
  case a
  when Dog then "dog #{a.bark}"
  when nil then "none"
  else "other #{a.name}"
  end
end

#: (String?) -> String
def text(s)
  case s
  when nil then "nil"
  else "text #{s.size}"
  end
end

puts describe(Dog.new), describe(nil), describe(Animal.new), text("abc"), text(nil)
