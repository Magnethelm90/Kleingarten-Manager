package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/go-pdf/fpdf"
	qrcode "github.com/skip2/go-qrcode"
)

// Eingebettete Schrift (Liberation Sans, SIL Open Font License, siehe fonts/LICENSE-Liberation.txt).
// Sie ist Arial im Aussehen sehr ähnlich, deckt auch Sonderzeichen (ř, ł, ő …) ab und sieht auf jedem PC gleich aus.
//
//go:embed fonts/LiberationSans-Regular.ttf
var fontRegular []byte

//go:embed fonts/LiberationSans-Bold.ttf
var fontBold []byte

// Spaltenraster (mm), entspricht dem Raster der Excel-Rechnung.
var (
	colX = [7]float64{20, 58.8, 87.1, 105, 130.4, 148.3, 169.2}
	colW = [7]float64{38.8, 28.3, 17.9, 25.4, 17.9, 20.9, 20.8}
)

const (
	leftX  = 20.0
	rightX = 190.0
	rowH   = 5.1
)

// adressZeile schreibt eine Zeile des Anschriftenfelds (höchstens 90 mm breit, wie ein Fensterbrief es
// verlangt). Passt der Text nicht, wird die Schrift bis 8 pt verkleinert, danach auf zwei Zeilen umbrochen,
// damit er nie über den Rand läuft. Gibt die belegte Höhe zurück (mindestens eine Zeile).
func adressZeile(pdf *fpdf.Fpdf, x, y float64, txt, style string) float64 {
	const maxW = 90.0
	size := 10.0
	pdf.SetFont("lib", style, size)
	for size > 8 && pdf.GetStringWidth(txt) > maxW {
		size -= 0.5
		pdf.SetFont("lib", style, size)
	}
	pdf.SetXY(x, y)
	if pdf.GetStringWidth(txt) <= maxW {
		pdf.CellFormat(maxW, rowH, txt, "", 0, "L", false, 0, "")
		pdf.SetFont("lib", style, 10)
		return rowH
	}
	pdf.SetFont("lib", style, 9)
	pdf.MultiCell(maxW, 4.4, txt, "", "L", false)
	h := pdf.GetY() - y + 0.7
	pdf.SetFont("lib", style, 10)
	if h < rowH {
		h = rowH
	}
	return h
}

// invoiceNumber bildet die Rechnungsnummer, z. B. 100-35-95.
func invoiceNumber(s Settings, p Paechter) string {
	return strings.TrimSpace(s.RechnungsnrPraefix) + "-" + strings.TrimSpace(p.Mitgliedsnr)
}

// epcQRPayload baut den Text für den GiroCode (EPC069-12 / SEPA-Überweisung).
// Banking-Apps lesen daraus IBAN, Betrag und Verwendungszweck aus und füllen
// die Überweisung automatisch aus.
func epcQRPayload(s Settings, p Paechter, betrag float64) string {
	iban := strings.ReplaceAll(strings.TrimSpace(s.IBAN), " ", "")
	bic := strings.ReplaceAll(strings.TrimSpace(s.BIC), " ", "")
	name := trim(s.VereinName, 70)
	lines := []string{
		"BCD", "002", "1", "SCT", bic, name, iban,
		fmt.Sprintf("EUR%.2f", betrag),
		"", "", "Rechnung " + invoiceNumber(s, p),
	}
	return strings.Join(lines, "\n")
}

// ibanGueltig prüft Länge, Zeichen und die Prüfziffer (Modulo 97) einer IBAN.
func ibanGueltig(iban string) bool {
	iban = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(iban), " ", ""))
	if len(iban) < 15 || len(iban) > 34 {
		return false
	}
	if iban[0] < 'A' || iban[0] > 'Z' || iban[1] < 'A' || iban[1] > 'Z' || iban[2] < '0' || iban[2] > '9' || iban[3] < '0' || iban[3] > '9' {
		return false
	}
	rest := 0
	for _, c := range iban[4:] + iban[:4] {
		switch {
		case c >= '0' && c <= '9':
			rest = (rest*10 + int(c-'0')) % 97
		case c >= 'A' && c <= 'Z':
			rest = (rest*100 + int(c-'A') + 10) % 97
		default:
			return false
		}
	}
	return rest == 1
}

// invoiceMaxY ist die tiefste erlaubte Position (Oberkante) der letzten Zeile der Rechnung.
const invoiceMaxY = 290.0

