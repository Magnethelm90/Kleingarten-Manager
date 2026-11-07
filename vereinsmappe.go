package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Willkommensmappe für neue Pächter und Exposé für freie Gärten. Beide nutzen die Vereinsinfo: Ansprechpartner
// und kurze Hinweise (z. B. aus der Gartenordnung), die der Vorstand einmal einträgt.

// Vereinsinfo enthält Texte für Willkommensmappe, Exposé und Amtsübergabe.
type Vereinsinfo struct {
	Ansprechpartner string `json:"ansprechpartner"` // mehrzeilig, z. B. »Vorsitz: …, Tel. …«
	Hinweise        string `json:"hinweise"`        // mehrzeilig, z. B. Ruhezeiten, Wasser, Müll
}

func cleanVereinsinfo(v Vereinsinfo) Vereinsinfo {
	zeilen := func(s string, max, n int) string {
		var out []string
		for _, z := range strings.Split(s, "\n") {
			if z = trim(strings.Join(strings.Fields(z), " "), n); z != "" && len(out) < max {
				out = append(out, z)
			}
		}
		return strings.Join(out, "\n")
	}
	return Vereinsinfo{Ansprechpartner: zeilen(v.Ansprechpartner, 8, 120), Hinweise: zeilen(v.Hinweise, 15, 200)}
}

func (a *App) handleVereinsinfo(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		a.st.mu.Lock()
		v := a.st.d.Vereinsinfo
		a.st.mu.Unlock()
		writeJSON(w, 200, v)
		return
	}
	var in Vereinsinfo
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in = cleanVereinsinfo(in)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Vereinsinfo = in
	a.st.audit("Texte für Willkommensmappe und Exposé geändert")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

// ------------------------------------------------------------------ gemeinsame Bausteine

// mappeAbschnitt setzt eine Überschrift in Vereinsfarbe und gibt die neue y-Position zurück.
func mappeAbschnitt(pdf *fpdf.Fpdf, s Settings, y float64, titel string) float64 {
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.SetFont("lib", "B", 12)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, titel, "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	return y + 8
}

// mappeTabelle setzt Paare »Bezeichnung: Wert« in zwei Spalten.
func mappeTabelle(pdf *fpdf.Fpdf, y float64, zeilen [][2]string) float64 {
	for _, z := range zeilen {
		if z[1] == "" {
			continue
		}
		pdf.SetFont("lib", "", 10)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(55, 6, z[0], "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetFont("lib", "B", 10)
		pdf.SetXY(leftX+55, y)
		pdf.MultiCell(rightX-leftX-55, 6, z[1], "", "L", false)
		y = pdf.GetY()
	}
	return y + 3
}

func mappeText(pdf *fpdf.Fpdf, y float64, txt string, groesse float64) float64 {
	pdf.SetFont("lib", "", groesse)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, groesse*0.5, txt, "", "L", false)
	return pdf.GetY() + 3
}

