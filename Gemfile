# frozen_string_literal: true

source "https://rubygems.org"

# Type annotations: `rbs-inline` extracts `#:` comments, `rbs` validates them.
gem "rbs", "~> 4.0"
gem "rbs-inline", "~> 0.14", require: false

# The HTTP server examples run on under MRI (stdlib until Ruby 3.0). The
# transpiler maps it, and net/http's client, onto Go's net/http.
gem "webrick", "~> 1.9"

# Gems rb2go compiles unmodified (#82): rack 3.2.7 is what sig/gems/rack was written against.
gem "rack", "~> 3.2", ">= 3.2.7"
gem "cuba", "~> 4.0"

# Serves a Rack app on WEBrick under MRI; the transpiler maps it onto Go's net/http too.
gem "rackup", "~> 2.3"
