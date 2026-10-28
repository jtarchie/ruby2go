require_relative "application_controller"
require_relative "../route_helpers"

class CommentsController < ApplicationController
  def process_action(action_name)
    case action_name
    when :create
      create
    when :destroy
      destroy
    end
  end

  def create
    @article = Article.find(@params.fetch("article_id", "0").to_s.to_i)
    @comment = Comment.from_params(comment_params)
    @comment.article_id = @article.id
    if @comment.save
      redirect_to(RouteHelpers.article_path(@article.id), notice: "Comment was successfully created.")
    else
      redirect_to(RouteHelpers.article_path(@article.id), alert: "Could not create comment.")
    end
  end

  def destroy
    @article = Article.find(@params.fetch("article_id", "0").to_s.to_i)
    @comment = Comment.find(@params.fetch("id", "0").to_s.to_i)
    if @comment.article_id != @article.id
      head(:not_found)
      return
    end
    @comment.destroy
    redirect_to(RouteHelpers.article_path(@article.id), notice: "Comment was successfully deleted.")
  end

  def comment_params
    CommentParams.from_raw(@params)
  end
end
