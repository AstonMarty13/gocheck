package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoadSites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sites.txt")
	content := "https://example.com\n\n# a comment\n  https://example.org  \n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}

	urls, err := LoadSites(path)
	if err != nil {
		t.Fatalf("LoadSites: %v", err)
	}

	want := []string{"https://example.com", "https://example.org"}
	if len(urls) != len(want) {
		t.Fatalf("got %d urls (%v), want %d", len(urls), urls, len(want))
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Errorf("url %d = %q, want %q", i, urls[i], want[i])
		}
	}
}

func TestLoadSitesMissingFile(t *testing.T) {
	if _, err := LoadSites(filepath.Join(t.TempDir(), "nope.txt")); err == nil {
		t.Fatal("expected an error for a missing file, got nil")
	}
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantUp     bool
	}{
		{"200 is up", http.StatusOK, true},
		{"301 is up", http.StatusMovedPermanently, true},
		{"399 is up", 399, true},
		{"404 is down", http.StatusNotFound, false},
		{"500 is down", http.StatusInternalServerError, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
			}))
			defer srv.Close()

			m := New([]string{srv.URL}, time.Minute, time.Second)
			m.Check(context.Background(), srv.URL)

			got := m.Snapshot()
			if len(got) != 1 {
				t.Fatalf("snapshot has %d entries, want 1", len(got))
			}
			if got[0].Up != tc.wantUp {
				t.Errorf("Up = %v, want %v", got[0].Up, tc.wantUp)
			}
			// A 3xx is followed by the client, so only assert the code when
			// no redirect was involved.
			if tc.statusCode < 300 || tc.statusCode >= 400 {
				if got[0].StatusCode != tc.statusCode {
					t.Errorf("StatusCode = %d, want %d", got[0].StatusCode, tc.statusCode)
				}
			}
			if got[0].LastChecked.IsZero() {
				t.Error("LastChecked was not set")
			}
		})
	}
}

// TestCheckSendsUserAgent guards against the false-DOWN bug: hosts such as
// wikipedia.org reject Go's default user agent with a 403.
func TestCheckSendsUserAgent(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	m := New([]string{srv.URL}, time.Minute, time.Second)
	m.Check(context.Background(), srv.URL)

	ua := <-got
	if ua != userAgent {
		t.Errorf("User-Agent = %q, want %q", ua, userAgent)
	}
	if strings.HasPrefix(ua, "Go-http-client") {
		t.Error("still sending Go's default user agent")
	}
}

func TestCheckUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening any more

	m := New([]string{url}, time.Minute, time.Second)
	m.Check(context.Background(), url)

	got := m.Snapshot()[0]
	if got.Up {
		t.Error("Up = true for an unreachable host, want false")
	}
	if got.StatusCode != 0 {
		t.Errorf("StatusCode = %d for an unreachable host, want 0", got.StatusCode)
	}
}

// TestCheckTimeout covers the bug this package exists to avoid: without a
// client timeout a hanging site would block its polling goroutine forever.
func TestCheckTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer func() {
		close(release)
		srv.Close()
	}()

	m := New([]string{srv.URL}, time.Minute, 50*time.Millisecond)

	done := make(chan struct{})
	go func() {
		m.Check(context.Background(), srv.URL)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Check did not return: the HTTP client is missing a timeout")
	}

	if m.Snapshot()[0].Up {
		t.Error("Up = true after a timeout, want false")
	}
}

func TestCheckUnknownURLIsIgnored(t *testing.T) {
	m := New([]string{"https://example.com"}, time.Minute, time.Second)
	m.Check(context.Background(), "https://not-monitored.example")

	if n := len(m.Snapshot()); n != 1 {
		t.Errorf("snapshot has %d entries after checking an unknown URL, want 1", n)
	}
}

func TestSnapshotIsSortedAndCopied(t *testing.T) {
	m := New([]string{"https://c.example", "https://a.example", "https://b.example"}, time.Minute, time.Second)

	got := m.Snapshot()
	want := []string{"https://a.example", "https://b.example", "https://c.example"}
	for i := range want {
		if got[i].URL != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i].URL, want[i])
		}
	}

	// Mutating the snapshot must not affect the monitor's own state.
	got[0].Up = true
	if m.Snapshot()[0].Up {
		t.Error("Snapshot returned a view into internal state, want a copy")
	}
}

// TestConcurrentAccess is the reason CI runs with -race: checks and dashboard
// reads happen simultaneously in production.
func TestConcurrentAccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	m := New([]string{srv.URL}, time.Minute, time.Second)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			m.Check(context.Background(), srv.URL)
		}()
		go func() {
			defer wg.Done()
			_ = m.Snapshot()
		}()
	}
	wg.Wait()
}

func TestStartStopsOnContextCancel(t *testing.T) {
	var hits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	m := New([]string{srv.URL}, 10*time.Millisecond, time.Second)
	m.Start(ctx)

	time.Sleep(60 * time.Millisecond)
	cancel()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	afterCancel := hits
	mu.Unlock()

	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	final := hits
	mu.Unlock()

	if final != afterCancel {
		t.Errorf("polling continued after cancel: %d hits then %d", afterCancel, final)
	}
	if afterCancel == 0 {
		t.Error("no checks ran at all before cancel")
	}
}
