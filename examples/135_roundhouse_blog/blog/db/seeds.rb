module Seeds
  def self.run
    if Article.count.positive?
      nil
    else
      article1 = Article.create!(title: "Getting Started with Rails", body: "Rails is a web application framework running on the Ruby programming language. It makes building web apps faster and easier with conventions over configuration.")
      Comment.create!(article_id: article1.id, commenter: "Alice", body: "Great introduction! Rails really does make development faster.")
      Comment.create!(article_id: article1.id, commenter: "Bob", body: "I love how Rails handles database migrations automatically.")

      article2 = Article.create!(title: "Understanding MVC Architecture", body: "MVC stands for Model-View-Controller. Models handle data and business logic, Views display information to users, and Controllers coordinate between them.")
      Comment.create!(article_id: article2.id, commenter: "Carol", body: "This pattern really helps keep code organized!")

      article3 = Article.create!(title: "Ruby2JS: Rails Everywhere", body: "Ruby2JS transpiles Ruby to JavaScript, enabling Rails applications to run in browsers, on Node.js, and at the edge. Same code, different runtimes.")

      puts "Created #{Article.count} articles and #{Comment.count} comments"
    end
  end
end
