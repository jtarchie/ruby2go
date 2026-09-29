# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# Destructuring `|path, n|` needs typed elements, so `busiest` only compiles once @visits is typed.
class Router
  #: () -> void
  def initialize
    @routes = []
    @hits = {}
    @log = []
    @visits = []
  end

  # @rbs path: String
  def add(path)
    @routes << path
    @hits[path] = 0
    self
  end

  # @rbs path: String
  def hit(path)
    @hits[path] = (@hits[path] || 0) + 1 # untyped until @hits is: `add` decides its values
    @log.push("hit #{path}")
    @visits << [path, @hits[path] || 0]
  end

  def busiest = @visits.max_by { |_, n| n }&.first

  def report = @routes.sort.map { |r| "#{r}=#{@hits[r]}" }.join(" ")

  def last = @log.last
end

class TracingRouter < Router
  def trace = @log.map { |l| l.upcase }
end

#: (Array[untyped]) -> void
def tag(xs)
  xs << "tag"
end

class Shelf
  #: () -> void
  def initialize
    @books = []
    @notes = []
  end

  def fill
    @books << "Dune"
    @notes << "n"
    tag(@notes) # typed, @notes would be copied into Array[untyped]: it stays untyped
    self
  end

  def titles = @books.map { |b| b.upcase }

  def notes = @notes
end

class IvarContainersTest < Minitest::Test
  def setup
    @router = TracingRouter.new
    @router.add("/b").add("/a")
    @router.hit("/a")
    @router.hit("/a")
  end

  def test_empty_hash_ivar_takes_its_types_from_the_methods_that_fill_it
    assert_equal "/a=2 /b=0", @router.report
  end

  def test_empty_array_ivar_is_typed_by_push
    assert_equal "hit /a", @router.last
    assert_equal ["HIT /A", "HIT /A"], @router.trace
  end

  def test_typed_tuple_elements_allow_destructuring
    assert_equal "/a", @router.busiest
  end

  def test_an_array_passed_as_untyped_stays_untyped
    shelf = Shelf.new.fill
    assert_equal ["DUNE"], shelf.titles
    assert_equal ["n", "tag"], shelf.notes
  end
end
