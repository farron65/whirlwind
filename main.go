package main

import (
	"fmt"
	"io"
	"net/http"
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

	url := "https://httpbin.org/get"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	statusCodeDist := make(map[int]int)

	n := 10

	totalErrors := 0

	var total time.Duration
	var avgTime time.Duration


	for range n {
		r := makeRequest(client, url)
		total += r.duration
		if r.err != nil {
			fmt.Printf("got an error: %s", r.err)
			totalErrors++
		} else {
			statusCodeDist[r.statusCode]++
		}
	}

	fmt.Printf("Total time it took for %d req - %s\n", n, total)

	if statusCodeDist[http.StatusOK] > 0 {
    	avgTime = total / time.Duration(statusCodeDist[http.StatusOK])

		fmt.Printf("Avg time per request is: %s\n", avgTime)
	} else {
		fmt.Printf("Avg time per request: no successful requests\n")
	}

	fmt.Printf("Total errors: %d", totalErrors)
}
