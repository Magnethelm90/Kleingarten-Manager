package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// datenJSON liefert den kompletten Datenbestand ohne Bereichsliste und Änderungsprotokoll (beide ändern sich hier absichtlich).
func datenJSON(t *testing.T, a *App) string {
	t.Helper()
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	kopie := *a.st.d
	kopie.AusModule = nil
	kopie.AuditLog = nil // wächst bei jeder Änderung absichtlich
	raw, err := json.Marshal(&kopie)
	if err != nil {
		t.Fatal(err)
	}
	// wie beim Laden normalisieren (leere Listen statt null), damit nur echte Unterschiede auffallen
	d, err := parseDaten(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(d)
	return string(raw)
}

// TestKeinDatenverlust prüft mit dem vollen Beispielbestand (alle Bereiche befüllt), dass kein Feld verloren geht:
// beim Aus- und Einschalten aller Bereiche, beim Neustart von der Platte, bei Sicherung und Wiederherstellung
// und beim Lesen einer alten Datendatei ohne Bereichsliste.
func TestKeinDatenverlust(t *testing.T) {
	dir := t.TempDir()
	st, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{st: st, port: testPort, sessions: map[string]time.Time{}}
	h := a.routes(http.NotFoundHandler())
	if rec := do(h, "POST", "/api/admin/beispieldaten", map[string]any{}); rec.Code >= 300 {
		t.Fatalf("Beispieldaten: %d", rec.Code)
	}
	// ein Haken im Vereinsjahr, damit auch dieses neue Feld mitgeprüft wird
	a.st.mu.Lock()
	a.st.d.Vereinsjahr = map[string][]string{"2026": {"versammlung"}}
	_ = a.st.saveLocked()
	a.st.mu.Unlock()
	original := datenJSON(t, a)
	if len(original) < 20000 {
		t.Fatalf("Beispielbestand zu klein (%d Bytes) – Test wäre nicht aussagekräftig", len(original))
	}

	// 1. Alle Bereiche aus und wieder an: Daten unverändert
	aus, an := map[string]bool{}, map[string]bool{}
	for _, m := range module {
		aus[m], an[m] = false, true
	}
	do(h, "PUT", "/api/admin/module", aus)
	if got := datenJSON(t, a); got != original {
		t.Fatal("Ausblenden der Bereiche hat Daten verändert: " + unterschied(original, got))
	}
	do(h, "PUT", "/api/admin/module", an)
	if got := datenJSON(t, a); got != original {
		t.Fatal("Einblenden der Bereiche hat Daten verändert: " + unterschied(original, got))
	}

	// 2. Neustart: alles wieder von der Platte lesen
	st2, err := openStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a2 := &App{st: st2, port: testPort, sessions: map[string]time.Time{}}
	if got := datenJSON(t, a2); got != original {
		t.Fatal("nach dem Neustart weicht der Bestand ab – ein Feld wird nicht gespeichert: " + unterschied(original, got))
	}

	// 3. Sicherung anlegen, alles ändern, Sicherung zurückspielen
	a.st.mu.Lock()
	a.st.snapshotBackup("datenerhalt")
	a.st.mu.Unlock()
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "999", "name": "Später", "gartengroesse": 100})
	var name string
	for _, n := range a.st.sicherungsNamen() {
		if strings.Contains(n, "datenerhalt") {
			name = n
		}
	}
	if rec := do(h, "POST", "/api/admin/sicherungen/wiederherstellen", map[string]string{"name": name}); rec.Code != 200 {
		t.Fatalf("Wiederherstellen: %d %s", rec.Code, rec.Body)
	}
	if got := datenJSON(t, a); got != original {
		t.Fatal("Sicherung und Wiederherstellung liefern nicht denselben Bestand: " + unterschied(original, got))
	}

	// 4. Alte Datendatei (vor den Bereichen): Feld fehlt -> alle Bereiche sichtbar, Daten vollständig
	do(h, "PUT", "/api/admin/module", aus) // Liste steht jetzt in der Datei und wird unten entfernt
	raw, _ := os.ReadFile(filepath.Join(dir, dataFileName))
	if !strings.Contains(string(raw), `"ausModule"`) {
		t.Fatal("Bereichsliste fehlt in der Datei – Schritt 4 wäre wirkungslos")
	}
	var roh map[string]any
	if err := json.Unmarshal(raw, &roh); err != nil {
		t.Fatal(err)
	}
	delete(roh, "ausModule")
	alt, _ := json.Marshal(roh)
	altDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(altDir, dataFileName), alt, 0o600); err != nil {
		t.Fatal(err)
	}
	st3, err := openStore(altDir)
	if err != nil {
		t.Fatal(err)
	}
	st3.mu.Lock()
	for _, m := range module {
		if !st3.modulAn(m) {
			t.Errorf("alter Bestand: Bereich %s ist ausgeblendet", m)
		}
	}
	st3.mu.Unlock()
	if got := datenJSON(t, &App{st: st3}); got != original {
		t.Fatal("alter Bestand ohne Bereichsliste weicht ab: " + unterschied(original, got))
	}
}

// unterschied nennt die erste Stelle, an der sich zwei JSON-Texte unterscheiden, mit etwas Umfeld.
func unterschied(a, b string) string {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	von := max(0, i-120)
	return "\n vorher:  " + a[von:min(len(a), i+80)] + "\n nachher: " + b[von:min(len(b), i+80)]
}
