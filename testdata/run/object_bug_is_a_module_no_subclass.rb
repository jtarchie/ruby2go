# rbs_inline: enabled

module Walker
end

class Doc
end

class Page
  include Walker
end

d = Doc.new
pg = Page.new
puts d.class, pg.class
puts d.is_a?(Walker).inspect
puts pg.is_a?(Walker).inspect
