package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	url := "https://httpbin.org/get"

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	n := 10

	var total time.Duration
	var avgTime time.Duration

	for range n {
		now := time.Now()

		_, err := client.Get(url)
		if err != nil {
			fmt.Printf("got an error %s\n", err)
		}
		total += time.Since(now)
	}
	avgTime = total / time.Duration(n)

	fmt.Printf("Total time it took for %d req - %s sec\n", n, total)
	fmt.Printf("Avg time per request is: %f", avgTime.Seconds())
}