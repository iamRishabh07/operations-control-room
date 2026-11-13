// Package data embeds the committed historical extract, so that a build with
// no files of its own beside it (the serverless deployment) still carries it.
package data

import _ "embed"

// Seed is the content of seed.json.
//
//go:embed seed.json
var Seed []byte
