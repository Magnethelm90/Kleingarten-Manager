package main

import (
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestAlleAdminRoutenGeschuetzt liest alle registrierten /api/admin-Routen aus api.go und prüft, dass keine davon
// ohne Anmeldung erreichbar ist und der Kassenprüfer nur die ausdrücklich freigegebenen erreicht.
func TestAlleAdminRoutenGeschuetzt(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`HandleFunc\("([A-Z]+) (/api/admin/[^"]+)",\s*a\.(admin|adminOrPruef)\(`)
	treffer := re.FindAllStringSubmatch(string(src), -1)
	if len(treffer) < 100 {
		t.Fatalf("nur %d Admin-Routen gefunden – Muster prüfen", len(treffer))
	}
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]string{"new": "admingeheim1"})
	var adminCookie, pruefCookie *http.Cookie
	for _, c := range do(h, "POST", "/api/admin/login", map[string]string{"password": "admingeheim1"}).Result().Cookies() {
		if c.Name == "ga_session" {
			adminCookie = c
		}
	}
	do(h, "POST", "/api/admin/pruef-password", map[string]string{"new": "pruefgeheim1"}, adminCookie)
	for _, c := range do(h, "POST", "/api/pruef/login", map[string]string{"password": "pruefgeheim1"}).Result().Cookies() {
		if c.Name == "ga_pruef_session" {
			pruefCookie = c
		}
	}
	if adminCookie == nil || pruefCookie == nil {
		t.Fatal("Anmeldung fehlgeschlagen")
	}
	platzhalter := regexp.MustCompile(`\{[^}]+\}`)
	fuerPruefer := 0
	for _, m := range treffer {
		methode, pfad := m[1], platzhalter.ReplaceAllString(m[2], "x")
		if rec := do(h, methode, pfad, map[string]any{}); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s ohne Anmeldung: %d", methode, pfad, rec.Code)
		}
		if m[3] == "adminOrPruef" {
			fuerPruefer++
			continue
		}
		if rec := do(h, methode, pfad, map[string]any{}, pruefCookie); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
			t.Errorf("%s %s als Kassenprüfer: %d", methode, pfad, rec.Code)
		}
	}
	// Prüfer-Freigaben dürfen nur lesen bzw. prüfen – nichts an Pächtern, Einstellungen oder Sicherungen
	for _, m := range treffer {
		if m[3] == "adminOrPruef" && (strings.Contains(m[2], "paechter") || strings.Contains(m[2], "settings") || strings.Contains(m[2], "backup") || strings.Contains(m[2], "password")) {
			t.Errorf("Prüfer-Freigabe zu weit: %s %s", m[1], m[2])
		}
	}
	t.Logf("%d Admin-Routen geprüft, davon %d für Kassenprüfer freigegeben", len(treffer), fuerPruefer)
}

// TestAlleLeseRoutenOhneFehler ruft jede lesende Admin-Route ohne Pfadparameter mit Beispieldaten auf – bei allen
// Bereichen an, bei allen aus und bei jedem Bereich einzeln. Erwartet wird nie ein Serverfehler (5xx).
func TestAlleLeseRoutenOhneFehler(t *testing.T) {
	src, err := os.ReadFile("api.go")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`HandleFunc\("GET (/api/admin/[^"{}]+)",`)
	treffer := re.FindAllStringSubmatch(string(src), -1)
	if len(treffer) < 20 {
		t.Fatalf("nur %d Lese-Routen gefunden", len(treffer))
	}
	_, h := newTestApp(t)
	if rec := do(h, "POST", "/api/admin/beispieldaten", map[string]any{}); rec.Code >= 300 {
		t.Fatalf("Beispieldaten: %d %s", rec.Code, rec.Body.String())
	}
	konfig := []map[string]bool{{}}
	alleAn, alleAus := map[string]bool{}, map[string]bool{}
	for _, m := range module {
		alleAn[m], alleAus[m] = true, false
	}
	konfig = append(konfig, alleAn, alleAus)
	for _, m := range module {
		k := map[string]bool{}
		for _, x := range module {
			k[x] = x == m
		}
		konfig = append(konfig, k)
	}
	for i, k := range konfig {
		if len(k) > 0 {
			if rec := do(h, "PUT", "/api/admin/module", k); rec.Code != 200 {
				t.Fatalf("Module setzen: %d", rec.Code)
			}
		}
		for _, m := range treffer {
			rec := do(h, "GET", m[1], nil)
			if rec.Code >= 500 {
				t.Errorf("Konfiguration %d: GET %s -> %d %s", i, m[1], rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), "NaN") || strings.Contains(rec.Body.String(), "+Inf") {
				t.Errorf("Konfiguration %d: GET %s enthält NaN/Inf", i, m[1])
			}
		}
	}
}
