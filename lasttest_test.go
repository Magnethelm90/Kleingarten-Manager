package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLastGrosserVerein misst, wie schnell das Programm mit einem großen Verein bleibt (150 Gärten, 10 Jahre).
// Läuft nur mit KM_LAST=1 (dauert einige Minuten): KM_LAST=1 go test -run LastGrosser -v
func TestLastGrosserVerein(t *testing.T) {
	if os.Getenv("KM_LAST") == "" {
		t.Skip("Lasttest nur mit KM_LAST=1")
	}
	app, h := newTestApp(t)
	const jahre, anzahl = 10, 150
	start := decode[stateResp](t, do(h, "GET", "/api/state", nil)).Settings.Jahr
	for i := 1; i <= anzahl; i++ {
		do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": fmt.Sprint(i), "name": fmt.Sprintf("Pächter %d", i), "gartennr": fmt.Sprint(i),
			"gartengroesse": 250 + i%7*20, "email": fmt.Sprintf("p%d@example.org", i), "versand": "Emailsendung"})
	}
	zeit := func(name string, grenze time.Duration, f func()) {
		t0 := time.Now()
		f()
		d := time.Since(t0)
		t.Logf("%-34s %v", name, d.Round(time.Millisecond))
		if d > grenze {
			t.Errorf("%s dauert zu lange: %v (Grenze %v)", name, d, grenze)
		}
	}
	for y := start; y < start+jahre; y++ {
		q := fmt.Sprintf("?year=%d", y)
		st := decode[stateResp](t, do(h, "GET", "/api/state"+q, nil))
		for i, p := range st.Paechter {
			a := st.Ablesungen[p.ID]
			if a.WasserVJ == nil {
				a.WasserVJ, a.StromVJ = fp(100), fp(1000)
			}
			a.WasserAkt, a.StromAkt = fp(*a.WasserVJ+float64(20+i%15)), fp(*a.StromVJ+float64(150+i*7%200))
			a.Stunden = fp(float64(6 + i%14))
			do(h, "PUT", "/api/ablesung/"+p.ID+q, a)
		}
		for m := 1; m <= 6; m++ {
			do(h, "POST", "/api/admin/ausgaben"+q, Ausgabe{Datum: fmt.Sprintf("%d-%02d-15", y, m), Beschreibung: "Posten", Kategorie: "Instandhaltung", Betrag: 100})
		}
		if rec := do(h, "POST", "/api/invoices/issue"+q, issueReq{}); y == start {
			t.Logf("Ausstellen %d: %.300s", rec.Code, rec.Body.String())
		}
		st = decode[stateResp](t, do(h, "GET", "/api/state"+q, nil))
		for _, e := range st.Archive {
			do(h, "PUT", "/api/payment/"+e.ID, paymentReq{BezahltAm: fmt.Sprintf("%d-02-01", y+1)})
		}
		if y < start+jahre-1 {
			do(h, "POST", "/api/admin/jahreswechsel", map[string]any{})
		}
	}
	app.st.mu.Lock()
	nj, np, nr := len(app.st.d.Jahre), len(app.st.d.Paechter), 0
	nr = len(app.st.d.Rechnungen)
	app.st.mu.Unlock()
	t.Logf("%d Jahre, %d Pächter, %d Rechnungen", nj, np, nr)
	if nj < jahre || np != anzahl || nr < anzahl*(jahre-1) {
		t.Fatalf("Testdaten unvollständig")
	}
	if fi, err := os.Stat(app.st.path); err == nil {
		t.Logf("Datendatei: %.1f MB", float64(fi.Size())/1e6)
	}
	sek := time.Second
	zeit("Start-Zustand /api/state", 2*sek, func() { do(h, "GET", "/api/state", nil) })
	zeit("Cockpit /api/admin/aufgaben", 2*sek, func() { do(h, "GET", "/api/admin/aufgaben?alle=1", nil) })
	zeit("Kassenbericht", 2*sek, func() { do(h, "GET", "/api/admin/kassenbericht", nil) })
	zeit("Kassenbericht-Verlauf", 3*sek, func() { do(h, "GET", "/api/admin/kassenbericht-verlauf", nil) })
	zeit("Gesamtsuche", 2*sek, func() { do(h, "GET", "/api/admin/suche?q=pächter", nil) })
	zeit("Schwund (alle Jahre)", 2*sek, func() { do(h, "GET", "/api/admin/schwund", nil) })
	zeit("Archiv alle Jahre", 4*sek, func() { do(h, "GET", "/api/archiv-alle", nil) })
	zeit("Datencheck", 3*sek, func() { do(h, "GET", "/api/admin/datencheck", nil) })
	zeit("Ablesung speichern (1 Änderung)", 2*sek, func() {
		st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
		p := st.Paechter[0]
		a := st.Ablesungen[p.ID]
		do(h, "PUT", "/api/ablesung/"+p.ID, a)
	})
	zeit("Wiederherstellungs-Probe", 3*sek, func() {
		app.st.mu.Lock()
		app.st.probeLocked(time.Now())
		app.st.mu.Unlock()
	})
}
