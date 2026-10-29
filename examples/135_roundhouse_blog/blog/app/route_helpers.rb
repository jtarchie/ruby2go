module RouteHelpers
  def self.root_path
    "/"
  end

  def self.articles_path
    "/articles"
  end

  def self.new_article_path
    "/articles/new"
  end

  def self.article_path(id)
    "/articles/#{id}"
  end

  def self.edit_article_path(id)
    "/articles/#{id}/edit"
  end

  def self.article_comments_path(article_id)
    "/articles/#{article_id}/comments"
  end

  def self.article_comment_path(article_id, id)
    "/articles/#{article_id}/comments/#{id}"
  end

  def self.article_json_path(id)
    "/articles/#{id}" + ".json"
  end
end
