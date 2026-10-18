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
      dyn(h, :cycle, 1) { |k, v| out << k }
      [out, dyn(h, :select) { |k, v| v.to_s > "1" }, dyn(h, :filter) { |k| k.to_s == "a" }, dyn(h, :reject) { |_, v| v.nil? },
       dyn(h, :count) { |k, v| v }, dyn(h, :count), dyn(h, :find) { |k, v| v.to_s == "2" }, dyn(h, :detect) { |k, _| k == "z" },
       dyn(h, :inject, 0) { |a, pair| a + pair[1].to_s.size }, dyn(h, :each_with_object, []) { |pair, a| a << pair[0] },
       dyn(h, :any?) { |k, v| v.nil? }, dyn(h, :all?) { |pair| pair.size == 2 }, dyn(h, :none?), dyn(h, :one?, Array),
       dyn(h, :any?, ["a", 1])]
    end

    #: (untyped) -> untyped
    def self.hash_mutate(h)
      out = [] #: Array[untyped]
      out << dyn(h, :select!) { |k, v| v } << h.dup << dyn(h, :reject!) { |k, v| k == "q" } << dyn(h, :keep_if) { |k, v| true }.dup
      out << dyn(h, :transform_values!) { |v| v.to_s * 2 }.dup << dyn(h, :delete_if) { |k, v| k == "a" }.dup
      out << dyn(h, :filter!) { |k, v| false } << h
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
      rescue => e
        out << e.class
      end
      out
    end
  end
end
