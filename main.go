package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	flags := parseFlags()

	N := flags.N
	C := flags.C
	Z := flags.Z
	QPS := flags.QPS
	URL := flags.URL

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
						r := makeRequest(client, URL)
						resultChan <- r
					case <-ctx2.Done():
						return
					}
				}
			}
			for range N / C {
				select {
				case <-ticker.C:
					r := makeRequest(client, URL)
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
				if res.connDuration != -1 {
					connDurations = append(connDurations, res.connDuration)
				}
				if res.reqDuration != -1 {
					reqDurations = append(reqDurations, res.reqDuration)
				}
			}
		}
		done <- true
	}()

	wg.Wait()
	close(resultChan)
	<-done

	elapsed := time.Since(startTime)

	newReport := Report{
		url:            URL,
		totalResponses: totalResponses,
		totalTestTime:  totalTestTime,

		latencies:     latencies,
		connDurations: connDurations,
		reqDurations:  reqDurations,

		statusCodeDist: statusCodeDist,
		totalErrors:    totalErrors,
		elapsed:        elapsed,

		N:     N,
		C:     C,
		Z:     Z,
		QPS:   QPS,
		ctxOK: ctx.Err() == nil,
	}

	runReport(&newReport)
}
