package ratelimit

import "testing"

func TestClassifyGRPCResults(t *testing.T) {
	outputs := []grpcCallResult{
		{bizCode: 0}, {bizCode: 10429}, {bizCode: 10429}, {errOther: true},
	}
	ar := aggregateGRPCResults(outputs)
	if ar.TotalReqs != 4 || ar.Status200 != 1 || ar.Status429 != 2 || ar.StatusOther != 1 {
		t.Fatalf("unexpected aggregate: %+v", ar)
	}
}

func TestParseGRPCURLBizCode(t *testing.T) {
	if got := parseBizCode(`{"code":10429,"msg":"rate limited"}` + "\n"); got != 10429 {
		t.Errorf("parseBizCode = %d, want 10429", got)
	}
	if got := parseBizCode(`{"found":false}`); got != 0 {
		t.Errorf("parseBizCode = %d, want 0", got)
	}
}
