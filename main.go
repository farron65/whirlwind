package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	url := "https://httpbin.org/et"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	n := 10

	totalErrors := 0
	total200s := 0

	var total time.Duration
	var avgTime time.Duration

	for range n {
		now := time.Now()

		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("got an error %s\n", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			totalErrors += 1
		} else {
			total200s += 1
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		total += time.Since(now)
	}


	fmt.Printf("Total time it took for %d req - %s sec\n", n, total)

	if total200s > 0 {
    fmt.Printf("Avg time per request is: %.5f\n", avgTime.Seconds())
	} else {
		fmt.Printf("Avg time per request: no successful requests\n")
	}
	
	fmt.Printf("Total errors: %d", totalErrors)
}