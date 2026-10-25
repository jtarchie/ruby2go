// Package examples embeds examples/*/main.rb for `rugaroo web`'s example picker, kept out of the rugaroo library so its importers don't carry them.
package examples

import "embed"

// FS holds */main.rb.
//
//go:embed */main.rb
var FS embed.FS
