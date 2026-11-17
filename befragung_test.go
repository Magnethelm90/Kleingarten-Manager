package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestBefragungAblauf(t *testing.T) {
	app, h := newTestApp(t)
	_ = app
	in := Befragung{Titel: "Test", Datum: "2026-03-01", Rueckgabe: 10, Fragen: []BefragungFrage{
		{Text: "Ja?", Art: "janein", Zaehlung: []int{6, 3, 0}},
		{Text: "Was?", Art: "wahl", Optionen: []string{"A", "B", ""}, Zaehlung: []int{4, 5}},
		{Text: "Frei", Art: "text"},
	}}
	rec := do(h, "POST", "/api/admin/befragungen", in)
	if rec.Code != 200 {
		t.Fatalf("speichern: %d %s", rec.Code, rec.Body.String())
	}
	b := decode[Befragung](t, rec)
	if b.ID == "" || len(b.Fragen[1].Optionen) != 2 || len(b.Fragen[2].Zaehlung) != 0 {
		t.Fatalf("nicht bereinigt: %+v", b)
	}
	for _, art := range []string{"fragebogen", "erfassung", "auswertung"} {
		r := do(h, "GET", "/api/admin/befragungen/"+b.ID+"/pdf?art="+art, nil)
		if r.Code != 200 || !strings.HasPrefix(r.Body.String(), "%PDF") {
			t.Errorf("%s: %d", art, r.Code)
		}
	}
	// mehr Antworten als Bögen
	b.Fragen[0].Zaehlung = []int{9, 9, 0}
	if r := do(h, "POST", "/api/admin/befragungen", b); r.Code != http.StatusBadRequest {
		t.Errorf("zu viele Antworten: %d", r.Code)
	}
	// ohne Titel / ohne Fragen
	if r := do(h, "POST", "/api/admin/befragungen", Befragung{Fragen: in.Fragen}); r.Code != 400 {
		t.Errorf("ohne Titel: %d", r.Code)
	}
	if r := do(h, "POST", "/api/admin/befragungen", Befragung{Titel: "x"}); r.Code != 400 {
		t.Errorf("ohne Fragen: %d", r.Code)
	}
	if r := do(h, "DELETE", "/api/admin/befragungen/"+b.ID, nil); r.Code != 200 {
		t.Errorf("löschen: %d", r.Code)
	}
}

func TestAufnahmeantragPDF(t *testing.T) {
	app, h := newTestApp(t)
	app.st.mu.Lock()
	app.st.d.Warteliste = []Bewerber{{ID: "w1", Seit: "2026-01-01", Name: "Erika Muster", Strasse: "Weg 1", PLZOrt: "12345 Ort"}}
	app.st.mu.Unlock()
	r := do(h, "GET", "/api/admin/warteliste/w1/brief?art=aufnahmeantrag&garten=12", nil)
	if r.Code != 200 || !strings.HasPrefix(r.Body.String(), "%PDF") {
		t.Fatalf("%d", r.Code)
	}
}
