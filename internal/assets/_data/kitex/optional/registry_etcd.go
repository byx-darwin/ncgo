// Optional Etcd registry/discovery add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/registry/etcd.go in your project,
// run `go get github.com/kitex-contrib/registry-etcd`, then wire it in bootstrap:
//
//  etcdCfg := registry.EtcdConfig{Endpoints: []string{"127.0.0.1:2379"}}
//  r, err := registry.NewEtcdRegistry(etcdCfg)
//  if err != nil { log.Fatal(err) }
//  server.Run(kitexserver.WithRegistry(r))
//
// Client-side discovery:
//
//  resolver, err := registry.NewEtcdResolver(etcdCfg)
//  if err != nil { log.Fatal(err) }
//  cli, err := echoclient.New(ctx, cfg, kitexclient.WithResolver(resolver))
//
// Required dependencies:
//
//  go get github.com/kitex-contrib/registry-etcd
//  go get github.com/byx-darwin/go-tools/go-common

package registry

import (
	"net"
	"time"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/cloudwego/kitex/pkg/discovery"
	kitexregistry "github.com/cloudwego/kitex/pkg/registry"
	etcd "github.com/kitex-contrib/registry-etcd"
	etcdretry "github.com/kitex-contrib/registry-etcd/retry"
)

type EtcdConfig struct {
	Endpoints          []string                `json:"endpoints" yaml:"endpoints"`
	Username           string                  `json:"username" yaml:"username"`
	Password           string                  `json:"password" yaml:"password"`
	DialTimeoutSeconds int                     `json:"dial_timeout_seconds" yaml:"dial_timeout_seconds"`
	ServicePrefix      string                  `json:"service_prefix" yaml:"service_prefix"`
	RegistryRetry      EtcdRegistryRetryConfig `json:"registry_retry" yaml:"registry_retry"`
}

type EtcdRegistryRetryConfig struct {
	Enabled             bool `json:"enabled" yaml:"enabled"`
	MaxAttemptTimes     int  `json:"max_attempt_times" yaml:"max_attempt_times"`
	ObserveDelaySeconds int  `json:"observe_delay_seconds" yaml:"observe_delay_seconds"`
	RetryDelaySeconds   int  `json:"retry_delay_seconds" yaml:"retry_delay_seconds"`
}

type RegistryInfoConfig struct {
	PublicAddr    string            `json:"public_addr" yaml:"public_addr"`
	Weight        int               `json:"weight" yaml:"weight"`
	WarmUpSeconds int               `json:"warm_up_seconds" yaml:"warm_up_seconds"`
	Tags          map[string]string `json:"tags" yaml:"tags"`
}

func NewEtcdRegistry(cfg EtcdConfig) (kitexregistry.Registry, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.RegistryRetry.Enabled {
		return etcd.NewEtcdRegistryWithRetry(cfg.Endpoints, cfg.RegistryRetry.retryConfig(), cfg.options()...)
	}
	return etcd.NewEtcdRegistry(cfg.Endpoints, cfg.options()...)
}

func NewEtcdResolver(cfg EtcdConfig) (discovery.Resolver, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return etcd.NewEtcdResolver(cfg.Endpoints, cfg.options()...)
}

func NewRegistryInfo(cfg RegistryInfoConfig) (*kitexregistry.Info, error) {
	if cfg.Weight < 0 || cfg.WarmUpSeconds < 0 {
		return nil, goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").New("weight and warm_up_seconds must not be negative")
	}
	info := &kitexregistry.Info{Weight: cfg.Weight, Tags: cfg.Tags, WarmUp: seconds(cfg.WarmUpSeconds)}
	if cfg.PublicAddr == "" {
		return info, nil
	}
	addr, err := net.ResolveTCPAddr("tcp", cfg.PublicAddr)
	if err != nil {
		return nil, goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").With("public_addr", cfg.PublicAddr).Wrap(err)
	}
	info.Addr = addr
	info.SkipListenAddr = true
	return info, nil
}

func (c EtcdConfig) Validate() error {
	if len(c.Endpoints) == 0 {
		return goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").New("etcd endpoints is empty")
	}
	if c.DialTimeoutSeconds < 0 {
		return goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").New("dial_timeout_seconds must not be negative")
	}
	return c.RegistryRetry.Validate()
}

func (c EtcdRegistryRetryConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.MaxAttemptTimes < 0 || c.ObserveDelaySeconds < 0 || c.RetryDelaySeconds < 0 {
		return goerror.In("kitex.registry").Code("registry_config_invalid").Public("registry configuration is invalid").New("registry retry settings must not be negative")
	}
	return nil
}

func (c EtcdConfig) options() []etcd.Option {
	opts := make([]etcd.Option, 0, 4)
	if c.Username != "" || c.Password != "" {
		opts = append(opts, etcd.WithAuthOpt(c.Username, c.Password))
	}
	if c.DialTimeoutSeconds > 0 {
		opts = append(opts, etcd.WithDialTimeoutOpt(seconds(c.DialTimeoutSeconds)))
	}
	if c.ServicePrefix != "" {
		opts = append(opts, etcd.WithEtcdServicePrefix(c.ServicePrefix))
	}
	return opts
}

func (c EtcdRegistryRetryConfig) retryConfig() *etcdretry.Config {
	opts := make([]etcdretry.Option, 0, 3)
	if c.MaxAttemptTimes > 0 {
		opts = append(opts, etcdretry.WithMaxAttemptTimes(uint(c.MaxAttemptTimes)))
	}
	if c.ObserveDelaySeconds > 0 {
		opts = append(opts, etcdretry.WithObserveDelay(seconds(c.ObserveDelaySeconds)))
	}
	if c.RetryDelaySeconds > 0 {
		opts = append(opts, etcdretry.WithRetryDelay(seconds(c.RetryDelaySeconds)))
	}
	return etcdretry.NewRetryConfig(opts...)
}

func seconds(value int) time.Duration {
	if value <= 0 {
		return 0
	}
	return time.Duration(value) * time.Second
}
