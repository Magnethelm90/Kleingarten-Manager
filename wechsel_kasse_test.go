package main

import "testing"

func TestKassenberichtWechselHinweis(t *testing.T) {
	a, h := newTestApp(t)
	jahr := a.st.d.Settings.Jahr
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "5", "name": "Ausge Schieden", "gartennr": "5", "gartengroesse": 100}))
	q := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "6", "name": "Bleibt Da", "gartennr": "6", "gartengroesse": 100}))
	if k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht", nil)); k.Wechsel == nil || len(k.Wechsel) != 0 {
		t.Fatalf("ohne Wechsel: %+v", k.Wechsel)
	}
	a.st.mu.Lock()
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == p.ID {
			a.st.d.Paechter[i].AusgeschiedenAm = "2026-05-01"
		}
	}
	a.st.mu.Unlock()
	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht", nil))
	if len(k.Wechsel) != 1 || k.Wechsel[0].PaechterID != p.ID || !k.Wechsel[0].OhneRechnung {
		t.Fatalf("Schlussrechnung fehlt: %+v", k.Wechsel)
	}
	// Schlussrechnung ausgestellt, aber unbezahlt: bleibt als »offen« stehen; bezahlt: verschwindet
	r := &Rechnung{ID: "w1", Jahr: jahr, PaechterID: p.ID, Nummer: "100-5", Status: statusGueltig}
	r.Result.Gesamt = 80
	a.st.mu.Lock()
	a.st.d.Rechnungen = append(a.st.d.Rechnungen, r)
	a.st.mu.Unlock()
	k = decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht", nil))
	if len(k.Wechsel) != 1 || k.Wechsel[0].OhneRechnung || k.Wechsel[0].Offen != 80 {
		t.Fatalf("offen: %+v", k.Wechsel)
	}
	do(h, "PUT", "/api/payment/w1", map[string]any{"bezahltAm": "2026-06-01"})
	if k = decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht", nil)); len(k.Wechsel) != 0 {
		t.Fatalf("bezahlt: %+v", k.Wechsel)
	}
	_ = q
}
