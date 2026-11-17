package main

import (
	"strings"
	"testing"
)

func TestEhrenamtNachweis(t *testing.T) {
	a, h := newTestApp(t)
	jahr := a.st.d.Settings.Jahr
	p1 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Anna Fleiß", "gartennr": "1", "gartengroesse": 100}))
	p2 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Bernd Ruhe", "gartennr": "2", "gartengroesse": 100}))
	p3 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "3", "name": "Carla Amt", "gartennr": "3", "gartengroesse": 100}))

	pdfUrl := "/api/admin/ehrenamt"
	if c := do(h, "GET", pdfUrl, nil).Code; c != 400 {
		t.Errorf("ohne Daten: %d", c)
	}
	do(h, "POST", "/api/admin/stundenkonto/buchen", map[string]any{"paechterId": p1.ID, "jahr": jahr, "stunden": 7.5})
	a.st.mu.Lock()
	a.st.d.Termine = append(a.st.d.Termine, Termin{ID: "t1", Datum: "2026-04-10", Titel: "Frühjahrsputz", Art: "Arbeitseinsatz"})
	a.st.d.Einsatzstunden = append(a.st.d.Einsatzstunden, EinsatzStunden{TerminID: "t1", Jahr: jahr, PaechterID: p1.ID, Stunden: 4})
	a.st.d.Aemter = append(a.st.d.Aemter, Amt{ID: "a1", Amt: "Kassenwart/in", Name: " carla amt ", Seit: "2020-03-01"},
		Amt{ID: "a2", Amt: "Früheres Amt", Name: "Carla Amt", Seit: "2010-01-01", Bis: "2015-12-31"})
	v, _ := a.st.viewLocked(jahr)
	n1 := a.st.ehrenamtLocked(v, p1)
	n3 := a.st.ehrenamtLocked(v, p3)
	a.st.mu.Unlock()

	// Einsatzstunden stehen immer auch im Stundenkonto; hier nur nachgetragene 7,5 h: davon 4 h aus dem Einsatz, Rest weitere
	if n1.Summe != 7.5 || len(n1.Einsatz) != 1 || n1.Weitere != 3.5 {
		t.Errorf("Anna: %+v", n1)
	}
	if len(n3.Aemter) != 1 || n3.Aemter[0].Amt != "Kassenwart/in" || n3.leer() {
		t.Errorf("Carla: %+v", n3)
	}
	for _, id := range []string{p1.ID, p3.ID, ""} {
		rec := do(h, "GET", pdfUrl+"?paechterId="+id, nil)
		if rec.Code != 200 || !strings.HasPrefix(rec.Body.String(), "%PDF") {
			t.Errorf("PDF %q: %d %s", id, rec.Code, rec.Body.String())
		}
	}
	if c := do(h, "GET", pdfUrl+"?paechterId="+p2.ID, nil).Code; c != 400 {
		t.Errorf("ohne Stunden/Ämter: %d", c)
	}
	if c := do(h, "GET", pdfUrl+"?paechterId=gibtesnicht", nil).Code; c != 404 {
		t.Errorf("unbekannt: %d", c)
	}
}
