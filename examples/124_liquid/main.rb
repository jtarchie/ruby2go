# rbs_inline: enabled

# trace: liquid builds its filter classes at run time (Class.new subclasses that include filter modules), so rugaroo compiles what an MRI run loads (decisions 296, 331)

# The unmodified liquid gem (Jekyll 4's template language): parse a template once, render it with data.
require "liquid"

TEMPLATE = <<~LIQUID
  {%- assign title = page.title | default: "Untitled" -%}
  # {{ title | upcase }}
  {% comment %}This line never renders.{% endcomment %}
  By {{ page.author | capitalize }}, {{ page.tags | size }} tags: {{ page.tags | sort | join: ", " }}

  {% for item in items -%}
  {{ forloop.index }}. {{ item.name | append: ":" }} {{ item.price | times: item.qty | round: 2 }}
  {%- if item.qty > 2 %} (bulk){% elsif item.qty == 0 %} (sold out){% endif %}
  {% endfor -%}
  {%- capture summary %}{{ items | size }} lines, {{ items | map: "qty" | join: "+" }} units{% endcapture %}
  Summary: {{ summary }}.
  {% case page.status %}{% when "draft" %}Draft{% when "live", "published" %}Live{% else %}Unknown{% endcase %}
  {% unless page.hidden %}Visible{% endunless %} / {% cycle "odd", "even" %} {% cycle "odd", "even" %}
  {{ "Ruby & Go" | escape }} {{ "  trim me  " | strip }} {{ "a-b-c" | split: "-" | reverse | join: "" }}
  {{ 10 | minus: 3 | divided_by: 2 }} {{ 7 | modulo: 3 }} {{ "hello world" | truncate: 8 }} {{ "x" | prepend: ">" }}
  {% raw %}{{ not rendered }}{% endraw %}
  {% for n in (1..5) %}{% if n == 4 %}{% break %}{% endif %}{{ n }}{% endfor %}
LIQUID

template = Liquid::Template.parse(TEMPLATE)
data = {
  "page" => { "title" => "release notes", "author" => "jt", "tags" => %w[go ruby compiler], "status" => "live" },
  "items" => [
    { "name" => "Pen", "price" => 1.25, "qty" => 4 },
    { "name" => "Ink", "price" => 3.5, "qty" => 1 },
    { "name" => "Pad", "price" => 2.0, "qty" => 0 }
  ],
}
puts template.render(data)
p template.errors

# A strict render raises on an unknown filter; the default one leaves the value as it was.
begin
  Liquid::Template.parse("{{ 'x' | nope }}").render!({}, strict_filters: true)
rescue Liquid::UndefinedFilter => e
  puts "error: #{e.message}"
end
puts Liquid::Template.parse("{{ 'x' | nope }}").render
