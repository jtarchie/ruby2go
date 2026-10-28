require_relative "application_record"
require_relative "../../runtime/broadcasts"
require_relative "../views"

class Comment < ApplicationRecord
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
    @article_loaded = false
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
    @__t_created_at ||= ActiveSupport.parse_db_time(@created_at_raw)
  end

  def created_at_raw
    @created_at_raw
  end

  def created_at=(value)
    self.created_at_raw = ActiveSupport.format_db_time(value)
  end

  def created_at_raw=(value)
    @__t_created_at = nil
    @created_at_raw = value
  end

  def updated_at
    @__t_updated_at ||= ActiveSupport.parse_db_time(@updated_at_raw)
  end

  def updated_at_raw
    @updated_at_raw
  end

  def updated_at=(value)
    self.updated_at_raw = ActiveSupport.format_db_time(value)
  end

  def updated_at_raw=(value)
    @__t_updated_at = nil
    @updated_at_raw = value
  end

  def self.table_name
    "comments"
  end

  def self.schema_columns
    [:id, :article_id, :commenter, :body, :created_at, :updated_at]
  end

  def self.schema_time_columns
    [:created_at, :updated_at]
  end

  def self.instantiate(row)
    instance = Comment.from_row(CommentRow.from_raw(row))
    instance.mark_persisted!
    instance
  end

  def self.from_row(row)
    instance = Comment.new
    instance.id = row.id
    instance.article_id = row.article_id
    instance.commenter = (row.commenter).nil? ? nil : (row.commenter).to_s
    instance.body = (row.body).nil? ? nil : (row.body).to_s
    instance.created_at_raw = row.created_at
    instance.updated_at_raw = row.updated_at
    instance._note_hydrated
    instance
  end

  def self.from_stmt(stmt)
    instance = Comment.new
    instance.id = Db.column_int(stmt, 0)
    instance.article_id = Db.column_int(stmt, 1)
    instance.commenter = Db.column_text_opt(stmt, 2)
    instance.body = Db.column_text_opt(stmt, 3)
    instance.created_at_raw = Db.column_text(stmt, 4)
    instance.updated_at_raw = Db.column_text(stmt, 5)
    instance.mark_persisted!
    instance._note_hydrated
    instance
  end

  def initialize(attrs = ActiveRecord::Base::EMPTY_ATTRS)
    super()
    self.id = (attrs[:id] || 0).to_i
    self.article_id = attrs[:article_id] || 0
    self.commenter = (attrs[:commenter]).nil? ? nil : (attrs[:commenter]).to_s
    self.body = (attrs[:body]).nil? ? nil : (attrs[:body]).to_s
    self.created_at_raw = attrs[:created_at] || ""
    self.created_at_raw = ActiveSupport.format_db_time(attrs[:created_at]) unless (attrs[:created_at].nil?)
    self.updated_at_raw = attrs[:updated_at] || ""
    self.updated_at_raw = ActiveSupport.format_db_time(attrs[:updated_at]) unless (attrs[:updated_at].nil?)
    self.article_id = attrs[:article].id unless (attrs[:article].nil?)
  end

  def attributes
    { "article_id" => @article_id, "commenter" => @commenter, "body" => @body, "created_at" => @created_at_raw, "updated_at" => @updated_at_raw }
  end

  def [](name)
    case name
    when :id
      @id
    when :article_id
      @article_id
    when :commenter
      (@commenter).nil? ? nil : (@commenter).to_s
    when :body
      (@body).nil? ? nil : (@body).to_s
    when :created_at
      @created_at_raw
    when :updated_at
      @updated_at_raw
    end
  end

  def []=(name, value)
    case name
    when :id
      @id = (value).to_i
    when :article_id
      @article_id = (value).to_i
    when :commenter
      @commenter = (value).nil? ? nil : (value).to_s
    when :body
      @body = (value).nil? ? nil : (value).to_s
    when :created_at
      @created_at_raw = (value).to_s
    when :updated_at
      @updated_at_raw = (value).to_s
    end
  end

  def update(attrs)
    self.article_id = (attrs[:article_id]).to_i if attrs.key?(:article_id)
    self.commenter = (attrs[:commenter]).nil? ? nil : (attrs[:commenter]).to_s if attrs.key?(:commenter)
    self.body = (attrs[:body]).nil? ? nil : (attrs[:body]).to_s if attrs.key?(:body)
    self.created_at_raw = (attrs[:created_at]).to_s if attrs.key?(:created_at)
    self.created_at_raw = ActiveSupport.format_db_time(attrs[:created_at]) unless ((attrs[:created_at]).to_s.nil?)
    self.updated_at_raw = (attrs[:updated_at]).to_s if attrs.key?(:updated_at)
    self.updated_at_raw = ActiveSupport.format_db_time(attrs[:updated_at]) unless ((attrs[:updated_at]).to_s.nil?)
    save
  end

  def update!(attrs)
    self.article_id = (attrs[:article_id]).to_i if attrs.key?(:article_id)
    self.commenter = (attrs[:commenter]).nil? ? nil : (attrs[:commenter]).to_s if attrs.key?(:commenter)
    self.body = (attrs[:body]).nil? ? nil : (attrs[:body]).to_s if attrs.key?(:body)
    self.created_at_raw = (attrs[:created_at]).to_s if attrs.key?(:created_at)
    self.created_at_raw = ActiveSupport.format_db_time(attrs[:created_at]) unless ((attrs[:created_at]).to_s.nil?)
    self.updated_at_raw = (attrs[:updated_at]).to_s if attrs.key?(:updated_at)
    self.updated_at_raw = ActiveSupport.format_db_time(attrs[:updated_at]) unless ((attrs[:updated_at]).to_s.nil?)
    save!
    self
  end

  def fill_timestamps(creating)
    now = ActiveSupport.db_now
    if !(creating) || @updated_at_raw.nil? || (@updated_at_raw || "").empty?
      @__t_updated_at = nil
      @updated_at_raw = now
    end
    if creating && (@created_at_raw.nil? || (@created_at_raw || "").empty?)
      @__t_created_at = nil
      @created_at_raw = now
    end
  end

  def self._adapter_find_by_id(id)
    stmt = Db.prepare("SELECT id, article_id, commenter, body, created_at, updated_at FROM comments WHERE id = " + Db.escape_int(id) + " LIMIT 1")
    result = nil
    result = Comment.from_stmt(stmt) if Db.step?(stmt)
    Db.finalize(stmt)
    result
  end

  def self._adapter_all
    stmt = Db.prepare("SELECT id, article_id, commenter, body, created_at, updated_at FROM comments")
    results = []
    results << Comment.from_stmt(stmt) while Db.step?(stmt)
    Db.finalize(stmt)
    results
  end

  def self._adapter_last
    stmt = Db.prepare("SELECT id, article_id, commenter, body, created_at, updated_at FROM comments ORDER BY id DESC LIMIT 1")
    result = nil
    result = Comment.from_stmt(stmt) if Db.step?(stmt)
    Db.finalize(stmt)
    result
  end

  def _adapter_insert
    Db.exec("INSERT INTO comments (article_id, commenter, body, created_at, updated_at) VALUES (" + Db.escape_int(@article_id) + ", " + Db.escape_string_opt(@commenter) + ", " + Db.escape_string_opt(@body) + ", " + Db.escape_string(@created_at_raw) + ", " + Db.escape_string(@updated_at_raw) + ")")
    Db.last_insert_rowid
  end

  def _adapter_update
    Db.exec("UPDATE comments SET article_id = " + Db.escape_int(@article_id) + ", commenter = " + Db.escape_string_opt(@commenter) + ", body = " + Db.escape_string_opt(@body) + ", created_at = " + Db.escape_string(@created_at_raw) + ", updated_at = " + Db.escape_string(@updated_at_raw) + " WHERE id = " + Db.escape_int(@id))
  end

  def _adapter_delete
    Db.exec("DELETE FROM comments WHERE id = " + Db.escape_int(@id))
  end

  def self._adapter_count
    stmt = Db.prepare("SELECT COUNT(*) FROM comments")
    Db.step?(stmt)
    result = Db.column_int(stmt, 0)
    Db.finalize(stmt)
    result
  end

  def self._adapter_exists_by_id?(id)
    stmt = Db.prepare("SELECT 1 FROM comments WHERE id = " + Db.escape_int(id) + " LIMIT 1")
    result = Db.step?(stmt)
    Db.finalize(stmt)
    result
  end

  def self._adapter_truncate
    Db.exec("DELETE FROM comments")
    Db.exec("DELETE FROM sqlite_sequence WHERE name = 'comments'")
  end

  def self.delete_all
    Db.exec("DELETE FROM comments")
    nil
  end

  def _adapter_reload
    stmt = Db.prepare("SELECT id, article_id, commenter, body, created_at, updated_at FROM comments WHERE id = " + Db.escape_int(@id) + " LIMIT 1")
    if Db.step?(stmt)
      @id = Db.column_int(stmt, 0)
      @article_id = Db.column_int(stmt, 1)
      @commenter = Db.column_text_opt(stmt, 2)
      @body = Db.column_text_opt(stmt, 3)
      @__t_created_at = nil
      @created_at_raw = Db.column_text(stmt, 4)
      @__t_updated_at = nil
      @updated_at_raw = Db.column_text(stmt, 5)
      mark_persisted!
    end
    Db.finalize(stmt)
    self
  end

  def self._columns_sql
    "comments.id AS id, comments.article_id AS article_id, comments.commenter AS commenter, comments.body AS body, comments.created_at AS created_at, comments.updated_at AS updated_at"
  end

  def self._hydrate_all(sql)
    stmt = Db.prepare(sql)
    results = []
    results << Comment.from_stmt(stmt) while Db.step?(stmt)
    Db.finalize(stmt)
    results
  end

  def self.from_params(p)
    instance = Comment.new
    instance.commenter = p.commenter if p.commenter_provided
    instance.body = p.body if p.body_provided
    instance
  end

  def update_from_comment_params(p)
    if !(p.commenter_provided)
      nil
    else
      self.commenter = p.commenter
    end
    if !(p.body_provided)
      nil
    else
      self.body = p.body
    end
    save
  end

  def update_from_comment_params!(p)
    if !(p.commenter_provided)
      nil
    else
      self.commenter = p.commenter
    end
    if !(p.body_provided)
      nil
    else
      self.body = p.body
    end
    save!
    self
  end

  def validate
    errors << "Commenter can't be blank" if @commenter.nil? || (@commenter || "").empty?
    errors << "Body can't be blank" if @body.nil? || (@body || "").empty?
    errors << "Article must exist" if @article_id.nil? || @article_id == 0 || !(Article.exists?(@article_id))
  end

  def article
    if @article_id == 0
      nil
    else
      stmt = Db.prepare("SELECT id, title, body, created_at, updated_at FROM articles WHERE id = " + Db.escape_int(@article_id) + " LIMIT 1")
      result = nil
      result = Article.from_stmt(stmt) if Db.step?(stmt)
      Db.finalize(stmt)
      result
    end
  end

  def article=(value)
    if value.nil?
      @article_id = 0
    else
      @article_id = value.id
      if value.id == 0
        @article_cache = value
        @article_loaded = true
      end
    end
  end

  def dom_prefix
    "comment"
  end

  def to_param
    @id.to_s
  end

  def dom_record_key
    @id.to_s
  end

  def cache_key
    if persisted?
      "comments/#{@id}"
    else
      "comments/new"
    end
  end

  def cache_key_with_version
    "#{cache_key}-#{@updated_at_raw}"
  end

  def after_create_commit
    Broadcasts.append(stream: "article_#{@article_id}_comments", target: "comments", html: ActionView::ViewHelpers.broadcast_render(ActionView::ViewHelpers.begin_broadcast_render, Views::Comments.comment(self)))
    parent = article
    return if parent.nil?

    Broadcasts.replace(stream: "articles", target: "#{parent.dom_prefix}_#{parent.dom_record_key}", html: ActionView::ViewHelpers.broadcast_render(ActionView::ViewHelpers.begin_broadcast_render, Views::Articles.article(parent)))
  end

  def after_update_commit
    Broadcasts.replace(stream: "article_#{@article_id}_comments", target: "#{dom_prefix}_#{dom_record_key}", html: ActionView::ViewHelpers.broadcast_render(ActionView::ViewHelpers.begin_broadcast_render, Views::Comments.comment(self)))
  end

  def after_destroy_commit
    Broadcasts.remove(stream: "article_#{@article_id}_comments", target: "#{dom_prefix}_#{dom_record_key}")
    parent = article
    return if parent.nil?

    Broadcasts.replace(stream: "articles", target: "#{parent.dom_prefix}_#{parent.dom_record_key}", html: ActionView::ViewHelpers.broadcast_render(ActionView::ViewHelpers.begin_broadcast_render, Views::Articles.article(parent)))
  end

  def to_gid_param
    GlobalID.param("Comment", @id)
  end

  def _autosave_article
    if @article_loaded && @article_id == 0 && !(@article_cache.nil?)
      @article_cache.save
      @article_id = @article_cache.id
    end
  end

  def before_validation
    _autosave_article
  end
end
