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
