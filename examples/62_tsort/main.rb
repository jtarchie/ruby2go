# rbs_inline: enabled
# args: --seed 1

require "tsort"
require "minitest/autorun"

class BuildGraph
  include TSort #[String]

  #: () -> void
  def initialize
    @deps = {} #: Hash[String, Array[String]]
  end

  #: (String, *String) -> void
  def task(name, *deps)
    @deps[name] = deps
  end

  #: () { (String) -> void } -> void
  def tsort_each_node(&block) = @deps.each_key(&block)

  #: (String) { (String) -> void } -> void
  def tsort_each_child(node, &block) = @deps.fetch(node, []).each(&block)
end

class TSortTest < Minitest::Test
  #: () -> BuildGraph
  def build
    graph = BuildGraph.new
    graph.task("parse")
    graph.task("compile", "parse")
    graph.task("link", "compile")
    graph.task("test", "link")
    graph.task("package", "compile", "link")
    graph.task("deploy", "package", "test")
    graph
  end

  #: () -> BuildGraph
  def cycle
    graph = BuildGraph.new
    graph.task("a", "b")
    graph.task("b", "c")
    graph.task("c", "a")
    graph
  end

  def test_tsort_orders_dependencies_first
    assert_equal ["parse", "compile", "link", "test", "package", "deploy"], build.tsort
  end

  def test_an_acyclic_graph_has_singleton_components
    assert_equal [["parse"], ["compile"], ["link"], ["test"], ["package"], ["deploy"]], build.strongly_connected_components
  end

  def test_tsort_each_yields_in_order
    seen = [] #: Array[String]
    build.tsort_each { |t| seen << t }
    assert_equal ["parse", "compile", "link", "test", "package", "deploy"], seen
  end

  def test_a_cycle_raises_cyclic
    e = assert_raises(TSort::Cyclic) { cycle.tsort }
    assert_equal 'topological sort failed: ["a", "b", "c"]', e.message
  end

  def test_a_cycle_is_one_component
    assert_equal [["a", "b", "c"]], cycle.strongly_connected_components
  end
end
