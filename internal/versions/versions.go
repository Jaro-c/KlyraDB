package versions

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type eolEntry struct {
	Cycle string `json:"cycle"`
}

var (
	mu    sync.Mutex
	cache = map[string][]string{}
)

// FetchLatest returns the n most recent major version cycles for the given
// endoflife.date product, sorted descending by version number.
// Falls back to fallback if the fetch fails or returns no results.
func FetchLatest(product string, n int, fallback []string) []string {
	mu.Lock()
	v, hit := cache[product]
	mu.Unlock()
	if hit {
		return take(v, n)
	}

	v = fetchCycles(product)
	if len(v) == 0 {
		return fallback
	}
	sortDesc(v)

	mu.Lock()
	cache[product] = v
	mu.Unlock()
	return take(v, n)
}

func fetchCycles(product string) []string {
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Get("https://endoflife.date/api/" + product + ".json")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var entries []eolEntry
	if json.NewDecoder(resp.Body).Decode(&entries) != nil {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Cycle != "" {
			out = append(out, e.Cycle)
		}
	}
	return out
}

func take(v []string, n int) []string {
	if n >= len(v) {
		return v
	}
	return v[:n]
}

func sortDesc(v []string) {
	sort.Slice(v, func(i, j int) bool { return versionGT(v[i], v[j]) })
}

// versionGT compares version strings segment-by-segment as integers.
// Handles "10.11" > "10.6", "9.3" > "8.4", etc.
func versionGT(a, b string) bool {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		ai, _ := strconv.Atoi(ap[i])
		bi, _ := strconv.Atoi(bp[i])
		if ai != bi {
			return ai > bi
		}
	}
	return len(ap) > len(bp)
}
