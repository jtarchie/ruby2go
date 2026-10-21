# rbs_inline: enabled

# trace: liquid builds its filter classes at run time, so rb2go compiles what an MRI run loads (decisions 296, 331)

# The unmodified liquid gem: parse a template once and render it many times with fresh data.
require "liquid"

TEMPLATE = <<~LIQUID
  {%- assign title = page.title | default: "Untitled" -%}
  # {{ title | upcase }}
  {% for item in items -%}
  {{ forloop.index }}. {{ item.name | append: ":" }} {{ item.price | times: item.qty | round: 2 }}
  {%- if item.qty > 2 %} (bulk){% elsif item.qty == 0 %} (sold out){% endif %}
  {% endfor -%}
  {% capture summary %}{{ items | size }} lines, {{ items | map: "qty" | join: "+" }} units{% endcapture %}
  Summary: {{ summary | strip }}. {{ page.tags | sort | join: ", " }}
  {% case page.status %}{% when "draft" %}Draft{% else %}Live{% endcase %} {% cycle "odd", "even" %}
LIQUID

template = Liquid::Template.parse(TEMPLATE)
bytes = 0
out = ""
400.times do |n|
  items = Array.new(30) { |i| { "name" => "item#{i}", "price" => (i + n) * 0.5, "qty" => (i * n) % 5 } }
  data = { "page" => { "title" => "run #{n}", "tags" => %w[go ruby liquid], "status" => n.even? ? "draft" : "live" }, "items" => items }
  out = template.render(data)
  bytes += out.bytesize
end
puts bytes
puts out.lines.first(4).join
