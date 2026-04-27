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
	Cycle  string `json:"cycle"`
	Latest string `json:"latest"`
}

var (
	mu    sync.Mutex
	cache = map[string][]string{}
)

// FetchLatest returns the n most recent MAJOR version families for the given
// endoflife.date product. Only the latest release within each major is kept
// (e.g., Redis cycles 8.6, 8.4, 8.2 → one entry: 8.6).
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
	v = dedupByMajor(v)

	mu.Lock()
	cache[product] = v
	mu.Unlock()
	return take(v, n)
}

// MajorMatch reports whether an installed version belongs to the same major
// family as a cycle name returned by FetchLatest.
// Examples: MajorMatch("8.0.36", "8.4") → true (both major 8)
//
//	MajorMatch("7.4.0", "8.4") → false
func MajorMatch(installedVer, cycle string) bool {
	if installedVer == "" || cycle == "" {
		return false
	}
	return majorKey(installedVer) == majorKey(cycle)
}

// MajorKey returns the first "."-separated segment of a version string.
// "18.3" → "18", "10.11.6" → "10", "8.6.2" → "8"
func MajorKey(v string) string {
	return majorKey(v)
}

// majorKey is the unexported implementation used internally.
func majorKey(v string) string {
	if i := strings.Index(v, "."); i != -1 {
		return v[:i]
	}
	return v
}

// dedupByMajor keeps only the highest cycle per major version family.
// Input must be pre-sorted descending so the first occurrence is the latest.
func dedupByMajor(v []string) []string {
	seen := make(map[string]bool, len(v))
	out := make([]string, 0, len(v))
	for _, cycle := range v {
		mk := majorKey(cycle)
		if !seen[mk] {
			seen[mk] = true
			out = append(out, cycle)
		}
	}
	return out
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
		if e.Latest != "" {
			out = append(out, e.Latest)
		} else if e.Cycle != "" {
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
