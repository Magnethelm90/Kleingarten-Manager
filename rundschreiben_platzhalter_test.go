package main

import "testing"

func TestPlatzhalter(t *testing.T) {
	app, _ := newTestApp(t)
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	p := Paechter{ID: "p1", Name: "Anna Muster", Gartennr: "7", Mitgliedsnr: "12"}
	app.st.d.Rechnungen = append(app.st.d.Rechnungen, &Rechnung{ID: "r1", PaechterID: "p1", Status: statusGueltig, Nummer: "X-1", Result: Result{Gesamt: 80}})
	got := app.st.platzhalterLocked("Hallo {Name}, {Garten}, Nr {Mitgliedsnr}, offen {Offen}", &p)
	if got != "Hallo Anna Muster, Garten 7, Nr 12, offen "+fmtEUR(80) {
		t.Fatalf("Brief: %q", got)
	}
	if got := app.st.platzhalterLocked("Hallo {Name}", nil); got != "Hallo Gartenfreundin / Gartenfreund" {
		t.Fatalf("neutral: %q", got)
	}
	if got := app.st.platzhalterLocked("ohne", nil); got != "ohne" {
		t.Fatal(got)
	}
}
