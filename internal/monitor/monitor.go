// Package monitor polls a list of URLs and keeps their latest status in memory.
package monitor

import (
	"bufio"
	"context"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// userAgent identifies gocheck to the sites it polls. Without it Go sends
// "Go-http-client/...", which some hosts (wikipedia.org among them) answer
// with 403 — a false DOWN.
const userAgent = "gocheck/1.0 (+https://github.com/AstonMarty13/gocheck)"

// Status is the outcome of the most recent check for a single URL.
type Status struct {
	URL         string
	Up          bool
	StatusCode  int
	ResponseMS  int64
	LastChecked time.Time
}

// Monitor polls a fixed set of URLs on an interval and stores the latest
// Status for each. It is safe for concurrent use.
type Monitor struct {
	client   *http.Client
	interval time.Duration
	urls     []string

	mu       sync.RWMutex
	statuses map[string]*Status
}

// New returns a Monitor for the given URLs. timeout bounds each individual
// HTTP request; interval is the delay between two checks of the same URL.
func New(urls []string, interval, timeout time.Duration) *Monitor {
	m := &Monitor{
		client:   &http.Client{Timeout: timeout},
		interval: interval,
		urls:     append([]string(nil), urls...),
		statuses: make(map[string]*Status, len(urls)),
	}
	for _, u := range m.urls {
		m.statuses[u] = &Status{URL: u}
	}
	return m
}

// Start launches one polling goroutine per URL and returns immediately.
// All goroutines stop when ctx is cancelled.
func (m *Monitor) Start(ctx context.Context) {
	for _, u := range m.urls {
		go m.poll(ctx, u)
	}
}

func (m *Monitor) poll(ctx context.Context, url string) {
	m.Check(ctx, url)

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.Check(ctx, url)
		}
	}
}

// Check performs a single HTTP GET against url and records the result.
// Unknown URLs are ignored.
func (m *Monitor) Check(ctx context.Context, url string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		m.record(url, false, 0, 0)
		return
	}
	req.Header.Set("User-Agent", userAgent)

	start := time.Now()
	resp, err := m.client.Do(req)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		m.record(url, false, 0, elapsed)
		return
	}
	resp.Body.Close()
	m.record(url, resp.StatusCode < 400, resp.StatusCode, elapsed)
}

func (m *Monitor) record(url string, up bool, code int, elapsedMS int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.statuses[url]
	if !ok {
		return
	}
	s.Up = up
	s.StatusCode = code
	s.ResponseMS = elapsedMS
	s.LastChecked = time.Now()
}

// Snapshot returns a copy of every Status, sorted by URL. Callers get values,
// not pointers, so they can read them without holding the lock.
func (m *Monitor) Snapshot() []Status {
	m.mu.RLock()
	out := make([]Status, 0, len(m.statuses))
	for _, s := range m.statuses {
		out = append(out, *s)
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

// LoadSites reads one URL per line, skipping blank lines and # comments.
func LoadSites(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var urls []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}
	return urls, scanner.Err()
}
