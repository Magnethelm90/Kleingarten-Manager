package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// Gutschrift: nachträgliche Korrektur einer ausgestellten Rechnung (z. B. falsch abgelesener Zähler, Preisnachlass,
// Auslage doppelt berechnet). Die Rechnung selbst bleibt unverändert und wird nicht gelöscht; stattdessen entsteht ein
// eigenes Schreiben mit eigener Nummer. Der zu zahlende Betrag sinkt um die Summe der Gutschriften. War die Rechnung
// schon bezahlt, ergibt sich daraus ein Guthaben, das der Verein erstattet.

type Gutschrift struct {
	Nummer string  `json:"nummer"` // GS-<Rechnungsnummer>-<lfd. Nr.>
	Datum  string  `json:"datum"`  // JJJJ-MM-TT
	Betrag float64 `json:"betrag"` // positiv
	Grund  string  `json:"grund"`
}

func gutschriftSumme(r *Rechnung) float64 {
	s := 0.0
	for _, g := range r.Gutschriften {
		s += g.Betrag
	}
	return round2(s)
}

// zuZahlen ist der Rechnungsbetrag nach Abzug aller Gutschriften (immer positiv; bei Guthaben-Rechnungen unverändert).
func zuZahlen(r *Rechnung) float64 { return round2(absf(r.Result.Gesamt) - gutschriftSumme(r)) }

