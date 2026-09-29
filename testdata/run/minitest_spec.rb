# Minitest::Spec compiled to classes and methods (decision 83): describe/it/specify/let/subject/before/after,
# nested describes, the class form, and specs mixed with Test classes, in MRI's run order (-v).
# args: --seed 5 -v
require "minitest/autorun"

LOG = [] #: Array[String]

class PlainTest < Minitest::Test
  def test_plain = assert(true)
end

class CounterSpec < Minitest::Spec
  let(:calls) { [] } #: Array[String]
  let(:maybe) do
    calls << "maybe"
    false
  end
  subject { "subj" }

  before do
    LOG << "outer before"
  end

  after do
    LOG << "outer after"
  end

  #: () -> String
  def helper = "help"

  it "memoizes false" do
    maybe
    maybe
    assert_equal ["maybe"], calls
  end

  it "has subject and helper" do
    assert_equal "subj", subject
    assert_equal "help", helper
  end

  describe "nested" do
    before do
      LOG << "inner before"
    end

    it "sees outer lets" do
      refute maybe
    end
  end
end

describe "Top" do
  it("one") { assert true }
  specify "two" do
    refute false
  end
end

class LogTest < Minitest::Test
  def test_zz_log_last = assert(true)
end

Minitest.after_run { puts LOG.tally.sort.inspect }
