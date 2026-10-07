# rbs_inline: enabled

require_relative "stringio"
require_relative "strscan"

# REXML (decision 114): a DOM over a Go tokenizer (prelude/go/rexml.go)
# that keeps text and attribute values as written, an XPath subset, and
# the Default and Pretty formatters ported from rexml 3.4.
module REXML
  class ParseException < RuntimeError; end

  # Any node with a parent: text, comments, instructions, declarations, elements.
  class Child < Object
    attr_accessor :parent #: REXML::Parent?

    #: () -> void
    def initialize
      @parent = nil
    end

    #: () -> REXML::Child?
    def next_sibling
      pa = parent
      return nil unless pa

      i = pa.index(self)
      pa.children[i + 1]
    end

    #: () -> REXML::Child?
    def previous_sibling
      pa = parent
      return nil unless pa

      i = pa.index(self)
      i > 0 ? pa.children[i - 1] : nil
    end

    #: () -> self
    def remove
      pa = parent
      pa&.delete(self)
      self
    end

    #: () -> REXML::Document?
    def document
      n = parent
      while n
        return n if n.is_a?(Document)

        n = n.parent
      end
      nil
    end
  end

  # A text node. One read from a document keeps its source text (raw), entities and all; one made through the API escapes on output.
  class Text < Child
    attr_accessor :raw #: bool

    #: (String, ?bool, ?REXML::Parent?, ?bool) -> void
    def initialize(string, respect_whitespace = false, parent = nil, raw = false)
      super()
      s = string.gsub(/\r\n?/, "\n")
      s = s.gsub(/ +/, " ").gsub(/\n+/, "\n").gsub(/\t+/, "\t") unless respect_whitespace
      @string = s
      @raw = raw
      @parent = parent
    end

    # The text as it is written to XML: escaped.
    #: () -> String
    def to_s = @raw ? @string : REXML.__normalize(@string)

    # The text with entities replaced: what a program reads.
    #: () -> String
    def value = REXML.__unnormalize(@string)

    #: (String) -> String
    def value=(val)
      @string = val.gsub(/\r\n?/, "\n")
      @raw = false
      val
    end

    #: (String) -> self
    def <<(more)
      @string += more.gsub(/\r\n?/, "\n")
      self
    end

    #: () -> bool
    def empty? = @string.empty?

    #: () -> String
    def inspect = @string.inspect

    #: () -> Symbol
    def node_type = :text

    #: () -> String
    def __string = @string
  end

  class CData < Text
    #: (String) -> void
    def initialize(string)
      super(string, true, nil, true)
    end

    #: () -> String
    def to_s = __string

    #: () -> String
    def value = __string

    #: () -> String
    def inspect = "#<REXML::CData #{__string.inspect}>"

    #: () -> Symbol
    def node_type = :cdata
  end

  class Comment < Child
    attr_accessor :string #: String

    #: (String) -> void
    def initialize(string)
      super()
      @string = string
    end

    #: () -> String
    def to_s = @string

    #: () -> Symbol
    def node_type = :comment
  end

  class Instruction < Child
    attr_accessor :target #: String
    attr_accessor :content #: String?

    #: (String, ?String?) -> void
    def initialize(target, content = nil)
      super()
      @target = target
      @content = content
    end

    #: () -> String
    def to_s
      c = content
      c ? "<?#{target} #{c}?>" : "<?#{target}?>"
    end

    #: () -> String
    def inspect = "<?#{target} ... ?>"

    #: () -> Symbol
    def node_type = :processing_instruction
  end

  class XMLDecl < Child
    attr_accessor :version #: String
    attr_accessor :standalone #: String?
    attr_reader :writethis #: bool

    #: (?String, ?String?, ?String?) -> void
    def initialize(version = "1.0", encoding = nil, standalone = nil)
      super()
      @version = version
      @encoding = encoding
      @standalone = standalone
      @writethis = true
    end

    #: () -> String
    def encoding = @encoding || "UTF-8"

    #: () -> void
    def nowrite
      @writethis = false
    end

    #: () -> String
    def content
      rv = "version='#{version}'"
      enc = @encoding
      rv += " encoding='#{enc}'" if enc
      sa = standalone
      rv += " standalone='#{sa}'" if sa
      rv
    end

    #: () -> String
    def to_s = writethis ? "<?xml #{content}?>" : ""

    #: () -> String
    def inspect = "<?xml ... ?>"

    #: () -> Symbol
    def node_type = :xmldecl
  end

  # A node with children: elements and documents.
  class Parent < Child
    #: () -> void
    def initialize
      super
      @children = [] #: Array[REXML::Child]
    end

    #: () -> Array[REXML::Child]
    def children = @children

    #: (REXML::Child) -> REXML::Child
    def add(child)
      child.parent&.delete(child)
      child.parent = self
      @children << child
      child
    end

    #: (REXML::Child) -> REXML::Child
    def <<(child) = add(child)

    #: (REXML::Child) -> REXML::Child?
    def delete(child)
      i = @children.index { |c| c.equal?(child) }
      return nil unless i

      @children.delete_at(i)
      child.parent = nil
      child
    end

    #: (REXML::Child) -> Integer
    def index(child) = @children.index { |c| c.equal?(child) } || -1

    #: () -> Integer
    def size = @children.size

    #: () { (REXML::Child) -> void } -> void
    def each_child
      @children.each { |c| yield c }
    end
  end

  class Element < Parent
    attr_reader :name #: String
    attr_reader :attributes #: REXML::Attributes
    attr_reader :elements #: REXML::Elements

    #: (?String, ?REXML::Parent?) -> void
    def initialize(name = "UNDEFINED", parent = nil)
      super()
      @name = name
      @attributes = Attributes.new(self)
      @elements = Elements.new(self)
      parent&.add(self)
    end

    #: () -> String
    def expanded_name = name

    #: (String) -> String
    def name=(n)
      @name = n
    end

    # A child element named name (with attrs set on it), or the Element given; returns the new child.
    #: (untyped, ?Hash[String, String]?) -> REXML::Element
    def add_element(element, attrs = nil)
      el = element.is_a?(Element) ? element : Element.new(element.to_s)
      add(el)
      attrs&.each { |k, v| el.attributes[k] = v }
      el
    end

    # Removes a child element: the Element itself, the nth (1-based), or the first an XPath finds.
    #: (untyped) -> REXML::Element?
    def delete_element(element) = elements.delete(element)

    #: () -> bool
    def has_elements? = !elements.empty?

    #: (?String?) { (REXML::Element) -> void } -> void
    def each_element(xpath = nil)
      elements.each(xpath) { |e| yield e }
    end

    #: (String) -> Array[REXML::Element]
    def get_elements(xpath) = elements.to_a(xpath)

    #: () -> REXML::Element?
    def next_element
      n = next_sibling
      n = n.next_sibling while n && !n.is_a?(Element)
      n.is_a?(Element) ? n : nil
    end

    #: () -> REXML::Element?
    def previous_element
      n = previous_sibling
      n = n.previous_sibling while n && !n.is_a?(Element)
      n.is_a?(Element) ? n : nil
    end

    #: () -> bool
    def has_text? = !text.nil?

    # The value of the first text child (of the element path names), entities replaced.
    #: (?String?) -> String?
    def text(path = nil) = get_text(path)&.value

    #: (?String?) -> REXML::Text?
    def get_text(path = nil)
      if path
        el = elements[path]
        return el ? el.get_text : nil
      end
      @children.each { |c| return c if c.is_a?(Text) }
      nil
    end

    # Replaces the first text child (nil removes it).
    #: (String?) -> String?
    def text=(text)
      old = get_text
      if text.nil?
        old&.remove
        return nil
      end
      t = Text.new(text, true, nil, false)
      if old
        i = index(old)
        old.parent = nil
        t.parent = self
        @children[i] = t
      else
        add(t)
      end
      text
    end

    # Appends to the last child when that is text, else adds a text child (returning self, or nil when appended, as REXML does).
    #: (String) -> REXML::Element?
    def add_text(text)
      last = @children.last
      if last.is_a?(Text)
        last << text
        return nil
      end
      add(Text.new(text, true, nil, false))
      self
    end

    #: (String) -> REXML::Attribute?
    def attribute(name) = attributes.get_attribute(name)

    #: () -> bool
    def has_attributes? = !attributes.empty?

    #: (String, String) -> void
    def add_attribute(key, value)
      attributes[key] = value
    end

    #: (Hash[String, String]) -> void
    def add_attributes(hash)
      hash.each { |k, v| attributes[k] = v }
    end

    #: (String) -> REXML::Attribute?
    def delete_attribute(key) = attributes.delete(key)

    # An attribute's value for a String, the nth child for an Integer.
    #: (untyped) -> untyped
    def [](name_or_index)
      return attributes[name_or_index] if name_or_index.is_a?(String)

      @children[name_or_index]
    end

    #: () -> Array[REXML::Text]
    def texts
      out = [] #: Array[REXML::Text]
      @children.each { |c| out << c if c.is_a?(Text) }
      out
    end

    #: () -> Array[REXML::Comment]
    def comments
      out = [] #: Array[REXML::Comment]
      @children.each { |c| out << c if c.is_a?(Comment) }
      out
    end

    #: () -> REXML::Element?
    def root
      n = self #: REXML::Element
      while true # rubocop:disable Style/InfiniteLoop
        pa = n.parent
        return n if pa.nil? || pa.is_a?(Document)
        return nil unless pa.is_a?(Element)

        n = pa
      end
    end

    #: () -> REXML::Parent
    def root_node
      n = self #: REXML::Parent
      while (pa = n.parent)
        n = pa
      end
      n
    end

    #: () -> String
    def inspect
      rv = "<#{expanded_name}"
      attributes.each_attribute { |a| rv += " #{a.to_string}" }
      children.empty? ? "#{rv}/>" : "#{rv}> ... </>"
    end

    #: () -> String
    def to_s
      buf = [] #: Array[String]
      Formatters::Default.new.__write(self, buf)
      buf.join
    end

    # To output ($stdout by default; an IO or StringIO): indent > -1 pretty-prints with that indentation.
    #: (?untyped, ?Integer) -> void
    def write(output = nil, indent = -1)
      formatter = indent > -1 ? Formatters::Pretty.new(indent) : Formatters::Default.new
      formatter.write(self, output)
    end

    #: () -> Symbol
    def node_type = :element
  end

  class Document < Element
    # Parses source (a String); no source makes an empty document.
    #: (?String?) -> void
    def initialize(source = nil)
      super("", nil)
      __build(source) if source && !source.empty?
    end

    #: () -> String
    def expanded_name = ""

    #: () -> REXML::Element?
    def root = elements[1]

    #: () -> REXML::XMLDecl
    def xml_decl
      first = children.first
      return first if first.is_a?(XMLDecl)

      d = XMLDecl.new
      d.nowrite
      d.parent = self
      children.unshift(d)
      d
    end

    #: () -> String
    def version = xml_decl.version

    #: () -> String
    def encoding = xml_decl.encoding

    #: () -> String?
    def stand_alone? = xml_decl.standalone

    #: (REXML::Child) -> REXML::Child
    def add(child)
      if child.is_a?(XMLDecl)
        child.parent&.delete(child)
        child.parent = self
        if children.first.is_a?(XMLDecl)
          children[0] = child
        else
          children.unshift(child)
        end
        return child
      end
      raise "attempted adding second root element to document" if child.is_a?(Element) && !elements.empty?

      super
    end

    #: () -> String
    def to_s
      buf = [] #: Array[String]
      Formatters::Default.new.__write(self, buf)
      buf.join
    end

    #: () -> String
    def inspect = "<UNDEFINED> ... </>"

    #: () -> Symbol
    def node_type = :document

    #: (String) -> void
    def __build(source)
      stack = [self] #: Array[REXML::Element]
      REXML.__tokens(source).each do |ev|
        e = ev #: Array[untyped]
        top = stack.last || self #: REXML::Element
        case e.fetch(0)
        when :xmldecl
          add(XMLDecl.new(e.fetch(1) || "1.0", e.fetch(2), e.fetch(3)))
        when :start
          el = Element.new(e.fetch(1).to_s)
          top.add(el)
          pairs = e.fetch(2) #: Array[untyped]
          pairs.each do |pair|
            kv = pair #: Array[untyped]
            el.attributes.__set_raw(kv.fetch(0).to_s, kv.fetch(1).to_s)
          end
          stack << el
        when :end
          stack.pop
        when :text
          last = top.children.last
          if last.is_a?(Text) && !last.is_a?(CData)
            last << e.fetch(1).to_s
          else
            top.add(Text.new(e.fetch(1).to_s, true, nil, true))
          end
        when :comment
          top.add(Comment.new(e.fetch(1).to_s))
        when :cdata
          top.add(CData.new(e.fetch(1).to_s))
        when :pi
          top.add(Instruction.new(e.fetch(1).to_s, e.fetch(2)))
        end
      end
    end
  end

  class Attribute < Object
    attr_reader :name #: String
    attr_accessor :element #: REXML::Element?

    # value is normalized (escaped), as REXML's Attribute.new takes it.
    #: (String, String, ?REXML::Element?) -> void
    def initialize(name, value, element = nil)
      @name = name
      @normalized = value
      @element = element
    end

    #: () -> String
    def expanded_name = name

    # The escaped value, as written.
    #: () -> String
    def to_s = @normalized

    # The value with entities replaced.
    #: () -> String
    def value = REXML.__unnormalize(@normalized)

    #: () -> String
    def to_string = "#{name}='#{@normalized.gsub("'", "&apos;")}'"

    #: () -> String
    def inspect = to_string

    #: (untyped) -> bool
    def ==(other)
      return false unless other.is_a?(Attribute)

      o = other #: REXML::Attribute
      o.name == name && o.value == value
    end

    #: () -> Integer
    def hash = name.hash + value.hash
  end

  class Attributes < Object
    #: (REXML::Element) -> void
    def initialize(element)
      @element = element
      @attrs = {} #: Hash[String, REXML::Attribute]
    end

    # The attribute's value with entities replaced, or nil.
    #: (String) -> String?
    def [](name) = @attrs[name]&.value

    # Sets the attribute from an unescaped value (nil deletes it).
    #: (String, String?) -> String?
    def []=(name, value)
      if value.nil?
        @attrs.delete(name)
        return nil
      end
      @attrs[name] = Attribute.new(name, REXML.__normalize(value), @element)
      value
    end

    #: (String, String) -> void
    def __set_raw(name, value)
      @attrs[name] = Attribute.new(name, value, @element)
    end

    #: (String) -> REXML::Attribute?
    def get_attribute(name) = @attrs[name]

    #: () { (REXML::Attribute) -> void } -> void
    def each_attribute
      @attrs.each_value { |a| yield a }
    end

    #: () { (String, String) -> void } -> void
    def each
      @attrs.each_value { |a| yield a.name, a.value }
    end

    #: () -> Array[REXML::Attribute]
    def to_a = @attrs.values

    #: () -> Integer
    def size = @attrs.size

    #: () -> Integer
    def length = @attrs.size

    #: () -> bool
    def empty? = @attrs.empty?

    #: () -> Array[String]
    def keys = @attrs.keys

    # Removes the named attribute (or the Attribute given) and returns it.
    #: (untyped) -> REXML::Attribute?
    def delete(attribute)
      key = attribute.is_a?(Attribute) ? attribute.name : attribute.to_s
      @attrs.delete(key)
    end
  end

  # An element's child elements, 1-based, or found by XPath.
  class Elements < Object
    #: (REXML::Element) -> void
    def initialize(element)
      @element = element
    end

    #: () -> Array[REXML::Element]
    def __all
      out = [] #: Array[REXML::Element]
      @element.children.each { |c| out << c if c.is_a?(Element) }
      out
    end

    # The nth child element (1-based) for an Integer, the first element an XPath finds for a String.
    #: (untyped) -> REXML::Element?
    def [](index)
      if index.is_a?(Integer)
        raise "index (#{index}) must be >= 1" if index < 1

        return __all[index - 1]
      end
      XPath.match(@element, index.to_s).each { |n| return n if n.is_a?(Element) }
      nil
    end

    #: (?String?) { (REXML::Element) -> void } -> void
    def each(xpath = nil)
      to_a(xpath).each { |e| yield e }
    end

    #: (?String?) -> Array[REXML::Element]
    def to_a(xpath = nil)
      return __all if xpath.nil?

      out = [] #: Array[REXML::Element]
      XPath.match(@element, xpath).each { |n| out << n if n.is_a?(Element) }
      out
    end

    #: () -> Integer
    def size = __all.size

    #: () -> bool
    def empty? = __all.empty?

    #: (?REXML::Element?) -> REXML::Element
    def add(element = nil)
      el = element || Element.new
      @element.add(el)
      el
    end

    #: (REXML::Element) -> REXML::Element
    def <<(element) = add(element)

    #: (untyped) -> REXML::Element?
    def delete(element)
      el = element.is_a?(Element) ? element : self[element]
      return nil unless el

      @element.delete(el)
      el
    end

    #: (REXML::Element) -> Integer
    def index(element)
      i = __all.index { |e| e.equal?(element) }
      i ? i + 1 : -1
    end
  end

  # One XPath location step: an axis, a node test and predicates.
  class XPathStep < Object
    attr_reader :axis #: Symbol
    attr_reader :test #: String
    attr_reader :preds #: Array[String]

    #: (Symbol, String, Array[String]) -> void
    def initialize(axis, test, preds)
      @axis = axis
      @test = test
      @preds = preds
    end
  end

  # An XPath subset (decision 114): `/`, `//`, `.`, `..`, `*`, names, `text()`, `node()`, `@name`, `@*`,
  # and predicates `[n]`, `[last()]`, `[@a]`, `[@a='v']`, `[@a!='v']`, `[name]`, `[name='v']`, `[text()='v']`.
  module XPath
    #: (untyped, String) -> untyped
    def self.first(node, path) = match(node, path).first

    #: (untyped, String) { (untyped) -> void } -> void
    def self.each(node, path)
      match(node, path).each { |n| yield n }
    end

    #: (untyped, String) -> Array[untyped]
    def self.match(node, path)
      p = path.strip
      ctx = [] #: Array[untyped]
      if p.start_with?("/")
        start = node.is_a?(Element) ? node.root_node : node
        ctx << start
      else
        ctx << node
      end
      return ctx if p == "/"

      __steps(p).each do |step|
        if step.axis == :descendant
          expanded = [] #: Array[untyped]
          ctx.each { |n| __descendants_or_self(n, expanded) }
          ctx = expanded
        end
        out = [] #: Array[untyped]
        ctx.each do |n|
          cands = __candidates(n, step)
          step.preds.each { |pr| cands = __filter(cands, pr) }
          cands.each { |c| out << c unless out.any? { |o| o.equal?(c) } }
        end
        ctx = out
      end
      ctx
    end

    #: (untyped, Array[untyped]) -> void
    def self.__descendants_or_self(n, out)
      out << n
      return unless n.is_a?(Parent)

      par = n #: REXML::Parent
      par.children.each { |c| __descendants_or_self(c, out) if c.is_a?(Element) }
    end

    #: (untyped, REXML::XPathStep) -> Array[untyped]
    def self.__candidates(n, step)
      out = [] #: Array[untyped]
      t = step.test
      return [n] if t == "."
      if t == ".."
        out << n.parent if n.is_a?(Child) && n.parent
        return out
      end

      if t.start_with?("@")
        __attributes(n, t[1..] || "", out) if n.is_a?(Element)
        return out
      end
      __children(n, t, out) if n.is_a?(Parent)
      out
    end

    #: (REXML::Element, String, Array[untyped]) -> void
    def self.__attributes(el, name, out)
      el.attributes.each_attribute { |a| out << a if name == "*" || a.name == name }
    end

    #: (REXML::Parent, String, Array[untyped]) -> void
    def self.__children(par, test, out)
      par.children.each { |c| out << c if __test(c, test) }
    end

    #: (untyped, String) -> bool
    def self.__test(c, test)
      case test
      when "node()" then true
      when "text()" then c.is_a?(Text)
      when "comment()" then c.is_a?(Comment)
      when "*" then c.is_a?(Element)
      else
        c.is_a?(Element) && __el(c).name == test
      end
    end

    #: (Array[untyped], String) -> Array[untyped]
    def self.__filter(cands, pred)
      pr = pred.strip
      if pr.match?(/\A\d+\z/)
        hit = cands[pr.to_i - 1]
        return hit ? [hit] : []
      end
      if pr == "last()"
        hit = cands.last
        return hit ? [hit] : []
      end
      if (m = pr.match(/\A@([\w:.\-]+)\z/))
        name = m[1] || ""
        return cands.select { |c| c.is_a?(Element) && !__el(c).attributes[name].nil? }
      end
      if (m = pr.match(/\A@([\w:.\-]+)\s*(!?=)\s*('[^']*'|"[^"]*")\z/))
        name = m[1] || ""
        want = (m[3] || "  ")[1...-1] || ""
        eq = m[2] == "="
        return cands.select { |c| c.is_a?(Element) && (__el(c).attributes[name] == want) == eq }
      end
      if (m = pr.match(/\Atext\(\)\s*(!?=)\s*('[^']*'|"[^"]*")\z/))
        want = (m[2] || "  ")[1...-1] || ""
        eq = m[1] == "="
        return cands.select { |c| c.is_a?(Element) && (__el(c).text == want) == eq }
      end
      if (m = pr.match(/\A([\w:.\-]+)\s*(!?=)\s*('[^']*'|"[^"]*")\z/))
        name = m[1] || ""
        want = (m[3] || "  ")[1...-1] || ""
        eq = m[2] == "="
        return cands.select { |c| c.is_a?(Element) && __child_text?(c, name, want, eq) }
      end
      if (m = pr.match(/\A([\w:.\-]+)\z/))
        name = m[1] || ""
        return cands.select { |c| c.is_a?(Element) && !__el(c).elements.to_a(name).empty? }
      end
      raise "rb2go: XPath predicate [#{pred}] is not supported (decision 114)"
    end

    # An XPath node known to be an Element, typed.
    #: (REXML::Element) -> REXML::Element
    def self.__el(e) = e

    #: (REXML::Element, String, String, bool) -> bool
    def self.__child_text?(el, name, want, eq) = el.elements.to_a(name).any? { |k| (k.text == want) == eq }

    #: (String) -> Array[REXML::XPathStep]
    def self.__steps(path)
      steps = [] #: Array[REXML::XPathStep]
      i = 0
      n = path.size
      while i < n
        axis = :child
        two = path[i, 2]
        one = path[i]
        axis = :descendant if two == "//"
        i += 2 if two == "//"
        i += 1 if two != "//" && one == "/"
        j = i
        j += 1 while j < n && path[j] != "/" && path[j] != "["
        test = path[i...j] || ""
        raise "rb2go: XPath #{path.inspect}: empty step" if test.empty?

        i = j
        preds = [] #: Array[String]
        while i < n && path[i] == "["
          k = i + 1
          quote = ""
          while k < n && (path[k] != "]" || !quote.empty?)
            ch = path[k] || ""
            opening = quote.empty? && (ch == "'" || ch == "\"")
            closing = !quote.empty? && ch == quote
            quote = ch if opening
            quote = "" if closing
            k += 1
          end
          preds << (path[(i + 1)...k] || "")
          i = k + 1
        end
        steps << XPathStep.new(axis, test, preds)
      end
      steps
    end
  end

  module Formatters
    class Default < Object
      #: (?bool) -> void
      def initialize(ie_hack = false)
        @ie_hack = ie_hack
      end

      # Writes node to output ($stdout, an IO or a StringIO).
      #: (untyped, untyped) -> void
      def write(node, output)
        buf = [] #: Array[String]
        __write(node, buf)
        REXML.__emit(output, buf.join)
      end

      #: (untyped, Array[String]) -> void
      def __write(node, out)
        case node
        when Document then write_document(node, out)
        when Element then write_element(node, out)
        when Instruction then write_instruction(node, out)
        when XMLDecl then out << node.to_s
        when Comment then write_comment(node, out)
        when CData then write_cdata(node, out)
        when Text then write_text(node, out)
        when Attribute then out << node.to_string
        else raise "XML FORMATTING ERROR"
        end
      end

      #: (REXML::Document, Array[String]) -> void
      def write_document(node, out)
        node.children.each { |c| __write(c, out) }
      end

      #: (REXML::Element, Array[String]) -> void
      def write_element(node, out)
        out << "<#{node.expanded_name}"
        node.attributes.to_a.sort_by(&:name).each { |a| out << " #{a.to_string}" }
        if node.children.empty?
          out << " " if @ie_hack
          out << "/"
        else
          out << ">"
          node.children.each { |c| __write(c, out) }
          out << "</#{node.expanded_name}"
        end
        out << ">"
      end

      #: (REXML::Text, Array[String]) -> void
      def write_text(node, out)
        out << node.to_s
      end

      #: (REXML::Comment, Array[String]) -> void
      def write_comment(node, out)
        out << "<!--#{node}-->"
      end

      #: (REXML::CData, Array[String]) -> void
      def write_cdata(node, out)
        out << "<![CDATA[#{node}]]>"
      end

      #: (REXML::Instruction, Array[String]) -> void
      def write_instruction(node, out)
        out << node.to_s
      end
    end

    # Indents elements and re-flows text to width columns, as rexml's Pretty formatter: whitespace-only text is dropped.
    class Pretty < Default
      attr_accessor :compact #: bool
      attr_accessor :width #: Integer

      #: (?Integer, ?bool) -> void
      def initialize(indentation = 2, ie_hack = false)
        super(ie_hack)
        @indentation = indentation
        @level = 0
        @width = 80
        @compact = false
      end

      #: (REXML::Element, Array[String]) -> void
      def write_element(node, out)
        out << " " * @level
        out << "<#{node.expanded_name}"
        node.attributes.each_attribute { |a| out << " #{a.to_string}" }
        if node.children.empty?
          out << " " if @ie_hack
          out << "/"
        else
          out << ">"
          skip = false
          if compact && node.children.all? { |c| c.is_a?(Text) }
            inner = [] #: Array[String]
            old = @level
            @level = 0
            node.children.each { |c| __write(c, inner) }
            @level = old
            s = inner.join
            if s.size < width
              out << s
              skip = true
            end
          end
          unless skip
            out << "\n"
            @level += @indentation
            node.children.each do |c|
              next if c.is_a?(Text) && c.to_s.strip.empty?

              __write(c, out)
              out << "\n"
            end
            @level -= @indentation
            out << " " * @level
          end
          out << "</#{node.expanded_name}"
        end
        out << ">"
      end

      #: (REXML::Text, Array[String]) -> void
      def write_text(node, out)
        s = node.to_s.gsub(/\s/, " ").gsub(/ +/, " ")
        s = REXML.__wrap(s, width - @level)
        s = s.gsub("\n", "\n" + " " * @level) if @level >= 0
        out << " " * @level + s
      end

      #: (REXML::Comment, Array[String]) -> void
      def write_comment(node, out)
        out << " " * @level
        super
      end

      #: (REXML::CData, Array[String]) -> void
      def write_cdata(node, out)
        out << " " * @level
        super
      end

      #: (REXML::XMLDecl) -> REXML::XMLDecl
      def __decl(d) = d

      #: (REXML::Document, Array[String]) -> void
      def write_document(node, out)
        kids = node.children
        first = kids.first #: untyped
        kids.each_with_index do |c, i|
          next if c.instance_of?(Text)

          skip_newline = i == 0 || (i == 1 && first.is_a?(XMLDecl) && !__decl(first).writethis)
          out << "\n" unless skip_newline
          __write(c, out)
        end
      end
    end
  end

  #: (String) -> Array[untyped]
  def self.__tokens(source) = %x{ return rbRexmlTokens(string(source)) }

  #: (String) -> String
  def self.__normalize(s) = %x{ return String(rbRexmlNormalize(string(s))) }

  #: (String) -> String
  def self.__unnormalize(s) = %x{ return String(rbRexmlUnnormalize(string(s))) }

  #: (untyped, String) -> void
  def self.__emit(output, s) = %x{ rbRexmlEmit(output, string(s)) }

  # Pretty's wrap: break at the last space at or before width, repeatedly.
  #: (String, Integer) -> String
  def self.__wrap(s, width) = %x{
    str := string(s)
    var parts []string
    for len([]rune(str)) > int(width) {
      r := []rune(str)
      place := -1
      for i := min(int(width), len(r)-1); i >= 0; i-- {
        if r[i] == ' ' {
          place = i
          break
        }
      }
      if place < 0 {
        break
      }
      parts = append(parts, string(r[:place]))
      str = string(r[place+1:])
    }
    parts = append(parts, str)
    return String(strings.Join(parts, "\\n"))
  }
end
