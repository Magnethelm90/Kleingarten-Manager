package main

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/go-pdf/fpdf"
)

// Jahresbericht: eine Zusammenfassung des Abrechnungsjahres für die Mitgliederversammlung – Kennzahlen mit
// Vorjahresvergleich, Kassenbericht, Ausgaben nach Kategorie, Kassenbestand im Verlauf, Vereinsleben und die
// kommenden Termine. Enthält bewusst keine Namen, damit er ausgelegt oder an die Wand projiziert werden kann.

type jahresberichtDaten struct {
	Settings    Settings
	Jahr        int
	K           kassenbericht
	VK          *kassenbericht // Vorjahr, falls vorhanden
	Paechter    int
	Flaeche     float64
	Stunden     float64
	MitStunden  int
	PflichtOK   int
	Offen       float64
	Verlauf     []kassenberichtJahr // ältestes zuerst, höchstens 8 Jahre
	Uebergaben  []Uebergabe
	Warteliste  int
	Termine     []Termin
	ErstelltAm  time.Time
	VorjahrJahr int
}

func (s *Store) jahresberichtLocked(year int, jetzt time.Time) (jahresberichtDaten, bool) {
	v, ok := s.viewLocked(year)
	if !ok {
		return jahresberichtDaten{}, false
	}
	d := jahresberichtDaten{Settings: v.Settings, Jahr: year, K: s.kassenberichtMitVorjahrLocked(v), ErstelltAm: jetzt, VorjahrJahr: year - 1}
	if vv, ok := s.viewLocked(year - 1); ok {
		vk := s.kassenberichtLocked(vv)
		d.VK = &vk
	}
	for _, p := range v.Paechter {
		if p.AusgeschiedenAm != "" {
			continue // der Garten zählt beim Nachfolger
		}
		d.Paechter++
		d.Flaeche += p.Gartengroesse
		if a := v.Ablesungen[p.ID]; a.Stunden != nil {
			d.Stunden += *a.Stunden
			d.MitStunden++
			if *a.Stunden >= v.Settings.Pflichtstunden {
				d.PflichtOK++
			}
		}
	}
	for _, r := range s.d.Rechnungen {
		if r.Jahr == year && r.Status == statusGueltig && r.Result.Gesamt > 0 && !r.Bereinigt {
			d.Offen += openAmount(r)
		}
	}
	d.Offen = round2(d.Offen)
	verlauf := s.kassenberichtVerlaufLocked()
	for i := len(verlauf) - 1; i >= 0; i-- {
		if verlauf[i].Jahr <= year {
			d.Verlauf = append(d.Verlauf, verlauf[i])
		}
	}
	if len(d.Verlauf) > 8 {
		d.Verlauf = d.Verlauf[len(d.Verlauf)-8:]
	}
	for _, u := range s.d.Uebergaben {
		if u.Jahr == year {
			d.Uebergaben = append(d.Uebergaben, u)
		}
	}
	sort.SliceStable(d.Uebergaben, func(i, j int) bool { return d.Uebergaben[i].Datum < d.Uebergaben[j].Datum })
	d.Warteliste = len(s.d.Warteliste)
	heute := jetzt.Format("2006-01-02")
	for _, t := range s.d.Termine {
		if t.Datum >= heute && len(d.Termine) < 8 {
			d.Termine = append(d.Termine, t)
		}
	}
	return d, true
}

// veraenderung beschreibt die Änderung gegenüber dem Vorjahr, z. B. »+12 % ggü. 2024«.
func veraenderung(akt, vor float64, vorjahr int) string {
	if vor == 0 || akt == 0 {
		return "" // ohne Werte (z. B. Jahr noch nicht abgelesen) wäre »−100 %« irreführend
	}
	p := (akt - vor) / math.Abs(vor) * 100
	if math.Abs(p) < 0.5 {
		return fmt.Sprintf("wie %d", vorjahr)
	}
	return fmt.Sprintf("%+.0f %% ggü. %d", p, vorjahr)
}

