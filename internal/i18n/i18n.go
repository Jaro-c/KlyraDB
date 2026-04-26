package i18n

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"strings"
)

type Lang struct {
	Code    string
	Dir     string
	strings map[string]string
}

var (
	catalogs  = map[string]map[string]string{}
	rtl       = map[string]bool{"ar": true, "he": true, "fa": true, "ur": true}
	fallback  = "en"
)

// Load reads every *.json file from the provided filesystem at the given dir
// and registers it as a locale keyed by the filename (without extension).
func Load(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		var m map[string]string
		if err := json.Unmarshal(b, &m); err != nil {
			return err
		}
		code := strings.TrimSuffix(e.Name(), ".json")
		catalogs[strings.ToLower(code)] = m
	}
	return nil
}

// Detect resolves the best matching locale based on env vars.
// Priority: LC_ALL > LC_MESSAGES > LANG. Falls back to "en".
// Lookup tries full code (e.g. "pt-br") then language only ("pt") then fallback.
func Detect() *Lang {
	code := codeFromEnv()

	// 1) full match
	if m, ok := catalogs[code]; ok {
		return build(code, m)
	}
	// 2) strip region: "pt-br" → "pt"
	if i := strings.Index(code, "-"); i > 0 {
		base := code[:i]
		if m, ok := catalogs[base]; ok {
			return build(base, m)
		}
	}
	// 3) fallback
	return build(fallback, catalogs[fallback])
}

func build(code string, m map[string]string) *Lang {
	dir := "ltr"
	short := code
	if i := strings.Index(code, "-"); i > 0 {
		short = code[:i]
	}
	if rtl[short] {
		dir = "rtl"
	}
	return &Lang{Code: code, Dir: dir, strings: m}
}

func (l *Lang) T(key string) string {
	if v, ok := l.strings[key]; ok && v != "" {
		return v
	}
	if v, ok := catalogs[fallback][key]; ok {
		return v
	}
	return key
}

func (l *Lang) All() map[string]string {
	out := make(map[string]string, len(l.strings))
	for k, v := range catalogs[fallback] {
		out[k] = v
	}
	for k, v := range l.strings {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// Available returns the list of registered locale codes.
func Available() []string {
	out := make([]string, 0, len(catalogs))
	for k := range catalogs {
		out = append(out, k)
	}
	return out
}

func codeFromEnv() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if raw := os.Getenv(v); raw != "" {
			return normalize(raw)
		}
	}
	return fallback
}

func normalize(raw string) string {
	raw = strings.ToLower(raw)
	if i := strings.IndexAny(raw, ".@"); i > 0 {
		raw = raw[:i]
	}
	raw = strings.ReplaceAll(raw, "_", "-")
	if raw == "c" || raw == "posix" || raw == "" {
		return fallback
	}
	return raw
}
