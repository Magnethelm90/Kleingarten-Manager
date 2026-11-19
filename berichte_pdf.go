package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// PDF-Fassungen der Berichte-Kacheln (Kostenvergleich, Arbeitsstunden, Wartungs-Check, Jahresmeldung), damit sich alles,
// was der Vorstand ansieht, auch ausdrucken oder abheften lässt.

func einheitText(wert float64, einheit string) string {
	zahl := strings.ReplaceAll(strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", wert), "0"), "."), ".", ",")
	switch einheit {
	case "€":
		return fmtEUR(wert)
	case "":
		return zahl
	}
	return zahl + " " + einheit
}

func (a *App) jahresmeldungPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	m, ok := a.st.jahresmeldungLocked(jahr, time.Now())
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Dieses Jahr gibt es nicht")
	}
	pdf := neuesPDF(s, fmt.Sprintf("Jahresmeldung %d", jahr), false)
	zeilen := [][]string{}
	for _, z := range m.Zeilen {
		zeilen = append(zeilen, []string{z.Gruppe, z.Angabe, einheitText(z.Wert, z.Einheit)})
	}
	pdfListe(pdf, s, fmt.Sprintf("Jahresmeldung %d", jahr), "Zahlen für den Landes- oder Regionalverband. Das Formular des Verbands kann andere Angaben verlangen.",
		[]string{"Bereich", "Angabe", "Wert"}, []float64{40, 90, 40}, zeilen, 10)
	b, err := pdfBytes(pdf)
	return b, fmt.Sprintf("Jahresmeldung_%d.pdf", jahr), err
}

func (a *App) stundenkontoPDF(r *http.Request) ([]byte, string, error) {
	jahr := a.yearParam(r)
	a.st.mu.Lock()
	k, ok := a.st.stundenkontoLocked(jahr)
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Dieses Jahr gibt es nicht")
	}
	pdf := neuesPDF(s, fmt.Sprintf("Arbeitsstunden %d", jahr), false)
	zeilen := [][]string{}
	for _, z := range k.Zeilen {
		fehlt, ersatz := "erfüllt", "–"
		if !z.Erfuellt {
			fehlt = stundenText(z.Fehlt)
		}
		if z.Ersatz > 0 {
			ersatz = fmtEUR(z.Ersatz)
		}
		zeilen = append(zeilen, []string{z.Gartennr, z.Name, stundenText(z.Ist), fehlt, ersatz})
	}
	hinweis := fmt.Sprintf("Pflicht: %s Stunden je Garten, je fehlende Stunde %s. %d von %d Gärten haben die Pflicht erfüllt, zusammen %s Stunden. Zu erwartende Ersatzzahlungen: %s.",
		stundenText(k.Pflicht), fmtEUR(k.JeStunde), k.Erfuellt, len(k.Zeilen), stundenText(k.Summe), fmtEUR(k.ErsatzSumme))
	pdfListe(pdf, s, fmt.Sprintf("Arbeitsstunden %d", jahr), hinweis,
		[]string{"Garten", "Name", "Geleistet", "Fehlt", "Ersatzzahlung"}, []float64{20, 70, 25, 25, 30}, zeilen, 10)
	b, err := pdfBytes(pdf)
	return b, fmt.Sprintf("Arbeitsstunden_%d.pdf", jahr), err
}

