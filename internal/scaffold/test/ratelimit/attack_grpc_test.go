package ratelimit

import (
	"testing"
	"time"
)

func TestParseBizCode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int
	}{
		// Real wire format: kitex BizStatusError over grpcurl.
		{
			"kitex biz error wire format",
			"ERROR:\n  Code: 13\n  Message: rpc error: code = Internal desc = biz error: code=10429, msg=rate limited; retry after 60s\n\n",
			10429,
		},
		{
			"kitex biz error compact",
			"rpc error: code = Internal desc = biz error: code=10429, msg=rate limited\n",
			10429,
		},
		// Negative: gRPC status code names (not digits) must NOT match.
		{
			"grpc unavailable no biz code",
			"ERROR:\n  Code: 14\n  Message: rpc error: code = Unavailable desc = connection refused\n\n",
			0,
		},
		{
			"grpc internal no biz code",
			"rpc error: code = Internal desc = some other internal error\n",
			0,
		},
		// JSON fallback (grpcurl -type=json or synthetic).
		{"rate limited json", `{"code":10429,"msg":"rate limited"}` + "\n", 10429},
		{"no code field", `{"found":false}`, 0},
		{"malformed non-json", "grpcurl error: connection refused\n", 0},
		{"regex fallback json-in-garbage", `ERROR: transport failed {"code":10429} <trailing garbage`, 10429},
		{"empty", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseBizCode(tc.in); got != tc.want {
				t.Errorf("parseBizCode(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestClassifyGRPCResults(t *testing.T) {
	outputs := []grpcCallResult{
		{bizCode: 0, latency: 100 * time.Millisecond},
		{bizCode: 10429, latency: 200 * time.Millisecond},
		{bizCode: 10429, latency: 300 * time.Millisecond},
		{errOther: true, latency: 400 * time.Millisecond},
	}
	ar := aggregateGRPCResults(outputs)
	if ar.TotalReqs != 4 || ar.Status200 != 1 || ar.Status429 != 2 || ar.StatusOther != 1 {
		t.Fatalf("unexpected aggregate: %+v", ar)
	}
	// mean of 100,200,300,400 = 250ms
	if ar.AvgLatency != 250*time.Millisecond {
		t.Errorf("AvgLatency = %v, want 250ms", ar.AvgLatency)
	}
	// percentile floor-index: sorted[floor((n-1)*0.99)] = sorted[int(3*0.99)] = sorted[2] = 300ms
	if ar.P99Latency != 300*time.Millisecond {
		t.Errorf("P99Latency = %v, want 300ms", ar.P99Latency)
	}
}

func TestClassifyGRPCResultsEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("aggregateGRPCResults panicked on nil input: %v", r)
		}
	}()
	ar := aggregateGRPCResults(nil)
	if ar.TotalReqs != 0 || ar.Status200 != 0 || ar.Status429 != 0 || ar.StatusOther != 0 {
		t.Fatalf("unexpected aggregate for nil input: %+v", ar)
	}
	if ar.AvgLatency != 0 || ar.P99Latency != 0 {
		t.Fatalf("expected zero latencies for nil input, got avg=%v p99=%v", ar.AvgLatency, ar.P99Latency)
	}
}
