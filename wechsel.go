package main

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Pächterwechsel: Ein Garten geht unterjährig an einen neuen Pächter. Der bisherige Pächter bekommt eine
// Schlussrechnung über den Verbrauch bis zur Übergabe (ohne Beiträge für das Folgejahr), bleibt dafür bis
// zum Jahreswechsel im laufenden Jahr und kommt dann von selbst in den Papierkorb. Der neue Pächter startet
// mit den Zählerständen der Übergabe. Über alles entsteht ein Übergabeprotokoll zum Unterschreiben.

// Uebergabe hält einen Pächterwechsel fest. Namen und Anschriften sind eine Kopie vom Tag der Übergabe,
// damit das Protokoll später unverändert neu gedruckt werden kann.
type Uebergabe struct {
	ID             string   `json:"id"`
	Jahr           int      `json:"jahr"`
	Datum          string   `json:"datum"` // JJJJ-MM-TT
	Gartennr       string   `json:"gartennr"`
	Gartengroesse  float64  `json:"gartengroesse"`
	AltID          string   `json:"altId"`
	AltMitgliedsnr string   `json:"altMitgliedsnr"`
	AltName        string   `json:"altName"`
	AltAnschrift   string   `json:"altAnschrift"`
	NeuID          string   `json:"neuId"`
	NeuMitgliedsnr string   `json:"neuMitgliedsnr"`
	NeuName        string   `json:"neuName"`
	NeuAnschrift   string   `json:"neuAnschrift"`
	Wasserzaehler  string   `json:"wasserzaehler"`
	WasserStand    *float64 `json:"wasserStand"`
	Stromzaehler   string   `json:"stromzaehler"`
	StromStand     *float64 `json:"stromStand"`
	Abloese        *float64 `json:"abloese,omitempty"`
	Schluessel     string   `json:"schluessel,omitempty"`
	Bemerkungen    string   `json:"bemerkungen,omitempty"`
	Erstellt       string   `json:"erstellt"` // RFC3339
}

// fristAbgelaufen: Aufbewahrungsfrist des Protokolls vorbei und Personenbezug noch nicht entfernt.
func (u Uebergabe) fristAbgelaufen(aktuell int) bool {
	return fristAbgelaufen(jahrAusZeit(u.Datum), aktuell) && u.AltName != fristAbgelaufenText
}

const minSchlussZahlungsfrist = 14 // Tage

// schlussSettings gibt einer Schlussrechnung, die vor dem regulären Rechnungsdatum ausgestellt wird, das
// heutige Datum als Rechnungsdatum und ein Zahlungsziel mit derselben Frist wie bei den Jahresrechnungen
// (mindestens 14 Tage). Für alle anderen Rechnungen bleiben die Einstellungen unverändert.
func schlussSettings(s Settings, a Ablesung, jetzt time.Time) Settings {
	if a.Schluss == "" {
		return s
	}
	heute := jetzt.Format("2006-01-02")
	rd, err1 := parseDate(s.Rechnungsdatum)
	zz, err2 := parseDate(s.Zahlungsziel)
	if err1 != nil || heute >= s.Rechnungsdatum {
		return s
	}
	frist := minSchlussZahlungsfrist
	if err2 == nil {
		if tage := int(zz.Sub(rd).Hours() / 24); tage > frist {
			frist = tage
		}
	}
	h, _ := parseDate(heute)
	s.Rechnungsdatum = heute
	s.Zahlungsziel = h.AddDate(0, 0, frist).Format("2006-01-02")
	return s
}

func anschrift(p Paechter) string {
	return strings.Join(nonEmpty(p.Strasse, p.PLZOrt), ", ")
}

