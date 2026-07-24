// Optional ClickHouse add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/clickhouse.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, mwclickhouse.Config{
//	    Addrs:           []string{"clickhouse-1:9000", "clickhouse-2:9000"},
//	    Database:        "default",
//	    Username:        "default",
//	    Password:        "secret",
//	    DialTimeout:     5,   // seconds
//	    MaxOpenConns:    5,
//	    MaxIdleConns:    5,
//	    ConnMaxLifetime: 300, // seconds
//	    Compress:        true,
//	})
//	do.Provide(injector, data.NewClickHouse)
//
// Required dependency:
//
//	go get github.com/byx-darwin/go-tools/go-middleware
//	go get github.com/byx-darwin/go-tools/go-common
//	go get github.com/byx-darwin/go-tools/go-framework

package data

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
	mwclickhouse "github.com/byx-darwin/go-tools/go-middleware/clickhouse"
)

// ClickHouse wraps a native ClickHouse driver connection.
type ClickHouse struct {
	Conn clickhouse.Conn
}

// NewClickHouse creates a ClickHouse connection from mwclickhouse.Config via
// go-middleware/clickhouse and pings the cluster with the injected startup context.
func NewClickHouse(ctx context.Context, cfg mwclickhouse.Config) (*ClickHouse, func(), error) {
	if cfg.DSN == "" && len(cfg.Addrs) == 0 {
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("mwclickhouse.Config: both DSN and Addrs are empty")
	}
	conn, err := mwclickhouse.NewClient(cfg)
	if err != nil {
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "connection").
			Code(mwclickhouse.CodeConnect).
			Public("database_unavailable").
			With("addrs_count", len(cfg.Addrs)).
			Wrapf(err, "clickhouse.NewClient")
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "connection").
			Code(mwclickhouse.CodeConnect).
			Public("database_unavailable").
			With("addrs_count", len(cfg.Addrs)).
			Wrapf(err, "clickhouse.Ping")
	}
	cleanup := func() { _ = conn.Close() }
	return &ClickHouse{Conn: conn}, cleanup, nil
}
