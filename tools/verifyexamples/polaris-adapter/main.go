// Compile-only verification entrypoint for the polaris canary adapter asset.
//
// This program does nothing at runtime — it exists solely so `go build ./...`
// in tools/verifyexamples/polaris-adapter/ exercises the adapter source against
// the real polaris-go SDK pinned in go.mod. The adapter is the ONLY place ncgo
// reconciles with polaris-go's API; if this compiles, the asset is wired
// correctly for the pinned SDK version.

package main

import "verifyexample/polaris-adapter/release"

// Reference the adapter constructors so the compiler pulls in
// polaris_canary_adapter.go's sdkClient / instanceFromPolaris bodies.
var (
	_ = release.NewPolarisSelector
	_ = release.NewPolarisInstanceLister
	_ = release.NewPolarisRuleLoader
)

func main() {}
