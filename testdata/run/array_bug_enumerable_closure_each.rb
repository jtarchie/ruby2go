# skip: include Enumerable on a class whose each is a closure (it rescues around yield, decision 4) fails go build: Each(func(Integer)) does not satisfy Enumerable_Self, which wants Each() iter.Seq[E]

# rbs_inline: enabled
class Safe
  include Enumerable #[Integer]

  #: () { (Integer) -> void } -> void
  def each
    [1, 2].each do |x|
      begin
        yield x
      rescue ZeroDivisionError
        puts "rescued"
      end
    end
  end
end

puts Safe.new.to_a.inspect
Safe.new.each { |x| puts 10 / (x - 1) }
