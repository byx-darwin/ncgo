// Optional ClickHouse add-on for Hertz HTTP services.
//
// To enable: copy this file to internal/base/data/clickhouse.go in your project,
// then register with samber/do:
//
//	do.ProvideValue[context.Context](injector, startupCtx) // context.WithTimeout(...) recommended
//	do.ProvideValue(injector, &clickhouse.Options{
//	    Addr:     []string{"clickhouse-1:9000", "clickhouse-2:9000"},
//	    Protocol: clickhouse.Native, // or clickhouse.HTTP
//	    Auth: clickhouse.Auth{
//	        Database: "default",
//	        Username: "default",
//	        Password: "secret",
//	    },
//	    TLS: nil, // *tls.Config for clickhouse:// over TLS
//	    Settings: clickhouse.Settings{
//	        "max_execution_time": 60,
//	    },
//	    Compression: &clickhouse.Compression{
//	        Method: clickhouse.CompressionLZ4,
//	        Level:  0,
//	    },
//	    DialTimeout:          30 * time.Second,
//	    ReadTimeout:          5 * time.Minute,
//	    MaxOpenConns:         10,
//	    MaxIdleConns:         5,
//	    ConnMaxLifetime:      time.Hour,
//	    ConnOpenStrategy:     clickhouse.ConnOpenInOrder,
//	    BlockBufferSize:      10,
//	    MaxCompressionBuffer: 10240,
//	    Debug:                false,
//	    Debugf:               nil, // func(format string, v ...any)
//	    HTTPHeaders:          nil, // map[string]string when Protocol == clickhouse.HTTP
//	    HTTPUrlPath:          "",
//	    DialContext:          nil, // func(ctx context.Context, addr string) (net.Conn, error)
//	    FreeBufOnConnRelease: false,
//	})
//	do.Provide(injector, data.NewClickHouse)
//
// Full field reference: https://pkg.go.dev/github.com/ClickHouse/clickhouse-go/v2#Options
//
// Required dependency:
//
//	go get github.com/ClickHouse/clickhouse-go/v2
//	go get github.com/byx-darwin/go-tools/go-common
//	go get github.com/byx-darwin/go-tools/go-framework

package data

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	goerror "github.com/byx-darwin/go-tools/go-common/error"
	frameworkerror "github.com/byx-darwin/go-tools/go-framework/error"
)

// CodeDatabaseUnavailable is the project-segment error code (>=40100) for
// database unavailability. go-framework has no equivalent code, so a project
// code is defined here and mapped to HTTP 503 at init time.
const CodeDatabaseUnavailable = 40503

func init() {
	goerror.RegisterHTTPStatuses(map[int]int{CodeDatabaseUnavailable: 503})
}

// ClickHouse wraps a native ClickHouse driver connection.
type ClickHouse struct {
	Conn driver.Conn
}

// NewClickHouse opens a connection from the full *clickhouse.Options struct
// and pings the cluster with the injected startup context.
func NewClickHouse(ctx context.Context, opts *clickhouse.Options) (*ClickHouse, func(), error) {
	if opts == nil {
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("clickhouse.Options is nil")
	}
	if len(opts.Addr) == 0 {
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "configuration").
			Code(frameworkerror.CodeConfigInvalid).
			Public("config_invalid").
			New("clickhouse.Options.Addr is empty")
	}
	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "connection").
			Code(CodeDatabaseUnavailable).
			Public("database_unavailable").
			With("addrs_count", len(opts.Addr)).
			Wrapf(err, "clickhouse.Open")
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, nil, goerror.
			In("clickhouse").
			Tags("analytics", "clickhouse", "connection").
			Code(CodeDatabaseUnavailable).
			Public("database_unavailable").
			With("addrs_count", len(opts.Addr)).
			Wrapf(err, "clickhouse.Ping")
	}
	cleanup := func() { _ = conn.Close() }
	return &ClickHouse{Conn: conn}, cleanup, nil
}
