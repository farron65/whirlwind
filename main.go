package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptrace"
	"os"
	"os/signal"
	"slices"
	"sync"
	"time"
)

type Result struct {
	duration   time.Duration
	statusCode int
	err        error
	connDuration time.Duration
	reqDuration time.Duration
}

func makeRequest(client *http.Client, url string) Result {

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
			}
			reqStart = time.Now()
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			reqDuration = time.Since(reqStart)	
		},
	}

	req, err := http.NewRequest("GET", url, nil);
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
		duration:   time.Since(now),
		statusCode: resp.StatusCode,
		err:        nil,
		connDuration: connDuration,
		reqDuration: reqDuration,
	}
}

func getPercentile(p float64, latencies []time.Duration) time.Duration {
	percentile := math.Floor(p / 100 * float64(len(latencies)))
	return latencies[int(percentile)]
}

func main() {
	// N is the total number of requests
	var reqToSend = flag.Int("n", 10, "total number of requests to send, 0 until duration is done")
	var numThreads = flag.Int("c", 4, "amount of concurrent workers to run")
	var duration = flag.Duration("z", 0*time.Second, "How long (duration) to run the test, 0 until C is done")
	var qps = flag.Int("qps", 10, "queries per second")

	flag.Parse()

	if *numThreads <= 0 {
		fmt.Println("Number of workers cannot be less than or equal to ZERO!")
		os.Exit(1)
	}

	if *qps <= 0 {
		fmt.Println("QPS cannot be less than or equal to ZERO!")
		os.Exit(1)
	}

	if *duration < 0 {
		fmt.Printf("Warning: -z cannot be negative (%v given), ignoring and running in count-mode\n", *duration)
	}

	N := *reqToSend
	C := *numThreads
	Z := *duration
	QPS := *qps

	url := "https://httpbin.org/get"

	fmt.Printf("Target: %s\n", url)
	if Z > 0 {
		fmt.Printf("Config: duration=%v concurrency=%d qps=%d\n\n", Z, C, QPS)
	} else {
		fmt.Printf("Config: requests=%d concurrency=%d qps=%d\n\n", N, C, QPS)
	}

	done := make(chan bool)

	tr := &http.Transport{
		MaxIdleConnsPerHost: min(C, 500),
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	statusCodeDist := make(map[int]int)

	var total time.Duration
	var avgLatency time.Duration

	latencies := []time.Duration{}
	connDurations := []time.Duration{}
	reqDurations := []time.Duration{}

	var wg sync.WaitGroup

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	ctx2, cancel2 := context.WithTimeout(ctx, Z)
	defer cancel2()

	totalResponses := 0
	totalErrors := 0

	resultChan := make(chan Result, C)

	startTime := time.Now()

	ticker := time.NewTicker(time.Duration(1e6/QPS) * time.Microsecond)
	defer ticker.Stop()

	for range C {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if Z > 0 {
				for {
					select {
					case <-ticker.C:
						r := makeRequest(client, url)
						resultChan <- r
					case <-ctx2.Done():
						return
					}
				}
			}
			for range N / C {
				select {
				case <-ticker.C:
					r := makeRequest(client, url)
					resultChan <- r
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		for res := range resultChan {
			if res.err != nil {
				fmt.Println(res.err)
				totalErrors++
			} else {
				total += res.duration
				totalResponses++
				statusCodeDist[res.statusCode]++

				latencies = append(latencies, res.duration)
				connDurations = append(connDurations, res.connDuration)
				reqDurations = append(reqDurations, res.reqDuration)
			}
		}
		done <- true
	}()

	wg.Wait()
	close(resultChan)
	<-done

	elapsed := time.Since(startTime)

	fmt.Println("--- Summary ---")

	if totalResponses > 0 {
		avgLatency = total / time.Duration(totalResponses)

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
		fmt.Printf("connDuration p99: %v |\treqDuration p99: %v\n", connP99, reqP99)

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

	if Z == 0 && totalResponses+totalErrors != N/C*C && ctx.Err() == nil {
		fmt.Printf("unexpected values, responses %d + errors %d does not equal to %d\n", totalResponses, totalErrors, N/C*C)
	}

	if Z > 0 {
		fmt.Printf("Sent %d requests in %v\n", totalResponses+totalErrors, elapsed)
	} else {
		fmt.Printf("Total time for %d requests: %v\n", N/C*C, elapsed)
	}

	fmt.Printf("Total errors: %d\n", totalErrors)
}