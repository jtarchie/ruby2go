module Demo
  # Block-taking Array methods on a typed Array held untyped answer through the view, one body for every element type.
  module Views
    # A computed send (the name is a value): the call reaches the dispatchers, where the view answers with its block.
    #: (untyped, Symbol, *untyped) ?{ (*untyped) -> untyped } -> untyped
    def self.dyn(x, name, *args, &blk) = x.public_send(name, *args, &blk)

    #: (untyped) -> untyped
    def self.blocks(x)
      [
        x.select { |v| v.to_s > "1" }, x.filter { |v| v.to_s < "3" }, x.find_all { |v| v }, x.reject { |v| v.to_s == "2" },
        x.find { |v| v.to_s > "1" }, x.detect { |v| v.nil? }, x.find_index { |v| v.to_s == "2" }, x.index { |v| v.to_s == "9" },
        x.all? { |v| v }, x.any? { |v| v.nil? }, x.none? { |v| v.to_s == "9" }, x.one? { |v| v.to_s == "1" },
        x.all?(Object), x.any?(Integer), x.none?, x.one?,
        x.inject(0) { |a, v| a + v.to_s.size }, x.reduce("") { |a, v| a.to_s + v.to_s }, x.each_with_object([]) { |v, a| a << v.to_s },
        x.sort_by { |v| v.to_s }, x.max_by { |v| v.to_s }, x.min_by { |v| v.to_s }, x.take_while { |v| v.to_s != "2" },
        x.drop_while { |v| v.to_s != "2" }, x.count { |v| v.to_s > "1" }, x.sort { |a, b| b.to_s <=> a.to_s },
        x.uniq { |v| v.to_s.size }, x.first, x.first(2),
      ]
    end

    # A computed send reaches the dispatchers, so the view answers with its block.
    #: (untyped, Array[Symbol]) -> untyped
    def self.sent(x, names)
      names.map { |n| x.send(n) { |v| v.to_s > "1" } }
    end

    #: (untyped) -> untyped
    def self.sent_more(x)
      out = [] #: Array[untyped]
      dyn(x, :each_with_index) { |v, i| out << [v, i] }
      dyn(x, :each_slice, 2) { |s| out << s }
      [out, dyn(x, :inject, 0) { |a, v| a + v.to_s.size }, dyn(x, :sort_by) { |v| v.to_s }, dyn(x, :max_by) { |v| v.to_s },
       dyn(x, :each_with_object, []) { |v, a| a << v }, dyn(x, :sort) { |a, b| b.to_s <=> a.to_s }, dyn(x, :count) { |v| v },
       dyn(x, :find_index) { |v| v.to_s == "1" }, dyn(x, :uniq) { |v| v.to_s.size }, dyn(x, :sum) { |v| v.to_s.size },
       dyn(x, :all?, Object), dyn(x, :none?), dyn(x, :map!) { |v| v }.equal?(x)]
    end

    #: (untyped) -> untyped
    def self.iterate(x)
      out = [] #: Array[untyped]
      r = [dyn(x, :each) { |v| out << v }.equal?(x), dyn(x, :each_with_index) { |v, i| out << [v, i] }.equal?(x)]
      dyn(x, :reverse_each) { |v| out << v }
      dyn(x, :each_index) { |i| out << i }
      dyn(x, :each_slice, 2) { |s| out << s }
      dyn(x, :each_cons, 2) { |s| out << s }
      dyn(x, :each_entry) { |v| out << v }
      dyn(x, :cycle, 2) { |v| out << v }
      n = 0
      dyn(x, :cycle) do |_|
        n += 1
        break if n > 4
      end
      out << n
      [r, out]
    end

    #: (untyped) -> untyped
    def self.mutate(x)
      out = [] #: Array[untyped]
      out << dyn(x, :map!) { |v| v }.equal?(x) << x.dup
      out << dyn(x, :select!) { |v| v } << dyn(x, :reject!) { |v| v.to_s == "9" } << x.dup
      out << dyn(x, :keep_if) { |v| v.to_s != "3" }.dup << dyn(x, :delete_if) { |v| v.nil? }.dup
      out << dyn(x, :sort_by!) { |v| v.to_s }.dup << dyn(x, :filter!) { |v| v.to_s != "1" }.dup
      out
    end

    #: (untyped) -> untyped
    def self.search(x)
      [dyn(x, :bsearch) { |v| v >= 2 }, dyn(x, :bsearch_index) { |v| v >= 2 }, dyn(x, :bsearch) { |v| 2 <=> v }, dyn(x, :bsearch) { |v| v > 9 }, x.sum, x.sum(10), dyn(x, :sum) { |v| v * 2 }]
    end

    # A Hash's block names through the view: the select family yields key and value, the rest the [key, value] pair.
    #: (untyped) -> untyped
    def self.hash_blocks(h)
      out = [] #: Array[untyped]
      dyn(h, :each) { |k, v| out << [k, v] }
      dyn(h, :each_pair) { |pair| out << pair }
      dyn(h, :each_key) { |k| out << k }
      dyn(h, :each_value) { |v| out << v }
      dyn(h, :each_with_index) { |pair, i| out << [pair[0], pair[1], i] }
      dyn(h, :each_slice, 2) { |s| out << s }
      dyn(h, :cycle, 1) { |k, _v| out << k }
      [out, dyn(h, :select) { |_k, v| v.to_s > "1" }, dyn(h, :filter) { |k| k.to_s == "a" }, dyn(h, :reject) { |_, v| v.nil? },
       dyn(h, :count) { |_k, v| v }, dyn(h, :count), dyn(h, :find) { |_k, v| v.to_s == "2" }, dyn(h, :detect) { |k, _| k == "z" },
       dyn(h, :inject, 0) { |a, pair| a + pair[1].to_s.size }, dyn(h, :each_with_object, []) { |pair, a| a << pair[0] },
       dyn(h, :any?) { |_k, v| v.nil? }, dyn(h, :all?) { |pair| pair.size == 2 }, dyn(h, :none?), dyn(h, :one?, Array),
       dyn(h, :any?, ["a", 1])]
    end

    #: (untyped) -> untyped
    def self.hash_mutate(h)
      out = [] #: Array[untyped]
      out << dyn(h, :select!) { |_k, v| v } << h.dup << dyn(h, :reject!) { |k, _v| k == "q" } << dyn(h, :keep_if) { |_k, _v| true }.dup
      out << dyn(h, :transform_values!) { |v| v.to_s * 2 }.dup << dyn(h, :delete_if) { |k, _v| k == "a" }.dup
      out << dyn(h, :filter!) { |_k, _v| false } << h
      out
    end

    #: (untyped) -> untyped
    def self.errors(x)
      out = [] #: Array[untyped]
      begin
        dyn(x, :each_slice, 0) { |s| s }
      rescue ArgumentError => e
        out << e.message
      end
      begin
        x.first(-1)
      rescue ArgumentError => e
        out << e.message
      end
      begin
        x.freeze.send(:map!) { |v| v }
      rescue StandardError => e
        out << e.class
      end
      out
    end

    # The element-level names (decision 304): reads, writes and their errors, one body for every element type.
    #: (untyped) -> untyped
    def self.core(x)
      out = [] #: Array[untyped]
      %i[length size empty? frozen? to_a entries deconstruct itself dup last pop shift].each { |n| out << dyn(x.dup, n) }
      out << dyn(x, :[], 1) << dyn(x, :[], -1) << dyn(x, :[], 9) << dyn(x, :[], 0, 2) << dyn(x, :[], 1..) << dyn(x, :[], 5, 1)
      out << dyn(x, :slice, 1, 1) << dyn(x, :at, -2) << dyn(x, :fetch, 1) << dyn(x, :fetch, 9, :none) << dyn(x, :fetch, 9) { |i| i * 2 }
      out << dyn(x, :fetch_values, 0, -1) << dyn(x, :dig, 0) << dyn(x, :last, 2) << dyn(x, :last, 9)
      out << dyn(x, :grep, Object).size << dyn(x, :grep_v, NilClass) << dyn(x, :grep, Object) { |v| v.to_s * 2 }
      y = x.dup
      out << dyn(y, :<<, x[0]) << dyn(y, :push, x[1], x[2]) << dyn(y, :append) << dyn(y, :unshift, x[2]) << dyn(y, :prepend, x[1])
      out << dyn(y, :[]=, 0, x[1]) << dyn(y, :[]=, 1, 2, x[0]) << dyn(y, :[]=, 0..1, [x[2], x[2]]) << y
      out << dyn(y, :pop) << dyn(y, :shift) << y << dyn(y, :clear) << dyn(y, :empty?)
      r = Random.new(42)
      out << x.public_send(:shuffle, random: r) << x.public_send(:sample, random: r) << x.public_send(:sample, 2, random: r)
      z = x.dup
      out << z.public_send(:shuffle!, random: r).equal?(z)
      out
    end

    #: (untyped) -> untyped
    def self.core_errors(x)
      out = [] #: Array[untyped]
      [-> { dyn(x, :fetch, 9) }, -> { dyn(x, :fetch_values, 0, 9) }, -> { dyn(x, :last, -1) }, -> { dyn(x.dup, :[]=, -9, x[0]) },
       -> { dyn(x.dup, :[]=, 0, -1, x[0]) }, -> { dyn(x.dup.freeze, :push, x[0]) }, -> { dyn(x.dup.freeze, :pop) },
       -> { dyn(x.dup.freeze, :[]=, 0, x[0]) }, -> { dyn(x, :sample, -1) }, -> { dyn(x, :[], "a") }].each do |f|
        f.call
        out << :none
      rescue IndexError, ArgumentError, FrozenError, TypeError => e
        out << e.class
        out << e.message unless e.is_a?(TypeError) # rugaroo words a conversion's TypeError apart (decision 20)
      end
      out
    end

    # The entry-level Hash names, defaults included (they stay per-instantiation wrappers: decision 304).
    #: (untyped, untyped) -> untyped
    def self.hash_core(h, k)
      out = [] #: Array[untyped]
      %i[length size empty? frozen? to_hash itself].each { |n| out << dyn(h, n) }
      out << dyn(h, :dup).equal?(h) << dyn(h, :key?, k) << dyn(h, :include?, "zz") << dyn(h, :member?, k) << dyn(h, :has_key?, k)
      out << dyn(h, :[], k) << dyn(h, :[], "zz") << dyn(h, :dig, k) << dyn(h, :fetch, k)
      g = h.dup
      out << dyn(g, :[]=, "n", g[k]) << dyn(g, :store, "m", g[k]) << g << dyn(g, :delete, k) << dyn(g, :delete, "zz")
      out << g
      out
    end

    #: (untyped, untyped) -> untyped
    def self.hash_core_errors(h, k)
      out = [] #: Array[untyped]
      [-> { dyn(h, :fetch, "zz") }, -> { dyn(h.dup.freeze, :delete, k) }, -> { dyn(h.dup.freeze, :[]=, k, h[k]) }].each do |f|
        f.call
        out << :none
      rescue KeyError, FrozenError => e
        out << e.class << e.message
        out << [e.key, e.receiver.size] if e.is_a?(KeyError)
      end
      out
    end
  end
end
