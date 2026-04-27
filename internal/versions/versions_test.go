package versions

import (
	"sync"
	"testing"
)

func TestVersionGT(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"10.11", "10.6", true},
		{"9.3", "8.4", true},
		{"8.0", "9.3", false},
		{"1.0", "1.0", false},
		{"10", "9", true},
		{"18", "17", true},
		{"7.4", "7.2", true},
	}
	for _, c := range cases {
		if got := versionGT(c.a, c.b); got != c.want {
			t.Errorf("versionGT(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSortDesc(t *testing.T) {
	v := []string{"8.0", "10.11", "9.3", "10.6"}
	sortDesc(v)
	want := []string{"10.11", "10.6", "9.3", "8.0"}
	for i, w := range want {
		if v[i] != w {
			t.Errorf("sortDesc[%d] = %q, want %q", i, v[i], w)
		}
	}
}

func TestTake(t *testing.T) {
	v := []string{"a", "b", "c", "d"}
	if got := take(v, 2); len(got) != 2 {
		t.Errorf("take(4, 2) len = %d, want 2", len(got))
	}
	if got := take(v, 10); len(got) != 4 {
		t.Errorf("take(4, 10) len = %d, want 4", len(got))
	}
	if got := take(v, 0); len(got) != 0 {
		t.Errorf("take(4, 0) len = %d, want 0", len(got))
	}
}

func TestMajorKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"8.6", "8"},
		{"10.11.6", "10"},
		{"18", "18"},
		{"9.7", "9"},
	}
	for _, c := range cases {
		if got := majorKey(c.in); got != c.want {
			t.Errorf("majorKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDedupByMajor(t *testing.T) {
	// Sorted descending input
	v := []string{"8.6", "8.4", "8.2", "8.0", "7.4", "7.2", "6.2"}
	got := dedupByMajor(v)
	want := []string{"8.6", "7.4", "6.2"}
	if len(got) != len(want) {
		t.Fatalf("dedupByMajor len = %d, want %d: %v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("dedupByMajor[%d] = %q, want %q", i, got[i], w)
		}
	}
}

func TestMajorMatch(t *testing.T) {
	cases := []struct {
		installed, cycle string
		want             bool
	}{
		{"8.0.36", "8.4", true},
		{"7.4.0", "8.4", false},
		{"10.11.6", "10.11", true},
		{"8.0", "8.2", true},
		{"", "8.6", false},
		{"8.6", "", false},
	}
	for _, c := range cases {
		if got := MajorMatch(c.installed, c.cycle); got != c.want {
			t.Errorf("MajorMatch(%q, %q) = %v, want %v", c.installed, c.cycle, got, c.want)
		}
	}
}

func TestFetchLatest_Cache(t *testing.T) {
	mu.Lock()
	cache["_test_cached"] = []string{"3.0", "2.0", "1.0"}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		delete(cache, "_test_cached")
		mu.Unlock()
	})

	got := FetchLatest("_test_cached", 2, []string{"0.1"})
	if len(got) != 2 || got[0] != "3.0" || got[1] != "2.0" {
		t.Errorf("FetchLatest cache: got %v", got)
	}
}

func TestFetchLatest_Fallback(t *testing.T) {
	mu.Lock()
	delete(cache, "_test_nonexistent_xyz_abc")
	mu.Unlock()

	fallback := []string{"9.9", "8.8"}
	got := FetchLatest("_test_nonexistent_xyz_abc", 2, fallback)
	if len(got) == 0 {
		t.Error("FetchLatest returned empty for nonexistent product")
	}
}

func TestFetchLatest_Limit(t *testing.T) {
	mu.Lock()
	cache["_test_limit"] = []string{"5.0", "4.0", "3.0", "2.0", "1.0"}
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		delete(cache, "_test_limit")
		mu.Unlock()
	})

	got := FetchLatest("_test_limit", 3, nil)
	if len(got) != 3 {
		t.Errorf("FetchLatest limit: got %d items, want 3", len(got))
	}
}

func TestFetchLatest_ConcurrentSafe(t *testing.T) {
	fallback := []string{"1.0"}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			FetchLatest("_test_concurrent", 2, fallback)
		}()
	}
	wg.Wait()
	mu.Lock()
	delete(cache, "_test_concurrent")
	mu.Unlock()
}