func buildJahresbericht(d jahresberichtDaten) ([]byte, error) {
	s := d.Settings
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(fmt.Sprintf("Jahresbericht %d", d.Jahr), true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AliasNbPages("{nb}")
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	lr, lg, lb := aufgehellt(mr, mg, mb, 0.55)
	pdf.SetFooterFunc(func() {
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(leftX, 287)
		pdf.CellFormat(rightX-leftX, 4, fmt.Sprintf("%s · Jahresbericht %d · Stand %s · Seite %d von {nb}", s.VereinName, d.Jahr,
			d.ErstelltAm.Format("02.01.2006"), pdf.PageNo()), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	y := 0.0
	neueSeite := func() {
		pdf.AddPage()
		zeichneKopf(pdf, s)
		y = 34
	}
	platz := func(h float64) {
		if y+h > 280 {
			neueSeite()
		}
	}
	ueberschrift := func(t string) {
		platz(16)
		pdf.SetFont("lib", "B", 12)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 7, t, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 9
	}

	neueSeite()
	pdf.SetFont("lib", "B", 20)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 10, fmt.Sprintf("Jahresbericht %d", d.Jahr), "", 0, "L", false, 0, "")
	pdf.SetTextColor(90, 96, 90)
	pdf.SetFont("lib", "", 10)
	y += 11
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 5, "Für die Mitgliederversammlung · Zahlen nach dem heutigen Stand der Abrechnung", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 11

	// Kennzahlen als Kacheln, drei je Reihe
	type kachel struct{ wert, titel, unten string }
	vk := d.VK
	vor := func(f func(k kassenbericht) float64) float64 {
		if vk == nil {
			return 0
		}
		return f(*vk)
	}
	durchschnitt := 0.0
	if d.MitStunden > 0 {
		durchschnitt = d.Stunden / float64(d.MitStunden)
	}
	bezahltQuote := ""
	if d.K.Summen.Gesamt > 0 {
		bezahltQuote = fmt.Sprintf("%.0f %% der Rechnungssumme", math.Min(100, d.K.EinnahmenBezahlt/d.K.Summen.Gesamt*100))
	}
	kacheln := []kachel{
		{fmt.Sprintf("%d", d.Paechter), "Gärten verpachtet", fmtFlex(d.Flaeche) + " m² Gartenfläche"},
		{fmtFlex(d.K.WasserVerbrauch) + " m³", "Wasserverbrauch", veraenderung(d.K.WasserVerbrauch, vor(func(k kassenbericht) float64 { return k.WasserVerbrauch }), d.VorjahrJahr)},
		{fmtNum(d.K.StromVerbrauch, 0) + " kWh", "Stromverbrauch", veraenderung(d.K.StromVerbrauch, vor(func(k kassenbericht) float64 { return k.StromVerbrauch }), d.VorjahrJahr)},
		{fmtFlex(d.Stunden) + " h", "Gemeinschaftsarbeit", fmt.Sprintf("Ø %s h je Garten · Pflicht erfüllt: %d von %d", fmtFlex(math.Round(durchschnitt*10)/10), d.PflichtOK, d.MitStunden)},
		{fmtEUR(d.K.Summen.Gesamt), "In Rechnung gestellt", veraenderung(d.K.Summen.Gesamt, vor(func(k kassenbericht) float64 { return k.Summen.Gesamt }), d.VorjahrJahr)},
		{fmtEUR(d.K.EinnahmenBezahlt), "Davon eingegangen", bezahltQuote},
	}
	kw := (rightX - leftX - 2*5) / 3
	for i, k := range kacheln {
		x := leftX + float64(i%3)*(kw+5)
		if i%3 == 0 && i > 0 {
			y += 27
		}
		pdf.SetFillColor(fr, fg, fb)
		pdf.RoundedRect(x, y, kw, 23, 2, "1234", "F")
		pdf.SetFillColor(mr, mg, mb)
		pdf.Rect(x, y+3, 1.2, 17, "F")
		pdf.SetFont("lib", "", 8.5)
		pdf.SetTextColor(90, 96, 90)
		pdf.SetXY(x+4, y+2.5)
		pdf.CellFormat(kw-6, 4.5, k.titel, "", 0, "L", false, 0, "")
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(x+4, y+7.5)
		pdf.CellFormat(kw-6, 7.5, passendKleinStil(pdf, "B", k.wert, kw-7, 15, 10), "", 0, "L", false, 0, "")
		pdf.SetTextColor(90, 96, 90)
		pdf.SetXY(x+4, y+16)
		pdf.CellFormat(kw-6, 4.5, passendKlein(pdf, k.unten, kw-7, 7.5, 6), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	}
	y += 33

	// Kasse
	ueberschrift("Kasse")
	zeile := func(label, wert, vorWert string, fett bool) {
		stil := ""
		if fett {
			stil = "B"
		}
		pdf.SetFont("lib", stil, 10)
		pdf.SetXY(leftX+2, y)
		pdf.CellFormat(100, 6.5, label, "", 0, "L", false, 0, "")
		pdf.SetXY(leftX+100, y)
		pdf.CellFormat(35, 6.5, wert, "", 0, "R", false, 0, "")
		pdf.SetFont("lib", "", 9)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(leftX+135, y)
		pdf.CellFormat(rightX-leftX-137, 6.5, vorWert, "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 6.5
	}
	vorEUR := func(f func(k kassenbericht) float64) string {
		if vk == nil {
			return ""
		}
		return fmtEUR(f(*vk))
	}
	pdf.SetFont("lib", "B", 8.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX+100, y)
	pdf.CellFormat(35, 5, strconv.Itoa(d.Jahr), "", 0, "R", false, 0, "")
	if vk != nil {
		pdf.SetXY(leftX+135, y)
		pdf.CellFormat(rightX-leftX-137, 5, strconv.Itoa(d.VorjahrJahr), "", 0, "R", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
	y += 6
	zeile("Kassenbestand zu Jahresbeginn", fmtEUR(d.K.Anfangsbestand), vorEUR(func(k kassenbericht) float64 { return k.Anfangsbestand }), false)
	zeile("+ Zahlungen der Pächter", fmtEUR(d.K.EinnahmenBezahlt), vorEUR(func(k kassenbericht) float64 { return k.EinnahmenBezahlt }), false)
	if d.K.GuthabenAusgezahlt != 0 || (vk != nil && vk.GuthabenAusgezahlt != 0) {
		zeile("− ausgezahlte Guthaben", fmtEUR(d.K.GuthabenAusgezahlt), vorEUR(func(k kassenbericht) float64 { return k.GuthabenAusgezahlt }), false)
	}
	zeile("− sonstige Ausgaben", fmtEUR(d.K.AusgabenSumme), vorEUR(func(k kassenbericht) float64 { return k.AusgabenSumme }), false)
	pdf.SetDrawColor(mr, mg, mb)
	pdf.SetLineWidth(0.3)
	pdf.Line(leftX+2, y+0.5, rightX, y+0.5)
	y += 1.5
	zeile("= Kassenbestand", fmtEUR(d.K.Kassenbestand), vorEUR(func(k kassenbericht) float64 { return k.Kassenbestand }), true)
	if d.Offen > 0 {
		pdf.SetFont("lib", "", 9)
		pdf.SetTextColor(150, 90, 20)
		pdf.SetXY(leftX+2, y+1)
		pdf.CellFormat(rightX-leftX, 5, "Noch offen aus Rechnungen dieses Jahres: "+fmtEUR(d.Offen), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 6
	}
	y += 6

	// Ausgaben nach Kategorie als waagerechte Balken
	if len(d.K.AusgabenKategorie) > 0 {
		ueberschrift("Sonstige Ausgaben nach Art")
		kat := append([]kategorieSumme{}, d.K.AusgabenKategorie...)
		sort.SliceStable(kat, func(i, j int) bool { return kat[i].Summe > kat[j].Summe })
		maxS := 0.0
		for _, k := range kat {
			maxS = math.Max(maxS, k.Summe)
		}
		balkenX, balkenW := leftX+52, rightX-leftX-52-30
		for _, k := range kat {
			platz(8)
			pdf.SetFont("lib", "", 9.5)
			pdf.SetXY(leftX+2, y)
			pdf.CellFormat(50, 6, passendKlein(pdf, k.Kategorie, 49, 9.5, 7), "", 0, "L", false, 0, "")
			w := 0.0
			if maxS > 0 {
				w = math.Max(0.8, balkenW*k.Summe/maxS)
			}
			pdf.SetFillColor(lr, lg, lb)
			pdf.Rect(balkenX, y+1.2, w, 3.8, "F")
			pdf.SetFont("lib", "B", 9.5)
			pdf.SetXY(rightX-30, y)
			pdf.CellFormat(30, 6, fmtEUR(k.Summe), "", 0, "R", false, 0, "")
			y += 7
		}
		y += 5
	}

	// Kassenbestand im Verlauf als senkrechte Balken
	if len(d.Verlauf) >= 2 {
		ueberschrift("Kassenbestand am Jahresende im Verlauf")
		const hoehe = 38.0
		platz(hoehe + 14)
		maxB, minB := 0.0, 0.0
		for _, v := range d.Verlauf {
			maxB = math.Max(maxB, v.Kassenbestand)
			minB = math.Min(minB, v.Kassenbestand)
		}
		spanne := maxB - minB
		if spanne == 0 {
			spanne = 1
		}
		nullY := y + 5 + hoehe*maxB/spanne
		breite := (rightX - leftX) / float64(len(d.Verlauf))
		for i, v := range d.Verlauf {
			x := leftX + float64(i)*breite + breite*0.2
			bw := breite * 0.6
			hh := hoehe * math.Abs(v.Kassenbestand) / spanne
			if v.Jahr == d.Jahr {
				pdf.SetFillColor(mr, mg, mb)
			} else {
				pdf.SetFillColor(lr, lg, lb)
			}
			if v.Kassenbestand < 0 {
				pdf.SetFillColor(200, 90, 80)
				pdf.Rect(x, nullY, bw, hh, "F")
			} else {
				pdf.Rect(x, nullY-hh, bw, hh, "F")
			}
			pdf.SetFont("lib", "B", 8)
			pdf.SetXY(x-breite*0.2, math.Min(nullY-hh, nullY)-4.5)
			pdf.CellFormat(breite, 4, fmtNum(v.Kassenbestand, 0)+" €", "", 0, "C", false, 0, "")
			pdf.SetFont("lib", "", 8.5)
			pdf.SetXY(x-breite*0.2, y+hoehe+7)
			pdf.CellFormat(breite, 4, strconv.Itoa(v.Jahr), "", 0, "C", false, 0, "")
		}
		pdf.SetDrawColor(160, 165, 158)
		pdf.SetLineWidth(0.2)
		pdf.Line(leftX, nullY, rightX, nullY)
		y += hoehe + 16
	}

	// Vereinsleben
	ueberschrift("Vereinsleben")
	punkt := func(t string) {
		platz(7)
		pdf.SetFillColor(mr, mg, mb)
		pdf.Circle(leftX+3, y+3, 0.9, "F")
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX+6, y)
		pdf.MultiCell(rightX-leftX-6, 6, t, "", "L", false)
		y = pdf.GetY() + 0.5
	}
	switch n := len(d.Uebergaben); n {
	case 0:
		punkt("Kein Pächterwechsel in diesem Jahr.")
	default:
		txt := fmt.Sprintf("%d Gartenübergabe(n): ", n)
		for i, u := range d.Uebergaben {
			if i > 0 {
				txt += ", "
			}
			txt += "Garten " + u.Gartennr + " (" + germanDate(u.Datum) + ")"
		}
		punkt(txt)
	}
	if d.Warteliste > 0 {
		punkt(fmt.Sprintf("%d Interessent(en) auf der Warteliste für einen Garten.", d.Warteliste))
	} else {
		punkt("Die Warteliste ist leer.")
	}
	if d.MitStunden > 0 {
		punkt(fmt.Sprintf("Pflichtstunden (%s h) erfüllt: %d von %d Gärten, zusammen %s Stunden Gemeinschaftsarbeit.",
			fmtFlex(s.Pflichtstunden), d.PflichtOK, d.MitStunden, fmtFlex(d.Stunden)))
	}
	y += 5

	// kommende Termine
	if len(d.Termine) > 0 {
		ueberschrift("Die nächsten Termine")
		for _, t := range d.Termine {
			if y+6.5 > 280 {
				break // nur so viele Termine, wie noch auf die Seite passen
			}
			dt, _ := parseDate(t.Datum)
			pdf.SetFont("lib", "B", 10)
			pdf.SetXY(leftX+2, y)
			pdf.CellFormat(28, 6, wochentage[dt.Weekday()]+" "+dt.Format("02.01.2006"), "", 0, "L", false, 0, "")
			zeit := ""
			if t.Von != "" {
				zeit = t.Von + " Uhr"
			}
			pdf.SetFont("lib", "", 9.5)
			pdf.SetTextColor(90, 90, 90)
			pdf.SetXY(leftX+32, y)
			pdf.CellFormat(20, 6, zeit, "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("lib", "", 10)
			pdf.SetXY(leftX+54, y)
			titel := t.Titel
			if t.Ort != "" {
				titel += " · " + t.Ort
			}
			pdf.CellFormat(rightX-leftX-54, 6, passendKlein(pdf, titel, rightX-leftX-55, 10, 7.5), "", 0, "L", false, 0, "")
			y += 6.5
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) jahresberichtPDF(r *http.Request) ([]byte, string, error) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	d, ok := a.st.jahresberichtLocked(year, time.Now())
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Dieses Jahr gibt es nicht")
	}
	pdf, err := buildJahresbericht(d)
	return pdf, fmt.Sprintf("Jahresbericht_%d.pdf", year), err
}

func (a *App) handleJahresbericht(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.jahresberichtPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleJahresberichtOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.jahresberichtPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	full, err := a.st.schreibeDruckdatei(name, pdf)
	if err != nil {
		writeErr(w, err)
		return
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]string{"datei": filepath.ToSlash(filepath.Join(druckOrdner, filepath.Base(full)))})
}
