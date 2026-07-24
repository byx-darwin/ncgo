// Optional Polaris registry/discovery add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/registry/polaris.go and the
// accompanying polaris.yaml to your project root, run
// `go get github.com/kitex-contrib/polaris`, then wire it in bootstrap:
//
//	reg, err := registry.NewRegistry(registry.PolarisConfig{
//	    ServiceName: cfg.Server.Registry.Name,
//	    ConfigFile:  "polaris.yaml",
//	})
//	if err != nil { log.Fatal(err) }
//	server.Run(kitexserver.WithRegistry(reg))
//
// Client-side discovery:
//
//	res, err := registry.NewResolver(registry.PolarisConfig{
//	    ServiceName: cfg.ServiceName,
//	    ConfigFile:  "polaris.yaml",
//	})
//	if err != nil { log.Fatal(err) }
//	cli, err := echoclient.New(ctx, cfg, kitexclient.WithResolver(res))
//
// Required dependencies:
//
//	go get github.com/kitex-contrib/polaris
//	go get github.com/byx-darwin/go-tools/go-common

package registry

import (
	"github.com/cloudwego/kitex/pkg/discovery"
	kitexregistry "github.com/cloudwego/kitex/pkg/registry"
	polaris "github.com/kitex-contrib/polaris"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
)

// PolarisConfig configures Polaris-backed service registry/discovery.
type PolarisConfig struct {
	ServiceName string            `json:"service_name" yaml:"service_name"`
	ConfigFile  string            `json:"config_file" yaml:"config_file"`
	Metadata    map[string]string `json:"metadata" yaml:"metadata"`
}

// NewRegistry creates a Polaris-backed kitex server registry.
func NewRegistry(cfg PolarisConfig) (kitexregistry.Registry, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return polaris.NewPolarisRegistry(polaris.ServerOptions{Metadata: cfg.Metadata}, cfg.configFiles()...)
}

// NewResolver creates a Polaris-backed kitex client resolver.
func NewResolver(cfg PolarisConfig) (discovery.Resolver, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return polaris.NewPolarisResolver(polaris.ClientOptions{SrcService: cfg.ServiceName}, cfg.configFiles()...)
}

// Validate checks required fields.
func (c PolarisConfig) Validate() error {
	if c.ServiceName == "" {
		return goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").New("service_name is empty")
	}
	return nil
}

func (c PolarisConfig) configFiles() []string {
	if c.ConfigFile == "" {
		return nil
	}
	return []string{c.ConfigFile}
}
