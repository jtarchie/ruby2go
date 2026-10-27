require_relative "../app/controllers/application_controller"
require_relative "../app/controllers/articles_controller"
require_relative "../app/controllers/comments_controller"

module RouteTable
  def self.table
    [ActionDispatch::Router::Route.new("GET", "/articles", :articles, :index), ActionDispatch::Router::Route.new("GET", "/articles/new", :articles, :new), ActionDispatch::Router::Route.new("POST", "/articles", :articles, :create), ActionDispatch::Router::Route.new("GET", "/articles/:id", :articles, :show), ActionDispatch::Router::Route.new("GET", "/articles/:id/edit", :articles, :edit), ActionDispatch::Router::Route.new("PATCH", "/articles/:id", :articles, :update), ActionDispatch::Router::Route.new("PUT", "/articles/:id", :articles, :update), ActionDispatch::Router::Route.new("DELETE", "/articles/:id", :articles, :destroy), ActionDispatch::Router::Route.new("POST", "/articles/:article_id/comments", :comments, :create), ActionDispatch::Router::Route.new("DELETE", "/articles/:article_id/comments/:id", :comments, :destroy)]
  end

  def self.root
    ActionDispatch::Router::Route.new("GET", "/", :articles, :index)
  end
end
