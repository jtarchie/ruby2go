require "json"
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

    # Blocks on JSON's untyped values: a Hash takes Hash's method, an Array Array's (decision 224).
    def json_kids(json) = JSON.parse(json)["kids"].select { |k| k["age"] > 3 }.map { |k| k["name"] }

    def json_find(json) = JSON.parse(json)["sigs"]&.find { |s| s.dig("h", "kid") == "b" }

    def json_big(json) = JSON.parse(json).select { |_, v| v > 1 }

    # Read before anything assigns it: untyped, nil (decision 220).
    def never_set_or(d) = @never_set || d

    def self.cached = @cache

    # An optional dependency that is not loaded: MRI raises NameError only if the read runs (decision 218).
    def setup_optional(on)
      return :skipped unless on

      ::NotLoadedOptionalDep.setup(x: 1)
    end
  end
end
