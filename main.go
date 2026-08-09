package main

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Result struct {
	duration time.Duration
	statusCode int
	err error
}

func makeRequest(client *http.Client, url string) Result {
	now := time.Now()
	resp, err := client.Get(url)
	
	if err != nil {
		return Result{
			duration: 0,
			statusCode: 0,
			err: err,
		}
	}

	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	return Result{
		duration: time.Since(now),
		statusCode: resp.StatusCode,
		err: nil,
	}
}

func main() {
	fmt.Println("hi")

	url := "http://localhost:9000/error"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	statusCodeDist := make(map[int]int)

	var total time.Duration
	var avgLatency time.Duration

	var wg sync.WaitGroup
	
	// N is the total number of requests
	N := 1000

	// C is the amount of concurrent workers to run
	C := 240

	totalResponses := 0
	totalErrors := 0

	resultChan := make(chan Result, C*1000)

	startTime := time.Now()

	for range C {
		wg.Add(1)
		go func ()  {
			defer wg.Done()

			// Each worker fires N/C requests
			//  e.g. N=1000, C=240 sends
        	// only 960 requests total, not 1000.
			for range N/C {
				r := makeRequest(client, url)
				resultChan <- r
			}
		}()
	}

	wg.Wait()

	fmt.Printf("Total time it took for %d requests: %v\n", C, time.Since(startTime))

	// Drain exactly as many results as were sent
	// after N/C*C truncation which can be less than N when N % C != 0
	for range (N/C*C) {
		r := <-resultChan
		if r.err != nil {
			fmt.Println(r.err)
			totalErrors++
		} else {
			total += r.duration
			totalResponses++
			statusCodeDist[r.statusCode]++
		}
	}
	
	if totalResponses > 0 {
    	avgLatency = total / time.Duration(totalResponses)
		fmt.Printf("Avg latency (all HTTP responses): %s\n", avgLatency)
	} else {
		fmt.Printf("Avg latency: no HTTP responses received (all requests failed at the transport level)\n")
	}

	fmt.Printf("Total errors: %d", totalErrors)
}
