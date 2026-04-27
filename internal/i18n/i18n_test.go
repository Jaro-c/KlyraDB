package i18n

import (
	"testing"
	"testing/fstest"
)

func resetCatalogs() {
	for k := range catalogs {
		delete(catalogs, k)
	}
}

func loadTestFS(t *testing.T) {
	t.Helper()
	resetCatalogs()
	fsys := fstest.MapFS{
		"locales/en.json": {Data: []byte(`{"greeting":"Hello","only.en":"English only"}`)},
		"locales/es.json": {Data: []byte(`{"greeting":"Hola"}`)},
		"locales/ar.json": {Data: []byte(`{"greeting":"مرحبا"}`)},
		"locales/pt-br.json": {Data: []byte(`{"greeting":"Olá"}`)},
	}
	if err := Load(fsys, "locales"); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestLoad_registersLocales(t *testing.T) {
	loadTestFS(t)
	avail := Available()
	want := map[string]bool{"en": true, "es": true, "ar": true, "pt-br": true}
	for _, code := range avail {
		if !want[code] {
			t.Errorf("unexpected locale %s", code)
		}
		delete(want, code)
	}
	for code := range want {
		t.Errorf("missing locale %s", code)
	}
}

func TestLoad_invalidJSON(t *testing.T) {
	resetCatalogs()
	fsys := fstest.MapFS{
		"locales/bad.json": {Data: []byte(`not json`)},
	}
	if err := Load(fsys, "locales"); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoad_missingDir(t *testing.T) {
	resetCatalogs()
	fsys := fstest.MapFS{}
	if err := Load(fsys, "no-such-dir"); err == nil {
		t.Error("expected error for missing directory")
	}
}

func TestFor_exactMatch(t *testing.T) {
	loadTestFS(t)
	l := For("es")
	if l.T("greeting") != "Hola" {
		t.Errorf("expected Hola, got %s", l.T("greeting"))
	}
}

func TestFor_regionStrip(t *testing.T) {
	loadTestFS(t)
	// pt-BR not in catalog, should strip to pt → not found → fallback en
	l := For("pt-br")
	if l.T("greeting") != "Olá" {
		t.Errorf("expected Olá for pt-br, got %s", l.T("greeting"))
	}
}

func TestFor_fallbackToEnglish(t *testing.T) {
	loadTestFS(t)
	l := For("zh")
	if l.T("greeting") != "Hello" {
		t.Errorf("expected English fallback, got %s", l.T("greeting"))
	}
}

func TestFor_unknownKey_returnsKey(t *testing.T) {
	loadTestFS(t)
	l := For("es")
	if l.T("no.such.key") != "no.such.key" {
		t.Errorf("expected key passthrough, got %s", l.T("no.such.key"))
	}
}

func TestFor_missingKeyFallsBackToEnglish(t *testing.T) {
	loadTestFS(t)
	l := For("es")
	if l.T("only.en") != "English only" {
		t.Errorf("expected English fallback for missing key, got %s", l.T("only.en"))
	}
}

func TestRTL_arabic(t *testing.T) {
	loadTestFS(t)
	l := For("ar")
	if l.Dir != "rtl" {
		t.Errorf("expected rtl for ar, got %s", l.Dir)
	}
}

func TestLTR_spanish(t *testing.T) {
	loadTestFS(t)
	l := For("es")
	if l.Dir != "ltr" {
		t.Errorf("expected ltr for es, got %s", l.Dir)
	}
}

func TestAll_mergesWithEnglishFallback(t *testing.T) {
	loadTestFS(t)
	l := For("es")
	all := l.All()
	if all["greeting"] != "Hola" {
		t.Errorf("expected translated value, got %s", all["greeting"])
	}
	if all["only.en"] != "English only" {
		t.Errorf("expected English fallback in All(), got %s", all["only.en"])
	}
}

func TestDetect_lcAll(t *testing.T) {
	loadTestFS(t)
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "")
	l := Detect()
	if l.T("greeting") != "Hola" {
		t.Errorf("expected Hola via LC_ALL, got %s", l.T("greeting"))
	}
}

func TestDetect_langFallback(t *testing.T) {
	loadTestFS(t)
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "ar_SA.UTF-8")
	l := Detect()
	if l.Code != "ar" {
		t.Errorf("expected ar via LANG, got %s", l.Code)
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"pt_BR.UTF-8", "pt-br"},
		{"es_ES.UTF-8", "es-es"},
		{"C", "en"},
		{"POSIX", "en"},
		{"", "en"},
		{"ar@euro", "ar"},
	}
	for _, c := range cases {
		got := normalize(c.in)
		if got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
