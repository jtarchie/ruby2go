# net-ftp's and drb's shapes in gem code (decision 416).
class Ftpish
  ONE = ->(v) { v }
  TWO = ->(v, local = false) {
    unless /\A(?<year>\d{4})(?:\.(?<frac>\d+))?/ =~ v
      raise ArgumentError, "bad"
    end

    usec = frac.to_i * 10 if frac
    [year, usec, local]
  }
  PARSERS = Hash.new(ONE)
  PARSERS["t"] = TWO
  KIND = :named if respond_to?(:name)

  class Placeholder
    def read = "none"
  end

  class Real
    def read = "real"
  end

  class Wire
    def initialize = (@sent = [])
    attr_reader :sent

    def send(msg, flags = 0)
      @sent << [msg, flags]
      msg.size
    end
  end

  def initialize
    @io = Placeholder.new
  end

  def connect
    @io = Real.new
    @io.read
  end

  def parse(k, v) = PARSERS[k].call(v)

  def get(text = nil)
    result = String.new unless text
    [1, 2].each { |x| result&.concat(x.to_s) }
    result
  end

  def drain(chunk)
    buf = +""
    rv = [chunk, 1, :x, "y"][0]
    buf << rv
    rv.clear
    [buf, rv]
  end

  def push(io, line) = io.send(line, 2)

  def by_name(obj, m) = obj.send(m)

  def soft_require
    require("set") rescue nil
    :required
  end

  ruby2_keywords def forward(*args) = args

  def pump
    n = 0
    [1].each do
      loop do
        n += 1
        break if n > 2
      end
    end
    n
  end

  module Proto
    def open(uri, config, first = true) = [uri, config, first]
    module_function :open
  end
end
