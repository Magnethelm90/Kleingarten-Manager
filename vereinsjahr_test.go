package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestVereinsjahr(t *testing.T) {
	app, h := newTestApp(t)
	punkt := func(d vereinsjahrAntwort, key string) (planPunkt, bool) {
		for _, m := range d.Monate {
			for _, p := range m.Punkte {
				if p.Key == key {
					return p, true
				}
			}
		}
		return planPunkt{}, false
	}
	d := decode[vereinsjahrAntwort](t, do(h, "GET", "/api/admin/vereinsjahr?jahr=2026", nil))
	if len(d.Monate) != 12 || d.Jahr != 2026 {
		t.Fatalf("12 Monate erwartet: %+v", d)
	}
	if _, ok := punkt(d, "sommerfest"); ok {
		t.Fatal("Barkasse ist im Grundpaket aus – der Punkt darf nicht erscheinen")
	}
	if p, ok := punkt(d, "wasseran"); !ok || p.Ziel != "admin:rundschreiben" {
		t.Fatalf("wasseran: %+v", p)
	}

	// Von Hand abhaken und zurücknehmen
	d = decode[vereinsjahrAntwort](t, do(h, "PUT", "/api/admin/vereinsjahr", map[string]any{"jahr": 2026, "key": "wasseran", "erledigt": true}))
	if p, _ := punkt(d, "wasseran"); !p.Erledigt || p.Erkannt {
		t.Fatalf("Haken fehlt: %+v", p)
	}
	d = decode[vereinsjahrAntwort](t, do(h, "PUT", "/api/admin/vereinsjahr", map[string]any{"jahr": 2026, "key": "wasseran", "erledigt": false}))
	if p, _ := punkt(d, "wasseran"); p.Erledigt {
		t.Fatal("Haken nicht zurückgenommen")
	}
	app.st.mu.Lock()
	if len(app.st.d.Vereinsjahr) != 0 {
		t.Fatalf("leere Jahre sollen verschwinden: %v", app.st.d.Vereinsjahr)
	}
	app.st.mu.Unlock()
	if rec := do(h, "PUT", "/api/admin/vereinsjahr", map[string]any{"jahr": 2026, "key": "gibtsnicht", "erledigt": true}); rec.Code != 400 {
		t.Fatalf("unbekannter Punkt: %d", rec.Code)
	}

	// Erkennung: Versammlung angesetzt, Rundschreiben aus → Ziel Termine, Jahreswechsel erledigt
	do(h, "PUT", "/api/admin/module", map[string]bool{"rundschreiben": false})
	app.st.mu.Lock()
	app.st.d.Termine = append(app.st.d.Termine, Termin{ID: "t1", Datum: "2026-03-14", Titel: "Mitgliederversammlung", Art: "Versammlung"})
	app.st.d.Settings.Jahr = 2027
	d = app.st.vereinsjahrLocked(2026, time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local))
	app.st.mu.Unlock()
	for key, soll := range map[string]bool{"einladung": true, "jahreswechsel": true, "rechnungen": false} {
		if p, _ := punkt(d, key); p.Erledigt != soll || p.Erkannt != soll {
			t.Fatalf("%s: %+v", key, p)
		}
	}
	if p, _ := punkt(d, "wasseran"); p.Ziel != "tab:termine" {
		t.Fatalf("ohne Rundschreiben soll auf Termine verwiesen werden: %+v", p)
	}
	if len(d.Monate[2].Termine) != 1 {
		t.Fatal("Termin im März fehlt")
	}
	// Abrechnungsrhythmus: Rechnung im Januar des Folgejahres bzw. im November desselben Jahres
	monatVon := func(d vereinsjahrAntwort, key string) (int, string) {
		for _, m := range d.Monate {
			for _, p := range m.Punkte {
				if p.Key == key {
					return m.Monat, p.Titel
				}
			}
		}
		return 0, ""
	}
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	app.st.d.Settings.Jahr, app.st.d.Settings.Rechnungsdatum, app.st.d.Settings.Zahlungsziel = 2025, "2026-01-10", "2026-02-15"
	app.st.d.Rechnungen = append(app.st.d.Rechnungen, &Rechnung{Jahr: 2025, Status: statusGueltig})
	d = app.st.vereinsjahrLocked(2026, time.Now())
	if m, titel := monatVon(d, "rechnungen"); m != 1 || !strings.Contains(titel, "2025") {
		t.Fatalf("Rechnungen im Januar für 2025 erwartet: %d %q", m, titel)
	}
	if m, _ := monatVon(d, "zahlungen"); m != 2 {
		t.Fatalf("Zahlungen im Februar erwartet: %d", m)
	}
	if p, _ := punkt(d, "rechnungen"); !p.Erkannt {
		t.Fatal("Rechnungen für 2025 sind erstellt – muss erkannt werden")
	}
	app.st.d.Settings.Rechnungsdatum, app.st.d.Settings.Zahlungsziel = "2025-11-20", "2025-12-15"
	d = app.st.vereinsjahrLocked(2025, time.Now())
	if m, titel := monatVon(d, "rechnungen"); m != 11 || !strings.Contains(titel, "2025") {
		t.Fatalf("Rechnungen im November für 2025 erwartet: %d %q", m, titel)
	}
	if p, _ := punkt(d, "jahreswechsel"); p.Erledigt {
		t.Fatal("Jahreswechsel 2025 ist noch nicht gemacht")
	}
}

// Ziele im Cockpit und im Vereinsjahr müssen admin:<bereich>[:<unterseite>] oder tab:<reiter>[:<unterseite>] sein,
// sonst öffnet der Knopf eine leere Seite.
func TestZielFormat(t *testing.T) {
	gueltig := regexp.MustCompile(`^(admin|tab):[a-z]+(:[a-z]+)?$`)
	for _, v := range vereinsjahrVorlagen {
		if !gueltig.MatchString(v.ziel) {
			t.Errorf("Vereinsjahr %s: Ziel %q", v.key, v.ziel)
		}
	}
	// alle add(…)-Aufrufe der Cockpit-Hinweise im Quelltext
	dateien, _ := filepath.Glob("*.go")
	aufruf := regexp.MustCompile(`"([a-z]+:[a-z]+(?::[a-z]+)?)",\s*"[^"]+"\)`)
	n := 0
	for _, f := range dateien {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, _ := os.ReadFile(f)
		for _, m := range aufruf.FindAllStringSubmatch(string(src), -1) {
			n++
			if !gueltig.MatchString(m[1]) {
				t.Errorf("%s: Ziel %q", f, m[1])
			}
		}
	}
	if n < 30 {
		t.Fatalf("nur %d Cockpit-Ziele gefunden – Muster prüfen", n)
	}
}
