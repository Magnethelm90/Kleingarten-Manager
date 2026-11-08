package main

import (
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
}
