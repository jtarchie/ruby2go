# rbs_inline: enabled
# args: --seed 1
# to_json on core types and user classes; JSON.generate.
require "json"
require "minitest/autorun"

class Post
  attr_reader :id #: Integer?
  attr_reader :title #: String

  #: (Integer?, String) -> void
  def initialize(id, title)
    @id = id
    @title = title
  end

  # Array#to_json passes a generator state to each element.
  #: (*untyped) -> String
  def to_json(*_state) = { "post" => { "id" => id, "title" => title } }.to_json
end

class JSONTest < Minitest::Test
  #: () -> void
  def test_scalars
    assert_equal '"plain \"quoted\" / \\\\ \n\t"', "plain \"quoted\" / \\ \n\t".to_json
    assert_equal "42", 42.to_json
    assert_equal "1.5", 1.5.to_json
    assert_equal "true", true.to_json
    assert_equal "null", nil.to_json
  end

  #: () -> void
  def test_arrays_and_hashes
    assert_equal '[1,"two",null,3.0]', [1, "two", nil, 3.0].to_json
    assert_equal "[[1,2],[]]", [[1, 2], []].to_json
    assert_equal '{"a":1,"b":[true,false],"c":{"d":null}}', { "a" => 1, "b" => [true, false], "c" => { "d" => nil } }.to_json
    assert_equal '{"name":"sym","é":"ünï"}', { name: "sym", "é" => "ünï" }.to_json
  end

  #: () -> void
  def test_generate
    assert_equal '{"k":"v"}', JSON.generate({ "k" => "v" })
    assert_equal "[1,2]", JSON.generate([1, 2])
  end

  # A user class's `to_json` is used directly and inside arrays.
  #: () -> void
  def test_user_class_to_json
    assert_equal '{"post":{"id":1,"title":"Hi"}}', Post.new(1, "Hi").to_json
    assert_equal '{"post":{"id":null,"title":"New"}}', Post.new(nil, "New").to_json
    posts = [Post.new(1, "a"), Post.new(2, "b")]
    assert_equal '[{"post":{"id":1,"title":"a"}},{"post":{"id":2,"title":"b"}}]', posts.to_json
  end

  #: () -> void
  def test_float_formatting
    floats = [1e20, 1.23e-5, 1e-10, 100.0, -0.0, 12.5, 1e15, 1e7, 0.1, 5e-324]
    assert_equal "[1e+20,0.0000123,1e-10,100.0,-0.0,12.5,1e+15,10000000.0,0.1,5e-324]", floats.to_json
  end

  # Control characters are escaped; DEL passes through.
  #: () -> void
  def test_control_characters
    assert_equal "\"\\u0001\\u001f\u007f\\u001b\"", "\u0001\u001f\u007f\e".to_json
  end
end
