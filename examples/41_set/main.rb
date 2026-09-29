# rbs_inline: enabled
# args: --seed 1

require "minitest/autorun"

# Tag index: which posts share tags, via Set algebra.
class Post
  attr_reader :title #: String
  attr_reader :tags #: Set[String]

  #: (String, Array[String]) -> void
  def initialize(title, tags)
    @title = title
    @tags = Set.new(tags)
  end
end

#: (Post, Post) -> Float
def jaccard(a, b)
  union = a.tags | b.tags
  return 0.0 if union.empty?
  (a.tags & b.tags).size.to_f / union.size
end

class TagIndexTest < Minitest::Test
  def setup
    @posts = [
      Post.new("Go generics", %w[go generics types]),
      Post.new("Ruby sets", %w[ruby types collections ruby]),
      Post.new("Typed Ruby to Go", %w[ruby go types]),
    ]
  end

  def test_merge_collects_unique_tags_in_insertion_order
    all_tags = Set.new #: Set[String]
    @posts.each { |p| all_tags.merge(p.tags) }
    assert_equal 'Set["go", "generics", "types", "ruby", "collections"]', all_tags.inspect
    assert_equal 5, all_tags.size
    assert_equal 'Set["ruby", "types", "collections"]', @posts[1].tags.inspect
  end

  def test_jaccard_similarity_of_each_pair
    pairs = [] #: Array[String]
    @posts.each_with_index do |a, i|
      @posts.each_with_index do |b, j|
        next unless i < j
        pairs << "#{a.title} ~ #{b.title}: #{jaccard(a, b).round(2)} shared=#{(a.tags & b.tags).to_a.sort.inspect}"
      end
    end
    assert_equal [
      'Go generics ~ Ruby sets: 0.2 shared=["types"]',
      'Go generics ~ Typed Ruby to Go: 0.5 shared=["go", "types"]',
      'Ruby sets ~ Typed Ruby to Go: 0.5 shared=["ruby", "types"]',
    ], pairs
  end

  def test_set_algebra
    go = @posts[0].tags
    ruby = @posts[1].tags
    assert_equal 'Set["go", "generics", "ruby", "collections"]', (go ^ ruby).inspect
    assert_equal 'Set["go", "generics"]', (go - ruby).inspect
    assert_equal true, go.disjoint?(Set["java"])
    assert_equal true, go.intersect?(ruby)
  end
end

class SetTest < Minitest::Test
  def test_subset_operators_and_equality
    assert_equal true, Set[1, 2] <= Set[1, 2, 3]
    assert_equal false, Set[1, 2] < Set[1, 2]
    assert_equal true, Set[3, 1] == Set[1, 3]
  end

  def test_add_query_finds_duplicates
    seen = Set.new #: Set[Integer]
    dups = [3, 1, 4, 1, 5, 9, 2, 6, 5, 3].reject { |n| seen.add?(n) }
    assert_equal [1, 5, 3], dups
    assert_equal [3, 1, 4, 5, 9, 2, 6], seen.to_a
    assert_equal [1, 2, 3, 4, 5, 6, 9], seen.sort
    assert_equal [4, 2, 6], seen.select(&:even?)

    # A Set in case/when matches by membership.
    found = case 9
            when seen then "9 seen"
            else "missing"
            end
    assert_equal "9 seen", found
  end

  def test_sets_hash_by_contents
    by_set = { Set[1, 2] => "pair" }
    assert_equal "pair", by_set[Set[2, 1]]
  end
end