func (a *App) handleGutschriftNeu(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Betrag float64 `json:"betrag"`
		Grund  string  `json:"grund"`
		Datum  string  `json:"datum"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Grund = trim(in.Grund, 300)
	in.Betrag = round2(in.Betrag)
	if in.Grund == "" {
		writeErr(w, bad("Bitte den Grund der Gutschrift angeben"))
		return
	}
	if !validNum(in.Betrag) || in.Betrag <= 0 {
		writeErr(w, bad("Bitte einen Betrag über 0 angeben"))
		return
	}
	if in.Datum == "" {
		in.Datum = time.Now().Format("2006-01-02")
	}
	if !gueltigesDatum(in.Datum, false) {
		writeErr(w, bad("Bitte ein gültiges Datum angeben"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	x := a.st.rechnungLocked(r.PathValue("id"))
	if x == nil {
		writeErr(w, notFound("Rechnung nicht gefunden"))
		return
	}
	if x.Status != statusGueltig || x.Bereinigt {
		writeErr(w, bad("Zu dieser Rechnung ist keine Gutschrift mehr möglich (ersetzt oder bereinigt)"))
		return
	}
	if x.Result.Gesamt <= 0 {
		writeErr(w, bad("Zu einem Guthaben gibt es keine Gutschrift. Bitte die Rechnung neu ausstellen"))
		return
	}
	if rest := zuZahlen(x); in.Betrag > rest+0.004 {
		writeErr(w, bad(fmt.Sprintf("Die Gutschrift darf höchstens %s betragen (Rechnungsbetrag abzüglich früherer Gutschriften)", fmtEUR(rest))))
		return
	}
	// »voll bezahlt« hieß bisher: der damalige Rechnungsbetrag. Den Betrag jetzt festschreiben, sonst würde die
	// Gutschrift die Zahlung stillschweigend mitkürzen und die Erstattung verschwinden.
	if x.BezahltAm != "" && x.BezahltBetrag == nil {
		b := zuZahlen(x)
		x.BezahltBetrag = &b
	}
	x.Ratenplan = nil // der Betrag ändert sich, der alte Plan passt nicht mehr
	x.Gutschriften = append(x.Gutschriften, Gutschrift{
		Nummer: "GS-" + x.Nummer + "-" + strconv.Itoa(len(x.Gutschriften)+1), Datum: in.Datum, Betrag: in.Betrag, Grund: in.Grund,
	})
	a.st.audit("Gutschrift %s über %s zu Rechnung %s", x.Gutschriften[len(x.Gutschriften)-1].Nummer, fmtEUR(in.Betrag), x.Nummer)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "nummer": x.Gutschriften[len(x.Gutschriften)-1].Nummer, "offen": openAmount(x)})
}

// handleGutschriftLoeschen nimmt die jüngste Gutschrift zurück (Tippfehler). Frühere bleiben bestehen, damit die
// Nummernfolge lückenlos bleibt.
func (a *App) handleGutschriftLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	x := a.st.rechnungLocked(r.PathValue("id"))
	if x == nil || len(x.Gutschriften) == 0 {
		writeErr(w, notFound("Keine Gutschrift vorhanden"))
		return
	}
	g := x.Gutschriften[len(x.Gutschriften)-1]
	x.Gutschriften = x.Gutschriften[:len(x.Gutschriften)-1]
	if len(x.Gutschriften) == 0 {
		x.Gutschriften = nil
	}
	a.st.audit("Gutschrift %s zurückgenommen", g.Nummer)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Store) rechnungLocked(id string) *Rechnung {
	for _, x := range s.d.Rechnungen {
		if x.ID == id {
			return x
		}
	}
	return nil
}

func (a *App) gutschriftPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	x := a.st.rechnungLocked(r.PathValue("id"))
	var cp Rechnung
	if x != nil {
		cp = *x
		cp.Gutschriften = append([]Gutschrift{}, x.Gutschriften...)
	}
	a.st.mu.Unlock()
	if x == nil || len(cp.Gutschriften) == 0 {
		return nil, "", notFound("Zu dieser Rechnung gibt es keine Gutschrift")
	}
	n := len(cp.Gutschriften)
	if v, err := strconv.Atoi(r.URL.Query().Get("nr")); err == nil && v >= 1 && v <= n {
		n = v
	}
	pdf, err := buildGutschrift(cp.Settings, cp.Paechter, &cp, n-1)
	if err != nil {
		return nil, "", err
	}
	return pdf, safeName(cp.Gutschriften[n-1].Nummer+"_"+cp.Paechter.Name, 80) + ".pdf", nil
}

// buildGutschrift erzeugt das Schreiben zur Gutschrift Nummer idx (0-basiert) der Rechnung.
func buildGutschrift(s Settings, p Paechter, rec *Rechnung, idx int) ([]byte, error) {
	if idx < 0 || idx >= len(rec.Gutschriften) {
		return nil, errors.New("Gutschrift nicht gefunden")
	}
	g := rec.Gutschriften[idx]
	pdf := neuesPDF(s, "Gutschrift "+g.Nummer, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)

	y := 31.0
	feld := func(x, w float64, txt, stil string, groesse float64, align string) {
		pdf.SetFont("lib", stil, groesse)
		pdf.SetXY(x, y)
		pdf.CellFormat(w, rowH, txt, "", 0, align, false, 0, "")
	}
	feld(leftX, rightX-leftX, s.Absenderzeile, "", 7.5, "L")
	y += 11
	y += adressZeile(pdf, leftX, y, p.Anrede, "")
	y += adressZeile(pdf, leftX, y, p.Name, "B")
	y += adressZeile(pdf, leftX, y, p.Strasse, "")
	y += adressZeile(pdf, leftX, y, p.PLZOrt, "")

	y += 4
	meta := [][2]string{
		{"Mitgliedsnr.:", p.Mitgliedsnr},
		{"Gutschriftnr.:", g.Nummer},
		{"Datum:", germanDate(g.Datum)},
		{"zur Rechnung:", rec.Nummer + " vom " + germanDate(s.Rechnungsdatum)},
	}
	for _, m := range meta {
		feld(95, 50, m[0], "", 10, "R")
		feld(145, 45, m[1], "B", 10, "R")
		y += rowH
	}

	y += 6
	pdf.SetTextColor(markenfarbe(s.Farbe))
	pdf.SetFont("lib", "B", 13)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 7, "Gutschrift", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 10

	vorher := absf(rec.Result.Gesamt)
	for i := 0; i < idx; i++ {
		vorher -= rec.Gutschriften[i].Betrag
	}
	vorher = round2(vorher)
	nachher := round2(vorher - g.Betrag)
	text := fmt.Sprintf("Zu Ihrer Rechnung %s schreiben wir Ihnen %s gut.", rec.Nummer, fmtEUR(g.Betrag))
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 5.2, text, "", "L", false)
	y = pdf.GetY() + 2
	pdf.SetFont("lib", "B", 10)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(25, 5.2, "Grund:", "", 0, "L", false, 0, "")
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX+25, y)
	pdf.MultiCell(rightX-leftX-25, 5.2, g.Grund, "", "L", false)
	y = pdf.GetY() + 4

	zeilen := [][2]string{
		{"Bisheriger Rechnungsbetrag", fmtEUR(vorher)},
		{"Diese Gutschrift", "− " + fmtEUR(g.Betrag)},
	}
	pdf.SetDrawColor(185, 190, 182)
	pdf.SetLineWidth(0.2)
	for _, z := range zeilen {
		feld(leftX, 100, z[0], "", 10, "L")
		feld(rightX-45, 45, z[1], "", 10, "R")
		y += rowH
		pdf.Line(leftX, y, rightX, y)
	}
	y += 3
	betragsBox(pdf, s, colX[3]-4, y, "Neuer Rechnungsbetrag:", fmtEUR(nachher))
	y += 14

	bezahlt := paidAmount(rec)
	var hinweis string
	switch {
	case rec.BezahltAm != "" && bezahlt > nachher+0.004:
		hinweis = fmt.Sprintf("Die Rechnung wurde bereits bezahlt. Der Verein erstattet Ihnen den zu viel gezahlten Betrag von %s auf das Ihnen bekannte Konto beziehungsweise verrechnet ihn mit der nächsten Abrechnung.", fmtEUR(round2(bezahlt-nachher)))
	case nachher <= 0.004:
		hinweis = "Damit ist die Rechnung ausgeglichen. Es ist nichts mehr zu zahlen."
	default:
		hinweis = fmt.Sprintf("Bitte überweisen Sie nur noch den neuen Rechnungsbetrag von %s (nicht den ursprünglichen Betrag) bis zum %s auf das Konto der %s, IBAN: %s, Verwendungszweck: %s. Eine bereits eingegangene Zahlung ist angerechnet.",
			fmtEUR(nachher), germanDate(s.Zahlungsziel), s.BankName, s.IBAN, rec.Nummer)
	}
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 4.9, hinweis, "", "L", false)
	y = pdf.GetY() + 6
	feld(leftX, rightX-leftX, s.Brieftexte.text("unterschrift", nil), "", 10, "L")
	y += rowH + 3
	feld(leftX, rightX-leftX, "Dieses Schreiben wird maschinell erstellt und ist ohne Unterschrift gültig.", "", 8, "L")
	if y > 285 {
		return nil, errors.New("Gutschrift passt nicht auf eine Seite")
	}
	return pdfBytes(pdf)
}

// gutschriftGruendeLeeren entfernt den Freitext der Gutschriften (kann Namen oder Gesundheitsangaben enthalten); die
// Beträge bleiben für die Buchführung.
func (r *Rechnung) gutschriftGruendeLeeren() {
	for i := range r.Gutschriften {
		r.Gutschriften[i].Grund = fristAbgelaufenText
	}
}
