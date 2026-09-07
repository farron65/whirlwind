package main

import (
	"context"
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
	duration     time.Duration
	statusCode   int
	err          error
	connDuration time.Duration
	reqDuration  time.Duration
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
			} else {
				connDuration = -1
			}
			reqStart = time.Now()
		},
		WroteRequest: func(w httptrace.WroteRequestInfo) {
			if reqStart.IsZero() {
				reqDuration = -1
			} else {
				reqDuration = time.Since(reqStart)
			}
		},
	}

	req, err := http.NewRequest("GET", url, nil)
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
		duration:     time.Since(now),
		statusCode:   resp.StatusCode,
		err:          nil,
		connDuration: connDuration,
		reqDuration:  reqDuration,
	}
}

func getPercentile(p float64, latencies []time.Duration) time.Duration {
	percentile := math.Floor(p / 100 * float64(len(latencies)))
	return latencies[int(percentile)]
}

func main() {
	flags := parseFlags()

	N := flags.N
	C := flags.C
	Z := flags.Z
	QPS := flags.QPS

	url := "https://trenden.netlify.app/"

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
		url:            url,
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
