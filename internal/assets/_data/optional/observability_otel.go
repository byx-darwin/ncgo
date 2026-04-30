// Optional Alibaba LoongSuite Go Agent add-on for Hertz and Kitex services.
//
// LoongSuite Go Agent instruments Go applications at compile time. It injects
// OpenTelemetry SDK setup code while building with the `otel` CLI, so service
// code does not need to call an InitOTel function.
//
// Typical setup:
//
//  curl -fsSL https://cdn.jsdelivr.net/gh/alibaba/loongsuite-go-agent@main/install.sh | sudo bash
//  otel version
//  otel go build ./...
//  OTEL_SERVICE_NAME=my-service OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318 ./my-service

package observability

// LoongSuiteConfig describes the standard OTEL_* environment variables read by
// binaries built with Alibaba LoongSuite Go Agent (`otel go build ...`).
type LoongSuiteConfig struct {
	ServiceName      string `json:"service_name" yaml:"service_name"`
	Endpoint         string `json:"endpoint" yaml:"endpoint"`
	TracesExporter   string `json:"traces_exporter" yaml:"traces_exporter"`
	MetricsExporter  string `json:"metrics_exporter" yaml:"metrics_exporter"`
	Protocol         string `json:"protocol" yaml:"protocol"`
	Headers          string `json:"headers" yaml:"headers"`
	HTTPExcludePaths string `json:"http_exclude_paths" yaml:"http_exclude_paths"`
}

// DefaultLoongSuiteConfig returns a minimal OTLP/http configuration. Override
// Endpoint for your collector and pass the resulting Env map to your process
// manager, container manifest, or local run script.
func DefaultLoongSuiteConfig(serviceName string) LoongSuiteConfig {
	return LoongSuiteConfig{
		ServiceName:     serviceName,
		Endpoint:        "http://127.0.0.1:4318",
		TracesExporter:  "otlp",
		MetricsExporter: "otlp",
		Protocol:        "http/protobuf",
	}
}

// Env renders non-empty LoongSuite/OpenTelemetry environment variables.
func (c LoongSuiteConfig) Env() map[string]string {
	env := map[string]string{}
	put := func(k, v string) {
		if v != "" {
			env[k] = v
		}
	}
	put("OTEL_SERVICE_NAME", c.ServiceName)
	put("OTEL_EXPORTER_OTLP_ENDPOINT", c.Endpoint)
	put("OTEL_TRACES_EXPORTER", c.TracesExporter)
	put("OTEL_METRICS_EXPORTER", c.MetricsExporter)
	put("OTEL_EXPORTER_OTLP_PROTOCOL", c.Protocol)
	put("OTEL_EXPORTER_OTLP_HEADERS", c.Headers)
	put("OTEL_INSTRUMENTATION_HTTP_EXCLUDE_PATHS", c.HTTPExcludePaths)
	return env
}
