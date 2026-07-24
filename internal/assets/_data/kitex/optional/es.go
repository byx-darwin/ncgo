// Optional Elasticsearch add-on for Kitex RPC services.
//
// To enable: copy this file to internal/base/data/es.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, mwes.Config{
//	    Addresses: []string{"http://es-1:9200", "http://es-2:9200"},
//	    Username:  "elastic",
//	    Password:  "secret",
//	    APIKey:    "",
//	    CloudID:   "",
//	    MaxRetries:          3,
//	    MaxIdleConnsPerHost: 0,
//	})
//	do.Provide(injector, data.NewES)
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common
//	go get github.com/byx-darwin/go-tools/go-framework

package data

import (
	"context"

	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	mwes "github.com/byx-darwin/go-tools/go-middleware/es"

	"github.com/elastic/go-elasticsearch/v8"
)

// CodeSearchUnavailable is the project-segment error code (>=40100) for search
// backend unavailability. go-middleware/es has no predefined error codes, so a
// project code is defined here and mapped to HTTP 503 at init time.
const CodeSearchUnavailable = 40506

func init() {
	goerror.RegisterHTTPStatuses(map[int]int{CodeSearchUnavailable: 503})
}

// ES wraps the official Elasticsearch v8 client.
type ES struct {
	Client *elasticsearch.Client
}

// NewES creates an Elasticsearch client from mwes.Config via go-middleware/es
// and validates connectivity with the injected startup context.
func NewES(ctx context.Context, cfg mwes.Config) (*ES, func(), error) {
	if len(cfg.Addresses) == 0 {
		return nil, nil, goerror.
			In("elasticsearch").
			Tags("search", "elasticsearch", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("mwes.Config.Addresses is empty")
	}
	cli, err := mwes.NewClient(cfg)
	if err != nil {
		return nil, nil, goerror.
			In("elasticsearch").
			Tags("search", "elasticsearch", "connection").
			Code(CodeSearchUnavailable).
			Public("search_unavailable").
			With("addresses_count", len(cfg.Addresses)).
			Wrapf(err, "elasticsearch.NewClient")
	}
	if _, err := cli.Ping(cli.Ping.WithContext(ctx)); err != nil {
		return nil, nil, goerror.
			In("elasticsearch").
			Tags("search", "elasticsearch", "connection").
			Code(CodeSearchUnavailable).
			Public("search_unavailable").
			With("addresses_count", len(cfg.Addresses)).
			Wrapf(err, "elasticsearch.Ping")
	}
	cleanup := func() {} // go-elasticsearch client has no Close()
	return &ES{Client: cli}, cleanup, nil
}
