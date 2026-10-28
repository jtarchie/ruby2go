require_relative "application_controller"
require_relative "../route_helpers"
require_relative "../views"

class ArticlesController < ApplicationController
  def process_action(action_name)
    case action_name
    when :index
      index
    when :show
      show
    when :new
      new_action
    when :edit
      edit
    when :create
      create
    when :update
      update
    when :destroy
      destroy
    end
  end

  def index
    stmt = Db.prepare("SELECT id, title, body, created_at, updated_at FROM articles ORDER BY created_at DESC")
    results = []
    while Db.step?(stmt)
      results << Article.from_stmt(stmt)
    end
    Db.finalize(stmt)
    __comments_ids = results.map { |a| a.id }
    __comments_stmt = Db.prepare("SELECT id, article_id, commenter, body, created_at, updated_at FROM comments WHERE article_id IN (" + Db.escape_int_list(__comments_ids) + ")")
    __comments_loaded = []
    while Db.step?(__comments_stmt)
      __comments_loaded << Comment.from_stmt(__comments_stmt)
    end
    Db.finalize(__comments_stmt)
    results.each do |a|
  __comments_group = []
  __comments_loaded.each { |r| __comments_group << r if r.article_id == a.id }
  a._preload_comments(__comments_group)
end
    @articles = results
    if request_format == :json
      render(Views::Articles.index_json(@articles), content_type: "application/json")
    else
      render(Views::Layouts.application(Views::Articles.index(@articles, @flash[:notice], @flash[:alert]), @flash[:notice], @flash[:alert]))
    end
  end

  def show
    @article = Article.find(@params.fetch("id", "0").to_s.to_i)
    if request_format == :json
      render(Views::Articles.show_json(@article), content_type: "application/json")
    else
      render(Views::Layouts.application(Views::Articles.show(@article, @flash[:notice], @flash[:alert]), @flash[:notice], @flash[:alert]))
    end
  end

  def new_action
    @article = Article.new
    render(Views::Layouts.application(Views::Articles._new(@article, @flash[:notice], @flash[:alert]), @flash[:notice], @flash[:alert]))
  end

  def edit
    @article = Article.find(@params.fetch("id", "0").to_s.to_i)
    render(Views::Layouts.application(Views::Articles.edit(@article, @flash[:notice], @flash[:alert]), @flash[:notice], @flash[:alert]))
  end

  def create
    @article = Article.from_params(article_params)
    if @article.save
      if request_format == :json
        render(Views::Articles.show_json(@article), status: :created, location: RouteHelpers.article_path(@article.id), content_type: "application/json")
      else
        redirect_to(RouteHelpers.article_path(@article.id), notice: "Article was successfully created.")
      end
    else
      render(Views::Layouts.application(Views::Articles._new(@article, @flash[:notice], @flash[:alert]), @flash[:notice], @flash[:alert]), status: :unprocessable_content)
    end
  end

  def update
    @article = Article.find(@params.fetch("id", "0").to_s.to_i)
    if @article.update_from_article_params(article_params)
      if request_format == :json
        render(Views::Articles.show_json(@article), status: :ok, location: RouteHelpers.article_path(@article.id), content_type: "application/json")
      else
        redirect_to(RouteHelpers.article_path(@article.id), notice: "Article was successfully updated.", status: :see_other)
      end
    else
      render(Views::Layouts.application(Views::Articles.edit(@article, @flash[:notice], @flash[:alert]), @flash[:notice], @flash[:alert]), status: :unprocessable_content)
    end
  end

  def destroy
    @article = Article.find(@params.fetch("id", "0").to_s.to_i)
    @article.destroy
    if request_format == :json
      head(:no_content, content_type: "application/json")
    else
      redirect_to(RouteHelpers.articles_path, notice: "Article was successfully destroyed.", status: :see_other)
    end
  end

  def article_params
    ArticleParams.from_raw(@params)
  end
end
