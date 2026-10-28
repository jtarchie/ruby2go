# rbs_inline: enabled

# The unmodified pairing_heap gem: Dijkstra's shortest paths on a MinPriorityQueue
# (decrease-key through change_priority), a MaxPriorityQueue, and a SimplePairingHeap
# with a custom comparator block.
require "pairing_heap"

GRAPH = {
  "a" => { "b" => 7, "c" => 9, "f" => 14 },
  "b" => { "a" => 7, "c" => 10, "d" => 15 },
  "c" => { "a" => 9, "b" => 10, "d" => 11, "f" => 2 },
  "d" => { "b" => 15, "c" => 11, "e" => 6 },
  "e" => { "d" => 6, "f" => 9 },
  "f" => { "a" => 14, "c" => 2, "e" => 9 }
}.freeze #: Hash[String, Hash[String, Integer]]

#: (String source) -> [Hash[String, Integer], Hash[String, String]]
def dijkstra(source)
  dist = { source => 0 } #: Hash[String, Integer]
  prev = {} #: Hash[String, String]
  queue = PairingHeap::MinPriorityQueue.new
  queue.push(source, 0)
  until queue.empty?
    node, d = queue.pop_with_priority
    next unless node && d

    GRAPH.fetch(node).each do |neighbor, weight|
      alt = d + weight
      known = dist[neighbor]
      next if known && known <= alt

      dist[neighbor] = alt
      prev[neighbor] = node
      if queue.include?(neighbor)
        queue.change_priority(neighbor, alt)
      else
        queue.push(neighbor, alt)
      end
    end
  end
  [dist, prev]
end

#: (Hash[String, String] prev, String target) -> Array[String]
def path_to(prev, target)
  path = [target]
  while (step = prev[path.first])
    path.unshift(step)
  end
  path
end

dist, prev = dijkstra("a")
dist.keys.sort.each do |node|
  puts "a -> #{node}: #{dist.fetch(node)} via #{path_to(prev, node).join(" ")}"
end

tasks = PairingHeap::MaxPriorityQueue.new
tasks.push("write docs", 2)
tasks.push("fix crash", 9)
tasks.push("refactor", 4)
tasks.change_priority("write docs", 5)
puts "next task: #{tasks.peek} (#{tasks.peek_priority}), #{tasks.size} queued"
order = [] #: Array[String]
order << tasks.pop.to_s until tasks.empty?
puts "by priority: #{order.join(", ")}"

#: (String l, String r) -> bool
def shorter_first(l, r) = l.length < r.length || (l.length == r.length && l <= r)

words = PairingHeap::SimplePairingHeap.new { |l, r| shorter_first(l, r) }
%w[pear fig banana kiwi apple date].each { |w| words.push(w) }
sorted = [] #: Array[String]
sorted << words.pop.to_s while words.any?
puts "shortest first: #{sorted.join(" ")}"
