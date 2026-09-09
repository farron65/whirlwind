package main

import (
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