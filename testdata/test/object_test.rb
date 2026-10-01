# rbs_inline: enabled

require "minitest/autorun"
require "singleton"
require "forwardable"

# Helpers for the checks that were testdata/run/object_bug_class_name_generated_collision.rb.
class User
  #: () -> String
  def to_s = "user"
end

class NewUser
  #: () -> String
  def to_s = "new user"
end

#: (singleton(ObjectTests::NcSquare)) -> void
def make(k)
  k.new(1)
end

# Helpers for the checks that were testdata/run/object_bugs.rb.
# user classes named like runtime helpers (Opt, Ref) must not clash with them
class Opt
  #: () -> String
  def to_s = "opt"
end

class Ref
  #: () -> String
  def to_s = "ref"
end

#: () -> Integer
def lim = ObjectTests::BG_LIMIT

#: (String?) -> String
def bg_kind(s) = s.class.name

# Joins of class objects and of instances (ternary, case), class objects as Hash keys,
# and a subclass reopened without restating its superclass.
#: (bool) -> singleton(ObjectTests::CmShape)
def object_pick(flag) = flag ? ObjectTests::CmSquare : ObjectTests::Tri

#: (Integer) -> ObjectTests::CmShape
def by_num(n)
  case n
  when 0 then ObjectTests::CmShape.new(0)
  when 1 then ObjectTests::CmSquare.new(1)
  else ObjectTests::Tri.new(2)
  end
end

OBJECT_TOP = 99

OBJECT_NS_NAME = "top"

OBJECT_NS_LIMIT = 10

#: (String) -> Integer
def rank_of(name) = ObjectTests::Kinds.const_get(name).rank

# A computed name on CtChild joins its own and inherited constants (Integer + String: untyped);
# same-typed constants join to their type; Struct/Data classes are constants too.
#: (String) -> untyped
def child_const(n) = ObjectTests::CtChild.const_get(n)

#: (Integer, ?Integer) -> Integer
def top(a, b = a + ObjectTests::DA_LIMIT) = a * b

#: (ObjectTests::DaBase) -> Integer
def via_base(b) = b.calc

#: (ObjectTests::InAnimal) -> String
def in_kind(a)
  case a
  when ObjectTests::Puppy then "puppy #{a.bark}"
  when ObjectTests::InDog then "dog #{a.bark}"
  when ObjectTests::Cat then "cat"
  else "other"
  end
end

#: (Class) -> String
def cname(k) = k.name

#: (Module) -> String
def mname(k) = k.name

#: (Integer) -> ObjectTests::MtVec
def vec(n) = ObjectTests::MtVec.new(n)

#: (String) -> String
def object_trace(s)
  ObjectTests::NS_TRACE << "init #{s}"
  s
end

#: (Integer) -> ObjectTests::Coin
def coin(n) = ObjectTests::Coin.new(n)

#: (ObjectTests::OcBase) -> String
def object_show(b)
  "#{b.name} #{b.me.name} #{b.dup_me.name}"
end

#: (ObjectTests::ListNode?) -> Integer
def object_total(n)
  sum = 0
  while n
    sum += n.val
    n = n.nxt
  end
  sum
end

#: (ObjectTests::Tree) -> Integer
def tsum(t) = t.val + t.kids.reduce(0) { |a, k| a + tsum(k) }

# Helpers for the checks that were testdata/run/object_visibility.rb.
#: (Integer) -> String
def top_helper(n) = "top#{n}"

