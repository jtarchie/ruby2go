require_relative "application_record"
require_relative "../../runtime/broadcasts"
require_relative "../views"

class Article < ApplicationRecord
  def id
    @id
  end

  def id=(value)
    @id = value
  end

  def title
    @title
  end

  def title=(value)
    @title = value
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
    "articles"
  end

  def self.schema_columns
    [:id, :title, :body, :created_at, :updated_at]
  end

  def self.schema_time_columns
    [:created_at, :updated_at]
  end

  def self.instantiate(row)
    instance = Article.from_row(ArticleRow.from_raw(row))
    instance.mark_persisted!
    instance
  end

  def self.from_row(row)
    instance = Article.new
    instance.id = row.id
    instance.title = (row.title).nil? ? nil : (row.title).to_s
    instance.body = (row.body).nil? ? nil : (row.body).to_s
    instance.created_at_raw = row.created_at
    instance.updated_at_raw = row.updated_at
    instance._note_hydrated
    instance
  end

  def self.from_stmt(stmt)
    instance = Article.new
    instance.id = Db.column_int(stmt, 0)
    instance.title = Db.column_text_opt(stmt, 1)
    instance.body = Db.column_text_opt(stmt, 2)
    instance.created_at_raw = Db.column_text(stmt, 3)
    instance.updated_at_raw = Db.column_text(stmt, 4)
    instance.mark_persisted!
    instance._note_hydrated
    instance
  end

  def initialize(attrs = ActiveRecord::Base::EMPTY_ATTRS)
    super()
    self.id = (attrs[:id] || 0).to_i
    self.title = (attrs[:title]).nil? ? nil : (attrs[:title]).to_s
    self.body = (attrs[:body]).nil? ? nil : (attrs[:body]).to_s
    self.created_at_raw = attrs[:created_at] || ""
    self.created_at_raw = ActiveSupport.format_db_time(attrs[:created_at]) unless (attrs[:created_at].nil?)
    self.updated_at_raw = attrs[:updated_at] || ""
    self.updated_at_raw = ActiveSupport.format_db_time(attrs[:updated_at]) unless (attrs[:updated_at].nil?)
    @comments_cache = nil
    @comments_loaded = false
  end

  def attributes
    { "title" => @title, "body" => @body, "created_at" => @created_at_raw, "updated_at" => @updated_at_raw }
  end

  def [](name)
    case name
    when :id
      @id
    when :title
      (@title).nil? ? nil : (@title).to_s
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
    when :title
      @title = (value).nil? ? nil : (value).to_s
    when :body
      @body = (value).nil? ? nil : (value).to_s
    when :created_at
      @created_at_raw = (value).to_s
    when :updated_at
      @updated_at_raw = (value).to_s
    end
  end

  def update(attrs)
    self.title = (attrs[:title]).nil? ? nil : (attrs[:title]).to_s if attrs.key?(:title)
    self.body = (attrs[:body]).nil? ? nil : (attrs[:body]).to_s if attrs.key?(:body)
    self.created_at_raw = (attrs[:created_at]).to_s if attrs.key?(:created_at)
    self.created_at_raw = ActiveSupport.format_db_time(attrs[:created_at]) unless ((attrs[:created_at]).to_s.nil?)
    self.updated_at_raw = (attrs[:updated_at]).to_s if attrs.key?(:updated_at)
    self.updated_at_raw = ActiveSupport.format_db_time(attrs[:updated_at]) unless ((attrs[:updated_at]).to_s.nil?)
    save
  end

  def update!(attrs)
    self.title = (attrs[:title]).nil? ? nil : (attrs[:title]).to_s if attrs.key?(:title)
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
    stmt = Db.prepare("SELECT id, title, body, created_at, updated_at FROM articles WHERE id = " + Db.escape_int(id) + " LIMIT 1")
    result = nil
    result = Article.from_stmt(stmt) if Db.step?(stmt)
    Db.finalize(stmt)
    result
  end

  def self._adapter_all
    stmt = Db.prepare("SELECT id, title, body, created_at, updated_at FROM articles")
    results = []
    results << Article.from_stmt(stmt) while Db.step?(stmt)
    Db.finalize(stmt)
    results
  end

  def self._adapter_last
    stmt = Db.prepare("SELECT id, title, body, created_at, updated_at FROM articles ORDER BY id DESC LIMIT 1")
    result = nil
    result = Article.from_stmt(stmt) if Db.step?(stmt)
    Db.finalize(stmt)
    result
  end

  def _adapter_insert
    Db.exec("INSERT INTO articles (title, body, created_at, updated_at) VALUES (" + Db.escape_string_opt(@title) + ", " + Db.escape_string_opt(@body) + ", " + Db.escape_string(@created_at_raw) + ", " + Db.escape_string(@updated_at_raw) + ")")
    Db.last_insert_rowid
  end

  def _adapter_update
    Db.exec("UPDATE articles SET title = " + Db.escape_string_opt(@title) + ", body = " + Db.escape_string_opt(@body) + ", created_at = " + Db.escape_string(@created_at_raw) + ", updated_at = " + Db.escape_string(@updated_at_raw) + " WHERE id = " + Db.escape_int(@id))
  end

  def _adapter_delete
    Db.exec("DELETE FROM articles WHERE id = " + Db.escape_int(@id))
  end

  def self._adapter_count
    stmt = Db.prepare("SELECT COUNT(*) FROM articles")
    Db.step?(stmt)
    result = Db.column_int(stmt, 0)
    Db.finalize(stmt)
    result
  end

  def self._adapter_exists_by_id?(id)
    stmt = Db.prepare("SELECT 1 FROM articles WHERE id = " + Db.escape_int(id) + " LIMIT 1")
    result = Db.step?(stmt)
    Db.finalize(stmt)
    result
  end

  def self._adapter_truncate
    Db.exec("DELETE FROM articles")
    Db.exec("DELETE FROM sqlite_sequence WHERE name = 'articles'")
  end

  def self.delete_all
    Db.exec("DELETE FROM articles")
    nil
  end

  def _adapter_reload
    stmt = Db.prepare("SELECT id, title, body, created_at, updated_at FROM articles WHERE id = " + Db.escape_int(@id) + " LIMIT 1")
    if Db.step?(stmt)
      @id = Db.column_int(stmt, 0)
      @title = Db.column_text_opt(stmt, 1)
      @body = Db.column_text_opt(stmt, 2)
      @__t_created_at = nil
      @created_at_raw = Db.column_text(stmt, 3)
      @__t_updated_at = nil
      @updated_at_raw = Db.column_text(stmt, 4)
      mark_persisted!
    end
    Db.finalize(stmt)
    self
  end

  def self._columns_sql
    "articles.id AS id, articles.title AS title, articles.body AS body, articles.created_at AS created_at, articles.updated_at AS updated_at"
  end

  def self._hydrate_all(sql)
    stmt = Db.prepare(sql)
    results = []
    results << Article.from_stmt(stmt) while Db.step?(stmt)
    Db.finalize(stmt)
    results
  end

  def self.from_params(p)
    instance = Article.new
    instance.title = p.title if p.title_provided
    instance.body = p.body if p.body_provided
    instance
  end

  def update_from_article_params(p)
    if !(p.title_provided)
      nil
    else
      self.title = p.title
    end
    if !(p.body_provided)
      nil
    else
      self.body = p.body
    end
    save
  end

  def update_from_article_params!(p)
    if !(p.title_provided)
      nil
    else
      self.title = p.title
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
    errors << "Title can't be blank" if @title.nil? || (@title || "").empty?
    errors << "Body can't be blank" if @body.nil? || (@body || "").empty?
    unless (@body.nil?)
      len = @body.length
      errors << "Body is too short (minimum is 10 characters)" if len < 10
    end
  end

  def comments
    return __comments_cache if @comments_loaded
    stmt = Db.prepare("SELECT id, article_id, commenter, body, created_at, updated_at FROM comments WHERE article_id = " + Db.escape_int(@id))
    results = []
    results << Comment.from_stmt(stmt) while Db.step?(stmt)
    Db.finalize(stmt)
    results
  end

  def _preload_comments(list)
    @comments_cache = list
    @comments_loaded = true
  end

  def comments_loaded?
    @comments_loaded
  end

  def comments_target
    __comments_cache
  end

  def comment_ids
    comments.map { |rh_id_record| rh_id_record.id }
  end

  def before_destroy
    comments.each { |c| c.destroy }
  end

  def dom_prefix
    "article"
  end

  def to_param
    @id.to_s
  end

  def dom_record_key
    @id.to_s
  end

  def cache_key
    if persisted?
      "articles/#{@id}"
    else
      "articles/new"
    end
  end

  def cache_key_with_version
    "#{cache_key}-#{@updated_at_raw}"
  end

  def after_create_commit
    Broadcasts.prepend(stream: "articles", target: "articles", html: ActionView::ViewHelpers.broadcast_render(ActionView::ViewHelpers.begin_broadcast_render, Views::Articles.article(self)))
  end

  def after_update_commit
    Broadcasts.replace(stream: "articles", target: "#{dom_prefix}_#{dom_record_key}", html: ActionView::ViewHelpers.broadcast_render(ActionView::ViewHelpers.begin_broadcast_render, Views::Articles.article(self)))
  end

  def after_destroy_commit
    Broadcasts.remove(stream: "articles", target: "#{dom_prefix}_#{dom_record_key}")
  end

  def to_gid_param
    GlobalID.param("Article", @id)
  end

  def __comments_cache
    c = @comments_cache
    if c.nil?
      c = []
      @comments_cache = c
    end
    c
  end
end
