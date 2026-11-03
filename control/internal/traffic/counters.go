package traffic

import (
	"sort"
	"time"
)

// Monotonic counters for the Prometheus endpoint. Unlike the time-series buckets
// they only ever increase (until the process restarts, which Prometheus' rate()
// handles as a counter reset). Host labels reuse the collector's host cap
// (maxHosts + "(other)"), so cardinality stays bounded.

// BlockKinds are the label values of corvus_blocked_total{kind}.
var BlockKinds = []string{"exploit", "rule", "body", "rate_limit"}

var blockKindOf = map[string]string{
	"blocked_exploit": "exploit",
	"blocked_rule":    "rule",
	"blocked_body":    "body",
	"rate_limited":    "rate_limit",
}

type hostCounters struct {
	class   [4]uint64 // 2xx(and below) / 3xx / 4xx / 5xx
	blocked [4]uint64 // indexed like BlockKinds
}

// count updates the cumulative counters; caller holds c.mu. r.H is already capped.
func (c *Collector) count(r *Record, kinds []string) {
	if c.counters == nil {
		c.counters = map[string]*hostCounters{}
	}
	hc := c.counters[r.H]
	if hc == nil {
		hc = &hostCounters{}
		c.counters[r.H] = hc
	}
	switch {
	case r.S >= 500:
		hc.class[3]++
	case r.S >= 400:
		hc.class[2]++
	case r.S >= 300:
		hc.class[1]++
	default:
		hc.class[0]++
	}
	for _, k := range kinds {
		bk, ok := blockKindOf[k]
		if !ok {
			continue
		}
		for i, name := range BlockKinds {
			if name == bk {
				hc.blocked[i]++
			}
		}
	}
}

// HostCounters is a snapshot of one host's cumulative counters.
type HostCounters struct {
	Host    string
	Class   map[string]uint64 // "2xx".."5xx"
	Blocked map[string]uint64 // BlockKinds
}

// Counters returns the cumulative counters per host, sorted by host.
func (c *Collector) Counters() []HostCounters {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]HostCounters, 0, len(c.counters))
	for h, hc := range c.counters {
		x := HostCounters{Host: h, Class: map[string]uint64{}, Blocked: map[string]uint64{}}
		for i, cl := range []string{"2xx", "3xx", "4xx", "5xx"} {
			x.Class[cl] = hc.class[i]
		}
		for i, k := range BlockKinds {
			x.Blocked[k] = hc.blocked[i]
		}
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}

// HostLatency is a host's latency quantiles (seconds) over a recent window.
type HostLatency struct {
	Host          string
	Count         int64
	P50, P95, P99 float64
}

// Latency returns per-host latency quantiles over the last d (10 s resolution),
// only for hosts with requests in the window.
func (c *Collector) Latency(d time.Duration, now time.Time) []HostLatency {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := []HostLatency{}
	for h, s := range c.hosts {
		_, bs := s.buckets(now.Unix(), d)
		var agg bucket
		for _, b := range bs {
			agg.merge(b)
		}
		if agg.count == 0 {
			continue
		}
		out = append(out, HostLatency{Host: h, Count: agg.count, P50: agg.quantile(.5) / 1000, P95: agg.quantile(.95) / 1000, P99: agg.quantile(.99) / 1000})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Host < out[j].Host })
	return out
}