type wechselReq struct {
	AltID string `json:"altId"`
	Datum string `json:"datum"`
	// WasserVJ/StromVJ: nur nötig, wenn für den bisherigen Pächter noch kein Vorjahresstand eingetragen ist
	WasserVJ     *float64 `json:"wasserVJ"`
	StromVJ      *float64 `json:"stromVJ"`
	WasserStand  *float64 `json:"wasserStand"`
	StromStand   *float64 `json:"stromStand"`
	Stunden      *float64 `json:"stunden"`
	Neu          Paechter `json:"neu"`
	WartelisteID string   `json:"wartelisteId"`
	Abloese      *float64 `json:"abloese"`
	Schluessel   string   `json:"schluessel"`
	Bemerkungen  string   `json:"bemerkungen"`
}

func (a *App) handlePaechterwechsel(w http.ResponseWriter, r *http.Request) {
	var in wechselReq
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if _, err := parseDate(in.Datum); err != nil {
		writeErr(w, bad("Bitte das Datum der Übergabe eintragen"))
		return
	}
	for _, x := range []*float64{in.WasserVJ, in.StromVJ, in.WasserStand, in.StromStand, in.Stunden, in.Abloese} {
		if !validNumPtr(x) {
			writeErr(w, bad("Zählerstände, Stunden und Ablösesumme müssen Zahlen ab 0 sein"))
			return
		}
	}
	if in.WasserStand == nil || in.StromStand == nil {
		writeErr(w, bad("Bitte beide Zählerstände bei der Übergabe eintragen"))
		return
	}
	if in.Stunden == nil {
		writeErr(w, bad("Bitte die bis zur Übergabe geleisteten Arbeitsstunden eintragen (0, wenn keine)"))
		return
	}
	in.Schluessel = trim(in.Schluessel, 200)
	in.Bemerkungen = trim(in.Bemerkungen, 1500)

	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	d := a.st.d
	ai := -1
	for i := range d.Paechter {
		if d.Paechter[i].ID == in.AltID && !d.Paechter[i].Geloescht {
			ai = i
			break
		}
	}
	if ai < 0 {
		writeErr(w, notFound("Bisheriger Pächter nicht gefunden"))
		return
	}
	alt := d.Paechter[ai]
	if alt.AusgeschiedenAm != "" {
		writeErr(w, bad("Für "+alt.Name+" wurde der Garten schon am "+germanDate(alt.AusgeschiedenAm)+" übergeben"))
		return
	}
	jahr := d.Settings.Jahr
	if a.st.validInvoiceLocked(jahr, alt.ID) != nil {
		writeErr(w, bad(fmt.Sprintf("Für %s ist die Rechnung %d schon ausgestellt. Bitte zuerst den Jahreswechsel machen; "+
			"die Schlussrechnung gehört dann ins neue Abrechnungsjahr.", alt.Name, jahr)))
		return
	}

	neu := in.Neu
	neu.Gartennr = alt.Gartennr
	if neu.Gartengroesse == 0 {
		neu.Gartengroesse = alt.Gartengroesse
	}
	if strings.TrimSpace(neu.WasserzaehlerNr) == "" {
		neu.WasserzaehlerNr = alt.WasserzaehlerNr
	}
	if strings.TrimSpace(neu.StromzaehlerNr) == "" {
		neu.StromzaehlerNr = alt.StromzaehlerNr
	}
	neu.Geloescht, neu.GeloeschtAm, neu.AusgeschiedenAm = false, "", ""
	neu, err := cleanPaechter(neu)
	if err != nil {
		writeErr(w, err)
		return
	}
	if a.st.nrTaken(neu.Mitgliedsnr, "") {
		msg := "Die Mitgliedsnummer " + neu.Mitgliedsnr + " gibt es schon"
		if strings.EqualFold(neu.Mitgliedsnr, alt.Mitgliedsnr) {
			msg += ". Der bisherige Pächter behält sie bis zum Jahreswechsel für seine Schlussrechnung, bitte eine andere Nummer vergeben"
		}
		writeErr(w, bad(msg))
		return
	}
	wi := -1
	if in.WartelisteID != "" {
		for i := range d.Warteliste {
			if d.Warteliste[i].ID == in.WartelisteID {
				wi = i
			}
		}
		if wi < 0 {
			writeErr(w, notFound("Eintrag auf der Warteliste nicht gefunden"))
			return
		}
	}

	ab := a.st.ensureYear(jahr).Ablesungen[alt.ID]
	if ab.WasserVJ == nil {
		ab.WasserVJ = in.WasserVJ
	}
	if ab.StromVJ == nil {
		ab.StromVJ = in.StromVJ
	}
	if ab.WasserVJ == nil || ab.StromVJ == nil {
		writeErr(w, bad("Für den bisherigen Pächter fehlt der Vorjahresstand. Bitte mit eintragen"))
		return
	}
	ab.WasserAkt, ab.StromAkt, ab.Stunden, ab.Schluss = in.WasserStand, in.StromStand, in.Stunden, in.Datum
	if _, neg := verbrauch(ab.WasserVJ, ab.WasserAkt, ab.WasserWechsel); neg {
		writeErr(w, bad("Der Wasserzählerstand bei der Übergabe ist kleiner als der Vorjahresstand"))
		return
	}
	if _, neg := verbrauch(ab.StromVJ, ab.StromAkt, ab.StromWechsel); neg {
		writeErr(w, bad("Der Stromzählerstand bei der Übergabe ist kleiner als der Vorjahresstand"))
		return
	}

	a.st.snapshotBackup("vor-Paechterwechsel")
	d.Jahre[yearKey(jahr)].Ablesungen[alt.ID] = ab
	d.Paechter[ai].AusgeschiedenAm = in.Datum
	a.st.gartenSchliessen(alt.ID, in.Datum)

	neu.ID = newID()
	d.Paechter = append(d.Paechter, neu)
	wv, sv := *in.WasserStand, *in.StromStand
	d.Jahre[yearKey(jahr)].Ablesungen[neu.ID] = Ablesung{WasserVJ: &wv, StromVJ: &sv}
	a.st.gartenOeffnenAb(neu.ID, neu.Gartennr, neu.Mitgliedsnr, neu.Name, in.Datum)
	if wi >= 0 {
		d.Warteliste = append(d.Warteliste[:wi], d.Warteliste[wi+1:]...)
	}

	u := Uebergabe{
		ID: newID(), Jahr: jahr, Datum: in.Datum, Gartennr: alt.Gartennr, Gartengroesse: neu.Gartengroesse,
		AltID: alt.ID, AltMitgliedsnr: alt.Mitgliedsnr, AltName: alt.Name, AltAnschrift: anschrift(alt),
		NeuID: neu.ID, NeuMitgliedsnr: neu.Mitgliedsnr, NeuName: neu.Name, NeuAnschrift: anschrift(neu),
		Wasserzaehler: alt.WasserzaehlerNr, WasserStand: in.WasserStand, Stromzaehler: alt.StromzaehlerNr, StromStand: in.StromStand,
		Abloese: in.Abloese, Schluessel: in.Schluessel, Bemerkungen: in.Bemerkungen, Erstellt: time.Now().Format(time.RFC3339),
	}
	d.Uebergaben = append(d.Uebergaben, u)
	a.st.audit("Pächterwechsel Garten %s am %s: %s %s → %s %s", alt.Gartennr, germanDate(in.Datum), alt.Mitgliedsnr, alt.Name, neu.Mitgliedsnr, neu.Name)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"uebergabe": u, "neu": neu})
}

