package main

import (
	"context"
	"io"
	"math"
	"net/http"
	"net/http/httptrace"
	"time"
)

type Result struct {
	duration     time.Duration
	statusCode   int
	err          error
	connDuration time.Duration
	reqDuration  time.Duration
}

func makeRequest(ctx context.Context, client *http.Client, url string) Result {

	now := time.Now()

	var connStart, reqStart time.Time
	var connDuration time.Duration
	var reqDuration time.Duration

	trace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			connStart = time.Now()
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			if !connInfo.Reused {
				connDuration = time.Since(connStart)
			} else {
				connDuration = -1
			}
			reqStart = time.Now()
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			if reqStart.IsZero() {
				reqDuration = -1
			} else {
				reqDuration = time.Since(reqStart)
			}
		},
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Result{
			duration:   0,
			statusCode: 0,
			err:        err,
		}
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	resp, err := client.Do(req)

	if err != nil {
		return Result{
			duration:   0,
			statusCode: 0,
			err:        err,
		}
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return Result{
		duration:     time.Since(now),
		statusCode:   resp.StatusCode,
		err:          nil,
		connDuration: connDuration,
		reqDuration:  reqDuration,
	}
}

func getPercentile(p float64, latencies []time.Duration) time.Duration {
	percentile := math.Floor(p / 100 * float64(len(latencies)))
	return latencies[int(percentile)]
}
