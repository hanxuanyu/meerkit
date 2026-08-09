// Package schema exposes the language-neutral JSON contracts used inside the
// gRPC BytesValue messages.
package schema

import "embed"

// Files contains all published protocol schemas.
//
//go:embed *.schema.json
var Files embed.FS
