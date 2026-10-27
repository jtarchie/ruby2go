# Loads every model/support class under app/models/. Generated
# from the emitted tree (see apply_models_aggregator). Each file
# requires its own LOAD-time deps (superclass, includes, class-body
# constants), so the order here is only for legibility; method-body
# references between these files rely on this aggregator having
# run by dispatch time.
require_relative "models/application_job"
require_relative "models/application_mailer"
require_relative "models/application_record"
require_relative "models/article"
require_relative "models/article_params"
require_relative "models/article_row"
require_relative "models/comment"
require_relative "models/comment_params"
require_relative "models/comment_row"
