# rbs_inline: enabled

require "minitest/autorun"

# Each must_/wont_ expectation passing, with minitest's assertion counts (decision 83).
describe "expectations" do
  let(:list) { [1, 2] } #: Array[Integer]

  it "compares values" do
    _(1 + 1).must_equal 2
    _(list).must_include 2
    _(list).wont_include 3
    _(nil).must_be_nil
    expect(1).wont_be_nil
    value("x").wont_equal "y"
    s = "a"
    _(s).must_be_same_as s
    _("a").wont_be_same_as "b"
  end

  it "checks types and predicates" do
    _(1).must_be_instance_of Integer
    _(1).must_be_kind_of Integer
    _(3).wont_be_instance_of String
    _(3).wont_be_kind_of String
    _(3).must_be :>, 2
    _(4).must_be :even?
    _(3).wont_be :even?
    _(1).wont_be :>, 2
    _(3).must_respond_to :even?
    _(3).wont_respond_to :nope
  end

  it "checks collections, numbers, patterns and paths" do
    _([]).must_be_empty
    _(list).wont_be_empty
    _(1.0).must_be_close_to 1.0001
    _(1.0).must_be_within_delta 1.00001
    _(100).must_be_within_epsilon 100.01
    _(1.0).wont_be_close_to 2.0
    _("abc").must_match(/b/)
    _("abc").wont_match(/z/)
    _("/").path_must_exist
    _("/no/such/path").path_wont_exist
  end

  it "checks raised errors" do
    e = _ { list.fetch(5) }.must_raise IndexError
    k = _ { { a: 1 }.fetch(:z) }.must_raise KeyError
    _(k.key).must_equal :z
    _(e.message).must_equal "index 5 outside of array bounds: -2...2"
  end
end
