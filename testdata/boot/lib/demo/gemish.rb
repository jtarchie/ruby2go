# Gem code rb2go's typing would reject, compiled leniently as rack's multipart parser and request are (decision 175).
module Demo
  class Gemish
    Pair = Struct.new :key, :value

    class Part < Struct.new(:body, :name)
      def label = "#{name}=#{body}"
    end

    KINDS = %w[text html].freeze

    attr_reader :state

    def initialize
      @state = :ready
    end

    def kind?(headers) = KINDS.include?(headers["kind"])

    def joined
      out = String.new
      out << "a" << "b" << "c"
      out
    end

    def first_big(xs)
      xs.each { |x| return x if x > 2 }
      nil
    end

    def recheck
      Integer("x")
    rescue ArgumentError => e
      raise e.class, e.message, cause: nil
    end
  end
end
