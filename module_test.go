package main

import (
	"os"
	"strings"
	"testing"
)

func TestModule(t *testing.T) {
	app, h := newTestApp(t)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if !st.Module["warteliste"] || st.Module["barkasse"] || st.Module["bauten"] {
		t.Fatalf("neue Installation soll mit dem Grundpaket starten: %v", st.Module)
	}

	// Ein überdimensioniertes Bauwerk erzeugt nur bei eingeschaltetem Laubenkataster einen Hinweis.
	if rec := do(h, "POST", "/api/admin/bauten", Bauwerk{Gartennr: "1", Art: "Laube", Flaeche: 30}); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	hatLaube := func() bool {
		for _, x := range decode[[]aufgabe](t, do(h, "GET", "/api/admin/aufgaben", nil)) {
			if strings.Contains(x.Titel, "24 m²") {
				return true
			}
		}
		return false
	}
	if hatLaube() {
		t.Fatal("ausgeblendeter Bereich darf im Cockpit nicht erscheinen")
	}
	m := decode[map[string]bool](t, do(h, "PUT", "/api/admin/module", map[string]bool{"bauten": true, "warteliste": false}))
	if !m["bauten"] || m["warteliste"] || m["barkasse"] {
		t.Fatalf("Umschalten falsch: %v", m)
	}
	if !hatLaube() {
		t.Fatal("eingeschalteter Laubenkataster muss warnen")
	}
	// Daten bleiben beim Ausblenden erhalten
	do(h, "PUT", "/api/admin/module", map[string]bool{"bauten": false})
	app.st.mu.Lock()
	n := len(app.st.d.Bauwerke)
	app.st.mu.Unlock()
	if n != 1 {
		t.Fatal("Bauwerk verloren")
	}

	// Beispieldaten zeigen alle Bereiche
	_, h2 := newTestApp(t)
	if rec := do(h2, "POST", "/api/admin/beispieldaten", nil); rec.Code != 200 {
		t.Fatal(rec.Body.String())
	}
	for k, an := range decode[stateResp](t, do(h2, "GET", "/api/state", nil)).Module {
		if !an {
			t.Fatalf("Beispieldaten: %s ausgeblendet", k)
		}
	}

	// Bestehende Datenbestände ohne Liste zeigen alles.
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/"+dataFileName, []byte(`{"version":1,"settings":{"jahr":2026}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	for k, an := range s.moduleLocked() {
		if !an {
			t.Fatalf("alter Datenbestand: %s ausgeblendet", k)
		}
	}
}
