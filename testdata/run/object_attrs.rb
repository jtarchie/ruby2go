# rbs_inline: enabled

# attr_reader/writer/accessor, ivar typing (initialize dry run, `#:`, `# @rbs`), sharing with subclasses.

class Account
  attr_reader :owner #: String
  attr_accessor :balance, :limit #: Integer
  attr_writer :note #: String?
  attr_accessor :nick #: String?

  # @rbs @history: Array[Integer]

  # @rbs @cache: String?

  #: (String, ?Integer) -> void
  def initialize(owner, balance = 0)
    @owner = owner
    @balance = balance
    @limit = -100
    @note = nil
    @nick = nil
    @history = []
    @tags = {} #: Hash[Symbol, String]
    @ratio = 0.5
  end

  #: (Integer) -> self
  def deposit(n)
    self.balance = balance + n
    @history << n
    self
  end

  #: (Integer) -> bool
  def withdraw(n)
    return false if balance - n < limit
    self.balance = balance - n
    @history << -n
    true
  end

  #: () -> Array[Integer]
  def history = @history

  #: () -> String
  def note_text = @note || "(none)"

  #: () -> String
  def summary
    @cache ||= "#{owner}:#{balance}"
  end

  #: () -> void
  def reset_cache
    @cache = nil
  end

  #: (Symbol, String) -> void
  def tag(k, v)
    @tags[k] = v
  end

  #: () -> Hash[Symbol, String]
  def tags = @tags

  #: () -> Float
  def ratio = @ratio

  #: () -> String
  def display_name
    n = nick
    return n.upcase if n
    owner
  end
end

class Savings < Account
  attr_reader :rate #: Float

  #: (String, Float) -> void
  def initialize(owner, rate)
    super(owner, 100)
    @rate = rate
    @limit = 0
  end

  #: () -> Integer
  def interest = (balance.to_f * rate).floor

  #: () -> Integer
  def first_deposit = @history.first(1).fetch(0)
end

a = Account.new("ann")
puts a.owner.inspect, a.balance.inspect, a.limit.inspect
a.deposit(50).deposit(25)
puts a.balance.inspect, a.history.inspect
puts a.withdraw(100).inspect, a.balance.inspect, a.withdraw(80).inspect, a.balance.inspect
puts a.history.inspect

puts a.note_text
a.note = "vip"
puts a.note_text
a.note = nil
puts a.note_text

puts a.nick.inspect, a.display_name
a.nick = "ännie"
puts a.nick.inspect, a.display_name

a.balance = 0
a.limit = a.limit + 5
puts a.balance.inspect, a.limit.inspect

puts a.summary
a.deposit(1)
puts a.summary
a.reset_cache
puts a.summary

a.tag(:b, "two")
a.tag(:a, "one")
a.tag(:b, "deux")
puts a.tags.inspect, a.ratio.inspect

a.balance = 42
puts a.balance.inspect

s = Savings.new("sam", 0.25)
puts s.owner, s.balance.inspect, s.limit.inspect, s.rate.inspect
s.deposit(20).history
puts s.interest.inspect, s.first_deposit.inspect, s.history.inspect
puts s.withdraw(121).inspect, s.withdraw(120).inspect, s.balance.inspect
puts s.summary

accounts = [a, s] #: Array[Account]
puts accounts.map(&:balance).inspect, accounts.map { |x| x.history.size }.inspect
accounts.each { |x| x.balance = x.balance * 2 }
puts accounts.map(&:balance).inspect

big = Account.new("big", 9_223_372_036_854_775_000)
puts big.balance.inspect

# Optional attrs narrowed by `unless`/`if`/ternary/`&.`; a setter drops the narrowing;
# an ivar first assigned outside initialize; a lazy `@x ||=`; an ivar updated inside a block.
class Profile
  attr_accessor :nick #: String?
  attr_reader :parent #: Profile?

  #: (String?, ?Profile?) -> void
  def initialize(nick, parent = nil)
    @nick = nick
    @parent = parent
  end

  #: () -> String
  def shout
    return "anon" unless nick
    nick.upcase
  end

  #: () -> String
  def lineage
    if parent
      "#{nick || "?"} < #{parent.lineage}"
    else
      nick || "root"
    end
  end

  #: () -> Integer
  def depth = parent ? parent.depth + 1 : 0

  #: () -> String
  def parent_nick = parent&.nick || "none"

  #: () -> String
  def reset
    return "none" unless nick
    first = nick.upcase
    self.nick = nil
    "#{first} then #{nick.inspect}"
  end
end

root = Profile.new(nil)
kid = Profile.new("kid", root)
baby = Profile.new("baby", kid)
puts root.shout, kid.shout, baby.lineage, root.lineage, baby.depth.inspect, root.depth.inspect
puts baby.parent_nick, root.parent_nick, kid.parent_nick
baby.nick = nil
puts baby.shout, baby.nick.inspect
kid.nick = "k2"
puts baby.lineage, kid.reset, kid.reset, kid.nick.inspect
up = baby.parent
puts up.lineage if up
puts (up&.parent&.nick).inspect, (root.parent&.nick).inspect

class Lazy
  #: () -> Array[Integer]
  def data
    @data ||= [1, 2]
  end

  #: () -> void
  def load
    @loaded = "yes"
    @sum = 0
    data.each { |x| @sum += x }
  end

  #: () -> String
  def loaded = "#{@loaded} #{@sum}"
end

lz = Lazy.new
lz.data << 3
lz.load
puts lz.data.inspect, lz.loaded

# A def after attr_accessor replaces its reader; a hand-written writer replaces attr's;
# a subclass restates an inherited attr_reader with the same type.
class Person
  attr_accessor :name #: String
  attr_reader :age #: Integer

  #: (String, Integer) -> void
  def initialize(name, age)
    @name = name
    @age = age
  end

  #: () -> String
  def name = @name.capitalize

  #: (Integer) -> void
  def age=(v)
    @age = v < 0 ? 0 : v
  end
end

class Kid < Person
  attr_reader :age #: Integer

  #: () -> String
  def info = "#{name}/#{age}"
end

per = Person.new("ann", 3)
puts per.name
per.name = "bob"
per.age = -4
puts per.name, per.age.inspect
kd = Kid.new("cy", 2)
kd.age = 7
puts kd.info
