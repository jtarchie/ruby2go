class CommentRow
  def id
    @id
  end

  def id=(value)
    @id = value
  end

  def article_id
    @article_id
  end

  def article_id=(value)
    @article_id = value
  end

  def commenter
    @commenter
  end

  def commenter=(value)
    @commenter = value
  end

  def body
    @body
  end

  def body=(value)
    @body = value
  end

  def created_at
    @created_at
  end

  def created_at=(value)
    @created_at = value
  end

  def updated_at
    @updated_at
  end

  def updated_at=(value)
    @updated_at = value
  end

  def initialize
    @id = 0
    @article_id = 0
    @commenter = nil
    @body = nil
    @created_at = ""
    @updated_at = ""
  end

  def self.from_raw(row)
    instance = CommentRow.new
    instance.id = (row["id"] || 0).to_i
    instance.article_id = (row["article_id"] || 0).to_i
    instance.commenter = if row["commenter"].nil?
      nil
    else
      (row["commenter"]).nil? ? nil : (row["commenter"]).to_s
                         end
    instance.body = if row["body"].nil?
      nil
    else
      (row["body"]).nil? ? nil : (row["body"]).to_s
                    end
    instance.created_at = (row["created_at"]).to_s
    instance.updated_at = (row["updated_at"]).to_s
    instance
  end
end
