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

	url := "http://localhost:9000/slow"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	statusCodeDist := make(map[int]int)

	var total time.Duration
	var avgTime time.Duration

	var wg sync.WaitGroup
	
	C := 240

	resultChan := make(chan Result, C)
	totalErrors := 0

	startTime := time.Now()

	for range C {
		wg.Add(1)
		go func ()  {

			defer wg.Done()
			r := makeRequest(client, url)
			resultChan <- r
		}()
	}

	wg.Wait()

	fmt.Printf("Total time it took for %d requests: %v\n", C, time.Since(startTime))
	fmt.Println(resultChan)
	fmt.Println(len(resultChan))

	for range C {
		r := <-resultChan

		total += r.duration

		if r.err != nil {
			fmt.Println(r.err)
			totalErrors++
		} else {
			statusCodeDist[r.statusCode]++
		}
		
	}
	
	if statusCodeDist[http.StatusOK] > 0 {
    	avgTime = total / time.Duration(statusCodeDist[http.StatusOK])

		fmt.Printf("Avg time per request is: %s\n", avgTime)
	} else {
		fmt.Printf("Avg time per request: no successful requests\n")
	}

	fmt.Printf("Total errors: %d", totalErrors)
}
