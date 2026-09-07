package main

import (
	"fmt"
	"slices"
	"time"
)

type Report struct {
	url            string
	totalResponses int
	totalTestTime  time.Duration

	latencies     []time.Duration
	connDurations []time.Duration
	reqDurations  []time.Duration

	statusCodeDist map[int]int
	totalErrors    int
	elapsed        time.Duration

	N     int
	C     int
	Z     time.Duration
	QPS   int
	ctxOK bool
}

func runReport(r *Report) {
	totalResponses := r.totalResponses
	totalTestTime := r.totalTestTime

	latencies := r.latencies
	connDurations := r.connDurations
	reqDurations := r.reqDurations

	statusCodeDist := r.statusCodeDist
	totalErrors := r.totalErrors
	elapsed := r.elapsed

	N := r.N
	C := r.C
	Z := r.Z
	QPS := r.QPS
	ctxOK := r.ctxOK

	var avgLatency time.Duration

	fmt.Printf("Target: %s\n", r.url)

	if Z > 0 {
		fmt.Printf("Config: duration=%v concurrency=%d qps=%d\n\n", Z, C, QPS)
	} else {
		fmt.Printf("Config: requests=%d concurrency=%d qps=%d\n\n", N, C, QPS)
	}

	fmt.Println("--- Summary ---")

	if totalResponses > 0 {
		avgLatency = totalTestTime / time.Duration(totalResponses)

		slices.Sort(latencies)
		slices.Sort(connDurations)
		slices.Sort(reqDurations)

		max := latencies[len(latencies)-1]

		p50 := getPercentile(50, latencies)
		p90 := getPercentile(90, latencies)
		p99 := getPercentile(99, latencies)

		connP99 := getPercentile(99, connDurations)
		reqP99 := getPercentile(99, reqDurations)

		fmt.Printf("Avg latency: %v\n", avgLatency)
		fmt.Println("min:", latencies[0])
		fmt.Println("max:", max)
		fmt.Printf("\np50: %v | p90: %v | p99: %v\n", p50, p90, p99)
		fmt.Printf("connDuration p99: %v (from %d fresh connections) | reqDuration p99: %v\n", connP99, len(connDurations), reqP99)

		fmt.Println("\nStatus code distribution:")
		keys := make([]int, 0, len(statusCodeDist))
		for code := range statusCodeDist {
			keys = append(keys, code)
		}
		slices.Sort(keys)
		for _, statusCode := range keys {
			fmt.Printf("[%d]: %d\n", statusCode, statusCodeDist[statusCode])
		}

		fmt.Println("\n--- Histogram ---")
		bucketWidth := 10 * time.Millisecond
		counts := make([]int, max/bucketWidth+1)

		for _, lat := range latencies {
			idx := lat / bucketWidth
			counts[idx] += 1
		}

		for idx, count := range counts {
			if count > 0 {
				low := time.Duration(idx) * bucketWidth
				high := time.Duration(idx+1) * bucketWidth
				fmt.Printf("%v-%v: %d\n", low, high, count)
			}
		}
	} else {
		fmt.Println("no HTTP responses received (all requests failed at the transport level)")
	}

	fmt.Println("\n--- Result ---")

	if Z == 0 && totalResponses+totalErrors != N/C*C && !ctxOK {
		fmt.Printf("unexpected values, responses %d + errors %d does not equal to %d\n", totalResponses, totalErrors, N/C*C)
	}

	if Z > 0 {
		fmt.Printf("Sent %d requests in %v\n", totalResponses+totalErrors, elapsed)
	} else {
		fmt.Printf("Total time for %d requests: %v\n", N/C*C, elapsed)
	}

	fmt.Printf("Total errors: %d\n", totalErrors)
}
