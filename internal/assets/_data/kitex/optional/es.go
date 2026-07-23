// Optional Elasticsearch add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/data/es.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, elasticsearch.Config{
//	    Addresses: []string{"http://es-1:9200", "http://es-2:9200"},
//	    Username:  "elastic",
//	    Password:  "secret",
//	    APIKey:    "",
//	    ServiceToken: "",
//	    CertificateFingerprint: "",
//	    Header:    nil, // http.Header for default headers
//	    CACert:    nil, // []byte PEM-encoded CA bundle
//	    RetryOnStatus:        []int{502, 503, 504, 429},
//	    DisableRetry:         false,
//	    MaxRetries:           3,
//	    CompressRequestBody:  true,
//	    DiscoverNodesOnStart: false,
//	    DiscoverNodesInterval: 0,
//	    EnableMetrics:        false,
//	    EnableDebugLogger:    false,
//	    EnableCompatibilityMode: false,
//	    RetryBackoff: nil, // func(attempt int) time.Duration
//	    Transport:    nil, // http.RoundTripper for custom TLS / proxy
//	    Logger:       nil, // estransport.Logger
//	    Selector:     nil, // estransport.Selector
//	    ConnectionPoolFunc: nil,
//	})
//	do.Provide(injector, data.NewES)
//
// Full field reference: https://pkg.go.dev/github.com/elastic/go-elasticsearch/v8#Config
//
// Required dependency:
//
//	go get github.com/elastic/go-elasticsearch/v8
//	go get github.com/byx-darwin/go-tools/go-common

package data

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	"github.com/elastic/go-elasticsearch/v8"
)

// ES wraps the official Elasticsearch v8 client.
type ES struct {
	Client *elasticsearch.Client
}

// NewES creates an Elasticsearch client from the full elasticsearch.Config struct
// and validates connectivity with the injected startup context.
func NewES(ctx context.Context, cfg elasticsearch.Config) (*ES, func(), error) {
	if len(cfg.Addresses) == 0 {
		return nil, nil, goerror.
			In("elasticsearch").
			Tags("search", "elasticsearch", "configuration").
			Code("elasticsearch_addresses_missing").
			Public("elasticsearch configuration is invalid").
			New("elasticsearch.Config.Addresses is empty")
	}
	cli, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, nil, goerror.
			In("elasticsearch").
			Tags("search", "elasticsearch", "connection").
			Code("elasticsearch_client_create_failed").
			Public("elasticsearch is unavailable").
			With("addresses_count", len(cfg.Addresses)).
			Wrapf(err, "elasticsearch.NewClient")
	}
	if _, err := cli.Ping(cli.Ping.WithContext(ctx)); err != nil {
		return nil, nil, goerror.
			In("elasticsearch").
			Tags("search", "elasticsearch", "connection").
			Code("elasticsearch_ping_failed").
			Public("elasticsearch is unavailable").
			With("addresses_count", len(cfg.Addresses)).
			Wrapf(err, "elasticsearch.Ping")
	}
	cleanup := func() {} // go-elasticsearch client has no Close()
	return &ES{Client: cli}, cleanup, nil
}
