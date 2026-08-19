package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	fmt.Println("hi")

	url := "http://localhost:9000/ok"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	statusCodeDist := make(map[int]int)

	var total time.Duration
	var avgLatency time.Duration

	var wg sync.WaitGroup

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// One-shot signal: drain goroutine sends true here once it has
	// processed every result (after resultChan is closed and drained)
	done := make(chan bool)

	// N is the total number of requests
	N := 1000

	// C is the amount of concurrent workers to run
	C := 240

	// QPS is the amount of queries sent in a second
	QPS := 100

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
			for range N / C {

				select {
				case <- ticker.C:
					r := makeRequest(client, url)
					resultChan <- r
				case <- ctx.Done():
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

	fmt.Printf("Total time it took for %d requests: %v\n", N/C*C, time.Since(startTime))

	if totalResponses > 0 {
		avgLatency = total / time.Duration(totalResponses)
		fmt.Printf("Avg latency (all HTTP responses): %s\n", avgLatency)
	} else {
		fmt.Printf("Avg latency: no HTTP responses received (all requests failed at the transport level)\n")
	}

	if totalResponses + totalErrors != N/C*C {
		fmt.Printf("unexpected values, responses %d + errors %d does not equal to %d\n", totalResponses, totalErrors, N/C*C)
	}

	fmt.Printf("Total errors: %d", totalErrors)
}
