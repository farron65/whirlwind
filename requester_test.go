package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetPercentile(t *testing.T) {
	tests := []struct {
		name      string
		p         float64
		latencies []time.Duration
		want      time.Duration
	}{
		{
			name:      "p50 on five sorted values",
			p:         50,
			latencies: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
			want:      30 * time.Millisecond,
		},
		{
			name:      "p0 returns first element",
			p:         0,
			latencies: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond},
			want:      10 * time.Millisecond,
		},
		{
			name:      "p99 near end of small slice",
			p:         99,
			latencies: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
			want:      50 * time.Millisecond,
		},
		{
			name:      "single element slice",
			p:         50,
			latencies: []time.Duration{100 * time.Millisecond},
			want:      100 * time.Millisecond,
		},
		{
			name:      "p90 on ten sorted values",
			p:         90,
			latencies: []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			want:      10,
		},
		{
			name:      "p100 returns last element",
			p:         100,
			latencies: []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			want:      10,
		},
		{
			name:      "empty slice returns zero",
			p:         50,
			latencies: []time.Duration{},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPercentile(tt.p, tt.latencies)
			if got != tt.want {
				t.Errorf("getPercentile(%v, %v) = %v, want %v", tt.p, tt.latencies, got, tt.want)
			}
		})
	}
}

func TestMakeRequest_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	res := makeRequest(context.Background(), client, srv.URL)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want %d", res.statusCode, http.StatusOK)
	}
	if res.duration <= 0 {
		t.Errorf("duration = %v, want > 0", res.duration)
	}
}

func TestMakeRequest_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	res := makeRequest(context.Background(), client, srv.URL)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.statusCode != http.StatusInternalServerError {
		t.Errorf("statusCode = %d, want %d", res.statusCode, http.StatusInternalServerError)
	}
}

func TestMakeRequest_ServerUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	res := makeRequest(context.Background(), client, srv.URL)

	if res.err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
	if res.statusCode != 0 {
		t.Errorf("statusCode = %d, want 0", res.statusCode)
	}
}

func TestMakeRequest_SlowServer(t *testing.T) {
	delay := 100 * time.Millisecond
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 2 * time.Second}
	res := makeRequest(context.Background(), client, srv.URL)

	if res.err != nil {
		t.Fatalf("unexpected error: %v", res.err)
	}
	if res.duration < delay {
		t.Errorf("duration = %v, want at least %v", res.duration, delay)
	}
}

func TestMakeRequest_BadURL(t *testing.T) {
	client := &http.Client{Timeout: 2 * time.Second}
	res := makeRequest(context.Background(), client, "://bad-url")

	if res.err == nil {
		t.Fatal("expected error for malformed URL, got nil")
	}
}
