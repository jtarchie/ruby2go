# frozen_string_literal: true

source "https://rubygems.org"

# Type annotations: `rbs-inline` extracts `#:` comments, `rbs` validates them.
gem "rbs", "~> 4.0"
gem "rbs-inline", "~> 0.14", require: false

# The HTTP server examples run on under MRI (stdlib until Ruby 3.0). The
# transpiler maps it, and net/http's client, onto Go's net/http.
gem "webrick", "~> 1.9"
