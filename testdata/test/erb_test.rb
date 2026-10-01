# rbs_inline: enabled

require "minitest/autorun"
require "erb"

# ERB (decision 111): each trim mode's output, comments, literal tags,
# percent lines and result_with_hash, as MRI renders them.
module ErbTests
  class ErbTest < Minitest::Test
    #: () -> Array[Integer]
    def pair = [1, 2]

    def test_trim_modes
      list = pair
      plain = ERB.new("<% list.each do |i| %>\n  item <%= i %>\n<% end %>\ndone\n")
      assert_equal "\n  item 1\n\n  item 2\n\ndone\n", plain.result(binding)
      angle = ERB.new("<% list.each do |i| %>\n  item <%= i %>\n<% end %>\ndone\n", trim_mode: "<>")
      assert_equal "  item 1\n  item 2\ndone\n", angle.result(binding)
      gt = ERB.new("<% list.each do |i| %>\n  item <%= i %>\n<% end %>\ndone\n", trim_mode: ">")
      assert_equal "  item 1  item 2done\n", gt.result(binding)
      dash = ERB.new("<%- list.each do |i| -%>\n  item <%= i -%>\n<%- end -%>\ndone\n", trim_mode: "-")
      assert_equal "  item 1  item 2done\n", dash.result(binding)
      percent = ERB.new("% list.each do |i|\n  item <%= i %>\n% end\n%% literal\ndone\n", trim_mode: "%")
      assert_equal "  item 1\n  item 2\n% literal\ndone\n", percent.result(binding)
      both = ERB.new("% list.each do |i|\n  <%= i -%>\n% end\nend\n", trim_mode: "%-")
      assert_equal "  1  2end\n", both.result(binding)
    end

    def test_comments_literals_and_hash
      assert_equal "a b <% c %%> d 2\n", ERB.new("a <%# note %>b <%% c %%> d <%= 1 + 1 %>\n").result
      assert_equal "Hi Ada, 42", ERB.new("Hi <%= name %>, <%= n * 2 %>").result_with_hash(name: "Ada", n: 21)
      total = 0
      ERB.new("<% pair.each { |i| total += i } %>").result(binding)
      assert_equal 3, total
      assert_equal "&lt;b&gt; &amp; &quot;q&quot;", ERB::Util.html_escape("<b> & \"q\"")
      assert_equal "x&#39;y", ERB::Util.h("x'y")
    end

    def test_multiline_and_nested
      tpl = ERB.new(<<~T, trim_mode: "-")
        <%- pair.each do |i| -%>
        <%- [i, i * 10].each do |j| -%>
        <%= i %>:<%= j %>
        <%- end -%>
        <%- end -%>
      T
      assert_equal "1:1\n1:10\n2:2\n2:20\n", tpl.result(binding)
    end
  end
end
