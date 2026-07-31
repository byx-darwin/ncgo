package ratelimit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type grpcCallResult struct {
	bizCode  int
	errOther bool
	latency  time.Duration
}

var bizCodeRE = regexp.MustCompile(`"code"\s*:\s*(\d+)`)

// parseBizCode extracts a kitex biz status code from grpcurl JSON output.
// Returns 0 when the call carried no biz error.
func parseBizCode(output string) int {
	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &payload); err == nil {
		return payload.Code
	}
	if m := bizCodeRE.FindStringSubmatch(output); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func aggregateGRPCResults(results []grpcCallResult) *attackResult {
	ar := &attackResult{}
	var total time.Duration
	var p99s []time.Duration
	for _, r := range results {
		ar.TotalReqs++
		switch {
		case r.errOther:
			ar.StatusOther++
		case r.bizCode == 10429:
			ar.Status429++
		case r.bizCode == 0:
			ar.Status200++
		default:
			ar.StatusOther++
		}
		total += r.latency
		p99s = append(p99s, r.latency)
	}
	if ar.TotalReqs > 0 {
		ar.AvgLatency = total / time.Duration(ar.TotalReqs)
	}
	ar.P99Latency = percentile(p99s, 0.99)
	return ar
}

func percentile(vals []time.Duration, p float64) time.Duration {
	if len(vals) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), vals...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// runRPCAttackCapture fires rate rps for duration against host:port invoking
// rpcMethod via grpcurl, aggregating biz-status outcomes (10429 = rejected).
func runRPCAttackCapture(ctx context.Context, opts E2EOptions, rpcMethod, payload string) (*attackResult, error) {
	if _, err := exec.LookPath("grpcurl"); err != nil {
		return nil, fmt.Errorf("grpcurl not found: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest")
	}
	dur, err := time.ParseDuration(opts.Duration)
	if err != nil {
		return nil, fmt.Errorf("duration: %w", err)
	}
	target := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	total := opts.Rate * int(dur.Seconds())
	results := make([]grpcCallResult, total)
	sem := make(chan struct{}, opts.Rate)
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < total; i++ {
		// pace: sleep until the i-th slot's scheduled time
		if want := start.Add(time.Duration(i) * time.Second / time.Duration(opts.Rate)); time.Now().Before(want) {
			time.Sleep(time.Until(want))
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			t0 := time.Now()
			cmd := exec.CommandContext(ctx, "grpcurl", "-plaintext", "-d", payload, target, rpcMethod)
			out, err := cmd.CombinedOutput()
			res := grpcCallResult{latency: time.Since(t0)}
			code := parseBizCode(string(out))
			if err != nil && code == 0 {
				res.errOther = true
			} else {
				res.bizCode = code
			}
			results[idx] = res
		}(i)
	}
	wg.Wait()
	return aggregateGRPCResults(results), nil
}

// waitForReadyTCP polls a TCP endpoint (host:port) until it accepts a
// connection, mirroring the interval/timeout semantics of waitForReady.
func waitForReadyTCP(ctx context.Context, host string, port int, interval, timeout time.Duration) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for %s after %s", addr, timeout)
			}
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				continue
			}
			conn.Close()
			fmt.Printf("[check] Service ready at %s\n", addr)
			return nil
		}
	}
}
