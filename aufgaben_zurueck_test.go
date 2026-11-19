package main

import (
	"testing"
	"time"
)

func TestAufgabeSchluesselIgnoriertZahlen(t *testing.T) {
	a := aufgabe{Titel: "Zählerablesung in 3 Tagen (20.11.2026)", Ziel: "admin:berichte", Knopf: "Ablesebogen"}
	b := aufgabe{Titel: "Zählerablesung in 2 Tagen (19.11.2026)", Ziel: "admin:berichte", Knopf: "Ablesebogen"}
	if aufgabeSchluessel(a) != aufgabeSchluessel(b) {
		t.Fatal("Schlüssel sollte von Zahlen unabhängig sein")
	}
	if aufgabeSchluessel(a) == aufgabeSchluessel(aufgabe{Titel: "Etwas anderes", Ziel: "admin:berichte", Knopf: "Ablesebogen"}) {
		t.Fatal("verschiedene Hinweise brauchen verschiedene Schlüssel")
	}
}

func TestAufgabeZurueckstellen(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Probe = nil
	app.st.d.Paechter = append(app.st.d.Paechter, Paechter{ID: "p1", Name: "Test", Gartennr: "1"})
	app.st.mu.Unlock()
	l := decode[[]aufgabe](t, do(h, "GET", "/api/admin/aufgaben", nil))
	if len(l) == 0 {
		t.Skip("kein Hinweis vorhanden")
	}
	k := l[0].Schluessel
	if len(k) != 16 {
		t.Fatalf("Schlüssel %q", k)
	}
	if rec := do(h, "POST", "/api/admin/aufgaben/zurueckstellen", map[string]any{"schluessel": k, "tage": 7}); rec.Code != 200 {
		t.Fatalf("zurückstellen: %d", rec.Code)
	}
	for _, x := range decode[[]aufgabe](t, do(h, "GET", "/api/admin/aufgaben", nil)) {
		if x.Schluessel == k {
			t.Fatal("zurückgestellter Hinweis sollte fehlen")
		}
	}
	gefunden := false
	for _, x := range decode[[]aufgabe](t, do(h, "GET", "/api/admin/aufgaben?alle=1", nil)) {
		if x.Schluessel == k && x.ZurueckBis == time.Now().AddDate(0, 0, 7).Format("2006-01-02") {
			gefunden = true
		}
	}
	if !gefunden {
		t.Fatal("mit alle=1 sollte der Hinweis samt Datum kommen")
	}
	do(h, "POST", "/api/admin/aufgaben/zurueckstellen", map[string]any{"schluessel": k, "tage": 0})
	back := false
	for _, x := range decode[[]aufgabe](t, do(h, "GET", "/api/admin/aufgaben", nil)) {
		back = back || x.Schluessel == k
	}
	if !back {
		t.Fatal("nach tage=0 sollte der Hinweis wieder da sein")
	}
	for _, bad := range []map[string]any{{"schluessel": "x", "tage": 3}, {"schluessel": k, "tage": 500}, {"schluessel": k, "tage": -1}} {
		if rec := do(h, "POST", "/api/admin/aufgaben/zurueckstellen", bad); rec.Code < 400 {
			t.Fatalf("ungültig %v wurde angenommen", bad)
		}
	}
}
