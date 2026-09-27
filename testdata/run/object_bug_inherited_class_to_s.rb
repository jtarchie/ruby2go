# rbs_inline: enabled

class Widget
  #: () -> String
  def self.name = "CustomWidget"
end

class SubWidget < Widget
end

class Gadget
  #: () -> String
  def self.to_s = "GadgetClass"

  #: () -> String
  def self.inspect = "GadgetInspect"
end

class SubGadget < Gadget
end

puts Widget.name, SubWidget.name, SubWidget.new.class.name
puts Gadget, SubGadget, "#{SubGadget}", SubGadget.inspect, [SubGadget].inspect