// buildInvoice erzeugt die Rechnung auf einer A4-Seite. Das Standardlayout bleibt unverändert;
// nur wenn ein längerer Hinweistext die Seite sprengen würde, werden einige Leerabstände im
// oberen Teil schrittweise verkleinert, bis alles passt.
func buildInvoice(s Settings, p Paechter, a Ablesung, r Result) ([]byte, error) {
	_, out, err := layoutFit(s, p, a, r, true)
	return out, err
}

// layoutFit ermittelt, wie stark die Leerabstände verkleinert werden müssen, damit die Rechnung auf
// eine Seite passt (0 = Standardlayout). Mit wantPDF wird die fertige PDF des erfolgreichen Versuchs
// gleich mitgeliefert, sonst nur gezeichnet (günstiger, z. B. für Sammeldateien).
func layoutFit(s Settings, p Paechter, a Ablesung, r Result, wantPDF bool) (float64, []byte, error) {
	if !r.Vollstaendig {
		return 0, nil, errors.New("Rechnung unvollständig: " + r.Status)
	}
	for squeeze := 0.0; squeeze <= 1.0001; squeeze += 0.1 {
		pdf := newInvoiceDoc("Rechnung "+invoiceNumber(s, p), s)
		if ende := drawInvoicePage(pdf, s, p, a, r, squeeze); ende > invoiceMaxY {
			continue
		}
		if !wantPDF {
			return squeeze, nil, nil
		}
		var buf bytes.Buffer
		if err := pdf.Output(&buf); err != nil {
			return 0, nil, err
		}
		return squeeze, buf.Bytes(), nil
	}
	return 0, nil, errors.New("Rechnung passt nicht auf eine Seite (Hinweistext zu lang?)")
}

// invoiceItem sind die Angaben einer Rechnung, wie sie im Archiv festgeschrieben sind.
type invoiceItem struct {
	S Settings
	P Paechter
	A Ablesung
	R Result
}

// buildInvoicesCombined setzt mehrere Rechnungen (je eine Seite) in einer PDF-Datei zusammen,
// z. B. zum Ausdrucken aller Postsendungen in einem Rutsch.
func buildInvoicesCombined(items []invoiceItem, title string) ([]byte, error) {
	if len(items) == 0 {
		return nil, errors.New("keine Rechnungen zum Zusammenstellen")
	}
	pdf := newInvoiceDoc(title, items[0].S)
	for _, it := range items {
		sq, _, err := layoutFit(it.S, it.P, it.A, it.R, false)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", invoiceNumber(it.S, it.P), err)
		}
		drawInvoicePage(pdf, it.S, it.P, it.A, it.R, sq)
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func newInvoiceDoc(title string, s Settings) *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(title, true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	return pdf
}

// renderInvoice zeichnet eine einzelne Rechnung; squeeze (0 bis 1) verkleinert die Leerabstände
// zwischen den Blöcken im Kopfbereich. Gibt zusätzlich die Position der letzten Zeile zurück.
func renderInvoice(s Settings, p Paechter, a Ablesung, r Result, squeeze float64) ([]byte, float64, error) {
	pdf := newInvoiceDoc("Rechnung "+invoiceNumber(s, p), s)
	y := drawInvoicePage(pdf, s, p, a, r, squeeze)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), y, nil
}

