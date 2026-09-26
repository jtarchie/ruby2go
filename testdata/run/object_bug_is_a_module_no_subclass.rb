# skip: is_a?(Module) on a class that neither includes the module nor has any subclass is a compile error ("cannot be checked"); decision 21 only rules out classes that might have a subclass including it, and here the answer is statically false

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
