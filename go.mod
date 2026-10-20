module github.com/jtarchie/ruby2go

go 1.26.0

require (
	github.com/danielgatis/go-ruby-prism v1.2.0
	github.com/dlclark/regexp2 v1.12.0
	github.com/tetratelabs/wazero v1.9.0
	golang.org/x/tools v0.50.0
)

require github.com/ebitengine/purego v0.11.2

// parser.WithRuntimeConfig (the wazero compilation cache, decision 156) until danielgatis/go-ruby-prism#6 is merged and tagged; then drop this and bump the require.
replace github.com/danielgatis/go-ruby-prism => github.com/jtarchie/go-ruby-prism v1.2.1-0.20261007113717-c29bf218662a
