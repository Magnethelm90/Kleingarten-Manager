package main

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestFaelligeMonate(t *testing.T) {
	for _, c := range []struct {
		jahr int
		tag  time.Time
		want int
	}{
		{2025, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), 12},
		{2027, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), 0},
		{2026, time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), 2},
		{2026, time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), 3},
	} {
		if got := faelligeMonate(c.jahr, c.tag); got != c.want {
			t.Errorf("%d am %s: %d, erwartet %d", c.jahr, c.tag.Format("02.01."), got, c.want)
		}
	}
}

func TestVorauszahlungUeberweisung(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	jahr := st.CurrentYear
	// Gärten ohne Vorauszahlungen in den Beispieldaten
	var p1, p2 Paechter
	for _, p := range st.Paechter {
		switch p.Gartennr {
		case "3":
			p1 = p
		case "5":
			p2 = p
		}
	}
	kasse := func() float64 {
		app.st.mu.Lock()
		defer app.st.mu.Unlock()
		vw, _ := app.st.viewLocked(jahr)
		return app.st.kassenberichtLocked(vw).Vorauszahlungen
	}
	kasseVorher := kasse()
	for _, x := range []struct {
		p Paechter
		b float64
	}{{p1, 25}, {p2, 30}} {
		if rec := do(h, "PUT", "/api/admin/vorauszahlungen/soll", map[string]any{"paechterId": x.p.ID, "betrag": x.b}); rec.Code != 200 {
			t.Fatalf("Soll: %d %s", rec.Code, rec.Body.String())
		}
	}
	abschlag := func(pid string) float64 {
		return decode[stateResp](t, do(h, "GET", "/api/state", nil)).Ablesungen[pid].Abschlag
	}
	vorher := abschlag(p1.ID)

	// von Hand: zwei Monate auf einmal
	v := decode[Vorauszahlung](t, do(h, "POST", "/api/admin/vorauszahlungen", map[string]any{"paechterId": p1.ID, "datum": fmt.Sprintf("%d-02-01", jahr), "betrag": 50}))
	if !v.Gebucht || v.Jahr != jahr || abschlag(p1.ID) != vorher+50 {
		t.Fatalf("Zahlung: %+v, Abschlag %v", v, abschlag(p1.ID))
	}
	ans := decode[vzAnsicht](t, do(h, "GET", fmt.Sprintf("/api/admin/vorauszahlungen?jahr=%d", jahr), nil))
	for _, z := range ans.Zeilen {
		if z.PaechterID == p1.ID && (z.Monate[0] != 1 || z.Monate[1] != 1 || z.Monate[2] != 0 || z.Summe != 50) {
			t.Errorf("Monate: %+v", z)
		}
	}
	for name, body := range map[string]map[string]any{
		"ohne Betrag":  {"paechterId": p1.ID, "datum": fmt.Sprintf("%d-03-01", jahr), "betrag": 0},
		"ohne Datum":   {"paechterId": p1.ID, "betrag": 25},
		"ohne Pächter": {"datum": fmt.Sprintf("%d-03-01", jahr), "betrag": 25},
		"Jahr zu weit": {"paechterId": p1.ID, "datum": fmt.Sprintf("%d-03-01", jahr), "betrag": 25, "jahr": jahr + 3},
	} {
		if rec := do(h, "POST", "/api/admin/vorauszahlungen", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}
	// löschen nimmt die Buchung zurück
	if rec := do(h, "DELETE", "/api/admin/vorauszahlungen/"+v.ID, nil); rec.Code != 200 || abschlag(p1.ID) != vorher {
		t.Errorf("löschen: %d, Abschlag %v", rec.Code, abschlag(p1.ID))
	}

	// Zahlung fürs nächste Jahr wird vorgemerkt und beim Jahreswechsel gebucht
	vm := decode[Vorauszahlung](t, do(h, "POST", "/api/admin/vorauszahlungen", map[string]any{"paechterId": p2.ID, "datum": fmt.Sprintf("%d-12-28", jahr), "jahr": jahr + 1, "betrag": 30}))
	if vm.Gebucht {
		t.Error("Zahlung fürs nächste Jahr sofort gebucht")
	}

	// Kontoauszug: Dauerauftrag mit Stichwort und eine Überweisung über drei Monate ohne Stichwort
	csvText := "Buchungstag;Verwendungszweck;Beguenstigter/Zahlungspflichtiger;Betrag\n" +
		fmt.Sprintf("01.03.%d;Dauerauftrag Vorauszahlung;%s;30,00\n", jahr, p2.Name) +
		fmt.Sprintf("02.03.%d;Garten;%s;75,00\n", jahr, p1.Name)
	kv := decode[kontoVorschau](t, doMultipart(h, "/api/admin/kontoauszug", "u.csv", []byte(csvText)))
	if len(kv.Buchungen) != 2 || kv.Buchungen[0].Status != "vorauszahlung" || kv.Buchungen[0].PaechterID != p2.ID ||
		kv.Buchungen[1].Status != "vorauszahlung" || kv.Buchungen[1].PaechterID != p1.ID || len(kv.Vorauszahler) != 7 {
		t.Fatalf("Zuordnung: %+v", kv)
	}
	var z []any
	for _, b := range kv.Buchungen {
		z = append(z, map[string]any{"vorauszahlung": b.PaechterID, "datum": b.Datum, "betrag": b.Betrag, "fingerprint": b.Fingerprint})
	}
	if rec := do(h, "POST", "/api/admin/kontoauszug/buchen", map[string]any{"zahlungen": z}); rec.Code != 200 {
		t.Fatalf("buchen: %d %s", rec.Code, rec.Body.String())
	}
	if abschlag(p1.ID) != vorher+75 {
		t.Errorf("Abschlag nach Kontoauszug: %v", abschlag(p1.ID))
	}

	// Kassenbericht zählt die gebuchten Vorauszahlungen, nicht die vorgemerkten
	if got := round2(kasse() - kasseVorher); got != 105 {
		t.Errorf("Kasse: %v mehr statt 105", got)
	}
	// Rückstand nach Ende des Jahres: zwölf Raten fällig
	app.st.mu.Lock()
	alt := app.st.vzAnsichtLocked(jahr, time.Date(jahr+1, 1, 15, 0, 0, 0, 0, time.UTC))
	app.st.mu.Unlock()
	for _, z := range alt.Zeilen {
		if z.PaechterID == p1.ID && (z.Faellig != 300 || z.Rueckstand != 225) {
			t.Errorf("Rückstand: %+v", z)
		}
	}
	if rec := do(h, "GET", fmt.Sprintf("/api/admin/vorauszahlungen/erinnerung?paechter=%s&jahr=%d", p1.ID, jahr), nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("Erinnerung: %d %s", rec.Code, rec.Body.String())
	}

	// Jahreswechsel bucht die vorgemerkte Zahlung
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	app.st.mu.Lock()
	var gebucht bool
	for _, x := range app.st.d.Vorauszahlungen {
		if x.ID == vm.ID {
			gebucht = x.Gebucht
		}
	}
	neu := app.st.d.Jahre[yearKey(jahr+1)].Ablesungen[p2.ID].Abschlag
	log := app.st.d.AuditLog[len(app.st.d.AuditLog)-1].Aktion
	app.st.mu.Unlock()
	if !gebucht || neu != 30 {
		t.Errorf("nach dem Jahreswechsel: gebucht %v, Abschlag %v (%s)", gebucht, neu, log)
	}
	// im abgeschlossenen Jahr lässt sich nichts mehr löschen
	app.st.mu.Lock()
	var altID string
	for _, x := range app.st.d.Vorauszahlungen {
		if x.Jahr == jahr {
			altID = x.ID
		}
	}
	app.st.mu.Unlock()
	if rec := do(h, "DELETE", "/api/admin/vorauszahlungen/"+altID, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("Löschen im abgeschlossenen Jahr: %d", rec.Code)
	}

	// Datenschutz
	app.st.mu.Lock()
	a, _ := app.st.auskunftLocked(p1.ID)
	app.st.d.entfernePerson(p1.ID)
	_, sollDa := app.st.d.VorauszahlungSoll[p1.ID]
	app.st.mu.Unlock()
	if len(a.Vorauszahl) == 0 || sollDa {
		t.Errorf("Datenschutz: Auskunft %d, Soll noch da %v", len(a.Vorauszahl), sollDa)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/vorauszahlungen", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}