// handleUebergaben listet die Pächterwechsel, neueste zuerst.
func (a *App) handleUebergaben(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	out := append([]Uebergabe{}, a.st.d.Uebergaben...)
	a.st.mu.Unlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Datum > out[j].Datum })
	writeJSON(w, 200, out)
}

func (a *App) uebergabeprotokollPDF(id string) ([]byte, string, error) {
	a.st.mu.Lock()
	var u *Uebergabe
	for i := range a.st.d.Uebergaben {
		if a.st.d.Uebergaben[i].ID == id {
			x := a.st.d.Uebergaben[i]
			u = &x
		}
	}
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if u == nil {
		return nil, "", notFound("Übergabe nicht gefunden")
	}
	pdf, err := buildUebergabeprotokoll(s, *u)
	if err != nil {
		return nil, "", err
	}
	return pdf, safeName("Uebergabeprotokoll_Garten_"+u.Gartennr+"_"+u.Datum, 80) + ".pdf", nil
}

func (a *App) handleUebergabeprotokoll(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.uebergabeprotokollPDF(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleUebergabeprotokollOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.uebergabeprotokollPDF(r.PathValue("id"))
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

// buildUebergabeprotokoll erzeugt das Übergabeprotokoll (A4 hoch) mit Zählerständen, Schlüsseln, Ablöse,
// Bemerkungen, einer kurzen Checkliste und Unterschriftsfeldern für beide Pächter und den Vorstand.
func buildUebergabeprotokoll(s Settings, u Uebergabe) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle("Übergabeprotokoll Garten "+u.Gartennr, true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)

	y := 36.0
	font := func(style string, size float64) { pdf.SetFont("lib", style, size) }
	zeile := func(x, w, h float64, txt, align string) {
		pdf.SetXY(x, y)
		pdf.CellFormat(w, h, txt, "", 0, align, false, 0, "")
	}
	ueberschrift := func(txt string) {
		font("B", 11)
		pdf.SetTextColor(mr, mg, mb)
		zeile(leftX, rightX-leftX, 6, txt, "L")
		pdf.SetTextColor(0, 0, 0)
		y += 7
	}

	font("B", 16)
	pdf.SetTextColor(mr, mg, mb)
	zeile(leftX, rightX-leftX, 8, "Übergabeprotokoll Garten Nr. "+u.Gartennr, "L")
	pdf.SetTextColor(0, 0, 0)
	y += 9
	font("", 10.5)
	groesse := ""
	if u.Gartengroesse > 0 {
		groesse = " · Gartengröße " + fmtFlex(u.Gartengroesse) + " m²"
	}
	zeile(leftX, rightX-leftX, 6, "Übergabe am "+germanDate(u.Datum)+groesse, "L")
	y += 10

	// zwei Kästen nebeneinander: bisheriger und neuer Pächter
	kastenW := (rightX - leftX - 6) / 2
	kasten := func(x float64, titel, name, nr, adr string) {
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetDrawColor(mr, mg, mb)
		pdf.SetLineWidth(0.3)
		pdf.RoundedRect(x, y, kastenW, 27, 1.6, "1234", "FD")
		pdf.SetTextColor(mr, mg, mb)
		font("B", 9)
		pdf.SetXY(x+3, y+2)
		pdf.CellFormat(kastenW-6, 5, titel, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(x+3, y+8)
		pdf.CellFormat(kastenW-6, 6, passendKleinStil(pdf, "B", name, kastenW-6, 11, 8), "", 0, "L", false, 0, "")
		font("", 9.5)
		pdf.SetXY(x+3, y+14)
		pdf.CellFormat(kastenW-6, 5, passendKlein(pdf, adr, kastenW-6, 9.5, 7), "", 0, "L", false, 0, "")
		font("", 9.5)
		pdf.SetXY(x+3, y+19.5)
		pdf.CellFormat(kastenW-6, 5, "Mitgliedsnr.: "+nr, "", 0, "L", false, 0, "")
	}
	kasten(leftX, "BISHERIGER PÄCHTER", u.AltName, u.AltMitgliedsnr, u.AltAnschrift)
	kasten(leftX+kastenW+6, "NEUER PÄCHTER", u.NeuName, u.NeuMitgliedsnr, u.NeuAnschrift)
	pdf.SetDrawColor(0, 0, 0)
	y += 34

	ueberschrift("Zählerstände bei der Übergabe (gemeinsam abgelesen)")
	tabelle := func(cells []string, fett bool, fuellen bool) {
		ws := []float64{40, 60, 70}
		x := leftX
		pdf.SetDrawColor(170, 175, 168)
		pdf.SetLineWidth(0.2)
		if fuellen {
			pdf.SetFillColor(fr, fg, fb)
		}
		for i, c := range cells {
			if fett {
				font("B", 9.5)
			} else {
				font("", 10)
			}
			pdf.SetXY(x, y)
			align := "L"
			if i == 2 {
				align = "R"
			}
			pdf.CellFormat(ws[i], 7.5, " "+c+" ", "1", 0, align, fuellen, 0, "")
			x += ws[i]
		}
		y += 7.5
	}
	stand := func(v *float64, einheit string) string {
		if v == nil {
			return ""
		}
		return fmtFlex(*v) + " " + einheit
	}
	tabelle([]string{"Zähler", "Zähler-Nr.", "Stand"}, true, true)
	tabelle([]string{"Wasser", u.Wasserzaehler, stand(u.WasserStand, "m³")}, false, false)
	tabelle([]string{"Strom", u.Stromzaehler, stand(u.StromStand, "kWh")}, false, false)
	y += 7

	ueberschrift("Vereinbarungen")
	font("", 10)
	abloese := "keine vereinbart"
	if u.Abloese != nil && *u.Abloese > 0 {
		abloese = fmtEUR(*u.Abloese) + " (zahlt der neue an den bisherigen Pächter)"
	}
	schluessel := u.Schluessel
	if schluessel == "" {
		schluessel = "–"
	}
	for _, kv := range [][2]string{{"Ablösesumme:", abloese}, {"Übergebene Schlüssel:", schluessel}} {
		font("", 10)
		zeile(leftX, 45, 6, kv[0], "L")
		font("B", 10)
		pdf.SetXY(leftX+45, y)
		pdf.MultiCell(rightX-leftX-45, 6, kv[1], "", "L", false)
		y = pdf.GetY() + 1
	}
	y += 5

	ueberschrift("Zustand des Gartens, Bemerkungen")
	font("", 10)
	start := y
	if u.Bemerkungen != "" {
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, 5.2, u.Bemerkungen, "", "L", false)
		y = pdf.GetY() + 2
	}
	// Platz für handschriftliche Ergänzungen, mindestens drei Linien
	pdf.SetDrawColor(190, 194, 188)
	for i := 0; i < 3 || y < start+20; i++ {
		y += 7
		pdf.Line(leftX, y, rightX, y)
	}
	y += 8

	ueberschrift("Bei der Übergabe erledigt")
	punkte := []string{
		"Garten und Laube geräumt, Abfälle entsorgt",
		"Zählerstände gemeinsam abgelesen und eingetragen",
		"Schlüssel vollständig übergeben",
		"Neuer Pächter hat Gartenordnung und Pachtvertrag erhalten",
	}
	for _, t := range punkte {
		pdf.SetDrawColor(80, 80, 80)
		pdf.SetLineWidth(0.3)
		pdf.Rect(leftX+1, y+1.3, 3.6, 3.6, "D")
		font("", 10)
		zeile(leftX+7, rightX-leftX-7, 6, t, "L")
		y += 6.6
	}

	// Unterschriften
	if y+14 > 258 {
		// lange Bemerkungen: Unterschriften auf eine zweite Seite statt in den Seitenrand
		pdf.AddPage()
		y = 30
	} else {
		y = maxf(y+14, 255)
	}
	font("", 10)
	zeile(leftX, rightX-leftX, 6, s.Ort+", den ______________________", "L")
	y += 18
	sw := (rightX - leftX - 12) / 3
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	for i, t := range []string{"Bisheriger Pächter", "Neuer Pächter", "Für den Vorstand"} {
		x := leftX + float64(i)*(sw+6)
		pdf.Line(x, y, x+sw, y)
		font("", 8.5)
		pdf.SetXY(x, y+1)
		pdf.CellFormat(sw, 5, t, "", 0, "C", false, 0, "")
	}
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX, 287)
	pdf.CellFormat(rightX-leftX, 4, s.VereinName+" · Übergabeprotokoll · erstellt mit dem Kleingarten-Manager", "", 0, "C", false, 0, "")
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
