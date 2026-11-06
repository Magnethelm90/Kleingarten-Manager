package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Papiere zur Lastschrift: das Mandat zum Unterschreiben und die Vorabankündigung (Pre-Notification) der
// monatlichen Vorauszahlungen. Die Vorabankündigung ist bei SEPA-Lastschriften Pflicht; eine Ankündigung für
// das ganze Jahr mit allen Terminen genügt.

func vereinsAnschrift(s Settings, g Gemeinnuetzigkeit) []string {
	zeilen := []string{s.VereinName}
	if g.Anschrift != "" {
		return append(zeilen, strings.Split(g.Anschrift, "\n")...)
	}
	if s.Absenderzeile != "" {
		return []string{s.Absenderzeile}
	}
	return append(zeilen, s.Ort)
}

// ibanGruppiert: DE12 3456 7890 …
func ibanGruppiert(iban string) string {
	var b strings.Builder
	for i, c := range normIBAN(iban) {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(c)
	}
	return b.String()
}

// naechsterBankarbeitstag verschiebt Samstag und Sonntag auf den Montag (Feiertage kennt das Programm nicht).
func naechsterBankarbeitstag(d time.Time) time.Time {
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDate(0, 0, 1)
	}
	return d
}

func feld(pdf *fpdf.Fpdf, x, y, w float64, label, wert string) {
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(90, 90, 90)
	pdf.SetXY(x, y)
	pdf.CellFormat(w, 4, label, "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.SetDrawColor(120, 120, 120)
	pdf.SetLineWidth(0.25)
	pdf.Rect(x, y+4, w, 8, "D")
	pdf.SetFont("lib", "B", 10.5)
	pdf.SetXY(x+2, y+5)
	pdf.CellFormat(w-4, 6, passendKleinStil(pdf, "B", wert, w-4, 10.5, 7), "", 0, "L", false, 0, "")
}

func buildMandatsformular(s Settings, g Gemeinnuetzigkeit, sepa SepaEinstellungen, p Paechter, m Mandat) ([]byte, error) {
	pdf := neuesPDF(s, "SEPA-Lastschriftmandat "+m.Referenz, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := 38.0
	pdf.SetFont("lib", "B", 17)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 8, "SEPA-Lastschriftmandat", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 9
	pdf.SetFont("lib", "", 9.5)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 5, "für wiederkehrende Zahlungen (SEPA-Basislastschrift)", "", 0, "L", false, 0, "")
	y += 10

	y = mappeAbschnitt(pdf, s, y, "Zahlungsempfänger")
	y = mappeTabelle(pdf, y, [][2]string{
		{"Name und Anschrift", strings.Join(vereinsAnschrift(s, g), ", ")},
		{"Gläubiger-Identifikationsnummer", sepa.GlaeubigerID},
		{"Mandatsreferenz", m.Referenz},
	})

	verein := s.VereinName
	y = mappeText(pdf, y, fmt.Sprintf("Ich ermächtige den Zahlungsempfänger %s, Zahlungen von meinem Konto mittels Lastschrift einzuziehen. "+
		"Zugleich weise ich mein Kreditinstitut an, die von %s auf mein Konto gezogenen Lastschriften einzulösen.", verein, verein), 10)
	y = mappeText(pdf, y, "Hinweis: Ich kann innerhalb von acht Wochen, beginnend mit dem Belastungsdatum, die Erstattung des belasteten Betrages verlangen. "+
		"Es gelten dabei die mit meinem Kreditinstitut vereinbarten Bedingungen.", 10)
	var umfang []string
	if m.Monatsbetrag > 0 {
		umfang = append(umfang, fmt.Sprintf("die monatliche Vorauszahlung von %s jeweils zum %d. eines Monats (bzw. folgenden Bankarbeitstag)", fmtEUR(m.Monatsbetrag), max(1, sepa.Faelligkeitstag)))
	}
	if m.Rechnungen || m.Monatsbetrag == 0 {
		umfang = append(umfang, "der Betrag der Jahresabrechnung (Pacht, Beiträge, Wasser, Strom) zum Zahlungsziel der Rechnung")
	}
	y = mappeText(pdf, y, "Eingezogen werden "+strings.Join(umfang, " sowie ")+". Termine und Beträge kündigt der Verein vorher an.", 10)
	y += 2

	y = mappeAbschnitt(pdf, s, y, "Zahlungspflichtiger (Kontoinhaber)")
	inhaber := m.Kontoinhaber
	if inhaber == "" {
		inhaber = p.Name
	}
	w := rightX - leftX
	feld(pdf, leftX, y, w, "Vorname und Name (Kontoinhaber)", inhaber)
	y += 15
	feld(pdf, leftX, y, w, "Straße und Hausnummer, Postleitzahl und Ort", strings.TrimSpace(strings.Trim(p.Strasse+", "+p.PLZOrt, ", ")))
	y += 15
	feld(pdf, leftX, y, w*0.7-2, "IBAN", ibanGruppiert(m.IBAN))
	feld(pdf, leftX+w*0.7+2, y, w*0.3-2, "BIC (bei deutschen Konten nicht nötig)", m.BIC)
	y += 15
	feld(pdf, leftX, y, w*0.45-2, "Ort, Datum", "")
	feld(pdf, leftX+w*0.45+2, y, w*0.55-2, "Unterschrift des Kontoinhabers", "")
	y += 20
	mappeText(pdf, y, fmt.Sprintf("Mitgliedsnummer %s · Garten %s · Bitte unterschrieben an den Vorstand zurückgeben.", p.Mitgliedsnr, p.Gartennr), 8.5)
	mappeFuss(pdf, s, s.VereinName+" · Gläubiger-ID "+sepa.GlaeubigerID)
	return pdfBytes(pdf)
}

func (a *App) mandatsformularPDF(r *http.Request) ([]byte, string, error) {
	pid := r.URL.Query().Get("paechter")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !glaeubigerIDGueltig(a.st.d.Sepa.GlaeubigerID) {
		return nil, "", bad("Bitte zuerst die Gläubiger-Identifikationsnummer des Vereins eintragen")
	}
	var p *Paechter
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == pid && !a.st.d.Paechter[i].Geloescht {
			p = &a.st.d.Paechter[i]
		}
	}
	if p == nil {
		return nil, "", notFound("Pächter nicht gefunden")
	}
	m := Mandat{Referenz: a.st.neueMandatsreferenzLocked(*p), Rechnungen: true}
	for _, x := range a.st.d.Mandate {
		if x.PaechterID == p.ID && x.Widerrufen == "" {
			m = x
		}
	}
	pdf, err := buildMandatsformular(a.st.d.Settings, a.st.d.Gemeinnuetzigkeit, a.st.d.Sepa, *p, m)
	return pdf, safeName("SEPA-Mandat_"+m.Referenz+"_"+p.Name, 80) + ".pdf", err
}

