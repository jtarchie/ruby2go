# rbs_inline: enabled

class Animal
  attr_reader :name #: String

  #: (String) -> void
  def initialize(name)
    @name = name
  end
end

class Dog < Animal
  #: () -> String
  def bark = "woof from #{name}"
end

class Puppy < Dog
  #: () -> String
  def yip = "yip from #{name}"
end

class Cat < Animal
  #: () -> String
  def meow = "meow from #{name}"
end

#: (untyped) -> String
def classify(v)
  case v
  when nil then "nil"
  when Integer then "Integer #{v + 1}"
  when Float then "Float #{v * 2}"
  when String then "String #{v.upcase} (#{v.size})"
  when Symbol then "Symbol #{v.inspect}"
  when Array then "Array of #{v.size}: #{v.inspect}"
  when Hash then "Hash with keys #{v.keys.inspect}"
  when Puppy then "Puppy: #{v.yip}"
  when Dog then "Dog: #{v.bark}"
  when Animal then "Animal #{v.name}"
  else "something else: #{v.inspect}"
  end
end

#: (untyped) -> String
def numeric_or_text(v)
  case v
  when Integer, Float then "number #{v.inspect}"
  when String, Symbol then "text #{v.inspect}"
  when nil then "nothing"
  else "other"
  end
end

#: (Animal) -> String
def speak(a)
  case a
  when Puppy then a.yip
  when Dog then a.bark
  when Cat then a.meow
  else "... from #{a.name}"
  end
end

#: (Animal) -> String
def first_match_wins(a)
  case a
  when Animal then "Animal first"
  when Dog then "never Dog"
  else "never else"
  end
end

#: (String?) -> String
def opt_case(s)
  case s
  when nil then "nil string"
  when String then "string #{s.size}"
  else "unreachable"
  end
end

#: (untyped) -> untyped
def ident(v) = v

module Zoo
  class Keeper
    #: () -> String
    def name = "keeper"
  end

  class Vet < Keeper
  end
end

module Tools
end

#: (untyped) -> String
def staff(v)
  case v
  when Zoo::Vet then "vet #{v.name}"
  when Zoo::Keeper then "keeper #{v.name}"
  else "visitor"
  end
end

#: (untyped) -> String
def what(v)
  case v
  when Class then "class #{v.name}"
  when Module then "module #{v.name}"
  when Exception then "exception #{v.message}"
  else "value"
  end
end

#: (untyped) -> String
def shape_of(v)
  case v
  when Array
    v.select { |e| !e.nil? }.map { |e| e.to_s }.join(",")
  when Hash
    v.keys.map { |k| k.to_s }.sort.join("/")
  else
    "leaf"
  end
end

#: (Animal?) -> String
def opt_animal(a)
  case a
  when Dog then "dog #{a.bark}"
  when nil then "no animal"
  else "some animal"
  end
end

#: (Integer?) -> String
def opt_int(n)
  case n
  when Integer then "int #{n + 1}"
  else "nil"
  end
end

puts "-- case/when on classes over untyped values"
vals = [nil, 0, -7, 2.5, "héllo", "", :sym, [1, "a"], [], { "a" => 1, "b" => 2 }, {}, true, false] #: Array[untyped]
vals.each { |v| puts classify(v) }
puts classify(Puppy.new("rex")), classify(Dog.new("fido")), classify(Cat.new("tom")), classify(Animal.new("gen"))

puts "-- several classes in one when"
vals.each { |v| puts numeric_or_text(v) }

puts "-- case on a typed struct subject narrows to subclasses"
animals = [Puppy.new("p"), Dog.new("d"), Cat.new("c"), Animal.new("a")] #: Array[Animal]
animals.each { |a| puts speak(a), first_match_wins(a) }

puts "-- case on T?"
puts opt_case(nil), opt_case("abc"), opt_case("")

puts "-- case on an expression, as a value"
kind = case ident(42)
       when String then "s"
       when Integer then "i"
       else "?"
       end
puts kind
label = case ident(nil)
        when nil then "was nil"
        else "not nil"
        end
puts label
out = case ident([3, 4])
      when Array then "array"
      else "no"
      end
puts out

puts "-- case without a matching branch and no else yields nil"
res = case ident(1.5)
      when String then "s"
      when Integer then "i"
      end
puts res.inspect

puts "-- case/when on exception objects"
[ArgumentError.new("bad arg"), KeyError.new("no key"), RuntimeError.new("boom")].each do |err|
  msg = case err
        when ArgumentError then "arg: #{err.message}"
        when KeyError then "key: #{err.message}"
        when StandardError then "std: #{err.message}"
        else "?"
        end
  puts msg
end

puts "-- case/when on values (== / ===)"
[1, 2, 5, 10].each do |n|
  word = case n
         when 1 then "one"
         when 2, 3 then "two or three"
         else "many"
         end
  puts word
end
["apple", "Banana", "cherry", ""].each do |s|
  puts(case s
       when /^a/ then "starts with a"
       when /^[A-Z]/ then "capitalized"
       when "" then "empty"
       else "other"
       end)
end

puts "-- namespaced classes, class objects and exceptions in when"
puts staff(Zoo::Vet.new), staff(Zoo::Keeper.new), staff("x")
puts what(Dog), what(Tools), what(String), what(Comparable), what(KeyError.new("kk")), what(3), what(nil)

puts "-- narrowed arms take blocks"
puts shape_of([1, nil, "b", 2.5]), shape_of({ "b" => 1, :a => 2 }), shape_of("s")

puts "-- class whens on a T? subject narrow it"
puts opt_animal(Puppy.new("pp")), opt_animal(nil), opt_animal(Cat.new("cc")), opt_int(4), opt_int(nil)
