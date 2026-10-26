# A run-time instance_eval of a file's text, as Homebrew's CaskLoader reads a tap's cask: the trace saw the text, so
# the call site runs it compiled (decision 374).
class TracedCask
  attr_reader :token, :ver

  #: (String) { () [self: TracedCask] -> void } -> void
  def initialize(token, &block)
    @token = token
    @ver = ""
    instance_eval(&block)
  end

  #: (String) -> String
  def version(v) = @ver = v

  # Homebrew's Cask::DSL: an unknown call raises, and a postflight block runs later on another object, whose method it
  # calls
  def method_missing(name, *) = raise(NoMethodError, "no #{name}")

  def respond_to_missing?(*) = false

  def postflight(&block) = @post = block

  def run_post = TracedPost.new.instance_eval(&@post)
end

class TracedPost
  def step(x) = "step #{x}"
end

class TracedCaskLoader
  def initialize(path) = @path = path

  #: (String) { () [self: TracedCask] -> void } -> TracedCask
  def cask(token, &block) = TracedCask.new(token, &block)

  def load
    instance_eval(File.read(@path), @path)
  end
end