// ------------------------------------------------------------------ Vorabankündigung

type vorabEmpfaenger struct {
	P Paechter
	M Mandat
}

func buildVorabankuendigung(s Settings, sepa SepaEinstellungen, jahr int, l []vorabEmpfaenger, heute time.Time) ([]byte, error) {
	if len(l) == 0 {
		return nil, bad("Es gibt keine Mandate mit monatlicher Vorauszahlung")
	}
	pdf := neuesPDF(s, fmt.Sprintf("Vorabankündigung Lastschriften %d", jahr), false)
	mr, mg, mb := markenfarbe(s.Farbe)
	tag := max(1, sepa.Faelligkeitstag)
	for _, e := range l {
		p, m := e.P, e.M
		pdf.AddPage()
		zeichneKopf(pdf, s)
		y := 31.0
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(60, 60, 60)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, rowH, s.Absenderzeile, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 8
		y += adressZeile(pdf, leftX, y, p.Anrede, "")
		y += adressZeile(pdf, leftX, y, p.Name, "B")
		y += adressZeile(pdf, leftX, y, p.Strasse, "")
		y += adressZeile(pdf, leftX, y, p.PLZOrt, "")
		y += 10
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, rowH, "Datum: "+heute.Format("02.01.2006"), "", 0, "R", false, 0, "")
		y += 8
		pdf.SetFont("lib", "B", 13)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 7, fmt.Sprintf("Ankündigung Ihrer Lastschriften %d", jahr), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 10
		y = mappeText(pdf, y, fmt.Sprintf("Liebe Gartenfreundin, lieber Gartenfreund,\n\nwie vereinbart ziehen wir die monatliche Vorauszahlung für Ihren Garten %s "+
			"per SEPA-Lastschrift von Ihrem Konto ein. Die Vorauszahlungen werden mit der Jahresabrechnung verrechnet.", p.Gartennr), 10)
		y = mappeTabelle(pdf, y, [][2]string{
			{"Betrag je Monat", fmtEUR(m.Monatsbetrag)},
			{"Konto", ibanGruppiert(m.IBAN)},
			{"Mandatsreferenz", m.Referenz},
			{"Gläubiger-ID", sepa.GlaeubigerID},
		})
		y = mappeAbschnitt(pdf, s, y, "Termine")
		var termine []string
		for mo := 1; mo <= 12; mo++ {
			d := naechsterBankarbeitstag(time.Date(jahr, time.Month(mo), tag, 0, 0, 0, 0, time.UTC))
			if d.Before(heute) {
				continue
			}
			termine = append(termine, d.Format("02.01.2006"))
		}
		if len(termine) == 0 {
			termine = []string{"keine weiteren Termine in diesem Jahr"}
		}
		y = mappeText(pdf, y, strings.Join(termine, "  ·  "), 10)
		y = mappeText(pdf, y, "Fällt ein Termin auf einen Feiertag, wird am folgenden Bankarbeitstag abgebucht. Den Restbetrag der Jahresabrechnung kündigen wir mit der Rechnung an. "+
			"Bitte sorgen Sie für ausreichende Deckung; eine zurückgegebene Lastschrift kostet Bankgebühren.", 9.5)
		y += 4
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6, "Mit freundlichen Gartengrüßen", "", 0, "L", false, 0, "")
		pdf.SetXY(leftX, y+6)
		pdf.CellFormat(rightX-leftX, 6, "Der Vorstand", "", 0, "L", false, 0, "")
		mappeFuss(pdf, s, s.VereinName+" · Vorabankündigung (Pre-Notification) der SEPA-Lastschriften "+strconv.Itoa(jahr))
	}
	return pdfBytes(pdf)
}

func (a *App) vorabankuendigungPDF(r *http.Request) ([]byte, string, error) {
	jahr := jahrParam(r)
	heute := time.Now()
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if h := a.st.sepaHinweisLocked(); h != "" {
		return nil, "", bad(h)
	}
	personen := map[string]Paechter{}
	for _, p := range a.st.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			personen[p.ID] = p
		}
	}
	var l []vorabEmpfaenger
	for _, m := range a.st.d.Mandate {
		if p, ok := personen[m.PaechterID]; ok && m.aktiv(heute) && m.Monatsbetrag > 0 {
			l = append(l, vorabEmpfaenger{p, m})
		}
	}
	pdf, err := buildVorabankuendigung(a.st.d.Settings, a.st.d.Sepa, jahr, l, heute)
	return pdf, fmt.Sprintf("Vorabankuendigung_Lastschriften_%d.pdf", jahr), err
}
