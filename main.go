package main

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
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
}

func makeRequest(client *http.Client, url string) Result {
	now := time.Now()
	resp, err := client.Get(url)

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
	}
}

func getPercentile(p float64, latencies []time.Duration) time.Duration {
	percentile := math.Floor(p / 100 * float64(len(latencies)))
	return latencies[int(percentile)]
}

func main() {
	fmt.Println("hi")

	// N is the total number of requests
	N := 1000

	// C is the amount of concurrent workers to run
	C := 240

	// Z is the duration for which the requests will fire
	Z := 0 * time.Second

	// QPS is the amount of queries sent in a second
	QPS := 100

	// One-shot signal: drain goroutine sends true here once it has
	// processed every result (after resultChan is closed and drained)
	done := make(chan bool)

	url := "https://httpbin.org/get"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	statusCodeDist := make(map[int]int)

	var total time.Duration
	var avgLatency time.Duration

	latencies := []time.Duration{}

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
			// Each worker fires N/C requests
			//  e.g. N=1000, C=240 sends
			// only 960 requests total, not 1000.
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

	// Drains resultChan until it's closed by main after wg.Wait()
	// stops once all sends are done and the channel is closed
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
			}
		}
		done <- true
	}()

	wg.Wait()
	// Safe to close now, wg.Wait() guarantees no worker will send again
	close(resultChan)

	// Blocks until the drain goroutine has finished processing
	// every result currently in the channel
	<-done

	

	if totalResponses > 0 {
		avgLatency = total / time.Duration(totalResponses)
		fmt.Printf("Avg latency (all HTTP responses): %s\n", avgLatency)

		slices.Sort(latencies)

		//Histogram vars
		bucketWidth := 10 * time.Millisecond
		max := latencies[len(latencies)-1]

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

		p50 := getPercentile(50, latencies)
		p90 := getPercentile(90, latencies)
		p99 := getPercentile(99, latencies)
		
		fmt.Println("min:", latencies[0])
		fmt.Println("max:", max)
		fmt.Printf("p50: %v | p90: %v | p99: %v\n", p50, p90, p99)
	} else {
		fmt.Printf("Avg latency: no HTTP responses received (all requests failed at the transport level)\n")
	}

	if Z == 0 && totalResponses+totalErrors != N/C*C && ctx.Err() == nil {
		fmt.Printf("unexpected values, responses %d + errors %d does not equal to %d\n", totalResponses, totalErrors, N/C*C)
	}

	if Z > 0 {
		fmt.Printf("Sent %d requests in %v\n", totalResponses+totalErrors, time.Since(startTime))
	} else {
		fmt.Printf("Total time it took for %d requests: %v\n", N/C*C, time.Since(startTime))
	}

	fmt.Printf("Total errors: %d", totalErrors)
}

