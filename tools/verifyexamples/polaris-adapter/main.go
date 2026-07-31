// Compile + test verification entrypoint for the polaris canary adapter assets.
//
// This program does nothing at runtime — it exists solely so `go build ./...`
// in tools/verifyexamples/polaris-adapter/ exercises the adapter source,
// SDK-neutral ops, and OTel observer against the real polaris-go SDK pinned in
// go.mod. The adapter is the ONLY place ncgo reconciles with polaris-go's API;
// if this compiles, the asset is wired correctly for the pinned SDK version.

package main

import "verifyexample/polaris-adapter/release"

// Reference the adapter + ops + observer constructors so the compiler pulls in
// polaris_canary_adapter.go's sdkClient / instanceFromPolaris bodies,
// release_ops.go's Engine, and polaris_canary_observer_otel.go's OTel observer.
var (
	_ = release.NewPolarisSelector
	_ = release.NewPolarisInstanceLister
	_ = release.NewPolarisRuleLoader
	_ = release.NewOTelObserver
	_ = release.NewEngine
)

func main() {}
