package main

import (
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

// TestZehnJahreBetrieb spielt zwölf Abrechnungsjahre durch, wie sie im Verein ablaufen: Zählerstände
// erfassen, Rechnungen ausstellen, Zahlungen verbuchen (auch verspätete), Ausgaben, Pächterwechsel,
// Jahreswechsel. Danach müssen alle Jahre lesbar, alte Rechnungen druckbar, die Zählerstände lückenlos
// fortgeschrieben und die Kasse von Jahr zu Jahr stimmig sein; nach Ablauf der Aufbewahrungsfrist wird
// genau das Richtige bereinigt.
func TestZehnJahreBetrieb(t *testing.T) {
	if testing.Short() {
		t.Skip("lange Simulation")
	}
	app, h := newTestApp(t)
	const start, jahre, anzahl = 2025, 12, 12
	type kasse struct{ Anfangsbestand, Kassenbestand float64 }

	ids := []string{}
	neuerPaechter := func(nr int) {
		p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
			"mitgliedsnr": fmt.Sprint(nr), "name": fmt.Sprintf("Pächter %d", nr), "gartennr": fmt.Sprint(nr),
			"gartengroesse": 250 + nr%7*20, "email": fmt.Sprintf("p%d@example.org", nr), "versand": "Emailsendung"}))
		ids = append(ids, p.ID)
	}
	for i := 1; i <= anzahl; i++ {
		neuerPaechter(i)
	}
	nextNr := anzahl + 1
	letzterStand := map[string][2]float64{}
	kassenbestandBeimWechsel := map[int]float64{}

	for y := start; y < start+jahre; y++ {
		q := fmt.Sprintf("?year=%d", y)
		st := decode[stateResp](t, do(h, "GET", "/api/state"+q, nil))
		if st.Year != y || st.ReadOnly {
			t.Fatalf("%d: falsches oder gesperrtes Jahr (%d, readOnly=%v)", y, st.Year, st.ReadOnly)
		}
		for i, p := range st.Paechter {
			a := st.Ablesungen[p.ID]
			// Vorjahresstand muss dem letztjährigen aktuellen Stand entsprechen
			if alt, ok := letzterStand[p.ID]; ok && (a.WasserVJ == nil || *a.WasserVJ != alt[0] || a.StromVJ == nil || *a.StromVJ != alt[1]) {
				t.Fatalf("%d %s: Vorjahresstand nicht übernommen: %v %v, erwartet %v", y, p.Name, a.WasserVJ, a.StromVJ, alt)
			}
			if a.WasserVJ == nil {
				a.WasserVJ, a.StromVJ = fp(100), fp(1000)
			}
			a.WasserAkt = fp(*a.WasserVJ + float64(20+i%15))
			a.StromAkt = fp(*a.StromVJ + float64(150+i*7%200))
			a.Stunden = fp(float64(6 + i%14))
			if rec := do(h, "PUT", "/api/ablesung/"+p.ID+q, a); rec.Code != 200 {
				t.Fatalf("%d Ablesung: %d %s", y, rec.Code, rec.Body.String())
			}
			letzterStand[p.ID] = [2]float64{*a.WasserAkt, *a.StromAkt}
		}
		do(h, "POST", "/api/admin/ausgaben"+q, Ausgabe{Datum: fmt.Sprintf("%d-03-31", y), Beschreibung: "Kontoführung", Betrag: 60, Wiederkehrend: y == start})
		do(h, "POST", "/api/admin/ausgaben"+q, Ausgabe{Datum: fmt.Sprintf("%d-06-15", y), Beschreibung: "Reparatur", Kategorie: "Instandhaltung", Betrag: 100 + float64(y-start)})

		if rec := do(h, "POST", "/api/invoices/issue"+q, issueReq{}); rec.Code != 200 {
			t.Fatalf("%d Ausstellen: %d %s", y, rec.Code, rec.Body.String())
		}
		st = decode[stateResp](t, do(h, "GET", "/api/state"+q, nil))
		if len(st.Issued) != len(st.Paechter) {
			t.Fatalf("%d: %d von %d Rechnungen ausgestellt", y, len(st.Issued), len(st.Paechter))
		}
		// alle bezahlen, bis auf zwei (eine davon zahlt im nächsten Jahr), eine nur teilweise
		n := 0
		for _, e := range st.Archive {
			n++
			switch n {
			case 1, 2:
				continue
			case 3:
				do(h, "PUT", "/api/payment/"+e.ID, paymentReq{BezahltAm: fmt.Sprintf("%d-02-01", y+1), BezahltBetrag: fp(50)})
			default:
				do(h, "PUT", "/api/payment/"+e.ID, paymentReq{BezahltAm: fmt.Sprintf("%d-02-01", y+1)})
			}
		}
		// wie im echten Leben: ausgestellt wird im Januar des Folgejahres
		app.st.mu.Lock()
		for _, r := range app.st.d.Rechnungen {
			if r.Jahr == y {
				r.Ausgestellt = fmt.Sprintf("%d-01-10T10:00:00+01:00", y+1)
			}
		}
		app.st.mu.Unlock()

		// Pächterwechsel: einer geht (Papierkorb), einer kommt
		do(h, "DELETE", "/api/admin/paechter/"+ids[y-start], nil)
		neuerPaechter(nextNr)
		nextNr++

		k := decode[kasse](t, do(h, "GET", "/api/admin/kassenbericht"+q, nil))
		kassenbestandBeimWechsel[y] = k.Kassenbestand
		if rec := do(h, "POST", "/api/admin/jahreswechsel", nil); rec.Code != 200 {
			t.Fatalf("%d Jahreswechsel: %d %s", y, rec.Code, rec.Body.String())
		}
		// verspätete Zahlung einer Vorjahresrechnung nach dem Jahreswechsel
		do(h, "PUT", "/api/payment/"+st.Archive[0].ID, paymentReq{BezahltAm: fmt.Sprintf("%d-04-01", y+1)})
	}

	// alle Jahre lesbar, abgeschlossene schreibgeschützt, Anfangsbestand = Kassenbestand beim Wechsel
	for y := start; y < start+jahre; y++ {
		q := fmt.Sprintf("?year=%d", y)
		st := decode[stateResp](t, do(h, "GET", "/api/state"+q, nil))
		if !st.ReadOnly || len(st.Archive) == 0 {
			t.Errorf("%d: readOnly=%v, %d Rechnungen", y, st.ReadOnly, len(st.Archive))
		}
		if y > start {
			k := decode[kasse](t, do(h, "GET", "/api/admin/kassenbericht"+q, nil))
			if math.Abs(k.Anfangsbestand-kassenbestandBeimWechsel[y-1]) > 0.005 {
				t.Errorf("%d: Anfangsbestand %.2f, Kassenbestand Ende %d war %.2f", y, k.Anfangsbestand, y-1, kassenbestandBeimWechsel[y-1])
			}
		}
	}
	// älteste Rechnung lässt sich noch drucken
	alle := decode[[]archiveEntry](t, do(h, "GET", "/api/archiv-alle", nil))
	if want := jahre * anzahl; len(alle) != want {
		t.Errorf("Archiv: %d Rechnungen, erwartet %d", len(alle), want)
	}
	aelteste := alle[len(alle)-1]
	if rec := do(h, "GET", "/api/archive/"+aelteste.ID+".pdf", nil); rec.Code != 200 {
		t.Errorf("Rechnung aus %d nicht mehr druckbar: %d", aelteste.Jahr, rec.Code)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	if st.Year != start+jahre || len(st.Years) != jahre+1 {
		t.Errorf("aktuelles Jahr %d, %d Jahre", st.Year, len(st.Years))
	}
	// offene Posten aus allen Vorjahren bleiben sichtbar (je Jahr eine nie bezahlte, eine Teilzahlung)
	if len(st.OffeneVorjahre) != 2*jahre {
		t.Errorf("offene Vorjahresrechnungen: %d, erwartet %d", len(st.OffeneVorjahre), 2*jahre)
	}

	// Größe und Geschwindigkeit nach zwölf Jahren
	fi, _ := os.Stat(app.st.path)
	t0 := time.Now()
	st2, err := openStore(app.st.dir)
	laden := time.Since(t0)
	if err != nil {
		t.Fatal(err)
	}
	t0 = time.Now()
	do(h, "GET", "/api/state", nil)
	zustand := time.Since(t0)
	t.Logf("nach %d Jahren: Datendatei %d KB, Laden %v, Seitenaufruf %v, %d Rechnungen", jahre, fi.Size()/1024, laden, zustand, len(st2.d.Rechnungen))
	if fi.Size() > 20<<20 || laden > 3*time.Second || zustand > 2*time.Second {
		t.Errorf("zu groß oder zu langsam: %d Bytes, Laden %v, Seitenaufruf %v", fi.Size(), laden, zustand)
	}

	// Aufbewahrungsfrist im Jahr 2037: Rechnungen ausgestellt 2026 (für 2025) sind fällig, sonst nichts
	app.st.mu.Lock()
	r, j, _ := app.st.d.bereinigeAbgelaufene(2037)
	app.st.mu.Unlock()
	if r != anzahl || j != 1 {
		t.Errorf("Bereinigung 2037: %d Rechnungen, %d Jahre, erwartet %d und 1", r, j, anzahl)
	}
	if v := decode[stateResp](t, do(h, "GET", fmt.Sprintf("/api/state?year=%d", start+1), nil)); v.Paechter[0].Name == fristAbgelaufenText {
		t.Errorf("Jahresunterlagen %d zu früh bereinigt", start+1)
	}
}