// drawInvoicePage fügt dem Dokument eine Seite mit der Rechnung hinzu und gibt die Position
// der letzten Zeile zurück.
func drawInvoicePage(pdf *fpdf.Fpdf, s Settings, p Paechter, a Ablesung, r Result, squeeze float64) float64 {
	gap := func(mm float64) float64 { return mm * (1 - 0.8*squeeze) }
	tr := func(s string) string { return s }
	pdf.AddPage()

	y := 0.0
	font := func(style string, size float64) { pdf.SetFont("lib", style, size) }
	text := func(x, w float64, txt, align string) {
		pdf.SetXY(x, y)
		pdf.CellFormat(w, rowH, tr(txt), "", 0, align, false, 0, "")
	}
	// Zelle im Spaltenraster: von Spalte a bis einschließlich Spalte b
	span := func(a, b int, txt, align string) {
		text(colX[a], colX[b]+colW[b]-colX[a], txt, align)
	}
	hline := func(at float64, width float64) {
		pdf.SetLineWidth(width)
		pdf.SetDrawColor(90, 90, 90)
		pdf.Line(leftX, at, rightX, at)
	}
	next := func(h float64) { y += h }

	// Kopf
	pdf.SetLineWidth(0.7)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(leftX, 14, rightX, 14)
	y = 17
	font("B", 14)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 8, tr(s.VereinName+" "+s.Ort), "", 0, "C", false, 0, "")
	hline(27, 0.25)

	y = 31
	font("", 7.5)
	pdf.SetTextColor(60, 60, 60)
	text(leftX, rightX-leftX, s.Absenderzeile, "L")
	pdf.SetTextColor(0, 0, 0)
	next(6)
	font("B", 10)
	text(leftX, rightX-leftX, p.Versand, "R")

	// Anschrift
	next(gap(5))
	next(adressZeile(pdf, leftX, y, p.Anrede, ""))
	next(adressZeile(pdf, leftX, y, p.Name, "B"))
	next(adressZeile(pdf, leftX, y, p.Strasse, ""))
	next(adressZeile(pdf, leftX, y, p.PLZOrt, "") - rowH)

	// Rechnungsdaten rechts
	next(rowH + gap(4))
	meta := [][2]string{
		{"Mitgliedsnr.:", p.Mitgliedsnr},
		{"Rechnungsdatum:", germanDate(s.Rechnungsdatum)},
		{"Rechnungsnr.:", invoiceNumber(s, p)},
		{"Gartennr.:", p.Gartennr},
	}
	for _, m := range meta {
		font("", 10)
		pdf.SetXY(115, y)
		pdf.CellFormat(50, rowH, tr(m[0]), "", 0, "R", false, 0, "")
		font("B", 10)
		pdf.SetXY(165, y)
		pdf.CellFormat(25, rowH, tr(m[1]), "", 0, "R", false, 0, "")
		next(rowH)
	}

	// Überschriften
	next(gap(5))
	font("B", 12)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, tr(fmt.Sprintf("Rechnung Jahresendabrechnung %d", s.Jahr)), "", 0, "L", false, 0, "")
	next(7)
	font("BU", 10)
	text(leftX, rightX-leftX, "1. Jahresverbrauch Wasser/Energie, geleistete Arbeitsstunden", "L")
	next(rowH + 1)

	// Verbrauchsblock (Wasser bzw. Energie)
	block := func(title, zaehlerNr string, vj, akt *float64, verbrauch *float64, unit, gpLabel, preisLabel string, gp, preis, summe float64, sumLabel string) {
		font("B", 10)
		text(colX[0], colW[0], title, "L")
		font("", 10)
		span(1, 1, "ZSt VJ:", "R")
		span(2, 2, fmtFlex(*vj), "R")
		span(3, 3, "ZSt Akt.:", "R")
		span(4, 4, fmtFlex(*akt), "R")
		span(5, 5, "Ges.-Verbr.:", "R")
		font("B", 10)
		span(6, 6, fmtFlex(*verbrauch)+" "+unit, "R")
		next(rowH)
		if strings.TrimSpace(zaehlerNr) != "" {
			font("", 8)
			pdf.SetTextColor(70, 70, 70)
			text(colX[0], colW[0], "Zähler-Nr.: "+zaehlerNr, "L")
			pdf.SetTextColor(0, 0, 0)
		}
		font("", 10)
		span(1, 1, gpLabel, "R")
		span(2, 2, fmtEUR(gp), "R")
		span(3, 3, preisLabel, "R")
		span(4, 4, fmtEUR(preis), "R")
		span(5, 5, sumLabel, "R")
		font("B", 10)
		span(6, 6, fmtEUR(summe), "R")
		next(rowH + 1.2)
		hline(y-0.6, 0.2)
		next(1.4)
	}
	block("1.1. Wasserverbrauch:", p.WasserzaehlerNr, a.WasserVJ, a.WasserAkt, r.WasserVerbrauch, "m³",
		"Grundpreis:", "Preis je m³:", s.WasserGrundpreis, s.WasserPreis, r.KostenWasser, "Summe:")
	block("1.2. Energieverbrauch:", p.StromzaehlerNr, a.StromVJ, a.StromAkt, r.EnergieVerbrauch, "kWh",
		"Grundpreis:", "Preis je kWh:", s.EnergieGrundpreis, s.EnergiePreis, r.KostenEnergie, "Summe:")

	// Arbeitsstunden
	font("", 10)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 4.8, tr(fmt.Sprintf(
		"1.3. Arbeitsstunden: laut der Beitrags- und Gebührenordnung sind %s Pflichtstunden zu erbringen. "+
			"Bis zum Ende der Saison erbrachte Arbeitsstunden: %s h.", fmtFlex(s.Pflichtstunden), fmtFlex(*a.Stunden))), "", "L", false)
	y = pdf.GetY() + 1
	text(colX[0], 80, "Beitrag für nicht erbrachte Pflichtstunden: "+fmtEUR(r.NachzahlungStunden), "L")
	span(4, 5, "Zwischensumme:", "R")
	font("B", 10)
	span(6, 6, fmtEUR(r.Zwischensumme1), "R")
	next(rowH + 1)
	hline(y, 0.2)
	next(2)

	// Beiträge
	font("BU", 10)
	text(leftX, rightX-leftX, "2. Beitragszahlung für das Folgejahr", "L")
	next(rowH + 0.6)
	pacht := func(label string, qm, betrag float64) {
		font("", 10)
		span(0, 0, label, "L")
		span(1, 2, fmtFlex(qm)+" m²", "R")
		span(3, 3, "Pachtbetrag:", "R")
		span(4, 4, fmtEUR(betrag), "R")
		next(rowH)
	}
	pacht("Gartengröße:", p.Gartengroesse, r.PachtGarten)
	pacht("Anteil Vereinsfläche:", s.VereinsflaecheQm, r.PachtVerein)
	pacht("Pachtanteil freie Gärten:", s.FreieGaertenQm, r.PachtFrei)
	next(0.6)
	hline(y, 0.2)
	next(1.4)

	font("", 10)
	span(0, 0, "Vereinsbeitrag:", "L")
	span(1, 1, fmtEUR(s.Vereinsbeitrag), "R")
	next(rowH)
	span(0, 0, "Territorialverband:", "L")
	span(1, 1, fmtEUR(s.Territorialverband), "R")
	span(3, 5, "Gesamt Mitgliedsbeitrag:", "R")
	span(6, 6, fmtEUR(r.Mitgliedsbeitrag), "R")
	next(rowH + 0.6)
	hline(y, 0.2)
	next(1.4)

	posten := func(label string, v float64) {
		font("", 10)
		span(0, 0, label, "L")
		if v != 0 {
			span(6, 6, fmtEUR(v), "R")
		}
		next(rowH)
	}
	posten("Versicherung:", a.Versicherung)
	posten("Umlage:", r.Umlage)
	posten("Grundsteuer:", a.Grundsteuer)
	posten("Sonstige Auslagen:", a.Auslagen)
	font("U", 8.5)
	text(leftX, 40, "Hinweis/Erläuterung:", "L")
	hinweisH := rowH
	if h := strings.TrimSpace(a.Hinweis); h != "" {
		font("", 9)
		pdf.SetXY(colX[1], y)
		pdf.MultiCell(rightX-colX[1], 4.4, tr(h), "", "L", false)
		if hh := pdf.GetY() - y; hh > hinweisH {
			hinweisH = hh
		}
	}
	next(hinweisH + 1)
	font("", 10)
	span(4, 5, "Zwischensumme:", "R")
	font("B", 10)
	span(6, 6, fmtEUR(r.Zwischensumme2), "R")
	next(rowH + 0.8)
	hline(y, 0.2)
	next(1.6)

	// Abzüge und Gesamtbetrag
	font("", 10)
	span(0, 5, "Abzüglich Vergütung von Arbeitsstunden / Aufwandsentschädigung:", "L")
	if r.Verguetung != 0 {
		span(6, 6, fmtEUR(-r.Verguetung), "R")
	}
	next(rowH)
	span(0, 5, "Abzüglich Abschlagszahlung:", "L")
	if a.Abschlag != 0 {
		span(6, 6, fmtEUR(-a.Abschlag), "R")
	}
	next(rowH + 1)
	guthaben := r.Gesamt < 0
	label := "Gesamtbetrag:"
	if guthaben {
		label = "Guthaben (zu Ihren Gunsten):"
	}
	font("B", 11)
	pdf.SetXY(colX[3], y)
	pdf.CellFormat(colX[5]+colW[5]-colX[3], 6, tr(label), "", 0, "R", false, 0, "")
	pdf.SetXY(colX[6]-6, y)
	pdf.CellFormat(colW[6]+6, 6, tr(fmtEUR(absf(r.Gesamt))), "TB", 0, "R", false, 0, "")
	next(6 + 5)

	// Zahlungshinweis
	einspruch := fmt.Sprintf("Bei Unstimmigkeiten der Rechnung sind diese bis %d Tage nach Eingang beim Vorstand anzuzeigen. "+
		"Nach Ablauf dieser Frist gilt die Rechnung als angenommen.", s.EinspruchTage)
	var pay string
	if guthaben || r.Gesamt == 0 {
		pay = "Es ergibt sich ein Guthaben zu Ihren Gunsten. " + einspruch
	} else {
		pay = fmt.Sprintf("Der Gesamtbetrag ist bis zum %s auf das Konto der %s, IBAN: %s; BIC: %s, Verwendungszweck: %s (*), zu überweisen. %s "+
			"Bei Verspätung der Überweisung werden Mahngebühren bzw. Verzugszinsen angerechnet.",
			germanDate(s.Zahlungsziel), s.BankName, s.IBAN, s.BIC, invoiceNumber(s, p), einspruch)
	}
	// GiroCode (QR-Code zum Bezahlen): die Banking-App liest IBAN, Betrag und
	// Verwendungszweck daraus und füllt die Überweisung automatisch aus.
	const qrSize = 24.0
	textW := rightX - leftX
	var qrPNG []byte
	if !guthaben && r.Gesamt != 0 && ibanGueltig(s.IBAN) {
		if png, err := qrcode.Encode(epcQRPayload(s, p, absf(r.Gesamt)), qrcode.Medium, 300); err == nil {
			qrPNG = png
			textW = rightX - leftX - qrSize - 5
		}
	}
	qrTop := y
	font("", 10)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(textW, 4.7, tr(pay), "", "L", false)
	y = pdf.GetY()
	if qrPNG != nil {
		// eigener Name je Seite: fpdf legt Bilder nach Namen ab, ein gemeinsamer Name würde in einer
		// Sammeldatei den QR-Code (und damit den Betrag) der ersten Rechnung auf alle Seiten übertragen
		imgName := fmt.Sprintf("girocode-%d", pdf.PageNo())
		pdf.RegisterImageOptionsReader(imgName, fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))
		pdf.ImageOptions(imgName, rightX-qrSize, qrTop, qrSize, qrSize, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		font("", 6.5)
		pdf.SetXY(rightX-qrSize, qrTop+qrSize+0.6)
		pdf.CellFormat(qrSize, 3, tr("GiroCode zum Bezahlen"), "", 0, "C", false, 0, "")
		if b := qrTop + qrSize + 3; b > y {
			y = b
		}
	}
	y += 2
	if !guthaben && r.Gesamt != 0 {
		text(leftX, rightX-leftX, "(*) Bei Zahlungsvorgängen bitte angeben!", "L")
		next(rowH + 3)
	}
	text(leftX, rightX-leftX, "Der Vorstand", "L")
	next(rowH + 3)
	font("", 8)
	text(leftX, rightX-leftX, "Die Rechnung wird maschinell erstellt und ist ohne Unterschrift gültig.", "L")

	return y
}

