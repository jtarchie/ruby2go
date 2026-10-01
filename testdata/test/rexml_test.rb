# rbs_inline: enabled

require "minitest/autorun"
require "rexml/document"

# REXML (decision 114): XPath results, escaping and formatting, each value MRI's.
module RexmlTests
  class RexmlTest < Minitest::Test
    SRC = "<lib><shelf n='1'><book id='a' year='2001'><t>One</t><by>X</by></book><book id='b'><t>Two &amp; Three</t><by>Y</by></book></shelf><shelf n='2'><book id='c' year='1999'><t>Four</t><by>X</by></book><!--note--><mag>M</mag></shelf></lib>" #: String

    #: () -> REXML::Element
    def root = REXML::Document.new(SRC).root || raise("no root")

    #: (Array[untyped]) -> Array[String]
    def rx_names(nodes) = nodes.map { |n| n.is_a?(REXML::Element) ? n.name : n.to_s }

    def test_xpath
      assert_equal ["shelf", "shelf"], rx_names(REXML::XPath.match(root, "/lib/shelf"))
      assert_equal ["book", "book", "book"], rx_names(REXML::XPath.match(root, "//book"))
      assert_equal ["book", "book"], rx_names(REXML::XPath.match(root, "//book[1]"))
      assert_equal ["book", "book"], rx_names(REXML::XPath.match(root, "//book[last()]"))
      assert_equal ["book", "book"], rx_names(REXML::XPath.match(root, "//book[@year]"))
      assert_equal ["book"], rx_names(REXML::XPath.match(root, "//book[@year='1999']"))
      assert_equal ["book", "book"], rx_names(REXML::XPath.match(root, "//book[@id!='a']"))
      assert_equal ["book", "book"], rx_names(REXML::XPath.match(root, "//book[by='X']"))
      assert_equal ["book", "book", "book"], rx_names(REXML::XPath.match(root, "//book[t]"))
      assert_equal ["book", "mag"], rx_names(REXML::XPath.match(root, "/lib/shelf[2]/*"))
      assert_equal ["One", "Two &amp; Three", "Four"], rx_names(REXML::XPath.match(root, "//t/text()"))
      assert_equal ["a", "b", "c"], rx_names(REXML::XPath.match(root, "//book/@id"))
      assert_equal ["1", "2"], rx_names(REXML::XPath.match(root, "//@n"))
      assert_equal ["shelf"], rx_names(REXML::XPath.match(root, "//book[2]/.."))
      assert_equal ["t", "t"], rx_names(REXML::XPath.match(root, "/lib/shelf/book[1]/t"))
      assert_equal ["book", "book", "book"], rx_names(REXML::XPath.match(root, "shelf/book"))
      assert_equal ["lib"], rx_names(REXML::XPath.match(root, "."))
      assert_equal ["mag"], rx_names(REXML::XPath.match(root, "//mag"))
      assert_equal ["t"], rx_names(REXML::XPath.match(root, "//shelf[@n='2']/book/t"))
      assert_equal ["book"], rx_names(REXML::XPath.match(root, "//book[by!='X']"))
      assert_equal [], rx_names(REXML::XPath.match(root, "//book[text()='nope']"))
      assert_equal ["book", "book", "book"], rx_names(REXML::XPath.match(root, "//*[@id]"))
      assert_equal ["lib", "shelf", "book", "t", "One", "X", "t", "Two &amp; Three", "Y", "book", "t", "Four", "X", "M"], rx_names(REXML::XPath.match(root, "//node()[1]"))
    end

    def test_text_and_attributes
      r = root
      book = r.elements["//book[@id='b']"] || raise("no book")
      assert_equal "Two & Three", book.text("t")
      assert_equal "Two &amp; Three", book.get_text("t").to_s
      assert_nil book.attributes["year"]
      assert_equal "b", book.attributes["id"]
      assert_equal "id='b'", book.attribute("id")&.to_string
      book.attributes["note"] = "it's <new> & \"ok\""
      assert_equal "it's <new> & \"ok\"", book.attributes["note"]
      assert_equal "<book id='b' note='it&apos;s &lt;new&gt; &amp; &quot;ok&quot;'><t>Two &amp; Three</t><by>Y</by></book>", book.to_s
      book.attributes["note"] = nil
      assert_equal 1, book.attributes.size
      assert_equal 2, r.elements.size
      assert_equal "shelf", r.elements[2]&.name
      assert_equal 3, r.get_elements("//book").size
      t = REXML::Text.new("a  b\n\n c")
      assert_equal "a b\n c", t.value
      assert_equal "a  b", REXML::Text.new("a  b", true).value
    end

    def test_mutation_and_formatting
      d = REXML::Document.new
      top = d.add_element("top", { "b" => "2", "a" => "1" })
      top.add_element("kid").add_text("hi")
      top.add_text("tail")
      assert_equal "<top a='1' b='2'><kid>hi</kid>tail</top>", d.to_s
      assert_equal "<top b='2' a='1'> ... </>", top.inspect
      f = REXML::Formatters::Pretty.new(3)
      f.compact = true
      out = StringIO.new
      f.write(top, out)
      assert_equal "<top b='2' a='1'>\n   <kid>hi</kid>\n   tail\n</top>", out.string
      kid = top.elements["kid"] || raise("no kid")
      top.delete_element(kid)
      assert_equal "<top a='1' b='2'>tail</top>", top.to_s
      assert_equal false, top.has_elements?
      assert_equal true, top.has_text?
    end

    # REXML's messages go on with the line, position and unconsumed input; rb2go gives the first line (decision 114), so it is not put through assert_raises.
    #: (String) -> String
    def rx_error(src)
      REXML::Document.new(src)
      "parsed"
    rescue REXML::ParseException => e
      e.message.lines.first&.chomp || ""
    end

    def test_parse_errors
      assert_equal "Missing end tag for 'b' (got 'a')", rx_error("<a><b></a>")
      assert_equal "Duplicate attribute \"x\"", rx_error("<a x='1' x='2'/>")
      assert_equal "Malformed XML: Content at the start of the document (got 'junk')", rx_error("junk")
      assert_equal "parsed", rx_error("<a>&undefined;</a>")
    end
  end
end
