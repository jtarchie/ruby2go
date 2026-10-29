# rbs_inline: enabled

# @rbs generic Node
module TSort
  class Cyclic < StandardError; end

  #: () { (Node) -> void } -> void
  def tsort_each_node = raise(NotImplementedError)

  #: (Node) { (Node) -> void } -> void
  def tsort_each_child(node) = raise(NotImplementedError)

  #: () -> Array[Node]
  def tsort
    result = [] #: Array[Node]
    tsort_each { |n| result << n }
    result
  end

  #: () { (Node) -> void } -> void
  def tsort_each
    each_strongly_connected_component do |component|
      if component.size == 1
        yield component.fetch(0)
      else
        raise Cyclic, "topological sort failed: #{component.inspect}"
      end
    end
  end

  #: () -> Array[Array[Node]]
  def strongly_connected_components
    result = [] #: Array[Array[Node]]
    each_strongly_connected_component { |c| result << c }
    result
  end

  #: () { (Array[Node]) -> void } -> void
  def each_strongly_connected_component
    id_map = {} #: Hash[Node, Integer]
    visited = {} #: Hash[Node, bool]
    stack = [] #: Array[Node]
    components = [] #: Array[Array[Node]]
    tsort_each_node do |node|
      __tsort_scc_from(node, id_map, visited, stack, components) unless visited.key?(node)
    end
    components.each { |c| yield c }
  end

  private

  #: (Node, Hash[Node, Integer], Hash[Node, bool], Array[Node], Array[Array[Node]]) -> Integer
  def __tsort_scc_from(node, id_map, visited, stack, components)
    node_id = id_map.size
    id_map[node] = node_id
    visited[node] = true
    minimum_id = node_id
    stack_length = stack.length
    stack << node

    tsort_each_child(node) do |child|
      if id_map.key?(child)
        child_id = id_map[child]
        minimum_id = child_id if child_id && child_id < minimum_id
        next
      end
      next if visited.key?(child)

      sub_minimum_id = __tsort_scc_from(child, id_map, visited, stack, components)
      minimum_id = sub_minimum_id if sub_minimum_id < minimum_id
    end

    if node_id == minimum_id
      component = stack[stack_length..] || [] #: Array[Node]
      (stack.length - stack_length).times { stack.pop }
      component.each { |n| id_map.delete(n) }
      components << component
    end

    minimum_id
  end
end