module ObjectTests
  # Helpers for the checks that were testdata/run/object_attrs.rb.
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

  # A def after attr_accessor replaces its reader; a hand-written writer replaces attr's;
  # a subclass restates an inherited attr_reader with the same type.
  class AtPerson
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

  class AtKid < AtPerson
    attr_reader :age #: Integer

    #: () -> String
    def info = "#{name}/#{age}"
  end

  # Helpers for the checks that were testdata/run/object_bug_attr_op_assign.rb.
  class AoCounter
    attr_accessor :n #: Integer
    attr_accessor :label #: String?

    #: () -> void
    def initialize
      @n = 0
      @label = nil
    end

    #: (Integer) -> void
    def drop(k)
      self.n -= k
    end
  end

  AoPoint = Struct.new(:x, :y) #: [Integer, Integer]

  class Shape
    #: () -> String
    def to_s = "shape"
  end

  class ShapeI
    #: () -> String
    def to_s = "shape i"
  end

  module Foo
    class Bar
      #: () -> String
      def to_s = "Foo::Bar"
    end
  end

  class Foo_Bar
    #: () -> String
    def to_s = "Foo_Bar"
  end

  # Helpers for the checks that were testdata/run/object_bug_comparable_struct.rb.
  class Version
    include Comparable

    attr_reader :major #: Integer
    attr_reader :minor #: Integer

    #: (Integer, Integer) -> void
    def initialize(major, minor)
      @major = major
      @minor = minor
    end

    def <=>(other)
      c = major <=> other.major
      return c unless c == 0
      minor <=> other.minor
    end

    #: () -> String
    def to_s = "v#{major}.#{minor}"
  end

  # Helpers for the checks that were testdata/run/object_bug_const_get_messages.rb.
  module CgM
  end

  # Helpers for the checks that were testdata/run/object_bug_hook_methods.rb.
  HK_LOG = [] #: Array[String]

  class Plugin
    #: (untyped) -> void
    def self.inherited(sub)
      HK_LOG << "inherited by #{sub}"
    end
  end

  class Alpha < Plugin
  end

  module Tracked
    #: (untyped) -> void
    def self.included(base)
      HK_LOG << "included in #{base}"
    end

    #: (untyped) -> void
    def self.extended(base)
      HK_LOG << "extended #{base}"
    end
  end

  class HkThing
    include Tracked
  end

  class HkOther
    extend Tracked
  end

  # Helpers for the checks that were testdata/run/object_bug_module_super.rb.
  module MsLoud
    #: () -> String
    def hi = "loud " + super
  end

  class MsA
    #: () -> String
    def hi = "a"
  end

  class MsB < MsA
    include MsLoud
  end

  # The target follows the module in each includer's ancestors: another
  # module, the includer's superclass, or nothing (NoMethodError).
  MS_LOG = [] #: Array[String]

  module Shout
    #: (Integer) -> Integer
    def calc(x) = super(x + 1) * 2

    #: (Integer) -> void
    def initialize(n)
      super
      MS_LOG << "shout init #{n}"
    end

    #: () -> String
    def hi = "shout " + super
  end

  module Quiet
    #: () -> String
    def hi = "quiet " + super
  end

  module MsBoth
    include Shout

    #: () -> String
    def hi = "both " + super
  end

  class P
    #: (Integer) -> void
    def initialize(n)
      @n = n
      MS_LOG << "p init #{n}"
    end

    #: () -> String
    def hi = "p#{@n}"

    #: (Integer) -> Integer
    def calc(x) = x + 100
  end

  class MsQ < P
    include Shout
  end

  class R < MsQ
    #: () -> String
    def hi = "r " + super
  end

  class S < P
    include Shout
    include Quiet
  end

  class T < P
    include MsBoth
  end

  class U
    include Quiet
  end

  # Helpers for the checks that were testdata/run/object_bug_narrowed_call_stmt.rb.
  class NcBuilder
    #: () -> void
    def initialize
      @parts = [] #: Array[String]
    end

    #: (String) -> self
    def add(part)
      @parts << part
      self
    end

    #: () -> String
    def build = @parts.join("-")
  end

  class NcHtmlBuilder < NcBuilder
  end

  module NcPlugins
    VERSION = "1.0"

    class Alpha
    end
  end

  NC_LOG = [] #: Array[String]

  class NcShape
    #: (Integer) -> void
    def initialize(n)
      NC_LOG << "made #{n}"
    end
  end

  class NcSquare < NcShape
  end

  # lim runs before BG_LIMIT is assigned
  BG_LIM_ERR = begin
    lim.to_s
  rescue NameError => e
    "NameError: #{e.message}"
  end

  BG_LIMIT = 5

  # const_get on an optional module constant keeps its nil
  module Settings
    DEFAULT = nil #: String?
  end

  # const_get through a non-module value raises TypeError
  module BgPlugins
    VERSION = "1.0"
  end

  # Data#with with no args returns self; with args, a new object
  Coord = Data.define(:lat, :lng) #: [Float, Float]

  # frozen? on a plain object, a Struct and a Data
  class BgPlain
  end

  BgPair = Struct.new(:a, :b) #: [Integer, Integer]

  BgVal = Data.define(:v) #: [Integer]

  # is_a? on a local used nowhere else
  class BgAnimal
  end

  class BgDog < BgAnimal
  end

  # constants/const_get/const_defined? see an included module's constants
  module BgConfig
    BG_LIMIT = 5
  end

  class BgUses
    include BgConfig

    OWN = 1
  end

  # optional constants at top level and in a module
  BG_NAME = "n" #: String?

  COUNT = 3 #: Integer?

  module Labels
    LABEL = "l" #: String?
  end

  # Struct/Data members named like keywords (begin, end, in, out)
  Span = Struct.new(:begin, :end) #: [Integer, Integer]

  Link = Data.define(:in, :out) #: [String, String]

  # a member named `other` does not shadow ==(other)
  Edge = Struct.new(:from, :other) #: [String, String]

  LinkOther = Data.define(:other) #: [Integer]

  class TapBox
    attr_reader :n #: Integer

    #: (Integer) -> void
    def initialize(n)
      @n = n
    end
  end

  SG_LOG = [] #: Array[String]

  class SgConfig
    include Singleton
    attr_accessor :level #: Integer

    #: () -> void
    def initialize
      @level = 1
      SG_LOG << "init"
    end
  end

  module SgApp
    class Registry
      include Singleton

      #: () -> void
      def initialize
        @names = [] #: Array[String]
      end

      #: (String) -> Array[String]
      def add(n) = @names << n
    end
  end

  # Helpers for the checks that were testdata/run/object_class_methods.rb.
  class CmShape
    attr_reader :size #: Integer

    #: (Integer) -> void
    def initialize(size)
      @size = size
    end

    #: () -> String
    def self.kind = "shape"

    #: () -> Integer
    def self.sides = 0

    #: () -> String
    def self.info = "#{kind}/#{sides}"

    #: (Integer) -> CmShape
    def self.build(n) = new(n + 1)

    #: (Integer) -> CmShape
    def self.build_explicit(n) = self.new(n * 2)

    #: () -> String
    def describe = "#{self.class.kind}:#{self.class.name}(#{size}) sides=#{self.class.sides}"

    #: () -> CmShape
    def grow = self.class.new(size + 100)
  end

  class CmSquare < CmShape
    def self.kind = "square"

    def self.sides = 4

    def self.info = "sq " + super
  end

  class Tri < CmShape
    def self.sides = 3
  end

  class Tiny < CmSquare
    #: () -> void
    def initialize = super(1)

    def self.kind = "tiny"
  end

  class CmRegistry
    #: () -> Integer
    def self.count
      @count ||= 0
    end

    #: () -> void
    def self.bump
      @count = count + 1
    end

    #: (String) -> String
    def self.fmt(s) = "<#{s}>"
  end

  class SubRegistry < CmRegistry
    def self.fmt(s) = super(s.upcase) + "!"
  end

  class Color
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    RED = Color.new("red")
    ALL = [RED, Color.new("blue")] #: Array[Color]

    #: () -> Color
    def self.default = RED

    #: (String) -> String
    def self.fmt(s) = "<#{s}>"
  end

  class Shade < Color
    #: (String, ?Integer) -> void
    def initialize(name, depth = 3)
      super(name)
      @depth = depth
    end

    def self.fmt(s) = super + "!"

    #: () -> String
    def self.plain = fmt("p")

    #: () -> Integer
    def depth = @depth
  end

  class CmDeep < Shade
    def initialize(name, depth = 9) = super
  end

  class Color
    #: () -> String
    def shout = name.upcase
  end

  class CmSquare
    #: () -> String
    def self.extra = "reopened #{kind}"

    #: () -> Integer
    def area = size * size
  end

  # A module's own class-level ivar; `self` in a class method is the (virtual) class object;
  # a class referenced before its definition; a constant holding class objects.
  module Tally
    #: () -> Integer
    def self.count
      @count ||= 0
    end

    #: () -> void
    def self.bump
      @count = count + 1
    end
  end

  class CmShape
    #: () -> singleton(CmShape)
    def self.me = self

    #: () -> Maker
    def maker = Maker.new(self)
  end

  class Maker
    #: (CmShape) -> void
    def initialize(s)
      @s = s
    end

    #: () -> String
    def made = "made #{@s.class.kind} #{@s.size}"
  end

  HANDLERS = [CmShape, CmSquare, Tiny]

  # Class methods taking value blocks (not iterators), with a default argument.
  class Conf
    attr_accessor :port #: Integer

    #: () -> void
    def initialize
      @port = 80
    end

    #: () { (Conf) -> void } -> Conf
    def self.build
      c = new
      yield c
      c
    end

    #: (?Integer) { (Integer) -> String } -> String
    def self.fmt(n = 1) = yield(n * 2)
  end

  # An explicit `< Object`; class-body constants built by a qualified class-method call and
  # from earlier constants, in source order.
  class CmTemp < Object
    attr_reader :deg #: Integer

    #: (Integer) -> void
    def initialize(deg)
      @deg = deg
    end

    #: (Integer) -> CmTemp
    def self.of(d) = new(d)

    FREEZING = CmTemp.of(0)
    BOILING = CmTemp.new(100)
    RANGE = BOILING.deg - FREEZING.deg
  end

  # A registry of class objects in a constant; an ivar holding a class object (a factory);
  # a module's to_s used by puts and interpolation.
  class Handler
    REGISTRY = {} #: Hash[String, singleton(Handler)]

    #: () -> String
    def self.tag = "base"

    #: (String, singleton(Handler)) -> void
    def self.register(name, k)
      REGISTRY[name] = k
    end

    #: (String) -> String
    def self.dispatch(name)
      k = REGISTRY[name]
      return "none" unless k
      k.new.run
    end

    #: () -> String
    def run = "run #{self.class.tag}"
  end

  class Posts < Handler
    def self.tag = "posts"
  end

  class Users < Handler
    def self.tag = "users"
  end

  class Factory
    #: (singleton(Handler)) -> void
    def initialize(klass)
      @klass = klass
    end

    #: () -> Handler
    def make = @klass.new

    #: () -> String
    def kind = @klass.tag
  end

  module Pretty
    #: () -> String
    def title = raise(NotImplementedError)

    #: () -> String
    def to_s = "pretty #{title}"
  end

  class CmDoc
    include Pretty

    #: () -> String
    def title = "d"
  end

  # respond_to? on class objects: class methods, subclass-only class methods through singleton(C), new, name.
  class Blob
    #: () -> String
    def self.kind = "blob"

    #: () -> String
    def area = "a"
  end

  class SubBlob < Blob
    #: () -> String
    def self.only_sub = "sub"
  end

  # A class's own name/to_s override: to_s and inspect do not call a custom name.
  class CmWidget
    #: () -> String
    def self.name = "CustomWidget"
  end

  class CmGadget
    #: () -> String
    def self.to_s = "GadgetClass"
  end

  # Helpers for the checks that were testdata/run/object_class_new_arity.rb.
  class NaBase
    #: (Integer) -> void
    def initialize(n)
      @n = n
    end

    #: () -> Integer
    def n = @n
  end

  class Two < NaBase
    #: (Integer, Integer) -> void
    def initialize(a, b)
      super(a + b)
    end
  end

  class Same < NaBase
  end

  # Helpers for the checks that were testdata/run/object_cmp_nil.rb.
  class Score
    attr_reader :v #: Float

    #: (Float) -> void
    def initialize(v)
      @v = v
    end

    #: (Score) -> Integer?
    def <=>(other) = v <=> other.v

    #: () -> String
    def to_s = "S#{v}"
  end

  class Bonus < Score
  end

  # Helpers for the checks that were testdata/run/object_constants.rb.
  module CtPlugins
    VERSION = "1.0"
    MAX = 3

    class Base
      #: () -> String
      def self.id = "base"

      #: () -> String
      def run = "base run"
    end

    class Alpha < Base
      def self.id = "alpha"

      def run = "alpha run"
    end

    class Beta < Base
      def self.id = "beta"
    end
  end

  module Kinds
    class Root
      #: () -> Integer
      def self.rank = 0
    end

    class One < Root
      def self.rank = 1
    end

    class Two < Root
      def self.rank = 2
    end
  end

  class CtParent
    COLOR = "red"
  end

  class CtChild < CtParent
    SIZE = 5
  end

  module Limits
    LOW = 1
    HIGH = 9
  end

  module CtGeo
    Coord = Data.define(:lat, :lng) #: [Float, Float]
    Tag = Struct.new(:text) #: [String]
    ORIGIN = Coord.new(0.0, 0.0)
  end

  # Helpers for the checks that were testdata/run/object_default_args_callee.rb.
  DA_LIMIT = 3

  module Greets
    #: (?String) -> String
    def hello(who = default_who) = "hello #{who}"

    #: () -> String
    def default_who = "module"
  end

  class DaBase
    include Greets

    #: () -> void
    def initialize
      @n = 10
    end

    #: (?Integer, ?Integer) -> Integer
    def calc(a = @n, b = a * 2) = a + b

    #: (Integer, ?Integer, *Integer) -> Integer
    def sum(first, second = first + 1, *rest) = first + second + rest.size

    #: (?Integer) { (Integer) -> void } -> void
    def times_up(n = DA_LIMIT)
      n.times { |i| yield i }
    end
  end

  class DaKid < DaBase
    #: (?Integer, ?Integer) -> Integer
    def calc(a = 1, b = 2) = super() * 100 + a + b

    #: () -> String
    def default_who = "kid"
  end

  class DaBox
    #: (Integer, ?Integer) -> void
    def initialize(w, h = w + 1)
      @w = w
      @h = h
    end

    #: () -> Integer
    def area = @w * @h
  end

  class DaShape
    #: (?String) -> String
    def self.make(kind = default_kind) = "made #{kind}"

    #: () -> String
    def self.default_kind = "shape"
  end

  class Circle < DaShape
    #: () -> String
    def self.default_kind = "circle"
  end

  class Log
    #: () -> void
    def initialize
      @lines = [] #: Array[String]
    end

    #: (String) -> String
    def note(s)
      @lines << s
      s
    end

    # the default is never read, but runs for its side effect
    #: (String, ?String) -> String
    def f(a, b = note("side")) = a

    #: () -> Integer
    def count = @lines.size
  end

  # Helpers for the checks that were testdata/run/object_extend_block.rb.
  module Describable
    #: (Integer) -> Integer
    def twice(n) = n * 2

    #: () -> String
    def describe = "I am #{name}"
  end

  class EbWidget
    extend Describable

    #: () -> String
    def self.kind = "widget"
  end

  class EbGadget < EbWidget
  end

  module Tools
    extend Describable
  end

  module EbColors
    extend Enumerable #[String]

    RED = "red"
    GREEN = "green"
    BLUE = "blue"

    #: () { (String) -> void } -> void
    def self.each(&block)
      [RED, GREEN, BLUE].each(&block)
    end
  end

  module Handlers
    extend Enumerable #[singleton(Handlers::Base)]

    class Base
      #: (String) -> bool
      def self.matches?(path) = false
    end

    class Posts < Base
      def self.matches?(path) = path.start_with?("/posts")
    end

    class Users < Base
      def self.matches?(path) = path.start_with?("/users")
    end

    #: () { (singleton(Base)) -> void } -> void
    def self.each(&block)
      constants.sort.map { |c| const_get(c) }.each(&block)
    end
  end

  class EbBag
    #: () -> void
    def initialize
      @items = [] #: Array[Integer]
    end

    #: (Integer) -> self
    def add(n)
      @items << n
      self
    end

    #: () { (Integer) -> void } -> void
    def each(&block) = @items.each(&block)

    #: () { (Integer) -> Integer } -> Array[Integer]
    def transform(&block) = @items.map(&block)

    #: () { (Integer) -> bool } -> Array[Integer]
    def keep(&block) = @items.select(&block)

    #: () { (Integer) -> String } -> String
    def first_as(&block) = block.call(@items.first(1).fetch(0))

    #: () { (Integer) -> void } -> void
    def each_twice(&block)
      each(&block)
      each(&block)
    end
  end

  # A class method overrides a method its class got from `extend`; the extended method
  # dispatches back to the override, per subclass, also through singleton(C).
  module Labelled
    #: () -> String
    def label = "#{name}:#{kind}"

    #: () -> String
    def kind = "base"
  end

  class Knob
    extend Labelled

    def self.kind = "knob"
  end

  class BigKnob < Knob
    def self.kind = "big"
  end

  class Lever
    extend Labelled
  end

  # Helpers for the checks that were testdata/run/object_inheritance.rb.
  class InBase
    attr_reader :tag #: String

    #: (String) -> void
    def initialize(tag)
      @tag = tag
    end

    #: () -> String
    def name = "InBase"

    #: () -> Integer
    def weight = 1

    #: () -> String
    def describe = "#{name}[#{tag}] w=#{weight}"

    #: (Integer) -> Integer
    def scale(n) = n * weight

    #: (String, ?String) -> String
    def greet(who, punct = "!") = "hi #{who}#{punct}"

    #: () -> String
    def to_s = "<#{name} #{tag}>"
  end

  class Middle < InBase
    def name = "Middle"

    def weight = super * 10

    def scale(n) = super(n) + 1

    def greet(who, punct = "?") = super.upcase
  end

  class InLeaf < Middle
    #: (String, Integer) -> void
    def initialize(tag, extra)
      super(tag)
      @extra = extra
    end

    def name = "InLeaf#{@extra}"

    def weight = super + @extra

    def describe = "leaf: #{super}"

    #: () -> Integer
    def extra = @extra
  end

  class Empty < InBase
  end

  class Fixed < InBase
    #: () -> void
    def initialize = super("fixed")
  end

  class Root
    #: () -> void
    def initialize
      super
      @n = 7 #: Integer
    end

    #: () -> Integer
    def n = @n
  end

  class Summer
    SCALE = 2

    #: (*Integer) -> void
    def initialize(*nums)
      @nums = nums
    end

    #: () -> Integer
    def total = @nums.reduce(0) { |a, b| a + b }

    #: () -> Integer
    def scaled = total * SCALE
  end

  class KidSummer < Summer
    SCALE = 5

    def initialize(*nums) = super

    #: () -> Integer
    def kid_scaled = total * SCALE
  end

  class InAnimal
    #: () -> String
    def name = "animal"
  end

  class InDog < InAnimal
    #: () -> String
    def bark = "woof"
  end

  class Puppy < InDog
  end

  class Cat < InAnimal
  end

  # zsuper passes the parameters' current values; super() passes none; super inside a block.
  class InCalc
    #: (Integer, ?Integer) -> Integer
    def compute(n, k = 10) = n * k

    #: () -> String
    def plain = "calc"

    #: () -> Array[String]
    def parts = ["a", "b"]

    #: (String) -> String
    def tag(s) = "<#{s}>"
  end

  class InKidCalc < InCalc
    def compute(n, k = 3)
      n += 1
      k *= 2
      super
    end

    def plain = super() + "!"

    def parts = super.map { |p| tag(p) + plain }

    def tag(s) = [1, 2].map { |i| super(s * i) }.join
  end

  class GrandCalc < InKidCalc
    def compute(n, k = 1) = super(n) + super(n, k)
  end

  # Three-level initialize chain with defaults; sibling subclasses with same-named ivars of different types.
  class InNode
    attr_reader :id #: Integer

    #: (?Integer) -> void
    def initialize(id = 1)
      @id = id
    end
  end

  class InNamed < InNode
    attr_reader :label #: String

    #: (String, ?Integer) -> void
    def initialize(label, id = 2)
      super(id)
      @label = label
    end
  end

  class Leafy < InNamed
    #: () -> void
    def initialize
      super("leafy")
    end
  end

  class IntBox < InNode
    #: () -> void
    def initialize
      super()
      @v = 41
    end

    #: () -> Integer
    def v = @v + 1
  end

  class StrBox < InNode
    #: () -> void
    def initialize
      super(9)
      @v = "s"
    end

    #: () -> String
    def v = @v * 2
  end

  # The abstract-method idiom: raise NotImplementedError in the base, which a bare rescue does not catch.
  class Figure
    #: () -> Integer
    def area = raise(NotImplementedError, "#{self.class.name}#area")

    #: () -> String
    def show = "area #{area}"
  end

  class Sq < Figure
    def area = 4
  end

  # An inherited `-> self` method keeps the subclass type through a chain, a local, and an array.
  class InBuilder
    #: () -> void
    def initialize
      @parts = [] #: Array[String]
    end

    #: (String) -> self
    def add(part)
      @parts << part
      self
    end

    #: () -> String
    def build = @parts.join("-")
  end

  class InHtmlBuilder < InBuilder
    #: () -> String
    def html = "<p>#{build}</p>"
  end

  # Helpers for the checks that were testdata/run/object_mid.rb.
  # an attr writer call's value is the assigned value, not the setter's return
  class MdBox
    attr_accessor :n #: Integer

    #: () -> void
    def initialize
      @n = 0
    end
  end

  # a class body can call its own class methods and new while defining constants
  class MdTemp
    attr_reader :deg #: Integer

    #: (Integer) -> void
    def initialize(deg)
      @deg = deg
    end

    #: (Integer) -> MdTemp
    def self.of(d) = new(d)

    FREEZING = of(0)
    BOILING = new(100)
  end

  # yielding class methods on a module and a class iterate from a call site
  module MdColors
    #: () { (String) -> void } -> void
    def self.each
      yield "red"
      yield "green"
    end
  end

  class MdCounter
    #: (Integer) { (Integer) -> void } -> void
    def self.upto(n)
      i = 0
      while i < n
        yield i
        i += 1
      end
    end
  end

  # inherit=false hides a superclass constant from constants/const_defined?/const_get
  class MdParent
    COLOR = "red"
  end

  class MdChild < MdParent
    SIZE = 5
  end

  # MdChild::X resolves constants and nested classes through the superclass
  class PathParent
    COLOR = "red"

    class Nested
      #: () -> String
      def hi = "nested"
    end
  end

  class PathChild < PathParent
    SIZE = 5
  end

  # a struct-class argument to generic then/reduce/include?, and singleton arrays
  class MdVec
    attr_reader :v #: Integer

    #: (Integer) -> void
    def initialize(v)
      @v = v
    end

    #: (MdVec) -> MdVec
    def plus(o) = MdVec.new(v + o.v)
  end

  class Vec3 < MdVec
  end

  # user-defined self.name/to_s/inspect are inherited by subclasses
  class MdWidget
    #: () -> String
    def self.name = "CustomWidget"
  end

  class SubWidget < MdWidget
  end

  class MdGadget
    #: () -> String
    def self.to_s = "GadgetClass"

    #: () -> String
    def self.inspect = "GadgetInspect"
  end

  class SubGadget < MdGadget
  end

  # is_a?(Module) is false for a class that neither includes it nor has subclasses
  module Walker
  end

  class MdDoc
  end

  class Page
    include Walker
  end

  # an ivar assignment as a method's last expression is its return value
  class MdMemo
    # @rbs @last: Integer?

    #: () -> void
    def initialize
      @last = nil
    end

    #: (Integer) -> Integer
    def store(n)
      @last = n * 2
    end

    #: () -> Integer?
    def last = @last
  end

  # `@x ||= [] #: T` types the ivar from the trailing annotation
  class ItemBox
    #: () -> Array[String]
    def items
      @items ||= [] #: Array[String]
    end
  end

  class Reg
    #: () -> Hash[String, Integer]
    def self.table
      @table ||= {} #: Hash[String, Integer]
    end
  end

  # a local declared as the superclass can be reassigned a subclass instance or class
  class MdBase
    #: () -> String
    def name = "MdBase"
  end

  class MdLeaf < MdBase
    def name = "MdLeaf"
  end

  # a method named like its class (calc/MdCalc, node/MdNode) survives subclassing
  class MdCalc
    #: (Integer) -> Integer
    def calc(n) = n * 2
  end

  class MdKidCalc < MdCalc
    def calc(n) = super + 1
  end

  class MdNode
    attr_reader :node #: String

    #: () -> void
    def initialize
      @node = "node"
    end
  end

  class NodeLeaf < MdNode
  end

  # an included module's constant resolves unqualified in the class and its subclasses
  module MdConfig
    LIMIT = 5
  end

  class MdUses
    include MdConfig

    #: () -> Integer
    def lim = LIMIT
  end

  class MdSub < MdUses
    #: () -> Integer
    def twice = LIMIT * 2
  end

  # Helpers for the checks that were testdata/run/object_mid2.rb.
  # self.class inside a module method names the including object's class
  module Describe
    #: () -> String
    def kind = self.class.name

    #: () -> String
    def label = "#{self.class}!"
  end

  class MtFoo
    include Describe
  end

  class Bar < MtFoo
  end

  # a module as an element type dispatches to each includer's override
  module Printable
    #: () -> String
    def title = raise(NotImplementedError)

    #: () -> String
    def to_s = "P(#{title})"
  end

  class MtDoc
    include Printable

    #: () -> String
    def title = "doc"
  end

  class MtMemo
    include Printable

    #: () -> String
    def title = "memo"
  end

  # != uses a user-defined ==, and Struct's generated one
  class NeVec
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (NeVec) -> bool
    def ==(o) = x == o.x
  end

  NePoint = Struct.new(:x, :y) #: [Integer, Integer]

  # a subclass may override with a different arity and a narrower return type
  class MtBase
    #: (Integer) -> String
    def f(n) = "base #{n}"

    #: () -> MtBase
    def me = self

    #: () -> String
    def name = "base"
  end

  class MtSub < MtBase
    #: () -> String
    def f = "sub"

    #: () -> MtSub
    def me = self

    def name = "sub"

    #: () -> String
    def only = "only"
  end

  # private attr_reader/attr_accessor are callable on self, including self.x =
  class MtVault
    #: () -> void
    def initialize
      @pin = 1234
      @hits = 0
    end

    #: () -> String
    def open
      self.hits = hits + 1
      "pin has #{pin.to_s.size} digits, hit #{hits}"
    end

    private

    attr_reader :pin #: Integer
    attr_accessor :hits #: Integer
  end

  # a bare `private` does not reach `def self.x`
  class PrivCounter
    #: () -> String
    def self.shown = "shown"

    private

    #: () -> String
    def self.hidden = "still public"

    #: () -> String
    def inst = "private instance"
  end

  # reopening with `< ::MtA` or `class MtM::K < MtM::MtBase` is the same superclass
  class MtA
  end

  class MtC < MtA
    #: () -> String
    def one = "one"
  end

  class MtC < ::ObjectTests::MtA
    #: () -> String
    def two = "two"
  end

  module MtM
    class MtBase
    end

    class K < MtBase
    end
  end

  class MtM::K < MtM::MtBase
    #: () -> String
    def three = "three"
  end

  # a singleton(T) value converts to Class/Module parameters and Hash[Class, _] keys
  class MtShape
  end

  class MtSquare < MtShape
  end

  # a Struct.new block can override initialize/inspect and call super
  MtPoint = Struct.new(:x, :y) do
    #: (Integer, ?Integer) -> void
    def initialize(x, y = 0)
      super(x, y)
    end

    #: () -> Integer
    def sum = x + y
  end #: [Integer, Integer]

  MtPair = Struct.new(:a, :b) do
    #: () -> String
    def inspect = "P" + super
  end #: [Integer, Integer]

  # Struct/Data == is false across a subclass, true within one
  EqPoint = Struct.new(:x, :y) #: [Integer, Integer]

  class EqPoint3 < EqPoint
  end

  MtVal = Data.define(:v) #: [Integer]

  class SubVal < MtVal
  end

  # super inside an overridden attr reader/writer reaches the parent's attr
  class AttrBase
    attr_accessor :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end
  end

  class MtLoud < AttrBase
    def name = super.upcase

    def name=(v)
      super(v + "!")
    end
  end

  # include?/== on collections use a user-defined typed ==
  class MtVec
    attr_reader :x #: Integer

    #: (Integer) -> void
    def initialize(x)
      @x = x
    end

    #: (MtVec) -> bool
    def ==(o) = x == o.x
  end

  # zsuper from a no-arg initialize lets the parent apply its own default
  class MtNode
    attr_reader :id #: Integer

    #: (?Integer) -> void
    def initialize(id = 1)
      @id = id
    end
  end

  class MtBox < MtNode
    #: () -> void
    def initialize
      super
    end
  end

  # Method-name tables (decision 77): public_instance_methods/instance_methods
  # list public methods of the class and its ancestors, short of Object.
  module PimShared
    def test_shared = 1
  end

  class PimBase
    def test_base = 1
    def helper = 2

    private

    def test_private = 3
  end

  class PimChild < PimBase
    include PimShared

    def test_child = 4
    def ==(other) = true
  end

  # Helpers for the checks that were testdata/run/object_modules.rb.
  module MoNamed
    #: () -> String
    def title = raise(NotImplementedError)

    #: () -> String
    def label = "[#{title}]"

    #: () -> String
    def shout = label.upcase
  end

  module Tagged
    #: () -> String
    def label = "tag:#{title}"

    #: () -> Integer
    def tag_len = title.size
  end

  module Greeter
    include MoNamed

    #: (String) -> String
    def greet(other) = "#{label} greets #{other}"
  end

  class MoDoc
    include MoNamed

    attr_reader :title #: String

    #: (String) -> void
    def initialize(title)
      @title = title
    end
  end

  class MoBoth
    include MoNamed
    include Tagged

    #: () -> String
    def title = "both"
  end

  class Override
    include MoNamed

    #: () -> String
    def title = "ov"

    def label = "<#{super}>"
  end

  class MoPerson
    include Greeter

    #: () -> String
    def title = "ann"
  end

  module MoLoud
    #: () -> String
    def hi = "loud"
  end

  class MoA
    #: () -> String
    def hi = "a"

    #: () -> String
    def base_only = "base"
  end

  class MoB < MoA
    include MoLoud
  end

  class MoC < MoB
    def hi = "c/" + super
  end

  class MoQ < MoDoc
  end

  module MoRegistry
    extend Enumerable #[String]

    #: () { (String) -> void } -> void
    def self.each
      yield "alpha"
      yield "beta"
      yield "gamma"
    end
  end

  module Util
    #: (Integer) -> Integer
    def self.double(n) = n * 2

    #: (Integer) -> Integer
    def self.quad(n) = double(double(n))
  end

  # MoA module's own constants resolve lexically inside its methods; `-> self`, iterator,
  # &block-forwarding and private methods in a module, reached from an includer and its subclass.
  module Greeting
    PREFIX = "hi"

    #: () -> String
    def name = raise(NotImplementedError)

    #: () -> String
    def greet = "#{PREFIX} #{name}#{suffix}"

    #: () -> self
    def touch = self

    #: () { (Integer) -> void } -> void
    def each_two
      yield 1
      yield 2
    end

    #: (Integer) { (Integer) -> Integer } -> Array[Integer]
    def mapped(n, &blk) = [n, n + 1].map(&blk)

    private

    #: () -> String
    def suffix = "!"
  end

  class Member
    include Greeting

    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
    end

    #: () -> String
    def loud = greet.upcase + suffix
  end

  class Junior < Member
    def name = "jr"
  end

  # Helpers for the checks that were testdata/run/object_namespaces.rb.
  # NS_TRACE records constant init order against the surrounding top-level code.
  NS_TRACE = ["start"] #: Array[String]

  FIRST = object_trace("first")

  module Outer
    OBJECT_NS_NAME = "outer"
    SECOND = object_trace("second")
    LIST = [1, 2, 3]
    TABLE = { "a" => 1 }
    EMPTY = [] #: Array[String]

    module Inner
      DEPTH = 2

      class Leaf
        #: () -> String
        def where = "#{OBJECT_NS_NAME} #{DEPTH} #{OBJECT_NS_LIMIT} #{SECOND}"

        #: () -> String
        def top = ::OBJECT_NS_NAME
      end
    end

    class Base
      SCALE = 3

      #: () -> Integer
      def scale = SCALE
    end

    class Derived < Base
      #: () -> Integer
      def doubled = SCALE * 2

      #: () -> Integer
      def list_sum = LIST.reduce(0) { |a, b| a + b }
    end
  end