// Wird eine Vorjahresrechnung erst nach dem Jahreswechsel bezahlt, steigt der Kassenbestand des
// Vorjahres, der Anfangsbestand des neuen Jahres aber nicht. Der Kassenbericht muss das zeigen.
func TestAnfangsbestandNachSpaeterZahlung(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Spät", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(1), WasserAkt: fp(2), StromVJ: fp(1), StromAkt: fp(2), Stunden: fp(12)})
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2026", nil))
	if k.VorjahrEndbestand == nil || *k.VorjahrEndbestand != k.Anfangsbestand {
		t.Fatalf("direkt nach dem Wechsel: Anfangsbestand %v, Vorjahr %v", k.Anfangsbestand, k.VorjahrEndbestand)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	do(h, "PUT", "/api/payment/"+st.Archive[0].ID, paymentReq{BezahltAm: "2026-03-01"})
	k = decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2026", nil))
	if k.VorjahrEndbestand == nil || math.Abs(*k.VorjahrEndbestand-k.Anfangsbestand-st.Archive[0].Gesamt) > 0.005 {
		t.Fatalf("nach später Zahlung: Anfangsbestand %v, Vorjahr %v, Rechnung %v", k.Anfangsbestand, k.VorjahrEndbestand, st.Archive[0].Gesamt)
	}
	k = decode[kassenbericht](t, do(h, "PUT", "/api/admin/anfangsbestand?year=2026", map[string]any{"betrag": *k.VorjahrEndbestand}))
	if *k.VorjahrEndbestand != k.Anfangsbestand {
		t.Errorf("nach dem Angleichen: %v / %v", k.Anfangsbestand, *k.VorjahrEndbestand)
	}
	if k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil)); k.VorjahrEndbestand != nil {
		t.Error("erstes Jahr hat kein Vorjahr")
	}
}