// buildMahnung erzeugt eine einfache Zahlungserinnerung für eine offene,
// bereits ausgestellte Rechnung (nutzt dieselbe Kopfzeile und denselben
// GiroCode wie die Rechnung selbst).
func buildMahnung(s Settings, p Paechter, rec *Rechnung) ([]byte, error) {
	offen := openAmount(rec)
	if offen <= 0 {
		return nil, errors.New("diese Rechnung ist bereits vollständig bezahlt")
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	tr := func(s string) string { return s }
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(tr("Zahlungserinnerung "+invoiceNumber(s, p)), true)
	pdf.SetAuthor(tr(s.VereinName), true)
	pdf.SetCreator(appName, true)
	pdf.AddPage()

	y := 0.0
	font := func(style string, size float64) { pdf.SetFont("lib", style, size) }
	text := func(x, w float64, txt, align string) {
		pdf.SetXY(x, y)
		pdf.CellFormat(w, rowH, tr(txt), "", 0, align, false, 0, "")
	}
	hline := func(at float64, width float64) {
		pdf.SetLineWidth(width)
		pdf.SetDrawColor(90, 90, 90)
		pdf.Line(leftX, at, rightX, at)
	}
	next := func(h float64) { y += h }

	pdf.SetLineWidth(0.7)
	pdf.SetDrawColor(0, 0, 0)
	pdf.Line(leftX, 14, rightX, 14)
	y = 17
	font("B", 14)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 8, tr(s.VereinName+" "+s.Ort), "", 0, "C", false, 0, "")
	hline(27, 0.25)

	y = 31
	font("", 7.5)
	pdf.SetTextColor(60, 60, 60)
	text(leftX, rightX-leftX, s.Absenderzeile, "L")
	pdf.SetTextColor(0, 0, 0)
	next(6)
	font("B", 10)
	text(leftX, rightX-leftX, p.Versand, "R")

	next(5)
	next(adressZeile(pdf, leftX, y, p.Anrede, ""))
	next(adressZeile(pdf, leftX, y, p.Name, "B"))
	next(adressZeile(pdf, leftX, y, p.Strasse, ""))
	next(adressZeile(pdf, leftX, y, p.PLZOrt, "") - rowH)

	next(rowH + 4)
	meta := [][2]string{
		{"Mitgliedsnr.:", p.Mitgliedsnr},
		{"Rechnungsnr.:", invoiceNumber(s, p)},
		{"Rechnungsdatum:", germanDate(s.Rechnungsdatum)},
		{"Zahlungsziel war:", germanDate(s.Zahlungsziel)},
	}
	for _, m := range meta {
		font("", 10)
		pdf.SetXY(115, y)
		pdf.CellFormat(50, rowH, tr(m[0]), "", 0, "R", false, 0, "")
		font("B", 10)
		pdf.SetXY(165, y)
		pdf.CellFormat(25, rowH, tr(m[1]), "", 0, "R", false, 0, "")
		next(rowH)
	}

	next(6)
	font("B", 13)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 7, tr("Zahlungserinnerung"), "", 0, "L", false, 0, "")
	next(9)
	font("", 10)
	bezahlt := paidAmount(rec)
	var body string
	if bezahlt > 0 {
		body = fmt.Sprintf("Für die Rechnung %s vom %s ist bislang ein Teilbetrag von %s eingegangen. Der restliche Betrag von %s ist bis heute nicht bei uns eingegangen.",
			invoiceNumber(s, p), germanDate(s.Rechnungsdatum), fmtEUR(bezahlt), fmtEUR(offen))
	} else {
		body = fmt.Sprintf("Für die Rechnung %s vom %s über %s ist bei uns noch kein Zahlungseingang zu verzeichnen.",
			invoiceNumber(s, p), germanDate(s.Rechnungsdatum), fmtEUR(offen))
	}
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 5.2, tr(body), "", "L", false)
	y = pdf.GetY() + 3
	hline(y, 0.2)
	next(3)

	font("B", 11)
	text(leftX, 90, "Noch offener Betrag:", "L")
	pdf.SetXY(colX[6]-6, y)
	pdf.CellFormat(colW[6]+6, 6, tr(fmtEUR(offen)), "TB", 0, "R", false, 0, "")
	next(6 + 5)

	const qrSize = 24.0
	textW := rightX - leftX
	var qrPNG []byte
	if strings.TrimSpace(s.IBAN) != "" {
		if png, err := qrcode.Encode(epcQRPayload(s, p, offen), qrcode.Medium, 300); err == nil {
			qrPNG = png
			textW = rightX - leftX - qrSize - 5
		}
	}
	pay := fmt.Sprintf("Bitte den offenen Betrag von %s umgehend auf das Konto der %s, IBAN: %s; BIC: %s, Verwendungszweck: %s (*) überweisen. "+
		"Sollte die Zahlung zwischenzeitlich bereits erfolgt sein, betrachten Sie dieses Schreiben als gegenstandslos.",
		fmtEUR(offen), s.BankName, s.IBAN, s.BIC, invoiceNumber(s, p))
	qrTop := y
	font("", 10)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(textW, 4.7, tr(pay), "", "L", false)
	y = pdf.GetY()
	if qrPNG != nil {
		pdf.RegisterImageOptionsReader("girocode", fpdf.ImageOptions{ImageType: "PNG"}, bytes.NewReader(qrPNG))
		pdf.ImageOptions("girocode", rightX-qrSize, qrTop, qrSize, qrSize, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		font("", 6.5)
		pdf.SetXY(rightX-qrSize, qrTop+qrSize+0.6)
		pdf.CellFormat(qrSize, 3, tr("GiroCode zum Bezahlen"), "", 0, "C", false, 0, "")
		if b := qrTop + qrSize + 3; b > y {
			y = b
		}
	}
	y += 2
	text(leftX, rightX-leftX, "(*) Bei Zahlungsvorgängen bitte angeben!", "L")
	next(rowH + 3)
	text(leftX, rightX-leftX, "Der Vorstand", "L")
	next(rowH + 3)
	font("", 8)
	text(leftX, rightX-leftX, "Die Zahlungserinnerung wird maschinell erstellt und ist ohne Unterschrift gültig.", "L")

	if y > 290 {
		return nil, errors.New("Mahnung passt nicht auf eine Seite")
	}
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func absf(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