end

ObjectTests::NS_TRACE << "middle"

module ObjectTests
  class Outer::Compact
    #: () -> String
    def name_seen = OBJECT_NS_NAME

    #: () -> String
    def qualified = Outer::OBJECT_NS_NAME
  end

  class NsOther < Outer::Base
    #: () -> Integer
    def inherited_scale = SCALE + 1
  end

  module Outer
    THIRD = object_trace("third")

    class Vec
      attr_reader :v #: Integer

      #: (Integer) -> void
      def initialize(v)
        @v = v
      end

      #: (Vec) -> Vec
      def plus(o) = Vec.new(v + o.v)
    end

    module Factory
      #: () -> Array[Vec]
      def self.make = [Vec.new(1), Vec.new(-2)]

      #: (singleton(Base)) -> Base
      def self.build(k) = k.new
    end

    ORIGIN = Inner::Leaf.new #: Inner::Leaf

    class Compact
      #: () -> String
      def reopened = OBJECT_NS_NAME
    end
  end

  # Classes nested in a class and inheriting from it; a nested constant shadowing the outer one;
  # relative paths (Config::PORT) resolved from a sibling module's lexical scope.
  class Expr
    PRECEDENCE = 0

    #: () -> String
    def show = "?"

    class Num < Expr
      attr_reader :v #: Integer

      #: (Integer) -> void
      def initialize(v)
        @v = v
      end

      def show = v.to_s
    end

    class Add < Expr
      PRECEDENCE = 1

      #: (Expr, Expr) -> void
      def initialize(l, r)
        @l = l
        @r = r
      end

      def show = "(#{@l.show} + #{@r.show})"

      #: () -> Integer
      def prec = PRECEDENCE

      #: (Integer, Integer) -> Add
      def self.of(a, b) = new(Num.new(a), Num.new(b))
    end

    #: () -> Integer
    def prec = PRECEDENCE
  end

  module App
    module Config
      PORT = 80
    end

    module Server
      #: () -> Integer
      def self.port = Config::PORT + 1

      class Handler
        #: () -> String
        def where = "#{Config::PORT}/#{Server.port}"
      end
    end
  end

  # RBS names in annotations resolve like constants: through the superclass (Item, OBJECT_NS_LIMIT from Holder)
  # and lexically outward (Vec is Outer::Vec inside Outer::Inner).
  class Holder
    class Item
      #: () -> String
      def hi = "item"
    end

    OBJECT_NS_LIMIT = 3
  end

  class Taker < Holder
    #: () -> Item
    def make = Item.new

    #: (Array[Item]) -> Integer
    def count(xs) = xs.size + OBJECT_NS_LIMIT
  end

  module Outer
    module Inner
      #: (Integer) -> Vec
      def self.vec(n) = Vec.new(n)
    end
  end

  # Helpers for the checks that were testdata/run/object_operators.rb.
  class OpVec
    attr_reader :x #: Integer
    attr_reader :y #: Integer

    #: (Integer, Integer) -> void
    def initialize(x, y)
      @x = x
      @y = y
    end

    #: (OpVec) -> OpVec
    def +(o) = OpVec.new(x + o.x, y + o.y)

    #: (OpVec) -> OpVec
    def -(o) = OpVec.new(x - o.x, y - o.y)

    #: (Integer) -> OpVec
    def *(k) = OpVec.new(x * k, y * k)

    #: () -> OpVec
    def -@ = OpVec.new(-x, -y)

    #: (OpVec) -> bool
    def ==(o) = x == o.x && y == o.y

    #: (Integer) -> Integer
    def [](i) = i == 0 ? x : y

    #: (Integer, Integer) -> void
    def []=(i, v)
      if i == 0
        @x = v
      else
        @y = v
      end
    end

    #: () -> String
    def to_s = "(#{x}, #{y})"

    #: () -> String
    def inspect = "#<OpVec #{x},#{y}>"

    #: () -> bool
    def zero? = x == 0 && y == 0

    #: () -> OpVec
    def negate! = self * -1

    #: (Integer) -> void
    def x=(v)
      @x = v
    end
  end

  # << returning self chains; unary +@ and ~; an ivar mutated inside a block.
  class OpBag
    #: () -> void
    def initialize
      @items = [] #: Array[Integer]
      @total = 0
    end

    #: (Integer) -> self
    def <<(x)
      @items << x
      self
    end

    #: () -> Integer
    def sum
      @total = 0
      @items.each { |i| @total += i }
      @total
    end

    #: () -> Integer
    def +@ = @items.size

    #: () -> Integer
    def ~ = -@items.size

    #: () -> Array[Integer]
    def evens = @items.select { |i| i.even? && i > @total / 100 }
  end

  # The default to_s/inspect name the class (MRI appends an address, so only the prefix is compared);
  # inspect does not call a user's to_s.
  class OpPlain
  end

  class Shown
    #: () -> String
    def to_s = "shown!"
  end

  module OpDeep
    class Thing
      #: () -> void
      def initialize
        @a = 1
      end
    end
  end

  # == taking untyped and checking the class first.
  class Coin
    attr_reader :cents #: Integer

    #: (Integer) -> void
    def initialize(cents)
      @cents = cents
    end

    #: (untyped) -> bool
    def ==(other) = other.is_a?(Coin) && cents == other.cents
  end

  # Helpers for the checks that were testdata/run/object_override_chain.rb.
  class OcBase
    #: (Integer) -> String
    def f(n) = "base #{n}"

    #: () -> OcBase
    def me = self

    #: () -> self
    def dup_me = self

    #: (self) -> bool
    def same?(o) = o.equal?(self)

    #: () -> String
    def name = "base"

    #: () -> String
    def call_f = f(7)

    #: (String) -> String
    def self.make(s) = "OcBase.make #{s}"
  end

  class OcSub < OcBase
    #: (?Integer) -> String
    def f(n = 3) = "sub #{n} " + super(n)

    #: () -> OcSub
    def me = self

    def dup_me = self

    def same?(o) = !o.equal?(self)

    def name = "sub"

    #: () -> String
    def only = "only"

    #: () -> String
    def self.make = "OcSub.make"
  end

  class SubSub < OcSub
    #: () -> String
    def f = "subsub"

    def name = "subsub"
  end

  class OcLeaf < SubSub
  end

  # Helpers for the checks that were testdata/run/object_struct_data.rb.
  SdPoint = Struct.new(
    :x, #: Integer
    :y  #: Integer
  )

  SdPair = Struct.new(:left, :right) #: [String, Float]

  Rec = Struct.new(:id, :name, :note) do
    #: () -> String
    def label = "#{id}:#{name || "-"}:#{note || "-"}"

    #: (Integer) -> Rec
    def self.numbered(n) = new(n)
  end #: [Integer, String?, String?]

  class SdNamed < Rec
    def label = "named " + super
  end

  module SdGeo
    Coord = Data.define(
      :lat, #: Float
      :lng  #: Float
    ) do
      #: () -> String
      def to_s = "(#{lat}, #{lng})"

      #: () -> Float
      def sum = lat + lng
    end

    Tag = Struct.new(:text) #: [String]
  end

  Money = Data.define(:cents, :currency) #: [Integer, String]

  Inner = Struct.new(:a) #: [Integer]

  Wrap = Struct.new(:inner, :list, :map) #: [Inner, Array[String], Hash[Symbol, Integer]]

  Kw = Struct.new(:type, :range, :func, :map, :go, :select) #: [String, Integer, String, String, bool, Integer]

  class SdThing
    attr_reader :type #: String
    attr_accessor :interface #: Integer

    #: () -> void
    def initialize
      @type = "t"
      @interface = 1
    end

    #: () -> String
    def len = "len"

    #: () -> String
    def string = "string"

    #: () -> String
    def default = "default"
  end

  SdVal = Data.define(:v) #: [Float]

  # Self-referential member types (an optional link, an array of children); a Struct subclass
  # adding its own attr and initialize that calls super.
  ListNode = Struct.new(:val, :nxt) #: [Integer, ListNode?]

  Tree = Struct.new(:val, :kids) #: [Integer, Array[Tree]]

  class Pt3 < SdPoint
    attr_reader :z #: Integer

    #: (Integer, Integer, Integer) -> void
    def initialize(x, y, z)
      super(x, y)
      @z = z
    end

    #: () -> Integer
    def sum = x + y + z
  end

  class ViVault
    attr_reader :name #: String

    #: (String) -> void
    def initialize(name)
      @name = name
      @pin = 1234
    end

    #: () -> String
    def open = "#{name} opened with #{masked} #{self.check(4)}"

    #: () -> String
    def via_top = top_helper(pin % 10)

    private

    #: () -> Integer
    def pin = @pin

    #: () -> String
    def masked = "*" * pin.to_s.size

    #: (Integer) -> bool
    def check(n) = pin.to_s.size == n

    public

    #: () -> String
    def status = "ok #{check(3)}"

    #: () -> String
    private def secret = "s3cret"

    #: () -> String
    def reveal = secret.reverse
  end

  class BigVault < ViVault
    #: () -> String
    def deep = "#{masked}|#{pin}|#{secret}"

    def status = "big " + super
  end

  # A private method called from inside a block, and a private hand-written setter through self.
  class Dial
    #: () -> void
    def initialize
      @pin = 0
    end

    #: (Integer) -> Array[Integer]
    def scaled(n) = [1, 2].map { |x| helper(x) * n }

    #: (Integer) -> Integer
    def set(v)
      self.pin = v
      @pin
    end

    private

    #: (Integer) -> Integer
    def helper(x) = x + @pin

    #: (Integer) -> void
    def pin=(v)
      @pin = v
    end
  end

  class ObjectAttrsTest < Minitest::Test
    def test_section_0
      a = Account.new("ann")
      assert_equal "ann", a.owner
      assert_equal 0, a.balance
      assert_equal -100, a.limit
      a.deposit(50).deposit(25)
      assert_equal 75, a.balance
      assert_equal [50, 25], a.history
      assert_equal true, a.withdraw(100)
      assert_equal -25, a.balance
      assert_equal false, a.withdraw(80)
      assert_equal -25, a.balance
      assert_equal [50, 25, -100], a.history
      assert_equal "(none)", a.note_text
      a.note = "vip"
      assert_equal "vip", a.note_text
      a.note = nil
      assert_equal "(none)", a.note_text
      assert_nil a.nick
      assert_equal "ann", a.display_name
      a.nick = "ännie"
      assert_equal "ännie", a.nick
      assert_equal "ÄNNIE", a.display_name
      a.balance = 0
      a.limit = a.limit + 5
      assert_equal 0, a.balance
      assert_equal -95, a.limit
      assert_equal "ann:0", a.summary
      a.deposit(1)
      assert_equal "ann:0", a.summary
      a.reset_cache
      assert_equal "ann:1", a.summary
      a.tag(:b, "two")
      a.tag(:a, "one")
      a.tag(:b, "deux")
      assert_equal "{b: \"deux\", a: \"one\"}", (a.tags).inspect
      assert_equal "0.5", (a.ratio).inspect
      a.balance = 42
      assert_equal 42, a.balance
      s = Savings.new("sam", 0.25)
      assert_equal "sam", s.owner
      assert_equal 100, s.balance
      assert_equal 0, s.limit
      assert_equal "0.25", (s.rate).inspect
      s.deposit(20).history
      assert_equal 30, s.interest
      assert_equal 20, s.first_deposit
      assert_equal [20], s.history
      assert_equal false, s.withdraw(121)
      assert_equal true, s.withdraw(120)
      assert_equal 0, s.balance
      assert_equal "sam:0", s.summary
      accounts = [a, s] #: Array[Account]
      assert_equal [42, 0], accounts.map(&:balance)
      assert_equal [4, 2], (accounts.map { |x| x.history.size })
      accounts.each { |x| x.balance = x.balance * 2 }
      assert_equal [84, 0], accounts.map(&:balance)
      big = Account.new("big", 9_223_372_036_854_775_000)
      assert_equal 9223372036854775000, big.balance
      root = Profile.new(nil)
      kid = Profile.new("kid", root)
      baby = Profile.new("baby", kid)
      assert_equal "anon", root.shout
      assert_equal "KID", kid.shout
      assert_equal "baby < kid < root", baby.lineage
      assert_equal "root", root.lineage
      assert_equal 2, baby.depth
      assert_equal 0, root.depth
      assert_equal "kid", baby.parent_nick
      assert_equal "none", root.parent_nick
      assert_equal "none", kid.parent_nick
      baby.nick = nil
      assert_equal "anon", baby.shout
      assert_nil baby.nick
      kid.nick = "k2"
      assert_equal "? < k2 < root", baby.lineage
      assert_equal "K2 then nil", kid.reset
      assert_equal "none", kid.reset
      assert_nil kid.nick
      up = baby.parent
      assert_equal "? < root", (up.lineage if up)
      assert_nil (up&.parent&.nick)
      assert_nil (root.parent&.nick)
      lz = Lazy.new
      lz.data << 3
      lz.load
      assert_equal [1, 2, 3], lz.data
      assert_equal "yes 6", lz.loaded
      per = AtPerson.new("ann", 3)
      assert_equal "Ann", per.name
      per.name = "bob"
      per.age = -4
      assert_equal "Bob", per.name
      assert_equal 0, per.age
      kd = AtKid.new("cy", 2)
      kd.age = 7
      assert_equal "Cy/7", kd.info
    end
  end

  class ObjectBugAttrOpAssignTest < Minitest::Test
    def test_section_0
      c = AoCounter.new
      c.n += 3
      c.n *= 4
      c.drop(20)
      assert_equal -8, c.n
      c.label ||= "first"
      c.label ||= "second"
      assert_equal "first", c.label
      pt = AoPoint.new(1, 2)
      pt.x += 10
      pt.y -= 1
      assert_equal "#<struct ObjectTests::AoPoint x=11, y=1>", (pt).inspect
    end
  end

  class ObjectBugClassNameGeneratedCollisionTest < Minitest::Test
    def test_section_0
      assert_equal "user", User.new.to_s
      assert_equal "new user", NewUser.new.to_s
      assert_equal "shape", Shape.new.to_s
      assert_equal "shape i", ShapeI.new.to_s
      assert_equal "Foo::Bar", Foo::Bar.new.to_s
      assert_equal "Foo_Bar", Foo_Bar.new.to_s
      assert_equal "NewUser", NewUser.name
      assert_equal "ObjectTests::ShapeI", ShapeI.name
      assert_equal "ObjectTests::Foo_Bar", Foo_Bar.name
      assert_equal "ObjectTests::Foo::Bar", Foo::Bar.name
    end
  end

  class ObjectBugComparableStructTest < Minitest::Test
    def test_section_0
      v1 = Version.new(1, 2)
      v2 = Version.new(1, 10)
      v3 = Version.new(2, 0)
      assert_equal true, (v1 < v2)
      assert_equal false, (v2 < v1)
      assert_equal true, (v1 <= v1)
      assert_equal true, (v3 > v2)
      assert_equal false, (v1 >= v3)
      assert_equal -1, (v1 <=> v2)
      assert_equal 1, (v3 <=> v1)
      assert_equal 0, (v1 <=> Version.new(1, 2))
      assert_equal true, (v2.between?(v1, v3))
      assert_equal false, (v3.between?(v1, v2))
      assert_equal true, (v1.between?(v1, v1))
      assert_equal "v1.10", (v1.clamp(v2, v3)).to_s
      assert_equal "v1.10", (v3.clamp(v1, v2)).to_s
      assert_equal "v1.10", (v2.clamp(v1, v3)).to_s
      assert_equal ["v1.2", "v1.10", "v2.0"], ([v3, v1, v2].sort.map(&:to_s))
      assert_equal "v1.2", ([v3, v1, v2].min.to_s)
      assert_equal "v2.0", ([v3, v1, v2].max.to_s)
    end
  end

  class ObjectBugConstGetMessagesTest < Minitest::Test
    def test_section_0
      e = assert_raises(TypeError) { CgM.const_get(42) }
      assert_equal "no implicit conversion of Integer into String", e.message
      e = assert_raises(TypeError) { CgM.const_get(nil) }
      assert_equal "no implicit conversion of nil into String", e.message
      e = assert_raises(NameError) { CgM.const_get(:lower) }
      assert_equal "wrong constant name lower", e.message
      e = assert_raises(NameError) { CgM.const_get("") }
      assert_equal "wrong constant name ", e.message
      e = assert_raises(NameError) { CgM.const_defined?("lower") }
      assert_equal "wrong constant name lower", e.message
    end
  end

  class ObjectBugHookMethodsTest < Minitest::Test
    def test_section_0
      assert_equal ["inherited by ObjectTests::Alpha", "included in ObjectTests::HkThing", "extended ObjectTests::HkOther"], HK_LOG
    end
  end

  class ObjectBugModuleSuperTest < Minitest::Test
    def test_section_0
      assert_equal "loud a", MsB.new.hi
      MS_LOG.clear
      assert_equal "shout p1", MsQ.new(1).hi
      assert_equal 208, MsQ.new(2).calc(3)
      assert_equal "r shout p4", R.new(4).hi
      assert_equal "quiet shout p5", S.new(5).hi
      assert_equal "both shout p6", T.new(6).hi
      assert_equal ["p init 1", "shout init 1", "p init 2", "shout init 2", "p init 4", "shout init 4",
                    "p init 5", "shout init 5", "p init 6", "shout init 6"], MS_LOG
      e = assert_raises(NoMethodError) { U.new.hi }
      assert_equal "super: no superclass method 'hi' for an instance of ObjectTests::U", e.message
    end
  end

  class ObjectBugNarrowedCallStmtTest < Minitest::Test
    def test_section_0
      h = NcHtmlBuilder.new
      h.add("a")
      h.add("b").add("c")
      assert_equal "a-b-c", h.build
      NcPlugins.const_get(:VERSION)
      NcPlugins.const_get(:Alpha)
      make(NcSquare)
      assert_equal ["made 1"], NC_LOG
    end
  end

  class ObjectBugsTest < Minitest::Test
    def test_section_0
      assert_equal "opt", Opt.new.to_s
      assert_equal "ref", Ref.new.to_s
      assert_equal "NameError: uninitialized constant ObjectTests::BG_LIMIT", BG_LIM_ERR
      assert_equal 5, lim
      assert_nil Settings::DEFAULT
      assert_equal [:DEFAULT], Settings.constants
      assert_nil Settings.const_get(:DEFAULT)
      assert_equal true, Settings.const_get("DEFAULT").nil?
      type_err = assert_raises(TypeError) { Object.const_get("ObjectTests::BgPlugins::VERSION::X") }
      assert_equal "ObjectTests::BgPlugins::VERSION::X does not refer to class/module", type_err.message
      here = Coord.new(lat: 1.0, lng: 2.0)
      assert_equal true, here.with.equal?(here)
      assert_equal false, (here.with(lat: 1.0).equal?(here))
      assert_equal true, (here.with == here)
      assert_equal false, BgPlain.new.frozen?
      assert_equal false, (BgPair.new(1, 2).frozen?)
      assert_equal true, BgVal.new(1).frozen?
      d = BgDog.new
      assert_equal true, d.is_a?(BgAnimal)
      assert_equal [:BG_LIMIT, :OWN], BgUses.constants.sort
      assert_equal true, BgUses.const_defined?(:BG_LIMIT)
      assert_equal 5, BgUses.const_get(:BG_LIMIT)
      assert_equal "String", bg_kind("a")
      assert_equal "NilClass", bg_kind(nil)
      assert_equal "NilClass", (nil.class).to_s
      assert_equal "n", BG_NAME
      assert_equal 3, COUNT
      assert_equal "l", Labels::LABEL
      loud_name = BG_NAME.upcase if BG_NAME
      assert_equal "N", loud_name
      assert_equal 4, (COUNT || 0) + 1
      span = Span.new(1, 5)
      assert_equal 1, span.begin
      assert_equal 5, span.end
      assert_equal "#<struct ObjectTests::Span begin=1, end=5>", (span).inspect
      assert_equal [1, 5], span.to_a
      assert_equal "{begin: 1, end: 5}", (span.to_h).inspect
      span.end = 7
      assert_equal true, (span == Span.new(1, 7))
      l = Link.new(in: "a", out: "b")
      assert_equal "a", l.in
      assert_equal "#<data ObjectTests::Link in=\"a\", out=\"b\">", (l).inspect
      assert_equal "#<data ObjectTests::Link in=\"a\", out=\"c\">", ((l.with(out: "c"))).inspect
      assert_equal true, (Edge.new("a", "b") == Edge.new("a", "b"))
      assert_equal false, (Edge.new("a", "b") == Edge.new("a", "c"))
      assert_equal true, (LinkOther.new(1) == LinkOther.new(1))
      assert_equal false, (LinkOther.new(1) == LinkOther.new(2))
      seen = [] #: Array[Integer]
      tb = TapBox.new(4).tap { |b| seen << b.n }
      assert_equal 4, tb.n
      assert_equal 7, (7.tap { |x| seen << x + 1 })
      assert_equal [4, 8], seen
      ups = [] #: Array[String]
      assert_equal 1, ("s".tap { |s| ups << s.upcase }.size)
      assert_equal ["S"], ups
      assert_equal [1, 2, 3], ([1, 2].tap { |a| a << 3 })
      SgConfig.instance.level = 3
      assert_equal ["init"], SG_LOG
      assert_equal 3, SgConfig.instance.level
      assert_equal true, SgConfig.instance.equal?(SgConfig.instance)
      SgApp::Registry.instance.add("a")
      assert_equal ["a", "b"], SgApp::Registry.instance.add("b")
      assert_equal true, SgConfig.instance.is_a?(Singleton)
    end
  end

  class ObjectClassMethodsTest < Minitest::Test
    def test_section_0
      assert_equal "shape", CmShape.kind
      assert_equal "square", CmSquare.kind
      assert_equal "shape", Tri.kind
      assert_equal "tiny", Tiny.kind
      assert_equal "shape/0", CmShape.info
      assert_equal "sq square/4", CmSquare.info
      assert_equal "shape/3", Tri.info
      assert_equal "sq tiny/4", Tiny.info
      assert_equal "shape:ObjectTests::CmShape(2) sides=0", CmShape.build(1).describe
      assert_equal "square:ObjectTests::CmSquare(2) sides=4", CmSquare.build(1).describe
      assert_equal "shape:ObjectTests::Tri(10) sides=3", Tri.build_explicit(5).describe
      assert_equal "square:ObjectTests::CmSquare(102) sides=4", CmSquare.new(2).grow.describe
      assert_equal "shape:ObjectTests::Tri(100) sides=3", Tri.new(0).grow.describe
      assert_equal "tiny:ObjectTests::Tiny(1) sides=4", Tiny.new.describe
      assert_equal 1, Tiny.new.size
      klasses = [CmShape, CmSquare, Tri] #: Array[singleton(CmShape)]
      assert_equal ["shape:ObjectTests::CmShape(7) sides=0", "square:ObjectTests::CmSquare(7) sides=4", "shape:ObjectTests::Tri(7) sides=3"], klasses.map { |k| k.new(7).describe }
      assert_equal ["shape", "square", "shape"], klasses.map(&:kind)
      assert_equal [0, 4, 3], (klasses.map { |k| k.sides })
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmSquare", "ObjectTests::Tri"], klasses.map(&:name)
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmSquare", "ObjectTests::Tri"], klasses.map(&:to_s)
      assert_equal "[ObjectTests::CmShape, ObjectTests::CmSquare, ObjectTests::Tri]", (klasses).inspect
      assert_equal [1, 1, 1], (klasses.map { |k| k.build(0).size })
      k = klasses.fetch(1)
      assert_equal "square", k.kind
      assert_equal "ObjectTests::CmSquare", k.name
      assert_equal "square:ObjectTests::CmSquare(3) sides=4", k.new(3).describe
      assert_equal "sq square/4", k.info
      k = klasses.fetch(2)
      assert_equal "shape", k.kind
      assert_equal true, (k == Tri)
      assert_equal false, (k == CmSquare)
      assert_equal true, (k != CmSquare)
      by_name = { "sq" => CmSquare, "tri" => Tri } #: Hash[String, singleton(CmShape)]
      found = by_name["tri"]
      assert_equal "shape", (found.kind if found)
      assert_equal ["sq", "tri"], by_name.keys
      assert_equal "[ObjectTests::CmSquare, ObjectTests::Tri]", (by_name.values).inspect
      s = CmSquare.new(9)
      assert_equal "ObjectTests::CmSquare", (s.class).to_s
      assert_equal "ObjectTests::CmSquare", s.class.name
      assert_equal "square", s.class.kind
      assert_equal true, (s.class == CmSquare)
      assert_equal false, (s.class == CmShape)
      shapes = [CmShape.new(1), CmSquare.new(2), Tri.new(3), Tiny.new] #: Array[CmShape]
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmSquare", "ObjectTests::Tri", "ObjectTests::Tiny"], (shapes.map { |x| x.class.name })
      assert_equal [0, 4, 3, 4], (shapes.map { |x| x.class.sides })
      assert_equal [false, true, false, false], (shapes.map { |x| x.class == CmSquare })
      assert_equal "Class", (CmShape.class).to_s
      assert_equal "Class", (CmSquare.class).to_s
      assert_equal "Class", (CmShape.class.class).to_s
      assert_equal "Module", (Comparable.class).to_s
      assert_equal "ObjectTests::CmShape", (CmShape).inspect
      assert_equal "ObjectTests::CmShape", CmShape.to_s
      assert_equal "ObjectTests::CmShape", CmShape.name
      assert_equal "Integer", (1.class).to_s
      assert_equal "String", ("s".class).to_s
      assert_equal "Symbol", (:sym.class).to_s
      assert_equal "Float", (1.5.class).to_s
      assert_equal "Array", ([1].class).to_s
      assert_equal "Hash", (({ a: 1 }.class)).to_s
      assert_equal "Class", (1.class.class).to_s
      assert_equal "Class", (Integer.class).to_s
      assert_equal "Integer", Integer.name
      assert_equal "String", (String).inspect
      assert_equal true, (1.class == Integer)
      assert_equal false, ("s".class == Symbol)
      CmRegistry.bump
      CmRegistry.bump
      SubRegistry.bump
      assert_equal 2, CmRegistry.count
      assert_equal 1, SubRegistry.count
      assert_equal "<a>", CmRegistry.fmt("a")
      assert_equal "<B>!", SubRegistry.fmt("b")
      assert_equal "<>!", SubRegistry.fmt("")
      mods = [CmShape, Comparable, Enumerable, CmRegistry] #: Array[Module]
      assert_equal ["ObjectTests::CmShape", "Comparable", "Enumerable", "ObjectTests::CmRegistry"], mods.map(&:name)
      assert_equal ["ObjectTests::CmShape", "Comparable", "Enumerable", "ObjectTests::CmRegistry"], mods.map(&:to_s)
      assert_equal "[ObjectTests::CmShape, Comparable, Enumerable, ObjectTests::CmRegistry]", (mods).inspect
      klass_list = [CmShape, CmRegistry, String] #: Array[Class]
      assert_equal ["ObjectTests::CmShape", "ObjectTests::CmRegistry", "String"], klass_list.map(&:name)
      assert_equal true, (klass_list.first(1).fetch(0) == CmShape)
      assert_equal "red", Color::RED.name
      assert_equal ["red", "blue"], Color::ALL.map(&:name)
      assert_equal "RED", Color.default.shout
      assert_equal "<x>!", Shade.fmt("x")
      assert_equal "<p>!", Shade.plain
      assert_equal 9, CmDeep.new("d").depth
      assert_equal 1, (CmDeep.new("e", 1).depth)
      assert_equal "S", Shade.new("s").shout
      assert_equal "square", object_pick(true).kind
      assert_equal "shape", object_pick(false).kind
      assert_equal "square:ObjectTests::CmSquare(4) sides=4", object_pick(true).new(4).describe
      assert_equal "square:ObjectTests::CmSquare(1) sides=4", by_num(1).describe
      assert_equal "ObjectTests::Tri", (by_num(2).class).to_s
      assert_equal "shape:ObjectTests::CmShape(0) sides=0", by_num(0).describe
      flag = CmShape.sides == 0
      kl = flag ? CmSquare : Tri
      assert_equal "square", kl.kind
      assert_equal "square:ObjectTests::CmSquare(5) sides=4", kl.new(5).describe
      assert_equal "ObjectTests::CmSquare", kl.name
      inst = flag ? Tri.new(1) : CmSquare.new(1)
      assert_equal "shape:ObjectTests::Tri(1) sides=3", inst.describe
      names = { CmSquare => "sq" } #: Hash[singleton(CmShape), String]
      names[Tri] = "tri"
      names[CmSquare] = "SQ"
      assert_equal "SQ", names[CmSquare]
      assert_equal "tri", names[Tri]
      assert_nil names[CmShape]
      assert_equal 2, names.size
      assert_equal "[ObjectTests::CmSquare, ObjectTests::Tri]", (names.keys).inspect
      tally = {} #: Hash[singleton(CmShape), Integer]
      [CmSquare, Tri, CmSquare, Tiny].each { |c| tally[c] = tally.fetch(c, 0) + 1 }
      assert_equal "{ObjectTests::CmSquare => 2, ObjectTests::Tri => 1, ObjectTests::Tiny => 1}", (tally).inspect
      assert_equal ["square", "shape", "tiny"], tally.keys.map(&:kind)
      assert_equal "reopened square", CmSquare.extra
      assert_equal "reopened tiny", Tiny.extra
      assert_equal 9, CmSquare.new(3).area
      assert_equal 1, Tiny.new.area
      Tally.bump
      Tally.bump
      assert_equal 2, Tally.count
      assert_equal "shape", CmShape.me.kind
      assert_equal "tiny", Tiny.me.kind
      assert_equal "ObjectTests::Tiny", Tiny.me.name
      assert_equal "made square 2", CmSquare.new(2).maker.made
      assert_equal "made tiny 1", Tiny.new.maker.made
      assert_equal ["shape", "square", "tiny"], HANDLERS.map(&:kind)
      assert_equal [0, 4, 4], (HANDLERS.map { |h| h.me.sides })
      assert_equal "[ObjectTests::CmShape, ObjectTests::CmSquare, ObjectTests::Tiny]", (HANDLERS).inspect
      conf = Conf.build { |x| x.port = 8080 }
      assert_equal 8080, conf.port
      assert_equal "n=2", (Conf.fmt { |n| "n=#{n}" })
      assert_equal "1010", (Conf.fmt(5) { |n| n.to_s * 2 })
      tmp = CmTemp.new(1)
      assert_equal 0, CmTemp::FREEZING.deg
      assert_equal 100, CmTemp::RANGE
      assert_equal true, tmp.is_a?(Object)
      assert_equal 1, tmp.deg
      Handler.register("p", Posts)
      Posts.register("u", Users)
      assert_equal "run posts", Handler.dispatch("p")
      assert_equal "run users", Users.dispatch("u")
      assert_equal "none", Handler.dispatch("x")
      assert_equal ["p", "u"], Handler::REGISTRY.keys
      assert_equal 2, Handler::REGISTRY.size
      fa = Factory.new(Users)
      assert_equal "run users", fa.make.run
      assert_equal "users", fa.kind
      assert_equal "run base", Factory.new(Handler).make.run
      assert_equal "pretty d", (CmDoc.new).to_s
      assert_equal "pretty d!", "#{CmDoc.new}!"
      assert_equal 8, CmDoc.new.to_s.size
      blobs = [Blob, SubBlob] #: Array[singleton(Blob)]
      assert_equal true, Blob.respond_to?(:kind)
      assert_equal false, Blob.respond_to?(:area)
      assert_equal true, SubBlob.respond_to?(:only_sub)
      assert_equal false, Blob.respond_to?(:only_sub)
      assert_equal [false, true], (blobs.map { |k| k.respond_to?(:only_sub) })
      assert_equal false, Blob.new.respond_to?(:kind)
      assert_equal true, Blob.respond_to?(:new)
      assert_equal true, Blob.respond_to?(:name)
      assert_equal "CustomWidget", CmWidget.name
      assert_equal "ObjectTests::CmWidget", (CmWidget).to_s
      assert_equal "ObjectTests::CmWidget", (CmWidget).inspect
      assert_equal "ObjectTests::CmWidget", "#{CmWidget}"
      assert_equal "[ObjectTests::CmWidget]", ([CmWidget]).inspect
      assert_equal "CustomWidget", CmWidget.new.class.name
      assert_equal "ObjectTests::CmGadget", CmGadget.name
      assert_equal "GadgetClass", (CmGadget).to_s
      assert_equal "ObjectTests::CmGadget", (CmGadget).inspect
      assert_equal "GadgetClass", "#{CmGadget}"
    end
  end

  class ObjectClassNewArityTest < Minitest::Test
    def test_section_0
      assert_equal 5, (Two.new(2, 3).n)
      # klass.new(1) through singleton(NaBase) on Two stays in testdata/run/object_class_new_arity.rb:
      # rb2go fails there with a Go type-assertion error, not ArgumentError (decision 19).
      ks = [NaBase, Same] #: Array[singleton(NaBase)]
      assert_equal ["ObjectTests::NaBase: 1", "ObjectTests::Same: 1"], ks.map { |k| "#{k.name}: #{k.new(1).n}" }
    end
  end

  class ObjectCmpNilTest < Minitest::Test
    def test_section_0
      a = Score.new(1.5)
      b = Score.new(0.5)
      assert_equal 1, (a <=> b)
      assert_equal ["S0.5", "S1.5"], ([a, b].sort.map(&:to_s))
      assert_equal "S0.1", ([a, b, Bonus.new(0.1)].min.to_s)
      z = 0.0 #: Float
      n = Score.new(z / z)
      assert_nil (n <=> a)
      e = assert_raises(ArgumentError) { [a, n].sort }
      assert_equal "comparison of ObjectTests::Score with ObjectTests::Score failed", e.message
      e = assert_raises(ArgumentError) { [Bonus.new(1.0), Bonus.new(z / z)].max }
      assert_equal "comparison of ObjectTests::Bonus with ObjectTests::Bonus failed", e.message
      x = 1.5 #: Float
      assert_equal -1, (x <=> 2)
      assert_equal 1, (2 <=> x)
      assert_equal 0, ((x <=> 2.0) || 0) + 1
    end
  end

  class ObjectConstantsTest < Minitest::Test
    def test_section_0
      assert_equal [:Alpha, :Base, :Beta, :MAX, :VERSION], CtPlugins.constants.sort
      assert_equal [:One, :Root, :Two], Kinds.constants.sort
      assert_equal [:COLOR, :SIZE], CtChild.constants.sort
      assert_equal [:COLOR], CtParent.constants
      assert_equal [], CtPlugins::Alpha.constants
      assert_equal "1.0", CtPlugins.const_get(:VERSION)
      assert_equal 3, CtPlugins.const_get("MAX")
      assert_equal 4, (CtPlugins.const_get(:MAX) + 1)
      assert_equal "alpha", CtPlugins.const_get(:Alpha).id
      assert_equal "base run", CtPlugins.const_get("Beta").new.run
      assert_equal "alpha run", CtPlugins.const_get(:Alpha).new.run
      assert_equal 1, rank_of("One")
      assert_equal 2, rank_of("Two")
      assert_equal 0, rank_of("Root")
      assert_equal [1, 0, 2], (Kinds.constants.sort.map { |c| Kinds.const_get(c).rank })
      assert_equal "red", CtChild.const_get(:COLOR)
      assert_equal 5, CtChild.const_get(:SIZE)
      assert_equal 99, Object.const_get(:OBJECT_TOP)
      assert_equal "ObjectTests::CtPlugins::Alpha", (Object.const_get("ObjectTests::CtPlugins::Alpha")).to_s
      assert_equal "1.0", Object.const_get("::ObjectTests::CtPlugins::VERSION")
      assert_equal 99, CtPlugins.const_get(:OBJECT_TOP)
      assert_equal "String", (CtPlugins.const_get("String")).to_s
      assert_equal true, (ObjectTests.const_get(:CtPlugins) == CtPlugins)
      assert_equal true, (CtPlugins.const_get("Alpha") == CtPlugins::Alpha)
      assert_equal false, (CtPlugins.const_get(:Beta) == CtPlugins::Alpha)
      assert_equal true, CtPlugins.const_defined?(:VERSION)
      assert_equal true, CtPlugins.const_defined?("Alpha")
      assert_equal false, CtPlugins.const_defined?(:Gamma)
      assert_equal true, CtChild.const_defined?(:COLOR)
      assert_equal true, Object.const_defined?("ObjectTests::CtPlugins::Base")
      assert_equal false, Object.const_defined?("ObjectTests::CtPlugins::Nope")
      assert_equal true, CtPlugins.const_defined?(:OBJECT_TOP)
      assert_equal true, Object.const_defined?(:OBJECT_TOP)
      e = assert_raises(NameError) { CtPlugins.const_get(:Gamma) }
      assert_equal "uninitialized constant ObjectTests::CtPlugins::Gamma", e.message
      e = assert_raises(NameError) { Object.const_get("Nowhere") }
      assert_equal "uninitialized constant Nowhere", e.message
      e = assert_raises(NameError) { Object.const_get("ObjectTests::CtPlugins::Alpha::Deep") }
      assert_equal "uninitialized constant ObjectTests::CtPlugins::Alpha::Deep", e.message
      assert_equal 5, child_const("SIZE")
      assert_equal "red", child_const("COLOR")
      assert_equal 5, CtChild.const_get("SIZE".downcase.upcase)
      assert_equal [18, 2], (Limits.constants.sort.map { |c| Limits.const_get(c) * 2 })
      assert_equal [:Coord, :ORIGIN, :Tag], CtGeo.constants.sort
      assert_equal "#<struct ObjectTests::CtGeo::Tag text=\"t\">", (CtGeo.const_get(:Tag).new("t")).inspect
      assert_equal [:lat, :lng], CtGeo.const_get(:Coord).members
      assert_equal "0.0", (CtGeo.const_get(:ORIGIN).lat).inspect
      assert_equal [], CtGeo::Tag.constants
      assert_equal "ObjectTests::CtGeo::Coord", CtGeo.const_get("Coord").name
    end

    # was the file's uncaught error at exit
    def test_const_get_missing_raises
      e = assert_raises(NameError) { Kinds.const_get(:Three) }
      assert_equal "uninitialized constant ObjectTests::Kinds::Three", e.message
    end
  end

  class ObjectDefaultArgsCalleeTest < Minitest::Test
    def test_section_0
      b = DaBase.new
      assert_equal 30, b.calc
      assert_equal 3, b.calc(1)
      assert_equal 2, (b.calc(1, 1))
      assert_equal 3, b.sum(1)
      assert_equal 6, (b.sum(1, 5))
      assert_equal 8, (b.sum(1, 5, 7, 8))
      ups = [] #: Array[Integer]
      b.times_up { |i| ups << i }
      b.times_up(2) { |i| ups << i }
      assert_equal [0, 1, 2, 0, 1], ups
      assert_equal 3003, via_base(DaKid.new)
      assert_equal 3007, DaKid.new.calc(5)
      assert_equal "hello module", b.hello
      assert_equal "hello kid", DaKid.new.hello
      assert_equal "hello you", b.hello("you")
      assert_equal 12, DaBox.new(3).area
      assert_equal 9, (DaBox.new(3, 3).area)
      assert_equal "made shape", DaShape.make
      assert_equal "made circle", Circle.make
      assert_equal "made x", Circle.make("x")
      assert_equal 10, top(2)
      assert_equal 4, (top(2, 2))
      l = Log.new
      assert_equal "x", l.f("x")
      assert_equal "y", (l.f("y", "z"))
      assert_equal 1, l.count
    end
  end

  class ObjectExtendBlockTest < Minitest::Test
    def test_section_0
      assert_equal 8, EbWidget.twice(4)
      assert_equal -2, EbGadget.twice(-1)
      assert_equal 0, Tools.twice(0)
      assert_equal "I am ObjectTests::EbWidget", EbWidget.describe
      assert_equal "I am ObjectTests::EbGadget", EbGadget.describe
      assert_equal "I am ObjectTests::Tools", Tools.describe
      assert_equal "widget", EbGadget.kind
      assert_equal ["RED", "GREEN", "BLUE"], (EbColors.map { |c| c.upcase })
      assert_equal 3, EbColors.count
      assert_equal ["red", "green", "blue"], EbColors.to_a
      assert_equal ["red", "green", "blue"], (EbColors.select { |c| c.include?("e") })
      assert_equal "blue", (EbColors.find { |c| c.size == 4 })
      assert_equal true, EbColors.include?("green")
      assert_equal false, EbColors.include?("pink")
      assert_equal ["red"], EbColors.first(1)
      assert_equal ["red", "blue", "green"], (EbColors.sort_by { |c| c.size })
      assert_equal "blue", EbColors.min
      assert_equal "red", EbColors.max
      assert_equal "ObjectTests::Handlers::Users", ((Handlers.detect { |h| h.matches?("/users/1") })).inspect
      assert_equal ["ObjectTests::Handlers::Base", "ObjectTests::Handlers::Posts", "ObjectTests::Handlers::Users"], (Handlers.map { |h| h.name })
      assert_equal 3, Handlers.count
      assert_equal "[ObjectTests::Handlers::Posts]", ((Handlers.select { |h| h.matches?("/posts/9") })).inspect
      assert_nil (Handlers.find { |h| h.matches?("/x") })
      bag = EbBag.new.add(3).add(1).add(2)
      items = [] #: Array[Integer]
      bag.each { |n| items << n }
      assert_equal [3, 1, 2], items
      assert_equal [9, 1, 4], (bag.transform { |n| n * n })
      assert_equal [3, 1], (bag.keep { |n| n.odd? })
      assert_equal "first=3", (bag.first_as { |n| "first=#{n}" })
      total = 0
      bag.each_twice { |n| total += n }
      assert_equal 12, total
      saw = [] #: Array[String]
      bag.each do |n|
        next if n == 1
        saw << "saw #{n}"
      end
      assert_equal ["saw 3", "saw 2"], saw
      assert_equal "ObjectTests::Knob:knob", Knob.label
      assert_equal "ObjectTests::BigKnob:big", BigKnob.label
      assert_equal "ObjectTests::Lever:base", Lever.label
      assert_equal "base", Lever.kind
      knobs = [Knob, BigKnob] #: Array[singleton(Knob)]
      assert_equal ["ObjectTests::Knob:knob", "ObjectTests::BigKnob:big"], knobs.map(&:label)
      assert_equal ["knob", "big"], (knobs.map { |k| k.kind })
    end
  end

  class ObjectInheritanceTest < Minitest::Test
    def test_section_0
      b = InBase.new("b")
      m = Middle.new("m")
      l = InLeaf.new("l", 3)
      e = Empty.new("e")
      f = Fixed.new
      assert_equal "InBase[b] w=1", b.describe
      assert_equal "Middle[m] w=10", m.describe
      assert_equal "leaf: InLeaf3[l] w=13", l.describe
      assert_equal "InBase[e] w=1", e.describe
      assert_equal "InBase[fixed] w=1", f.describe
      assert_equal 1, b.weight
      assert_equal 10, m.weight
      assert_equal 13, l.weight
      assert_equal 1, e.weight
      assert_equal 2, b.scale(2)
      assert_equal 21, m.scale(2)
      assert_equal 27, l.scale(2)
      assert_equal 0, b.scale(0)
      assert_equal -39, m.scale(-4)
      assert_equal "hi a!", b.greet("a")
      assert_equal "HI A?", m.greet("a")
      assert_equal "HI A.", (m.greet("a", "."))
      assert_equal "HI Z", (l.greet("z", ""))
      assert_equal "<InBase b>", (b).to_s
      assert_equal "<Middle m>", (m).to_s
      assert_equal "<InLeaf3 l>", (l).to_s
      assert_equal "<InBase e>", (e).to_s
      assert_equal "<InBase fixed>", (f).to_s
      assert_equal "<InBase b>", b.to_s
      assert_equal "<InLeaf3 l>", l.to_s
      assert_equal 7, Root.new.n
      all = [b, m, l, e, f] #: Array[InBase]
      assert_equal ["InBase", "Middle", "InLeaf3", "InBase", "InBase"], all.map(&:name)
      assert_equal [1, 10, 13, 1, 1], (all.map { |x| x.weight })
      assert_equal ["b", "m", "l", "e", "fixed"], all.map(&:tag)
      assert_equal 28, (all.reduce(0) { |acc, x| acc + x.scale(1) })
      assert_equal true, l.is_a?(InBase)
      assert_equal true, l.is_a?(Middle)
      assert_equal true, l.is_a?(InLeaf)
      assert_equal false, m.is_a?(InLeaf)
      assert_equal false, b.kind_of?(Middle)
      assert_equal [false, true, true, false, false], (all.map { |x| x.is_a?(Middle) })
      assert_equal true, l.is_a?(Object)
      assert_equal true, l.is_a?(BasicObject)
      assert_equal true, l.is_a?(Kernel)
      assert_equal false, l.is_a?(String)
      assert_equal true, (b == b)
      assert_equal false, (b == InBase.new("b"))
      assert_equal true, (b != InBase.new("b"))
      assert_equal false, (b != b)
      assert_equal true, b.equal?(b)
      assert_equal false, b.equal?(m)
      assert_equal false, (!b)
      assert_equal false, b.nil?
      assert_equal 3, (Summer.new(1, 2).total)
      assert_equal 6, (KidSummer.new(1, 2, 3).total)
      assert_equal 0, KidSummer.new.total
      assert_equal 2, Summer.new(1).scaled
      assert_equal 2, KidSummer.new(1).scaled
      assert_equal -10, (KidSummer.new(1, -3).kid_scaled)
      narrowed = [] #: Array[String]
      all.each do |x|
        if x.is_a?(InLeaf)
          narrowed << "leaf extra #{x.extra}"
        elsif x.is_a?(Middle)
          narrowed << "middle #{x.tag}"
        end
      end
      assert_equal ["middle m", "leaf extra 3"], narrowed
      found = all.find { |x| x.is_a?(Fixed) }
      assert_equal "fixed", (found.tag if found)
      list = [InAnimal.new, InDog.new, Puppy.new, Cat.new] #: Array[InAnimal]
      assert_equal ["other", "dog woof", "puppy woof", "cat"], list.map { |a| in_kind(a) }
      kc = InKidCalc.new
      assert_equal 12, kc.compute(1)
      assert_equal 4, (kc.compute(1, 1))
      assert_equal 0, (kc.compute(-1, 0))
      assert_equal "calc!", kc.plain
      assert_equal ["<a><aa>calc!", "<b><bb>calc!"], kc.parts
      assert_equal "<x><xx>", kc.tag("x")
      assert_equal 16, GrandCalc.new.compute(1)
      assert_equal 48, (GrandCalc.new.compute(2, 5))
      assert_equal 20, InCalc.new.compute(2)
      assert_equal 1, InNode.new.id
      assert_equal 2, InNamed.new("n").id
      assert_equal "m", (InNamed.new("m", 7).label)
      assert_equal 2, Leafy.new.id
      assert_equal "leafy", Leafy.new.label
      assert_equal 42, IntBox.new.v
      assert_equal "ss", StrBox.new.v
      assert_equal 9, StrBox.new.id
      assert_equal 1, IntBox.new.id
      nodes = [InNode.new, Leafy.new, IntBox.new, StrBox.new] #: Array[InNode]
      assert_equal [1, 2, 1, 9], nodes.map(&:id)
      assert_equal ["ObjectTests::InNode", "ObjectTests::Leafy", "ObjectTests::IntBox", "ObjectTests::StrBox"], (nodes.map { |x| x.class.name })
      assert_equal 1, (nodes.select { |x| x.is_a?(InNamed) }.size)
      k1 = nodes.fetch(0)
      k2 = nodes.fetch(1)
      by_obj = { k1 => "one" } #: Hash[InNode, String]
      by_obj[k2] = "two"
      assert_equal "one", by_obj[k1]
      assert_equal "two", by_obj[k2]
      assert_nil by_obj[InNode.new]
      assert_equal 2, by_obj.size
      assert_equal true, nodes.include?(k1)
      assert_equal false, [k1].include?(k2)
      assert_equal 2, ([k1, k1, k2].uniq.size)
      assert_equal false, (k1 == k2)
      assert_equal "ObjectTests::StrBox", (nodes.max_by(&:id).class).to_s
      assert_equal 1, nodes.min_by(&:id).id
      assert_equal [9, 2, 1, 1], (nodes.sort_by { |x| -x.id }.map(&:id))
      assert_equal "[ObjectTests::InNode, ObjectTests::Leafy, ObjectTests::IntBox, ObjectTests::StrBox]", (nodes.group_by(&:class).keys).inspect
      assert_equal true, k1.equal?(nodes.fetch(0))
      assert_equal "area 4", Sq.new.show
      nie = assert_raises(NotImplementedError) { Figure.new.show }
      assert_equal "ObjectTests::Figure#area", nie.message
      # a bare rescue (StandardError) must not catch NotImplementedError (a ScriptError)
      caught = "none"
      begin
        begin
          Figure.new.area
        rescue => e
          caught = "bare rescue caught #{e.class}"
        end
      rescue NotImplementedError
        caught = "only NotImplementedError caught it"
      end
      assert_equal "only NotImplementedError caught it", caught
      hb = InHtmlBuilder.new
      assert_equal "<p>a-b</p>", hb.add("a").add("b").html
      assert_equal "a-b-c", hb.add("c").build
      hx = hb.add("d")
      assert_equal "<p>a-b-c-d</p>", hx.html
      assert_equal true, hx.equal?(hb)
      builders = [InBuilder.new, InHtmlBuilder.new] #: Array[InBuilder]
      assert_equal ["z", "z"], (builders.map { |x| x.add("z").build })
    end
  end

  class ObjectMidTest < Minitest::Test
    def test_section_0
      box_a = MdBox.new
      ret_a = (box_a.n = 42)
      assert_equal 42, ret_a
      assert_equal 42, box_a.n
      box_c = MdBox.new
      ret_x = box_c.n = 7
      assert_equal 7, ret_x
      assert_equal 0, MdTemp::FREEZING.deg
      assert_equal 100, MdTemp::BOILING.deg
      colors = [] #: Array[String]
      MdColors.each { |color| colors << color }
      assert_equal ["red", "green"], colors
      counts = [] #: Array[Integer]
      MdCounter.upto(3) { |i| counts << i }
      assert_equal [0, 1, 2], counts
      assert_equal [:SIZE], MdChild.constants(false)
      assert_equal false, (MdChild.const_defined?(:COLOR, false))
      name_err = assert_raises(NameError) { MdChild.const_get(:COLOR, false) }
      assert_equal "uninitialized constant ObjectTests::MdChild::COLOR", name_err.message
      assert_equal 5, PathChild::SIZE
      assert_equal "red", PathChild::COLOR
      assert_equal "nested", PathChild::Nested.new.hi
      gvs = [MdVec.new(1), MdVec.new(-2), MdVec.new(5)] #: Array[MdVec]
      assert_equal 8, (MdVec.new(4).then { |vb| vb.v * 2 })
      assert_equal "v=1", (MdVec.new(1).then { |vb| "v=#{vb.v}" })
      assert_equal 4, (gvs.reduce(MdVec.new(0)) { |acc, vb| acc.plus(vb) }.v)
      assert_equal false, gvs.include?(MdVec.new(5))
      klasses = [MdVec, Vec3] #: Array[singleton(MdVec)]
      assert_equal true, klasses.include?(Vec3)
      assert_equal "CustomWidget", MdWidget.name
      assert_equal "CustomWidget", SubWidget.name
      assert_equal "CustomWidget", SubWidget.new.class.name
      assert_equal "GadgetClass", (MdGadget).to_s
      assert_equal "GadgetClass", (SubGadget).to_s
      assert_equal "GadgetClass", "#{SubGadget}"
      assert_equal "GadgetInspect", (SubGadget).inspect
      assert_equal "[GadgetInspect]", ([SubGadget]).inspect
      d = MdDoc.new
      pg = Page.new
      assert_equal "ObjectTests::MdDoc", (d.class).to_s
      assert_equal "ObjectTests::Page", (pg.class).to_s
      assert_equal false, d.is_a?(Walker)
      assert_equal true, pg.is_a?(Walker)
      m = MdMemo.new
      assert_nil m.last
      assert_equal 42, m.store(21)
      assert_equal 42, m.last
      ibox = ItemBox.new
      ibox.items << "a"
      Reg.table["x"] = 1
      assert_equal ["a"], ibox.items
      assert_equal "{\"x\" => 1}", (Reg.table).inspect
      sub_a = MdBase.new #: MdBase
      sub_a = MdLeaf.new
      assert_equal "MdLeaf", sub_a.name
      sub_x = MdBase.new
      sub_x = MdLeaf.new
      assert_equal "MdLeaf", sub_x.name
      sub_y = MdLeaf.new #: MdBase
      assert_equal "MdLeaf", sub_y.name
      sub_y = MdBase.new
      assert_equal "MdBase", sub_y.name
      sub_k = MdLeaf #: singleton(MdBase)
      assert_equal "ObjectTests::MdLeaf", sub_k.name
      sub_k = MdBase
      assert_equal "ObjectTests::MdBase", sub_k.name
      assert_equal 2, MdCalc.new.calc(1)
      assert_equal 5, MdKidCalc.new.calc(2)
      assert_equal "node", NodeLeaf.new.node
      assert_equal 5, MdUses.new.lim
      assert_equal 10, MdSub.new.twice
      assert_equal 5, MdUses::LIMIT
      assert_equal 5, MdSub::LIMIT
    end
  end

  class ObjectMid2Test < Minitest::Test
    def test_section_0
      assert_equal "ObjectTests::MtFoo", MtFoo.new.kind
      assert_equal "ObjectTests::Bar", Bar.new.kind
      assert_equal "ObjectTests::Bar!", Bar.new.label
      items = [MtDoc.new, MtMemo.new] #: Array[Printable]
      assert_equal ["doc", "memo"], (items.map { |i| i.title })
      assert_equal ["P(doc)", "P(memo)"], items.map(&:to_s)
      a = NeVec.new(1)
      assert_equal true, (a == NeVec.new(1))
      assert_equal false, (a != NeVec.new(1))
      assert_equal true, (a != NeVec.new(2))
      assert_equal false, (NePoint.new(1, 2) != NePoint.new(1, 2))
      assert_equal true, (NePoint.new(1, 2) != NePoint.new(2, 1))
      assert_equal "base 1", MtBase.new.f(1)
      assert_equal "sub", MtSub.new.f
      assert_equal "base", MtBase.new.me.name
      assert_equal "sub", MtSub.new.me.name
      assert_equal "only", MtSub.new.me.only
      v = MtVault.new
      assert_equal "pin has 4 digits, hit 1", v.open
      assert_equal "pin has 4 digits, hit 2", v.open
      assert_equal "shown", PrivCounter.shown
      assert_equal "still public", PrivCounter.hidden
      assert_equal "one", MtC.new.one
      assert_equal "two", MtC.new.two
      assert_equal "three", MtM::K.new.three
      ks = [MtSquare, MtShape] #: Array[singleton(MtShape)]
      assert_equal "ObjectTests::MtSquare", cname(MtSquare)
      assert_equal "ObjectTests::MtSquare", mname(MtSquare)
      assert_equal "ObjectTests::MtSquare", cname(ks.fetch(0))
      assert_equal "ObjectTests::MtShape", mname(ks.fetch(1))
      counts = {} #: Hash[Class, Integer]
      [MtSquare, MtShape, MtSquare].each { |k| counts[k] = counts.fetch(k, 0) + 1 }
      assert_equal "{ObjectTests::MtSquare => 2, ObjectTests::MtShape => 1}", (counts).inspect
      assert_equal "#<struct ObjectTests::MtPoint x=1, y=0>", (MtPoint.new(1)).inspect
      assert_equal 3, (MtPoint.new(1, 2).sum)
      assert_equal "P#<struct ObjectTests::MtPair a=1, b=2>", ((MtPair.new(1, 2))).inspect
      assert_equal false, (EqPoint.new(1, 2) == EqPoint3.new(1, 2))
      assert_equal false, (EqPoint3.new(1, 2) == EqPoint.new(1, 2))
      assert_equal true, (EqPoint3.new(1, 2) == EqPoint3.new(1, 2))
      assert_equal false, (MtVal.new(1) == SubVal.new(1))
      assert_equal false, (SubVal.new(1) == MtVal.new(1))
      assert_equal "#<data ObjectTests::SubVal v=1>", (SubVal.new(1)).inspect
      assert_equal "#<data ObjectTests::SubVal v=3>", ((SubVal.new(2).with(v: 3))).inspect
      l = MtLoud.new("ann")
      assert_equal "ANN", l.name
      l.name = "bob"
      assert_equal "BOB!", l.name
      vs = [vec(1), vec(2)] #: Array[MtVec]
      assert_equal true, vs.include?(vec(1))
      assert_equal false, vs.include?(vec(3))
      assert_equal true, (vs == [vec(1), vec(2)])
      assert_equal false, (vs == [vec(2), vec(1)])
      h = { a: vec(1) } #: Hash[Symbol, MtVec]
      assert_equal true, (h == { a: vec(1) })
      assert_equal 1, MtBox.new.id
      assert_equal [:test_base, :test_child, :test_shared], PimChild.public_instance_methods(true).grep(/^test_/).sort
      assert_equal [:==, :test_child], PimChild.public_instance_methods(false).sort
      assert_equal [:==, :test_child], PimChild.instance_methods(false).sort
      assert_equal true, PimChild.method_defined?(:test_base)
      assert_equal false, PimChild.public_method_defined?("test_private")
      assert_equal false, (PimChild.method_defined?(:test_base, false))
      pim_k = PimChild #: singleton(PimBase)
      assert_equal ["test_base", "test_child", "test_shared"], pim_k.public_instance_methods.grep(/^test_/).map(&:to_s).sort
      assert_equal [:test_shared], PimShared.instance_methods.sort
    end
  end

  class ObjectModulesTest < Minitest::Test
    def test_section_0
      d = MoDoc.new("readme")
      assert_equal "[readme]", d.label
      assert_equal "[README]", d.shout
      assert_equal "tag:both", MoBoth.new.label
      assert_equal "TAG:BOTH", MoBoth.new.shout
      assert_equal 4, MoBoth.new.tag_len
      assert_equal "<[ov]>", Override.new.label
      assert_equal "<[OV]>", Override.new.shout
      assert_equal "[ann] greets bob", MoPerson.new.greet("bob")
      assert_equal "[ANN]", MoPerson.new.shout
      assert_equal "[]", MoDoc.new("").label
      assert_equal "[ÜNÏ]", MoDoc.new("ünï").shout
      assert_equal "a", MoA.new.hi
      assert_equal "loud", MoB.new.hi
      assert_equal "c/loud", MoC.new.hi
      assert_equal "base", MoC.new.base_only
      assert_equal "[Q]", MoQ.new("q").shout
      assert_equal "[q]", MoQ.new("q").label
      assert_equal ["ALPHA", "BETA", "GAMMA"], (MoRegistry.map { |s| s.upcase })
      assert_equal 3, (MoRegistry.select { |s| s.include?("a") }.size)
      assert_equal "beta", (MoRegistry.find { |s| s.start_with?("b") })
      assert_equal ["alpha", "beta", "gamma"], MoRegistry.to_a
      assert_equal 3, MoRegistry.count
      assert_equal true, MoRegistry.include?("beta")
      assert_equal ["alpha", "beta"], MoRegistry.first(2)
      assert_equal ["alpha", "gamma", "beta"], (MoRegistry.sort_by { |s| -s.size })
      assert_equal 42, Util.double(21)
      assert_equal -12, Util.quad(-3)
      assert_equal 0, Util.quad(0)
      assert_equal "ObjectTests::Util", (Util).to_s
      assert_equal "ObjectTests::Util", Util.name
      assert_equal "Module", (Util.class).to_s
      assert_equal "ObjectTests::MoRegistry", (MoRegistry).inspect
      pe = Member.new("ann")
      jr = Junior.new("kid")
      assert_equal "hi ann!", pe.greet
      assert_equal "ann", pe.touch.touch.name
      assert_equal "HI ANN!!", pe.loud
      assert_equal "hi jr!", jr.touch.greet
      assert_equal "HI JR!!", jr.loud
      twos = [] #: Array[String]
      pe.each_two { |i| twos << "two #{i}" }
      jr.each_two do |i|
        next if i == 1
        twos << "junior two #{i}"
      end
      assert_equal ["two 1", "two 2", "junior two 2"], twos
      assert_equal [50, 60], (pe.mapped(5) { |x| x * 10 })
      assert_equal [-2, -1], (jr.mapped(-1) { |x| x - 1 })
      assert_equal true, pe.is_a?(Greeting)
      assert_equal true, jr.is_a?(Greeting)
      assert_equal true, jr.is_a?(Member)
      assert_equal true, pe.respond_to?(:greet)
      assert_equal false, pe.respond_to?(:suffix)
      assert_equal "hi", Greeting::PREFIX
      assert_equal "ObjectTests::Greeting", Greeting.name
      assert_equal "ObjectTests::Greeting", (Greeting).inspect
      assert_equal "Module", (Greeting.class).to_s
      members = [pe, jr] #: Array[Member]
      assert_equal ["hi ann!", "hi jr!"], members.map(&:greet)
      assert_equal ["ann", "jr"], (members.map { |x| x.touch.name })
    end
  end

  class ObjectNamespacesTest < Minitest::Test
    def test_section_0
      assert_equal ["start", "init first", "init second", "middle", "init third"], NS_TRACE
      assert_equal "outer 2 10 second", Outer::Inner::Leaf.new.where
      assert_equal "top", Outer::Inner::Leaf.new.top
      assert_equal 3, Outer::Derived.new.scale
      assert_equal 6, Outer::Derived.new.doubled
      assert_equal 6, Outer::Derived.new.list_sum
      assert_equal "top", Outer::Compact.new.name_seen
      assert_equal "outer", Outer::Compact.new.qualified
      assert_equal "outer", Outer::Compact.new.reopened
      assert_equal 4, NsOther.new.inherited_scale
      assert_equal 3, NsOther.new.scale
      assert_equal "outer", Outer::OBJECT_NS_NAME
      assert_equal 2, Outer::Inner::DEPTH
      assert_equal 3, Outer::Base::SCALE
      assert_equal [1, 2, 3], Outer::LIST
      assert_equal "{\"a\" => 1}", (Outer::TABLE).inspect
      assert_equal [], Outer::EMPTY
      assert_equal "outer 2 10 second", Outer::ORIGIN.where
      assert_equal 10, ::OBJECT_NS_LIMIT
      assert_equal "first", FIRST
      assert_equal "third", Outer::THIRD
      assert_equal "ObjectTests::Outer", Outer.name
      assert_equal "ObjectTests::Outer::Inner", Outer::Inner.name
      assert_equal "ObjectTests::Outer::Inner::Leaf", Outer::Inner::Leaf.name
      assert_equal "ObjectTests::Outer::Compact", Outer::Compact.name
      assert_equal "ObjectTests::NsOther", NsOther.name
      assert_equal "ObjectTests::Outer::Inner::Leaf", (Outer::Inner::Leaf).to_s
      assert_equal "ObjectTests::Outer::Inner::Leaf", (Outer::Inner::Leaf).inspect
      assert_equal "ObjectTests::Outer::Derived", (Outer::Derived.new.class).to_s
      assert_equal "ObjectTests::Outer::Derived", Outer::Derived.new.class.name
      assert_equal "Module", (Outer.class).to_s
      assert_equal "Module", (Outer::Inner.class).to_s
      assert_equal "Class", (Outer::Base.class).to_s
      Outer::LIST << 4
      assert_equal [1, 2, 3, 4], Outer::LIST
      assert_equal 10, Outer::Derived.new.list_sum
      assert_equal [1, -2], Outer::Factory.make.map(&:v)
      assert_equal -1, Outer::Factory.make.fetch(0).plus(Outer::Factory.make.fetch(1)).v
      assert_equal 3, Outer::Factory.build(Outer::Derived).scale
      assert_equal "ObjectTests::Outer::Derived", (Outer::Factory.build(Outer::Derived).class).to_s
      ex = Expr::Add.new(Expr::Num.new(1), Expr::Add.of(2, 3))
      assert_equal "(1 + (2 + 3))", ex.show
      assert_equal 1, ex.prec
      assert_equal 0, Expr::Num.new(4).prec
      assert_equal 0, Expr.new.prec
      assert_equal "(5 + 6)", (Expr::Add.of(5, 6).show)
      assert_equal "ObjectTests::Expr::Add", Expr::Add.name
      assert_equal "ObjectTests::Expr::Num", (Expr::Num.new(1).class).to_s
      assert_equal 1, Expr::Add::PRECEDENCE
      assert_equal 0, Expr::PRECEDENCE
      assert_equal 81, App::Server.port
      assert_equal "80/81", App::Server::Handler.new.where
      assert_equal "ObjectTests::App::Server::Handler", App::Server::Handler.name
      assert_equal 80, App::Config::PORT
      exprs = [Expr::Num.new(7), ex] #: Array[Expr]
      assert_equal ["7", "(1 + (2 + 3))"], exprs.map(&:show)
      assert_equal [false, true], (exprs.map { |x| x.is_a?(Expr::Add) })
      assert_equal "[ObjectTests::Expr::Num, ObjectTests::Expr::Add]", (exprs.map(&:class)).inspect
      tk = Taker.new
      assert_equal "item", tk.make.hi
      assert_equal 5, (tk.count([tk.make, Holder::Item.new]))
      assert_equal 5, Outer::Inner.vec(4).plus(Outer::Inner.vec(1)).v
    end
  end

  class ObjectOperatorsTest < Minitest::Test
    def test_section_0
      a = OpVec.new(1, 2)
      b = OpVec.new(3, -4)
      assert_equal "(4, -2)", ((a + b)).to_s
      assert_equal "(-2, 6)", ((a - b)).to_s
      assert_equal "(3, 6)", ((a * 3)).to_s
      assert_equal "(-1, -2)", (-a).to_s
      assert_equal "(0, 0)", ((a * 0)).to_s
      assert_equal "(1, 2)", ((a + b - b)).to_s
      assert_equal true, (a == OpVec.new(1, 2))
      assert_equal false, (a == b)
      assert_equal 1, a[0]
      assert_equal 2, a[1]
      assert_equal false, a.zero?
      assert_equal true, (OpVec.new(0, 0).zero?)
      assert_equal "(-1, -2)", (a.negate!).to_s
      assert_equal "[#<OpVec 1,2>, #<OpVec 3,-4>]", (([a, b])).inspect
      assert_equal "#<OpVec 1,2>", (a).inspect
      assert_equal "(1, 2)", "#{a}"
      assert_equal "[[#<OpVec 1,2>]]", ([[a]]).inspect
      a.x = 10
      a[1] = -7
      assert_equal "(10, -7)", (a).to_s
      assert_equal "#<OpVec 10,-7>", (a).inspect
      list = [a, b] #: Array[OpVec]
      assert_equal "[#<OpVec -10,7>, #<OpVec -3,4>]", ((list.map { |v| -v })).inspect
      assert_equal "(23, -18)", ((list.reduce(list.fetch(0)) { |acc, v| acc + v })).to_s
      bag = OpBag.new
      bag << 1 << 2 << 3
      bag << 4
      assert_equal 10, bag.sum
      assert_equal 4, (+bag)
      assert_equal -4, (~bag)
      assert_equal [2, 4], bag.evens
      pl = OpPlain.new
      sh = Shown.new
      th = OpDeep::Thing.new
      assert_equal true, pl.inspect.start_with?("#<ObjectTests::OpPlain")
      assert_equal true, pl.to_s.start_with?("#<ObjectTests::OpPlain")
      assert_equal false, sh.inspect.include?("shown!")
      assert_equal true, sh.inspect.start_with?("#<ObjectTests::Shown")
      assert_equal true, th.to_s.start_with?("#<ObjectTests::OpDeep::Thing")
      assert_equal true, [pl].inspect.start_with?("[#<ObjectTests::OpPlain")
      assert_equal true, "#{pl}".start_with?("#<ObjectTests::OpPlain")
      assert_equal "shown!", "#{sh}"
      c1 = Coin.new(5)
      assert_equal true, (c1 == Coin.new(5))
      assert_equal false, (c1 == Coin.new(6))
      assert_equal false, (c1 == 5)
      assert_equal false, (c1 == "5")
      assert_equal false, (c1 == nil)
      coins = [c1, coin(1)] #: Array[Coin]
      assert_equal true, coins.include?(coin(1))
      assert_equal false, coins.include?(coin(2))
      assert_equal 2, coins.uniq.size
      assert_equal 2, ([c1, coin(5)].uniq.size)
      assert_equal 6, (list.fetch(1).then { |v| v.x * 2 })
      assert_equal "c=3", (coin(3).then { |c| "c=#{c.cents}" })
      assert_equal 6, (coins.reduce(coin(0)) { |a, c| coin(a.cents + c.cents) }.cents)
    end
  end

  class ObjectOverrideChainTest < Minitest::Test
    def test_section_0
      assert_equal "base 1", OcBase.new.f(1)
      assert_equal "sub 3 base 3", OcSub.new.f
      assert_equal "sub 5 base 5", OcSub.new.f(5)
      assert_equal "subsub", SubSub.new.f
      assert_equal "subsub", OcLeaf.new.f
      assert_equal "sub 7 base 7", OcSub.new.call_f
      shown = [OcBase.new, OcSub.new, SubSub.new, OcLeaf.new].map { |b| object_show(b) }
      assert_equal ["base base base", "sub sub sub", "subsub subsub subsub", "subsub subsub subsub"], shown
      s = OcSub.new
      assert_equal false, s.same?(s)
      assert_equal false, OcBase.new.same?(s)
      assert_equal "only", OcLeaf.new.me.only
      assert_equal "only", OcLeaf.new.dup_me.only
      x = OcSub.new #: untyped
      assert_equal "sub 2 base 2", x.f(2)
      assert_equal "sub 3 base 3", x.f
      assert_equal "OcBase.make x", OcBase.make("x")
      assert_equal "OcSub.make", OcSub.make
      k = OcSub #: singleton(OcBase)
      e = assert_raises(ArgumentError) { k.make("y") }
      assert_equal "wrong number of arguments (given 1, expected 0)", e.message
      e = assert_raises(ArgumentError) { SubSub.new.call_f }
      assert_equal "wrong number of arguments (given 1, expected 0)", e.message
    end
  end

  class ObjectStructDataTest < Minitest::Test
    def test_section_0
      origin = SdPoint.new(0, 0)
      pt = SdPoint.new(3, -4)
      assert_equal 3, pt.x
      assert_equal -4, pt.y
      assert_equal "#<struct ObjectTests::SdPoint x=3, y=-4>", (pt).inspect
      assert_equal "#<struct ObjectTests::SdPoint x=3, y=-4>", pt.to_s
      assert_equal "#<struct ObjectTests::SdPoint x=0, y=0>", (origin).inspect
      pt.x = 10
      pt.y = pt.y - 1
      assert_equal "#<struct ObjectTests::SdPoint x=10, y=-5>", (pt).inspect
      assert_equal [10, -5], pt.to_a
      assert_equal "{x: 10, y: -5}", (pt.to_h).inspect
      assert_equal [:x, :y], pt.members
      assert_equal [:x, :y], SdPoint.members
      assert_equal true, (pt == SdPoint.new(10, -5))
      assert_equal false, (pt == origin)
      assert_equal true, (pt != origin)
      assert_equal true, (pt == pt)
      assert_equal "#<struct ObjectTests::SdPoint x=1, y=2>", ((SdPoint.new(y: 2, x: 1))).inspect
      assert_equal [7, 8], (SdPoint.new(x: 7, y: 8).to_a)
      assert_equal "ObjectTests::SdPoint", SdPoint.name
      assert_equal "ObjectTests::SdPoint", ((SdPoint.new(1, 1).class)).to_s
      assert_equal "ObjectTests::SdPoint", (SdPoint).inspect
      pair = SdPair.new("ünï", 1.0)
      assert_equal "#<struct ObjectTests::SdPair left=\"ünï\", right=1.0>", (pair).inspect
      assert_equal "ünï", pair.left
      assert_equal "1.0", (pair.right).inspect
      assert_equal "#<struct ObjectTests::SdPair left=\"\", right=-0.5>", ((SdPair.new("", -0.5))).inspect
      pair.left = "quote\"d"
      assert_equal "#<struct ObjectTests::SdPair left=\"quote\\\"d\", right=1.0>", (pair).inspect
      assert_equal "{left: \"quote\\\"d\", right: 1.0}", (pair.to_h).inspect
      assert_equal "#<struct ObjectTests::Rec id=1, name=nil, note=nil>", (Rec.new(1)).inspect
      assert_equal "#<struct ObjectTests::Rec id=2, name=\"two\", note=nil>", ((Rec.new(2, "two"))).inspect
      assert_equal "#<struct ObjectTests::Rec id=3, name=\"three\", note=\"n\">", ((Rec.new(3, "three", "n"))).inspect
      assert_equal "1:-:-", Rec.new(1).label
      assert_equal "2:b:-", (Rec.new(2, "b").label)
      assert_equal "9:-:-", Rec.numbered(9).label
      assert_equal "named 4:d:-", (SdNamed.new(4, "d").label)
      assert_equal "#<struct ObjectTests::SdNamed id=5, name=nil, note=nil>", (SdNamed.new(5)).inspect
      assert_equal "#<struct ObjectTests::SdNamed id=6, name=nil, note=nil>", (SdNamed.numbered(6)).inspect
      assert_equal "#<struct ObjectTests::Rec id=7, name=nil, note=\"kw\">", ((Rec.new(id: 7, note: "kw"))).inspect
      assert_equal true, (SdNamed.new(1) == SdNamed.new(1))
      assert_equal false, (Rec.new(1, "a") == Rec.new(1, "b"))
      o = Rec.new(8)
      o.name = "late"
      assert_equal "8:late:-", o.label
      assert_equal [8, "late", nil], o.to_a
      assert_equal [:id, :name, :note], o.members
      here = SdGeo::Coord.new(lat: 1.5, lng: -2.0)
      there = here.with(lng: 3.25)
      same = here.with
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=1.5, lng=-2.0>", (here).inspect
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=1.5, lng=3.25>", (there).inspect
      assert_equal "(1.5, -2.0)", (here).to_s
      assert_equal "(1.5, 3.25)", (there).to_s
      assert_equal "-0.5", (here.sum).inspect
      assert_equal "4.75", (there.sum).inspect
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=1.5, lng=-2.0>", (same).inspect
      assert_equal true, (same == here)
      assert_equal false, (here == there)
      assert_equal true, (here == SdGeo::Coord.new(1.5, -2.0))
      assert_equal "{lat: 1.5, lng: -2.0}", (here.to_h).inspect
      assert_equal [:lat, :lng], SdGeo::Coord.members
      assert_equal [:lat, :lng], here.members
      assert_equal "ObjectTests::SdGeo::Coord", SdGeo::Coord.name
      assert_equal "#<data ObjectTests::SdGeo::Coord lat=0.0, lng=0.0>", ((here.with(lat: 0.0, lng: 0.0))).inspect
      assert_equal "#<struct ObjectTests::SdGeo::Tag text=\"t\">", (SdGeo::Tag.new("t")).inspect
      assert_equal ["t"], SdGeo::Tag.new("t").to_a
      m = Money.new(1999, "EUR")
      assert_equal "#<data ObjectTests::Money cents=1999, currency=\"EUR\">", (m).inspect
      assert_equal 1999, m.cents
      assert_equal "#<data ObjectTests::Money cents=-5, currency=\"USD\">", ((Money.new(currency: "USD", cents: -5))).inspect
      assert_equal "#<data ObjectTests::Money cents=0, currency=\"EUR\">", ((m.with(cents: 0))).inspect
      assert_equal true, (m == Money.new(1999, "EUR"))
      assert_equal false, (m == m.with(currency: "GBP"))
      assert_equal "{cents: 1999, currency: \"EUR\"}", (m.to_h).inspect
      assert_equal 9000000000000, (Money.new(9_000_000_000_000, "JPY").cents)
      points = [SdPoint.new(2, 1), SdPoint.new(1, 2), SdPoint.new(1, 1)] #: Array[SdPoint]
      assert_equal [[1, 1], [1, 2], [2, 1]], (points.sort_by { |q| (q.x * 10) + q.y }.map(&:to_a))
      assert_equal [2, 1, 1], points.map(&:x)
      assert_equal "[#<struct ObjectTests::SdPoint x=2, y=1>, #<struct ObjectTests::SdPoint x=1, y=1>]", ((points.select { |q| q.y == 1 })).inspect
      w = Wrap.new(Inner.new(1), ["a", "b"], { k: 1 })
      assert_equal "#<struct ObjectTests::Wrap inner=#<struct ObjectTests::Inner a=1>, list=[\"a\", \"b\"], map={k: 1}>", (w).inspect
      assert_equal "[#<struct ObjectTests::Inner a=1>, [\"a\", \"b\"], {k: 1}]", (w.to_a).inspect
      assert_equal "{inner: #<struct ObjectTests::Inner a=1>, list: [\"a\", \"b\"], map: {k: 1}}", (w.to_h).inspect
      k = Kw.new("x", 2, "f", "m", true, 0)
      assert_equal "#<struct ObjectTests::Kw type=\"x\", range=2, func=\"f\", map=\"m\", go=true, select=0>", (k).inspect
      assert_equal "x", k.type
      assert_equal 2, k.range
      assert_equal true, k.go
      k.type = "y"
      assert_equal "{type: \"y\", range: 2, func: \"f\", map: \"m\", go: true, select: 0}", (k.to_h).inspect
      t = SdThing.new
      t.interface = 5
      assert_equal "t", t.type
      assert_equal 5, t.interface
      assert_equal "len", t.len
      assert_equal "string", t.string
      assert_equal "default", t.default
      assert_equal "#<data ObjectTests::SdVal v=-0.0>", (SdVal.new(-0.0)).inspect
      assert_equal "#<data ObjectTests::SdVal v=1.0e+20>", (SdVal.new(1e20)).inspect
      assert_equal "#<data ObjectTests::SdVal v=0.3333333333333333>", ((SdVal.new(1.0 / 3))).inspect
      list = ListNode.new(1, ListNode.new(2, ListNode.new(3)))
      assert_equal 6, object_total(list)
      assert_equal "#<struct ObjectTests::ListNode val=2, nxt=#<struct ObjectTests::ListNode val=3, nxt=nil>>", (list.nxt).inspect
      assert_equal "#<struct ObjectTests::ListNode val=9, nxt=nil>", (ListNode.new(9)).inspect
      assert_equal 0, object_total(nil)
      list.nxt = nil
      assert_equal 1, object_total(list)
      assert_equal true, (list == ListNode.new(1))
      tr = Tree.new(1, [Tree.new(2, []), Tree.new(3, [Tree.new(4, [])])])
      assert_equal 10, tsum(tr)
      assert_equal 2, tr.kids.size
      assert_equal "#<struct ObjectTests::Tree val=2, kids=[]>", (tr.kids.fetch(0)).inspect
      p3 = Pt3.new(1, 2, 3)
      assert_equal 6, p3.sum
      assert_equal 1, p3.x
      assert_equal [1, 2], p3.to_a
      assert_equal "#<struct ObjectTests::Pt3 x=1, y=2>", (p3).inspect
      assert_equal [:x, :y], p3.members
      assert_equal 3, p3.z
      p3.x = 10
      assert_equal 15, p3.sum
      assert_equal "{x: 10, y: 2}", (p3.to_h).inspect
      assert_equal [:x, :y], Pt3.members
      assert_equal "ObjectTests::Pt3", Pt3.name
    end

    # Struct instances are references: a second name sees the mutation; equal? is identity, == is by value.
    def test_struct_instances_are_references_a
      pa = SdPoint.new(1, 2)
      pb = pa
      pb.x = 9
      assert_equal 9, pa.x
      assert_equal true, pa.equal?(pb)
      assert_equal false, (pa.equal?(SdPoint.new(9, 2)))
      assert_equal true, (pa == SdPoint.new(9, 2))
      assert_equal false, (pa == nil)
      assert_equal false, (pa == 9)
      assert_equal false, (pa == [9, 2])
      assert_equal false, (Money.new(1, "x") == SdPoint.new(1, 2))
      assert_equal true, (pa != nil)
    end
  end

  class ObjectVisibilityTest < Minitest::Test
    def test_section_0
      v = ViVault.new("v")
      assert_equal "v opened with **** true", v.open
      assert_equal "ok false", v.status
      assert_equal "terc3s", v.reveal
      assert_equal "top4", v.via_top
      b = BigVault.new("b")
      assert_equal "b opened with **** true", b.open
      assert_equal "big ok false", b.status
      assert_equal "****|1234|s3cret", b.deep
      assert_equal "terc3s", b.reveal
      assert_equal true, v.respond_to?(:open)
      assert_equal false, v.respond_to?(:pin)
      assert_equal true, v.respond_to?(:status)
      assert_equal "top-1", top_helper(-1)
      d = Dial.new
      assert_equal [1, 2], d.scaled(1)
      assert_equal 5, d.set(5)
      assert_equal [60, 70], d.scaled(10)
      assert_equal false, d.respond_to?(:pin=)
    end
  end

  # Forwardable (decision 99).
  class Deck
    extend Forwardable
    include Enumerable #[Integer]

    # @rbs @cards: Array[Integer]

    def_delegators :@cards, :size, :each, :<<, :first, :[]
    def_delegator :@cards, :join, :to_text
    def_delegator :tally, :fetch, :count_of
    delegate %i[keys] => :@marks

    #: () -> void
    def initialize
      @cards = [3, 1, 2]
      @marks = { "a" => 1 } #: Hash[String, Integer]
    end

    #: () -> Hash[Integer, Integer]
    def tally = @cards.tally
  end

  class ForwardableTest < Minitest::Test
    def test_delegators
      d = Deck.new
      d << 5
      assert_equal [4, 3, 1, 5], [d.size, d.first, d[1], d[3]]
      assert_equal "3-1-2-5", d.to_text("-")
      assert_equal "3125", d.to_text
      assert_equal [1, 2, 3, 5], d.sort
      assert_equal 11, d.sum
      assert_equal 1, d.count_of(3)
      assert_equal 0, d.count_of(9, 0)
      assert_equal ["a"], d.keys
      assert_equal "key not found: 9", assert_raises(KeyError) { d.count_of(9) }.message
    end
  end
  # Exception#backtrace (decision 106): MRI 4.0's labels, first frame exact.
  class BacktraceTest < Minitest::Test
    #: (Exception) -> Array[String]
    def frames(e) = e.backtrace || []

    #: () -> void
    def deep
      [1].each { |x| [x].each { raise IOError, "deep" } }
    end

    #: () -> void
    def reraise
      deep
    rescue IOError => e
      raise e
    end

    #: () -> void
    def wrap
      deep
    rescue IOError
      raise ArgumentError, "wrapped"
    end

    def test_frames
      e = assert_raises(IOError) { deep }
      bt = frames(e)
      assert_match(/object_test\.rb:\d+:in 'block \(2 levels\) in ObjectTests::BacktraceTest#deep'\z/, bt.fetch(0))
      assert_match(/object_test\.rb:\d+:in 'Array#each'\z/, bt.fetch(1))
      assert_match(/object_test\.rb:\d+:in 'block in ObjectTests::BacktraceTest#deep'\z/, bt.fetch(2))
      assert_match(/object_test\.rb:\d+:in 'Array#each'\z/, bt.fetch(3))
      assert_match(/object_test\.rb:\d+:in 'ObjectTests::BacktraceTest#deep'\z/, bt.fetch(4))
      assert_match(/object_test\.rb:\d+:in 'block in ObjectTests::BacktraceTest#test_frames'\z/, bt.fetch(5))
      assert_equal bt.fetch(0).split(":").fetch(1), bt.fetch(1).split(":").fetch(1) # Array#each at the block's line
    end

    def test_reraise_keeps_frames_and_a_handler_starts_new_ones
      e = assert_raises(IOError) { reraise }
      assert_match(/in 'block \(2 levels\) in ObjectTests::BacktraceTest#deep'\z/, frames(e).fetch(0))
      e2 = assert_raises(ArgumentError) { wrap }
      assert_match(/object_test\.rb:\d+:in 'ObjectTests::BacktraceTest#wrap'\z/, frames(e2).fetch(0))
      assert_match(/in 'block in ObjectTests::BacktraceTest#test_reraise_keeps_frames_and_a_handler_starts_new_ones'\z/, frames(e2).fetch(1))
      assert_equal IOError, e2.cause.class
    end

    def test_set_backtrace_and_never_raised
      made = RuntimeError.new("made")
      assert_nil made.backtrace
      made.set_backtrace(["x.rb:1:in 'Object#y'"])
      assert_equal ["x.rb:1:in 'Object#y'"], made.backtrace
      e = assert_raises(RuntimeError) { raise made }
      assert_equal ["x.rb:1:in 'Object#y'"], e.backtrace
    end
  end

end
