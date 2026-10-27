# Loads every view module into the Views::* namespace. Generated
# from the emitted app/views/ tree (see apply_views_aggregator).
# Each file pulls its own dependencies, so the order here is only
# for legibility (partials first).
require_relative "views/articles/_article"
require_relative "views/articles/_article_json"
require_relative "views/articles/_form"
require_relative "views/comments/_comment"
require_relative "views/articles/edit"
require_relative "views/articles/index"
require_relative "views/articles/index_json"
require_relative "views/articles/new"
require_relative "views/articles/show"
require_relative "views/articles/show_json"
require_relative "views/layouts/application"
require_relative "views/layouts/mailer"
