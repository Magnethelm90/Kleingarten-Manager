package main

import (
	"testing"
	"time"
)

func TestAblesedatum(t *testing.T) {
	a, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erika", "gartengroesse": 300}))
	heute := time.Now().Format("2006-01-02")
	datum := func() string {
		a.st.mu.Lock()
		defer a.st.mu.Unlock()
		return a.st.d.Jahre[yearKey(a.st.d.Settings.Jahr)].Ablesungen[p.ID].Datum
	}
	// nur Vorjahresstand: noch kein Ablesetag
	do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 10})
	if d := datum(); d != "" {
		t.Fatalf("ohne aktuellen Stand schon Datum %q", d)
	}
	r := decode[ablesungResp](t, do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 10, "wasserAkt": 20}))
	if r.Datum != heute || datum() != heute {
		t.Fatalf("Datum nach erstem Stand: %q / %q", r.Datum, datum())
	}
	// von Hand geändert, danach Korrektur des Stands: Datum bleibt
	do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 10, "wasserAkt": 20, "datum": "2025-10-03"})
	do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserVJ": 10, "wasserAkt": 21, "datum": "2025-10-03"})
	if d := datum(); d != "2025-10-03" {
		t.Fatalf("Datum nach Korrektur: %q", d)
	}
	if rec := do(h, "PUT", "/api/ablesung/"+p.ID, map[string]any{"wasserAkt": 21, "datum": "3.10."}); rec.Code != 400 {
		t.Fatalf("ungültiges Datum: %d", rec.Code)
	}
	// Abgleich ergänzt ein fehlendes Datum, überschreibt aber keins
	f := func(x float64) *float64 { return &x }
	if c, _ := ablesungErgaenzen(Ablesung{WasserAkt: f(1)}, Ablesung{Datum: "2025-09-01"}); c.Datum != "2025-09-01" {
		t.Fatalf("Abgleich ergänzt nicht: %+v", c)
	}
	if c, _ := ablesungErgaenzen(Ablesung{Datum: "2025-10-03"}, Ablesung{Datum: "2025-09-01"}); c.Datum != "2025-10-03" {
		t.Fatalf("Abgleich überschreibt: %+v", c)
	}
	// Pächterwechsel: Ablesetag = Übergabe
	q := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "gartennr": "5", "name": "Jonas", "gartengroesse": 250}))
	do(h, "PUT", "/api/ablesung/"+q.ID, map[string]any{"wasserVJ": 5, "stromVJ": 7})
	if rec := do(h, "POST", "/api/admin/paechterwechsel", map[string]any{"altId": q.ID, "datum": "2025-08-15", "wasserStand": 9, "stromStand": 9, "stunden": 3,
		"neu": map[string]any{"mitgliedsnr": "3", "name": "Nora"}}); rec.Code != 200 {
		t.Fatalf("Wechsel: %d %s", rec.Code, rec.Body.String())
	}
	a.st.mu.Lock()
	d := a.st.d.Jahre[yearKey(a.st.d.Settings.Jahr)].Ablesungen[q.ID].Datum
	a.st.mu.Unlock()
	if d != "2025-08-15" {
		t.Fatalf("Ablesetag beim Wechsel: %q", d)
	}
}
