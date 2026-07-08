//go:build tools

// Package tools tracks build-time tool dependencies so `go mod` keeps them
// pinned. tfplugindocs regenerates the registry documentation from the provider
// schema; run `go generate ./...` from the repo root.
package tools

import (
	_ "github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs"
)
