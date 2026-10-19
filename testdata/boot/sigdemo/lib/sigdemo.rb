module Sigdemo
  # Unannotated: the gem's own sig/sigdemo.rbs types it; sig/broken.rbs only warns (decision 306).
  class Meter
    def initialize
      @readings = []
    end

    def add(value)
      @readings << value
      self
    end

    def push(value) = add(value)

    def total
      @readings.sum
    end

    def absorb(other)
      add(other.total)
    end

    def label(src)
      "#{src["name"]}:#{src["kind"]}"
    end

    def each_reading(sink)
      @readings.each { |r| sink.call(r) }
    end
  end
end
