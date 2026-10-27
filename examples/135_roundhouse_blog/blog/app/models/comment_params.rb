class CommentParams
  def initialize
    @commenter_provided = false
    @body_provided = false
    @commenter = ""
    @body = ""
  end

  def commenter
    @commenter
  end

  def commenter=(value)
    @commenter = value
  end

  def commenter_provided
    @commenter_provided
  end

  def commenter_provided=(value)
    @commenter_provided = value
  end

  def body
    @body
  end

  def body=(value)
    @body = value
  end

  def body_provided
    @body_provided
  end

  def body_provided=(value)
    @body_provided = value
  end

  def self.from_raw(params)
    sub = Params.sub(params, "comment")
    instance = CommentParams.new
    instance.commenter_provided = Params.provided(sub, "commenter")
    instance.commenter = Params.str(sub, "commenter", "")
    instance.body_provided = Params.provided(sub, "body")
    instance.body = Params.str(sub, "body", "")
    instance
  end

  def [](key)
    case key
    when :commenter
      @commenter
    when :body
      @body
    end
  end

  def to_h
    { "commenter" => @commenter, "body" => @body }
  end
end