func mappeFuss(pdf *fpdf.Fpdf, s Settings, text string) {
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX, 287)
	pdf.CellFormat(rightX-leftX, 4, text, "", 0, "C", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

func fmtM2(v float64) string { return fmtFlex(v) + " m²" }

// ------------------------------------------------------------------ Willkommensmappe

type willkommenDaten struct {
	S         Settings
	P         Paechter
	A         Ablesung
	Seit      string
	Info      Vereinsinfo
	Termine   []Termin
	Lageplan  lagePlan
	Rechnungs string
}

func (s *Store) willkommenLocked(id string, heute time.Time) (willkommenDaten, error) {
	var p *Paechter
	for i := range s.d.Paechter {
		if s.d.Paechter[i].ID == id && !s.d.Paechter[i].Geloescht {
			p = &s.d.Paechter[i]
		}
	}
	if p == nil {
		return willkommenDaten{}, notFound("Pächter nicht gefunden")
	}
	v, _ := s.viewLocked(s.d.Settings.Jahr)
	d := willkommenDaten{S: s.d.Settings, P: *p, A: v.Ablesungen[p.ID], Info: s.d.Vereinsinfo, Lageplan: s.lagePlanLocked(false),
		Rechnungs: invoiceNumber(s.d.Settings, *p)}
	for _, e := range s.d.GartenHistorie {
		if e.PaechterID == p.ID && e.Bis == "" {
			d.Seit = e.Seit
		}
	}
	ab, bis := heute.Format("2006-01-02"), heute.AddDate(1, 0, 0).Format("2006-01-02")
	for _, t := range s.d.Termine {
		if t.Datum >= ab && t.Datum < bis && len(d.Termine) < 14 {
			d.Termine = append(d.Termine, t)
		}
	}
	return d, nil
}

func buildWillkommensmappe(d willkommenDaten) ([]byte, error) {
	s, p := d.S, d.P
	pdf := neuesPDF(s, "Willkommen im "+s.VereinName, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := 38.0
	pdf.SetFont("lib", "B", 20)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 10, passendKleinStil(pdf, "B", "Herzlich willkommen, "+strings.TrimSpace(p.Anrede+" "+p.Name)+"!", rightX-leftX, 20, 12), "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 14
	y = mappeText(pdf, y, "schön, dass Sie jetzt zu unserer Gartengemeinschaft gehören. In dieser Mappe finden Sie das Wichtigste für den Anfang: "+
		"Ihren Garten, Ihre Zähler, die Termine und an wen Sie sich wenden können. Bitte heben Sie sie gut auf.", 10.5)
	y += 2

	y = mappeAbschnitt(pdf, s, y, "Ihr Garten")
	zaehler := func(nr string, stand *float64, einheit string) string {
		var t []string
		if nr != "" {
			t = append(t, "Nr. "+nr)
		}
		if stand != nil {
			t = append(t, fmt.Sprintf("Anfangsstand %s %s", fmtFlex(*stand), einheit))
		}
		return strings.Join(t, " · ")
	}
	gr := ""
	if p.Gartengroesse > 0 {
		gr = fmtM2(p.Gartengroesse)
	}
	seit := ""
	if d.Seit != "" {
		seit = germanDate(d.Seit)
	}
	y = mappeTabelle(pdf, y, [][2]string{
		{"Gartennummer", p.Gartennr}, {"Größe", gr}, {"Mitgliedsnummer", p.Mitgliedsnr}, {"Pächter seit", seit},
		{"Wasserzähler", zaehler(p.WasserzaehlerNr, d.A.WasserVJ, "m³")}, {"Stromzähler", zaehler(p.StromzaehlerNr, d.A.StromVJ, "kWh")},
	})

	y = mappeAbschnitt(pdf, s, y, "Gut zu wissen")
	var gut [][2]string
	if s.Pflichtstunden > 0 {
		t := fmt.Sprintf("%s Stunden Gemeinschaftsarbeit im Jahr", fmtFlex(s.Pflichtstunden))
		if s.NachzahlungJeStd > 0 {
			t += fmt.Sprintf("; für jede fehlende Stunde werden %s berechnet", fmtEUR(s.NachzahlungJeStd))
		}
		gut = append(gut, [2]string{"Gemeinschaftsarbeit", t})
	}
	gut = append(gut, [2]string{"Abrechnung", "einmal im Jahr: Pacht, Beiträge, Wasser und Strom nach Zählerstand"})
	if s.IBAN != "" {
		gut = append(gut, [2]string{"Bankverbindung", strings.TrimSpace(s.BankName + ", IBAN " + s.IBAN)})
		gut = append(gut, [2]string{"Verwendungszweck", "Rechnungsnummer, bei Ihnen meist " + d.Rechnungs})
	}
	y = mappeTabelle(pdf, y, gut)

	if d.Info.Ansprechpartner != "" {
		y = mappeAbschnitt(pdf, s, y, "Ihre Ansprechpartner")
		y = mappeText(pdf, y, d.Info.Ansprechpartner, 10)
	}
	if d.Info.Hinweise != "" {
		if y > 240 {
			pdf.AddPage()
			y = 20
		}
		y = mappeAbschnitt(pdf, s, y, "Bitte beachten")
		for _, z := range strings.Split(d.Info.Hinweise, "\n") {
			if y > 275 {
				pdf.AddPage()
				y = 20
			}
			y = mappeText(pdf, y, "•  "+z, 10) - 1.5
		}
	}
	mappeFuss(pdf, s, s.VereinName+" · Willkommensmappe für Garten "+p.Gartennr)

	// Seite 2: Lageplan und Termine
	pdf.AddPage()
	y = mappeAbschnitt(pdf, s, 16, "Hier liegt Ihr Garten")
	if len(d.Lageplan.Gaerten) > 0 {
		_, hoehe := zeichneLageplan(pdf, s, d.Lageplan, leftX, y, rightX, y+100, p.Gartennr)
		y += hoehe + 8
	} else {
		y = mappeText(pdf, y, "Der Lageplan ist im Programm noch nicht angelegt.", 10)
	}
	y = mappeAbschnitt(pdf, s, y, "Termine der nächsten zwölf Monate")
	if len(d.Termine) == 0 {
		mappeText(pdf, y, "Die Termine gibt der Vorstand rechtzeitig bekannt (Aushang im Schaukasten).", 10)
	}
	for _, t := range d.Termine {
		if y > 278 {
			break
		}
		pdf.SetFont("lib", "B", 10)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(42, 6, terminWann(Termin{Datum: t.Datum}), "", 0, "L", false, 0, "")
		pdf.SetFont("lib", "", 10)
		pdf.SetXY(leftX+44, y)
		zeit := ""
		if t.Von != "" {
			zeit = t.Von + " Uhr · "
		}
		pdf.CellFormat(rightX-leftX-44, 6, passendKlein(pdf, zeit+t.Titel+func() string {
			if t.Ort != "" {
				return " · " + t.Ort
			}
			return ""
		}(), rightX-leftX-44, 10, 7), "", 0, "L", false, 0, "")
		y += 6.5
	}
	mappeFuss(pdf, s, s.VereinName+" · Stand "+time.Now().Format("02.01.2006"))
	return pdfBytes(pdf)
}

func (a *App) willkommenPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	d, err := a.st.willkommenLocked(r.PathValue("id"), time.Now())
	a.st.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	pdf, err := buildWillkommensmappe(d)
	return pdf, safeName("Willkommen_Garten_"+d.P.Gartennr+"_"+d.P.Name, 80) + ".pdf", err
}

// ------------------------------------------------------------------ Exposé für einen Garten

type exposeDaten struct {
	S        Settings
	Nr       string
	Groesse  float64
	Frei     bool
	Wert     *Wertermittlung
	Foto     []byte
	FotoTyp  string
	Info     Vereinsinfo
	Lageplan lagePlan
}

func (s *Store) exposeLocked(nr string) (exposeDaten, error) {
	nr = strings.TrimSpace(nr)
	if nr == "" {
		return exposeDaten{}, bad("Bitte die Gartennummer angeben")
	}
	d := exposeDaten{S: s.d.Settings, Nr: nr, Info: s.d.Vereinsinfo, Lageplan: s.lagePlanLocked(false)}
	_, belegt := s.belegteGaertenLocked()[nr]
	d.Frei = !belegt
	bekannt := belegt
	// Größe: vom jüngsten Pächter dieses Gartens (auch ausgeschiedene)
	seit := ""
	for _, e := range s.d.GartenHistorie {
		if e.Gartennr != nr || e.Seit < seit {
			continue
		}
		for _, p := range s.d.Paechter {
			if p.ID == e.PaechterID && p.Gartengroesse > 0 {
				d.Groesse, seit, bekannt = p.Gartengroesse, e.Seit, true
			}
		}
	}
	if _, ok := s.d.Lageplan[nr]; ok {
		bekannt = true
	}
	for i := range s.d.Wertermittlungen {
		w := &s.d.Wertermittlungen[i]
		if w.Gartennr == nr && (d.Wert == nil || w.Datum > d.Wert.Datum) {
			d.Wert, bekannt = w, true
		}
	}
	if !bekannt {
		return d, notFound("Garten " + nr + " ist nicht bekannt")
	}
	// Foto: jüngstes Foto des Gartens (nur JPEG/PNG, nur Dokumente des Gartens, keine personenbezogenen)
	fotos := []GartenDokument{}
	for _, x := range s.d.Dokumente {
		ext := strings.ToLower(path.Ext(x.Datei))
		if x.Gartennr == nr && x.Art == "Foto" && x.PaechterID == "" && (ext == ".jpg" || ext == ".jpeg" || ext == ".png") {
			fotos = append(fotos, x)
		}
	}
	sort.SliceStable(fotos, func(i, j int) bool { return fotos[i].Datum > fotos[j].Datum })
	for _, f := range fotos {
		full, err := s.dokumentDatei(f.Datei)
		if err != nil {
			continue
		}
		if raw, err := os.ReadFile(full); err == nil && len(raw) <= maxDokumentGroesse {
			d.Foto, d.FotoTyp = raw, "JPG"
			if strings.ToLower(path.Ext(f.Datei)) == ".png" {
				d.FotoTyp = "PNG"
			}
			break
		}
	}
	return d, nil
}

func buildExpose(d exposeDaten) ([]byte, error) {
	s := d.S
	pdf := neuesPDF(s, "Garten "+d.Nr, false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := 38.0
	pdf.SetFont("lib", "", 11)
	pdf.SetTextColor(90, 96, 90)
	pdf.SetXY(leftX, y)
	unter := "Garten im " + s.VereinName
	if d.Frei {
		unter = "Zu vergeben im " + s.VereinName
	}
	pdf.CellFormat(rightX-leftX, 6, unter, "", 0, "L", false, 0, "")
	y += 7
	pdf.SetFont("lib", "B", 24)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 11, "Garten "+d.Nr, "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 16

	if d.Foto != nil {
		name := "foto-" + d.Nr
		info := pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: d.FotoTyp}, bytes.NewReader(d.Foto))
		if !pdf.Err() && info != nil && info.Width() > 0 && info.Height() > 0 {
			w := rightX - leftX
			h := info.Height() / info.Width() * w
			if h > 85 {
				h = 85
				w = info.Width() / info.Height() * h
			}
			pdf.ImageOptions(name, leftX, y, w, h, false, fpdf.ImageOptions{ImageType: d.FotoTyp}, 0, "")
			y += h + 6
		}
		pdf.ClearError() // ein unlesbares Foto darf das Exposé nicht verhindern
	}

	y = mappeAbschnitt(pdf, s, y, "Auf einen Blick")
	var zeilen [][2]string
	if d.Groesse > 0 {
		zeilen = append(zeilen, [2]string{"Größe", fmtM2(d.Groesse)})
	}
	if d.Wert != nil {
		teile := map[string]float64{}
		for _, p := range d.Wert.Positionen {
			if p.Kategorie != "Abzug" {
				teile[p.Kategorie] += p.wert()
			}
		}
		var was []string
		for _, k := range wertKategorien {
			if teile[k] > 0 {
				was = append(was, k)
			}
		}
		zeilen = append(zeilen, [2]string{"Ablösesumme", fmt.Sprintf("%s laut Wertermittlung vom %s", fmtEUR(d.Wert.Summe), germanDate(d.Wert.Datum))})
		if len(was) > 0 {
			zeilen = append(zeilen, [2]string{"Darin enthalten", strings.Join(was, ", ")})
		}
		var laube []string
		for _, p := range d.Wert.Positionen {
			if p.Kategorie == "Baulichkeiten" && len(laube) < 4 {
				laube = append(laube, p.Bezeichnung)
			}
		}
		if len(laube) > 0 {
			zeilen = append(zeilen, [2]string{"Baulichkeiten", strings.Join(laube, "; ")})
		}
	}
	if d.Groesse > 0 && s.PachtJeQm > 0 {
		jahr := d.Groesse*s.PachtJeQm + s.Vereinsbeitrag + s.Territorialverband + s.UmlageStandard
		zeilen = append(zeilen, [2]string{"Laufende Kosten", fmt.Sprintf("etwa %s im Jahr für Pacht, Beiträge und Umlage, dazu Wasser und Strom nach Verbrauch", fmtEUR(round2(jahr)))})
	}
	if s.Pflichtstunden > 0 {
		zeilen = append(zeilen, [2]string{"Gemeinschaftsarbeit", fmtFlex(s.Pflichtstunden) + " Stunden im Jahr"})
	}
	y = mappeTabelle(pdf, y, zeilen)

	if len(d.Lageplan.Gaerten) > 0 && y < 200 {
		y = mappeAbschnitt(pdf, s, y, "Lage in der Anlage")
		unten := 268.0
		if d.Info.Ansprechpartner != "" {
			unten = 245
		}
		if unten-y > 30 {
			_, hoehe := zeichneLageplan(pdf, s, d.Lageplan, leftX, y, rightX, unten, d.Nr)
			y += hoehe + 8
		}
	}
	if d.Info.Ansprechpartner != "" {
		if y > 255 {
			pdf.AddPage()
			y = 20
		}
		y = mappeAbschnitt(pdf, s, y, "Interesse? Bitte melden bei")
		mappeText(pdf, y, d.Info.Ansprechpartner, 10)
	}
	mappeFuss(pdf, s, s.VereinName+" · Angaben ohne Gewähr · Stand "+time.Now().Format("02.01.2006"))
	return pdfBytes(pdf)
}

func (a *App) exposePDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	d, err := a.st.exposeLocked(r.URL.Query().Get("gartennr"))
	a.st.mu.Unlock()
	if err != nil {
		return nil, "", err
	}
	pdf, err := buildExpose(d)
	return pdf, safeName("Garten_"+d.Nr+"_Expose", 80) + ".pdf", err
}
