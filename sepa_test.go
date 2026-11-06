package main

import (
	"bytes"
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
	"time"
)

// naechsterWerktag: ein Bankarbeitstag mindestens n Tage in der Zukunft.
func naechsterWerktag(n int) string {
	return naechsterBankarbeitstag(time.Now().AddDate(0, 0, n)).Format("2006-01-02")
}

func TestGlaeubigerIDUndSepaText(t *testing.T) {
	for id, ok := range map[string]bool{"DE98ZZZ09999999999": true, "de98 zzz0 9999 9999 99": true, "DE97ZZZ09999999999": false, "DE98ZZZ0999999999": false, "": false, "XX": false} {
		if glaeubigerIDGueltig(id) != ok {
			t.Errorf("%q: erwartet %v", id, ok)
		}
	}
	if got := sepaText("Jürgen Groß & Söhne <x>", 70); got != "Juergen Gross + Soehne x" {
		t.Errorf("sepaText: %q", got)
	}
	if got := sepaID("M 1-03/ä", 35); got != "M1-03/ae" {
		t.Errorf("sepaID: %q", got)
	}
}

func TestSepaLastschrift(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/beispieldaten", nil)
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	p1, p2, p3 := st.Paechter[0], st.Paechter[1], st.Paechter[2]
	faellig := naechsterWerktag(3)

	// ohne Gläubiger-ID kein Lauf
	if rec := do(h, "GET", "/api/admin/sepa/vorschau?art=vorauszahlung&faellig="+faellig, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("ohne Gläubiger-ID: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/admin/sepa/einstellungen", map[string]any{"glaeubigerId": "DE00ZZZ00000000000"}); rec.Code != http.StatusBadRequest {
		t.Errorf("falsche Gläubiger-ID angenommen: %d", rec.Code)
	}
	do(h, "PUT", "/api/admin/sepa/einstellungen", map[string]any{"glaeubigerId": "DE98ZZZ09999999999", "faelligkeitstag": 1})
	// Vereinskonto mit gültiger IBAN
	s := st.Settings
	s.IBAN = "DE89370400440532013000"
	if rec := do(h, "PUT", "/api/admin/settings", s); rec.Code != 200 {
		t.Fatalf("Einstellungen: %d %s", rec.Code, rec.Body.String())
	}

	// Mandate
	heute := time.Now().Format("2006-01-02")
	mandat := func(p Paechter, monat float64, rechnungen bool) Mandat {
		rec := do(h, "POST", "/api/admin/sepa/mandate", map[string]any{"paechterId": p.ID, "iban": "DE02 1203 0000 0000 2020 51", "unterschrieben": heute, "monatsbetrag": monat, "rechnungen": rechnungen})
		if rec.Code != 200 {
			t.Fatalf("Mandat: %d %s", rec.Code, rec.Body.String())
		}
		return decode[Mandat](t, rec)
	}
	m1 := mandat(p1, 25, true)
	mandat(p2, 30, false)
	if m1.Referenz == "" || m1.Kontoinhaber != sepaText(p1.Name, 70) {
		t.Errorf("Mandat: %+v", m1)
	}
	for name, body := range map[string]map[string]any{
		"IBAN falsch":      {"paechterId": p3.ID, "iban": "DE00123", "unterschrieben": heute},
		"zweites Mandat":   {"paechterId": p1.ID, "iban": "DE02120300000000202051", "unterschrieben": heute},
		"Referenz doppelt": {"paechterId": p3.ID, "iban": "DE02120300000000202051", "unterschrieben": heute, "referenz": m1.Referenz},
		"Zukunft":          {"paechterId": p3.ID, "iban": "DE02120300000000202051", "unterschrieben": "2099-01-01"},
	} {
		if rec := do(h, "POST", "/api/admin/sepa/mandate", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, rec.Code)
		}
	}

	// Vorauszahlung: Vorschau, Lauf, Abschlag gebucht, Monat nicht doppelt
	v := decode[laufVorschau](t, do(h, "GET", "/api/admin/sepa/vorschau?art=vorauszahlung&faellig="+faellig, nil))
	if len(v.Posten) != 2 || v.Summe != 55 {
		t.Fatalf("Vorschau: %+v", v)
	}
	abschlag := func(pid string) float64 {
		return decode[stateResp](t, do(h, "GET", "/api/state", nil)).Ablesungen[pid].Abschlag
	}
	vorher := abschlag(p1.ID)
	rec := do(h, "POST", "/api/admin/sepa/lauf", map[string]any{"art": "vorauszahlung", "faellig": faellig})
	if rec.Code != 200 {
		t.Fatalf("Lauf: %d %s", rec.Code, rec.Body.String())
	}
	lauf := decode[LastschriftLauf](t, rec)
	if abschlag(p1.ID) != vorher+25 {
		t.Errorf("Abschlag: %v, vorher %v", abschlag(p1.ID), vorher)
	}
	if rec := do(h, "POST", "/api/admin/sepa/lauf", map[string]any{"art": "vorauszahlung", "faellig": faellig}); rec.Code != http.StatusBadRequest {
		t.Errorf("zweiter Lauf im selben Monat: %d", rec.Code)
	}
	for name, f := range map[string]string{"gestern": time.Now().AddDate(0, 0, -1).Format("2006-01-02"), "unlesbar": "morgen"} {
		if rec := do(h, "GET", "/api/admin/sepa/vorschau?art=vorauszahlung&faellig="+f, nil); rec.Code != http.StatusBadRequest {
			t.Errorf("Fälligkeit %s: %d", name, rec.Code)
		}
	}
	// Kassenbericht zählt die Vorauszahlungen
	app.st.mu.Lock()
	vw, _ := app.st.viewLocked(st.CurrentYear)
	k := app.st.kassenberichtLocked(vw)
	app.st.mu.Unlock()
	if k.Vorauszahlungen != 55 {
		t.Errorf("Kassenbericht: %v", k.Vorauszahlungen)
	}

	// XML: wohlgeformt, richtige Summen, Mandat und Gläubiger-ID
	rec = do(h, "GET", "/api/admin/sepa/lauf/"+lauf.ID+"/xml", nil)
	if rec.Code != 200 {
		t.Fatalf("XML: %d", rec.Code)
	}
	var doc struct {
		Grp struct {
			Nb  int    `xml:"NbOfTxs"`
			Sum string `xml:"CtrlSum"`
		} `xml:"CstmrDrctDbtInitn>GrpHdr"`
		Tx []struct {
			Betrag string `xml:"InstdAmt"`
			Mandat string `xml:"DrctDbtTx>MndtRltdInf>MndtId"`
		} `xml:"CstmrDrctDbtInitn>PmtInf>DrctDbtTxInf"`
		Glaeubiger string `xml:"CstmrDrctDbtInitn>PmtInf>CdtrSchmeId>Id>PrvtId>Othr>Id"`
	}
	if err := xml.NewDecoder(bytes.NewReader(rec.Body.Bytes())).Decode(&doc); err != nil {
		t.Fatalf("XML nicht lesbar: %v", err)
	}
	if doc.Grp.Nb != 2 || doc.Grp.Sum != "55.00" || len(doc.Tx) != 2 || doc.Tx[0].Betrag != "25.00" || doc.Glaeubiger != "DE98ZZZ09999999999" || doc.Tx[0].Mandat != m1.Referenz {
		t.Errorf("XML-Inhalt: %+v", doc)
	}
	if !strings.Contains(rec.Body.String(), "pain.008.001.08") || !strings.Contains(rec.Body.String(), "<SeqTp>RCUR</SeqTp>") {
		t.Error("Schema oder Sequenz fehlt")
	}

	// Rücklastschrift nimmt die Buchung zurück und fehlt danach in der Datei
	if rec := do(h, "POST", "/api/admin/sepa/lauf/"+lauf.ID+"/rueck", map[string]any{"endToEnd": lauf.Posten[0].EndToEnd}); rec.Code != 200 {
		t.Fatalf("Rücklastschrift: %d %s", rec.Code, rec.Body.String())
	}
	if abschlag(p1.ID) != vorher {
		t.Errorf("Abschlag nach Rücklastschrift: %v", abschlag(p1.ID))
	}
	if rec := do(h, "POST", "/api/admin/sepa/lauf/"+lauf.ID+"/rueck", map[string]any{"endToEnd": lauf.Posten[0].EndToEnd}); rec.Code != http.StatusBadRequest {
		t.Errorf("doppelte Rücklastschrift: %d", rec.Code)
	}
	if !strings.Contains(do(h, "GET", "/api/admin/sepa/lauf/"+lauf.ID+"/xml", nil).Body.String(), "<NbOfTxs>1</NbOfTxs>") {
		t.Error("zurückgegangene Lastschrift steht noch in der Datei")
	}

	// Jahresrechnung: nur für Mandate mit Rechnungen, danach bezahlt, nicht doppelt
	do(h, "POST", "/api/invoices/issue", issueReq{})
	vr := decode[laufVorschau](t, do(h, "GET", "/api/admin/sepa/vorschau?art=rechnungen&faellig="+faellig, nil))
	if len(vr.Posten) != 1 || vr.Posten[0].PaechterID != p1.ID {
		t.Fatalf("Rechnungs-Vorschau: %+v", vr)
	}
	lr := decode[LastschriftLauf](t, do(h, "POST", "/api/admin/sepa/lauf", map[string]any{"art": "rechnungen", "faellig": faellig}))
	app.st.mu.Lock()
	var bezahlt string
	for _, r := range app.st.d.Rechnungen {
		if r.ID == lr.Posten[0].RechnungID {
			bezahlt = r.BezahltAm
		}
	}
	app.st.mu.Unlock()
	if bezahlt != faellig {
		t.Errorf("Rechnung nicht als bezahlt vermerkt: %q", bezahlt)
	}
	if vr2 := decode[laufVorschau](t, do(h, "GET", "/api/admin/sepa/vorschau?art=rechnungen&faellig="+faellig, nil)); len(vr2.Posten) != 0 {
		t.Errorf("Rechnung doppelt im Lauf: %+v", vr2.Posten)
	}
	// Vorauszahlung nach ausgestellter Rechnung: übersprungen mit Hinweis
	v2 := decode[laufVorschau](t, do(h, "GET", "/api/admin/sepa/vorschau?art=vorauszahlung&faellig="+naechsterWerktag(40), nil))
	if len(v2.Posten) != 0 || len(v2.Hinweise) != 2 {
		t.Errorf("Vorauszahlung nach Rechnung: %+v", v2)
	}

	// Papiere
	for _, pfad := range []string{"/api/admin/sepa/formular?paechter=" + p3.ID, "/api/admin/sepa/vorab?jahr=" + faellig[:4]} {
		if rec := do(h, "GET", pfad, nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
			t.Errorf("%s: %d %s", pfad, rec.Code, rec.Body.String())
		}
	}
	// genutztes Mandat lässt sich nicht löschen
	if rec := do(h, "DELETE", "/api/admin/sepa/mandate/"+m1.ID, nil); rec.Code != http.StatusBadRequest {
		t.Errorf("genutztes Mandat gelöscht: %d", rec.Code)
	}
	// Datenschutz: Mandate in der Auskunft, beim Löschen weg, Läufe bleiben ohne Verknüpfung
	app.st.mu.Lock()
	a, _ := app.st.auskunftLocked(p1.ID)
	app.st.d.entfernePerson(p1.ID)
	nachher := len(app.st.d.Mandate)
	app.st.mu.Unlock()
	if len(a.Mandate) != 1 || len(a.Lastschrift) != 2 || nachher != 1 {
		t.Errorf("Datenschutz: Auskunft %d/%d, Mandate danach %d", len(a.Mandate), len(a.Lastschrift), nachher)
	}
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/sepa", nil); rec.Code != 401 {
		t.Errorf("ohne Anmeldung: %d", rec.Code)
	}
}

func TestMandatVerfall(t *testing.T) {
	heute := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	if !(Mandat{Unterschrieben: "2023-01-01"}).verfallen(heute) {
		t.Error("ungenutzt seit über 36 Monaten muss verfallen sein")
	}
	if (Mandat{Unterschrieben: "2020-01-01", ZuletztGenutzt: "2025-12-01"}).verfallen(heute) {
		t.Error("zuletzt 2025 genutzt darf nicht verfallen sein")
	}
}
