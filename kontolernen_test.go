package main

import "testing"

func TestKontoLernen(t *testing.T) {
	a, h := newTestApp(t)
	p1 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "12", "name": "Hans Müller", "gartengroesse": 300}))
	p2 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "13", "name": "Berta Schmidt", "gartengroesse": 350}))
	abl := Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)}
	do(h, "PUT", "/api/ablesung/"+p1.ID+"?year=2025", abl)
	do(h, "PUT", "/api/ablesung/"+p2.ID+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	betrag := map[string]float64{}
	for _, r := range a.st.d.Rechnungen {
		betrag[r.PaechterID] = openAmount(r)
	}
	pruefe := func(b kontoBuchung, wantPID string, was string) {
		t.Helper()
		bs := []kontoBuchung{b}
		a.st.zuordnenLocked(bs)
		if wantPID == "" {
			if bs[0].RechnungID != "" {
				t.Errorf("%s: unerwartet zugeordnet %+v", was, bs[0])
			}
			return
		}
		var pid string
		for _, r := range a.st.d.Rechnungen {
			if r.ID == bs[0].RechnungID {
				pid = r.PaechterID
			}
		}
		if pid != wantPID {
			t.Errorf("%s: %+v", was, bs[0])
		}
	}
	// Name in anderer Reihenfolge
	pruefe(kontoBuchung{Name: "Müller Hans", Zweck: "Pacht", Betrag: betrag[p1.ID]}, p1.ID, "Namensreihenfolge")
	// unbekannter Absender: erst nach dem Lernen
	b := kontoBuchung{Name: "Eheleute K. Bauer GbR", Zweck: "Garten", Betrag: betrag[p2.ID]}
	pruefe(b, "", "unbekannter Absender")
	a.st.kontoLernenLocked("Eheleute K. Bauer GbR", p2.ID)
	pruefe(b, p2.ID, "gelernt")
	if len(a.st.d.KontoGelernt) != 1 {
		t.Fatal("Lernen")
	}
	for k := range a.st.d.KontoGelernt {
		if len(k) != 16 {
			t.Errorf("Schlüssel kein Hash: %q", k)
		}
	}
	// Mitgliedsnummer im Verwendungszweck
	pruefe(kontoBuchung{Name: "Irgendwer", Zweck: "Mitgliedsnr. 12 Jahresrechnung", Betrag: betrag[p1.ID]}, p1.ID, "Mitgliedsnummer")
	if mitgliedsnrAusZweck("Garten 5") != "" || mitgliedsnrAusZweck("Mitglied-Nr: 007") != "7" {
		t.Error("mitgliedsnrAusZweck")
	}
	// Datenschutz: Person weg → Gelerntes weg
	if !a.st.d.kontoGelerntPersonEntfernen(p2.ID) || len(a.st.d.KontoGelernt) != 0 {
		t.Error("Gelerntes nicht entfernt")
	}
}