func (a *App) kostenvergleichPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	k := a.st.kostenvergleichLocked()
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if len(k.Verein) == 0 {
		return nil, "", bad("Noch keine Jahre vorhanden")
	}
	pdf := neuesPDF(s, "Kostenvergleich", false)
	zeilen := [][]string{}
	for i := len(k.Verein) - 1; i >= 0; i-- {
		v := k.Verein[i]
		zeilen = append(zeilen, []string{fmt.Sprint(v.Jahr), einheitText(v.Wasser, "m³"), einheitText(v.Strom, "kWh"), fmtEUR(v.Ausgaben), fmtEUR(v.Rechnung)})
	}
	hinweis := ""
	if len(k.Hinweise) > 0 {
		hinweis = "Auffällig: " + strings.Join(k.Hinweise, "; ")
	}
	pdfListe(pdf, s, "Kostenvergleich über die Jahre", hinweis,
		[]string{"Jahr", "Wasser", "Strom", "Ausgaben", "Rechnungen"}, []float64{20, 38, 38, 38, 38}, zeilen, 10)
	// Verbrauch je Garten (Wasser m³ / Strom kWh), nur Gärten mit Verbrauch oder Hinweis
	kopf := []string{"Garten", "Pächter"}
	breiten := []float64{18, 52}
	for _, j := range k.Jahre {
		kopf = append(kopf, fmt.Sprint(j))
		breiten = append(breiten, 24)
	}
	kopf = append(kopf, "Hinweis")
	breiten = append(breiten, 40)
	garten := [][]string{}
	zahl := func(p *float64) string {
		if p == nil {
			return "–"
		}
		return einheitText(*p, "")
	}
	for _, g := range k.Gaerten {
		z := []string{g.Gartennr, g.Name}
		for _, w := range g.Werte {
			z = append(z, zahl(w.Wasser)+" / "+zahl(w.Strom))
		}
		z = append(z, strings.Join(g.Hinweise, "; "))
		garten = append(garten, z)
	}
	if len(garten) > 0 {
		pdfListe(pdf, s, "Verbrauch je Garten (Wasser m³ / Strom kWh)", "", kopf, breiten, garten, 8.5)
	}
	b, err := pdfBytes(pdf)
	return b, "Kostenvergleich.pdf", err
}

func (a *App) wartungPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	res := a.wartungLocked(time.Now())
	s := a.st.d.Settings
	a.st.mu.Unlock()
	pdf := neuesPDF(s, "Wartungs-Check", false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	y := mappeAbschnitt(pdf, s, 36, "Wartungs-Check")
	pdf.SetFont("lib", "", 8.5)
	pdf.SetTextColor(90, 90, 90)
	pdf.SetXY(leftX, y-2)
	pdf.MultiCell(rightX-leftX, 4.2, "Stand "+time.Now().Format("02.01.2006 15:04")+" Uhr. Prüft Datenordner, Sicherungen, zweiten Ordner, Passwort und Programmversion.", "", "L", false)
	y = pdf.GetY() + 4
	pdf.SetTextColor(0, 0, 0)
	stufen := map[string]string{"ok": "in Ordnung", "warn": "Hinweis", "err": "Handlungsbedarf"}
	farbe := map[string][3]int{"ok": {29, 107, 52}, "warn": {138, 90, 0}, "err": {179, 38, 30}}
	for _, z := range res.Zeilen {
		pdf.SetFont("lib", "", 9.5)
		// Platzbedarf des Textes vorab ermitteln, damit eine Zeile nicht über den Seitenrand läuft
		zeilen := pdf.SplitText(z.Text, rightX-leftX-50)
		bedarf := 6 + float64(len(zeilen))*4.6 + 3
		if y+bedarf > 270 {
			mappeFuss(pdf, s, s.VereinName+" · Wartungs-Check · Stand "+time.Now().Format("02.01.2006"))
			pdf.AddPage()
			zeichneKopf(pdf, s)
			y = 36
		}
		pdf.SetFont("lib", "B", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(50, 5.5, z.Titel, "", 0, "L", false, 0, "")
		c := farbe[z.Stufe]
		pdf.SetTextColor(c[0], c[1], c[2])
		pdf.CellFormat(40, 5.5, stufen[z.Stufe], "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "", 9.5)
		pdf.SetXY(leftX+50, y+5.5)
		pdf.MultiCell(rightX-leftX-50, 4.6, z.Text, "", "L", false)
		y = pdf.GetY() + 3
		pdf.SetDrawColor(215, 220, 210)
		pdf.SetLineWidth(0.2)
		pdf.Line(leftX, y-1, rightX, y-1)
	}
	mappeFuss(pdf, s, s.VereinName+" · Wartungs-Check · Stand "+time.Now().Format("02.01.2006"))
	b, err := pdfBytes(pdf)
	return b, "Wartungs-Check.pdf", err
}
