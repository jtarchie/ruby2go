class ArticleParams
  def initialize
    @title_provided = false
    @body_provided = false
    @title = ""
    @body = ""
  end

  def title
    @title
  end

  def title=(value)
    @title = value
  end

  def title_provided
    @title_provided
  end

  def title_provided=(value)
    @title_provided = value
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
    sub = Params.sub(params, "article")
    instance = ArticleParams.new
    instance.title_provided = Params.provided(sub, "title")
    instance.title = Params.str(sub, "title", "")
    instance.body_provided = Params.provided(sub, "body")
    instance.body = Params.str(sub, "body", "")
    instance
  end

  def [](key)
    case key
    when :title
      @title
    when :body
      @body
    end
  end

  def to_h
    { "title" => @title, "body" => @body }
  end
end
