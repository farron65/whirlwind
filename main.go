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

	var totalTestTime time.Duration

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
				totalTestTime += res.duration
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

	newReport := Report{
		totalResponses: totalResponses,
		totalTestTime: totalTestTime,

		latencies: latencies,
		connDurations: connDurations,
		reqDurations: reqDurations,

		statusCodeDist: statusCodeDist,
		totalErrors: totalErrors,
		elapsed: elapsed,

		N: N,
		C: C,
		Z: Z,
		ctxOK: ctx.Err() == nil,
	}

	runReport(&newReport)
}