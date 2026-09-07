package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

type ConfigFlags struct {
	N   int
	C   int
	Z   time.Duration
	QPS int
}

func parseFlags() ConfigFlags {
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

	return ConfigFlags{
		N:   N,
		C:   C,
		Z:   Z,
		QPS: QPS,
	}
}
