package main

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Lageplan: freie Gärten, Vergabe eines freien Gartens direkt von der Warteliste und der Lageplan als Aushang.
// Ein Garten ist frei, wenn er im Lageplan steht, aber kein aktiver Pächter ihn hat (z. B. nach einer Kündigung).

const (
	lpBreite = 120 // Kachelgröße im Programm in Bildpunkten (wie LP_W/LP_H in app.js)
	lpHoehe  = 64
)

// belegteGaertenLocked liefert die Gartennummern mit aktivem Pächter. Der Aufrufer hält s.mu.
func (s *Store) belegteGaertenLocked() map[string]Paechter {
	out := map[string]Paechter{}
	for _, p := range s.d.Paechter {
		if nr := strings.TrimSpace(p.Gartennr); nr != "" && !p.Geloescht && p.AusgeschiedenAm == "" {
			out[nr] = p
		}
	}
	return out
}

// handleGartenVergeben macht aus einem Eintrag der Warteliste den Pächter eines freien Gartens.
func (a *App) handleGartenVergeben(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		Gartennr        string  `json:"gartennr"`
		Mitgliedsnr     string  `json:"mitgliedsnr"`
		Gartengroesse   float64 `json:"gartengroesse"`
		Versand         string  `json:"versand"`
		WasserzaehlerNr string  `json:"wasserzaehlerNr"`
		StromzaehlerNr  string  `json:"stromzaehlerNr"`
		Seit            string  `json:"seit"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Gartennr = trim(in.Gartennr, 20)
	if in.Gartennr == "" {
		writeErr(w, bad("Bitte die Gartennummer angeben"))
		return
	}
	if in.Seit == "" {
		in.Seit = time.Now().Format("2006-01-02")
	}
	if _, err := parseDate(in.Seit); err != nil {
		writeErr(w, bad("Das Datum der Übergabe ist ungültig"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if p, belegt := a.st.belegteGaertenLocked()[in.Gartennr]; belegt {
		writeErr(w, bad("Garten "+in.Gartennr+" ist an "+p.Name+" verpachtet. Für einen Wechsel bitte »Wechsel« beim Pächter benutzen"))
		return
	}
	wi := -1
	for i := range a.st.d.Warteliste {
		if a.st.d.Warteliste[i].ID == id {
			wi = i
		}
	}
	if wi < 0 {
		writeErr(w, notFound("Eintrag auf der Warteliste nicht gefunden"))
		return
	}
	b := a.st.d.Warteliste[wi]
	versand := in.Versand
	if versand == "" {
		versand = "Postversand"
		if b.Email != "" {
			versand = "Emailsendung"
		}
	}
	p, err := cleanPaechter(Paechter{
		Mitgliedsnr: in.Mitgliedsnr, Gartennr: in.Gartennr, Anrede: b.Anrede, Name: b.Name, Strasse: b.Strasse, PLZOrt: b.PLZOrt,
		Versand: versand, Email: b.Email, Telefon: b.Telefon, Gartengroesse: in.Gartengroesse,
		WasserzaehlerNr: in.WasserzaehlerNr, StromzaehlerNr: in.StromzaehlerNr,
	})
	if err != nil {
		writeErr(w, err)
		return
	}
	if p.Gartengroesse <= 0 {
		writeErr(w, bad("Bitte die Gartengröße eintragen"))
		return
	}
	if a.st.nrTaken(p.Mitgliedsnr, "") {
		writeErr(w, bad("Die Mitgliedsnummer "+p.Mitgliedsnr+" gibt es schon"))
		return
	}
	p.ID = newID()
	a.st.d.Paechter = append(a.st.d.Paechter, p)
	a.st.ensureYear(a.st.d.Settings.Jahr).Ablesungen[p.ID] = Ablesung{}
	a.st.gartenVergeben(p.ID, p.Gartennr, p.Mitgliedsnr, p.Name, in.Seit)
	a.st.d.Warteliste = append(a.st.d.Warteliste[:wi], a.st.d.Warteliste[wi+1:]...)
	a.st.audit("Garten %s von der Warteliste vergeben: %s %s", p.Gartennr, p.Mitgliedsnr, p.Name)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, p)
}

type lageGarten struct {
	Nr, Name string
	Frei     bool
	X, Y     float64
	Umriss   []Punkt
}

// lagePlan ist alles, was zum Zeichnen des Lageplans gebraucht wird.
type lagePlan struct {
	Gaerten []lageGarten
	Karte   LageKarte
	Bild    []byte // Hintergrund als JPEG (leer = ohne)
}

// lageplanGaerten stellt die Gärten für den Ausdruck zusammen: gespeicherte Positionen oder, ohne Karte,
// ein Raster nach Gartennummer. Der Aufrufer hält s.mu.
func (s *Store) lageplanGaertenLocked(mitNamen bool) []lageGarten {
	belegt := s.belegteGaertenLocked()
	name := func(nr string) string {
		if p, ok := belegt[nr]; ok && mitNamen {
			return p.Name
		}
		return ""
	}
	var out []lageGarten
	if len(s.d.Lageplan) > 0 {
		for nr, pos := range s.d.Lageplan {
			_, ok := belegt[nr]
			out = append(out, lageGarten{Nr: nr, Name: name(nr), Frei: !ok, X: float64(pos.X), Y: float64(pos.Y), Umriss: pos.Umriss})
		}
		sort.SliceStable(out, func(i, j int) bool { return naturalLess(out[i].Nr, out[j].Nr) })
		return out
	}
	nrs := make([]string, 0, len(belegt))
	for nr := range belegt {
		nrs = append(nrs, nr)
	}
	sort.SliceStable(nrs, func(i, j int) bool { return naturalLess(nrs[i], nrs[j]) })
	for i, nr := range nrs {
		out = append(out, lageGarten{Nr: nr, Name: name(nr), X: float64(20 + (i%8)*(lpBreite+20)), Y: float64(20 + (i/8)*(lpHoehe+30))})
	}
	return out
}

// lagePlanLocked: Gärten samt Hintergrund, Leitungen und Einrichtungen. Der Aufrufer hält s.mu.
func (s *Store) lagePlanLocked(mitNamen bool) lagePlan {
	lp := lagePlan{Gaerten: s.lageplanGaertenLocked(mitNamen), Karte: s.d.LageKarte}
	lp.Karte.Objekte = append([]LageObjekt(nil), s.d.LageKarte.Objekte...)
	lp.Karte.Leitungen = append([]LageLinie(nil), s.d.LageKarte.Leitungen...)
	if lp.Karte.Bild != "" {
		lp.Bild, _ = s.karteDaten(lp.Karte.Bild)
	}
	return lp
}

// leer: nichts zu zeichnen (weder Gärten noch Bild).
func (lp lagePlan) leer() bool { return len(lp.Gaerten) == 0 && len(lp.Bild) == 0 }

// buildLageplan zeichnet den Lageplan als Aushang (A4 quer), maßstäblich zur Anordnung im Programm. Der
// Abrechnungsstand erscheint bewusst nicht: wer was bezahlt hat, gehört nicht an die Pinnwand.
func buildLageplan(s Settings, lp lagePlan, mitNamen bool) ([]byte, error) {
	if lp.leer() {
		return nil, bad("Es gibt noch keine Gärten mit Gartennummer")
	}
	const links, rechts = 12.0, 285.0
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle("Lageplan", true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AddPage()
	mr, mg, mb := markenfarbe(s.Farbe)
	pdf.SetFont("lib", "B", 16)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(links, 11)
	pdf.CellFormat(rechts-links, 8, "Lageplan – "+s.VereinName, "", 0, "L", false, 0, "")
	pdf.SetFillColor(mr, mg, mb)
	pdf.Rect(links, 21, rechts-links, 0.8, "F")
	pdf.SetTextColor(0, 0, 0)

	unten := 192.0
	legende := lageLegende(lp.Karte)
	if len(legende) > 0 {
		unten = 186
	}
	frei, _ := zeichneLageplan(pdf, s, lp, links, 30, rechts, unten, "")
	y := unten + 3
	if len(legende) > 0 {
		x := links
		pdf.SetFont("lib", "", 7.5)
		for _, e := range legende {
			w := pdf.GetStringWidth(e.text) + 9
			if x+w > rechts {
				break
			}
			e.zeichne(pdf, x+2, y+2)
			pdf.SetTextColor(60, 60, 60)
			pdf.SetXY(x+5, y)
			pdf.CellFormat(w-5, 4, e.text, "", 0, "L", false, 0, "")
			x += w
		}
		y += 6
	}
	pdf.SetFont("lib", "", 8)
	pdf.SetTextColor(100, 100, 100)
	hinweis := fmt.Sprintf("%d Gärten", len(lp.Gaerten))
	if frei > 0 {
		hinweis += fmt.Sprintf(", davon %d frei (gestrichelt)", frei)
	}
	if mitNamen {
		hinweis += " · enthält Namen – nur für den Vorstand"
	}
	pdf.SetXY(links, max(y, 199))
	pdf.CellFormat(rechts-links, 4, hinweis+" · Stand "+time.Now().Format("02.01.2006"), "", 0, "L", false, 0, "")
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type legendenEintrag struct {
	text    string
	zeichne func(pdf *fpdf.Fpdf, x, y float64)
}

// lageLegende: Erklärung der Kürzel und Linien, die im Plan vorkommen (in fester Reihenfolge).
func lageLegende(k LageKarte) []legendenEintrag {
	var out []legendenEintrag
	da := map[string]bool{}
	for _, o := range k.Objekte {
		da["o:"+o.Art] = true
	}
	for _, l := range k.Leitungen {
		da["l:"+l.Art] = true
	}
	for _, art := range []string{"wasserzaehler", "absperr", "schacht", "strom", "tor", "vereinsheim", "kompost", "parkplatz", "sonstiges"} {
		if da["o:"+art] {
			out = append(out, legendenEintrag{objektArten[art].Name, func(pdf *fpdf.Fpdf, x, y float64) { zeichneObjekt(pdf, art, x, y, 1.6) }})
		}
	}
	for _, art := range []string{"wasser", "strom", "abwasser", "weg"} {
		if da["l:"+art] {
			out = append(out, legendenEintrag{linienArten[art].Name, func(pdf *fpdf.Fpdf, x, y float64) {
				zeichneLinie(pdf, art, []float64{x - 2, x + 2}, []float64{y, y})
			}})
		}
	}
	return out
}

func zeichneObjekt(pdf *fpdf.Fpdf, art string, x, y, radius float64) {
	a := objektArten[art]
	pdf.SetDrawColor(255, 255, 255)
	pdf.SetLineWidth(0.3)
	pdf.SetFillColor(a.R, a.G, a.B)
	pdf.Circle(x, y, radius, "FD")
	pdf.SetFont("lib", "B", radius*4.2)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(x-radius, y-radius)
	pdf.CellFormat(radius*2, radius*2, a.Kurz, "", 0, "CM", false, 0, "")
	pdf.SetFont("lib", "", 7.5)
}

func zeichneLinie(pdf *fpdf.Fpdf, art string, xs, ys []float64) {
	l := linienArten[art]
	pdf.SetDrawColor(l.R, l.G, l.B)
	pdf.SetLineWidth(0.7)
	if l.Strichel {
		pdf.SetDashPattern([]float64{1.5, 1}, 0)
	}
	for i := 1; i < len(xs); i++ {
		pdf.Line(xs[i-1], ys[i-1], xs[i], ys[i])
	}
	pdf.SetDashPattern([]float64{}, 0)
	pdf.SetLineWidth(0.2)
}

// zeichneLageplan zeichnet die Gärten maßstäblich in das Rechteck (links, oben)–(rechts, unten), auf Wunsch
// über dem Hintergrundbild, mit Leitungen und Einrichtungen. Ist markiert gesetzt, wird dieser Garten kräftig
// hervorgehoben (Willkommensmappe, Exposé). Liefert die Zahl freier Gärten und die tatsächlich belegte Höhe.
func zeichneLageplan(pdf *fpdf.Fpdf, s Settings, lp lagePlan, links, oben, rechts, unten float64, markiert string) (int, float64) {
	if lp.leer() {
		return 0, 0
	}
	gs, k := lp.Gaerten, lp.Karte
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	erweitere := func(x0, y0, x1, y1 float64) {
		minX, minY = math.Min(minX, x0), math.Min(minY, y0)
		maxX, maxY = math.Max(maxX, x1), math.Max(maxY, y1)
	}
	for _, g := range gs {
		if len(g.Umriss) > 0 {
			for _, p := range g.Umriss {
				erweitere(float64(p.X), float64(p.Y), float64(p.X), float64(p.Y))
			}
			continue
		}
		erweitere(g.X, g.Y, g.X+lpBreite, g.Y+lpHoehe)
	}
	for _, o := range k.Objekte {
		erweitere(float64(o.X)-15, float64(o.Y)-15, float64(o.X)+15, float64(o.Y)+15)
	}
	for _, l := range k.Leitungen {
		for _, p := range l.Punkte {
			erweitere(float64(p.X), float64(p.Y), float64(p.X), float64(p.Y))
		}
	}
	bildB, bildH := float64(k.BildBreite)*k.Massstab, float64(k.BildHoehe)*k.Massstab
	if len(lp.Bild) > 0 && bildB > 0 && bildH > 0 {
		if math.IsInf(minX, 1) {
			erweitere(0, 0, bildB, bildH)
		} else {
			// etwas Umgebung zeigen, aber nicht über den Bildrand hinaus
			const rand = 40
			minX, minY = math.Max(0, minX-rand), math.Max(0, minY-rand)
			maxX, maxY = math.Min(math.Max(bildB, maxX), maxX+rand), math.Min(math.Max(bildH, maxY), maxY+rand)
		}
	}
	// ein Bildpunkt im Programm = faktor mm, höchstens so groß, dass eine Kachel 40 mm breit wird
	faktor := math.Min(math.Min((rechts-links)/(maxX-minX), (unten-oben)/(maxY-minY)), 40.0/lpBreite)
	versatzX := links + ((rechts-links)-(maxX-minX)*faktor)/2
	px := func(x float64) float64 { return versatzX + (x-minX)*faktor }
	py := func(y float64) float64 { return oben + (y-minY)*faktor }
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.88)

	mitBild := false
	if len(lp.Bild) > 0 && bildB > 0 {
		name := "lagekarte-" + k.Bild
		info := pdf.RegisterImageOptionsReader(name, fpdf.ImageOptions{ImageType: "JPG"}, bytes.NewReader(lp.Bild))
		if info != nil && pdf.Ok() {
			pdf.ClipRect(px(minX), py(minY), (maxX-minX)*faktor, (maxY-minY)*faktor, false)
			pdf.SetAlpha(math.Max(0.1, math.Min(1, k.Deckkraft)), "Normal")
			pdf.ImageOptions(name, px(0), py(0), bildB*faktor, bildH*faktor, false, fpdf.ImageOptions{ImageType: "JPG"}, 0, "")
			pdf.SetAlpha(1, "Normal")
			pdf.ClipEnd()
			mitBild = true
		}
	}

	for _, l := range k.Leitungen {
		xs, ys := make([]float64, len(l.Punkte)), make([]float64, len(l.Punkte))
		for i, p := range l.Punkte {
			xs[i], ys[i] = px(float64(p.X)), py(float64(p.Y))
		}
		zeichneLinie(pdf, l.Art, xs, ys)
	}

	frei := 0
	for _, g := range gs {
		hervor := markiert != "" && g.Nr == markiert
		if g.Frei {
			frei++
		}
		var x, y, w, h float64
		pdf.SetLineWidth(0.35)
		pdf.SetDrawColor(mr, mg, mb)
		if len(g.Umriss) > 0 {
			pts := make([]fpdf.PointType, len(g.Umriss))
			for i, p := range g.Umriss {
				pts[i] = fpdf.PointType{X: px(float64(p.X)), Y: py(float64(p.Y))}
			}
			switch {
			case hervor:
				pdf.SetLineWidth(0.8)
				pdf.SetFillColor(mr, mg, mb)
				pdf.SetAlpha(0.75, "Normal")
			case g.Frei:
				pdf.SetDashPattern([]float64{1.6, 1.2}, 0)
				pdf.SetFillColor(255, 255, 255)
				pdf.SetAlpha(0.55, "Normal")
			default:
				pdf.SetFillColor(fr, fg, fb)
				pdf.SetAlpha(0.6, "Normal")
			}
			pdf.Polygon(pts, "F")
			pdf.SetAlpha(1, "Normal")
			pdf.Polygon(pts, "D")
			pdf.SetDashPattern([]float64{}, 0)
			// Beschriftung so groß, wie die Fläche es hergibt
			cx, cy := umrissMitte(g.Umriss)
			bx0, by0, bx1, by1 := maxKartenKoord, maxKartenKoord, 0, 0
			for _, p := range g.Umriss {
				bx0, by0, bx1, by1 = min(bx0, p.X), min(by0, p.Y), max(bx1, p.X), max(by1, p.Y)
			}
			w = math.Max(6, math.Min(float64(bx1-bx0)*faktor*0.8, 40))
			h = math.Max(3.5, math.Min(float64(by1-by0)*faktor*0.6, lpHoehe*faktor))
			x, y = px(cx)-w/2, py(cy)-h/2
		} else {
			x, y = px(g.X), py(g.Y)
			w, h = lpBreite*faktor, lpHoehe*faktor
			switch {
			case hervor:
				pdf.SetFillColor(mr, mg, mb)
				pdf.SetLineWidth(0.8)
				pdf.RoundedRect(x, y, w, h, 1.5, "1234", "FD")
			case g.Frei:
				pdf.SetDashPattern([]float64{1.6, 1.2}, 0)
				pdf.SetFillColor(255, 255, 255)
				pdf.RoundedRect(x, y, w, h, 1.5, "1234", "FD")
				pdf.SetDashPattern([]float64{}, 0)
			default:
				pdf.SetFillColor(fr, fg, fb)
				pdf.RoundedRect(x, y, w, h, 1.5, "1234", "FD")
			}
		}
		groesse := math.Max(5, math.Min(16, h*0.42/0.3528*0.6))
		pdf.SetTextColor(mr, mg, mb)
		if hervor {
			pdf.SetTextColor(255, 255, 255)
		}
		mitUnterzeile := len(g.Umriss) == 0 || h > 8
		nrY, nrH := y+h*0.12, h*0.42
		if !mitUnterzeile {
			nrY, nrH = y, h
		}
		pdf.SetXY(x, nrY)
		pdf.CellFormat(w, nrH, passendKleinStil(pdf, "B", g.Nr, w-1, groesse, 4), "", 0, "C", false, 0, "")
		pdf.SetTextColor(60, 60, 60)
		unterzeile := g.Name
		if g.Frei && !hervor {
			unterzeile = "frei"
		}
		if unterzeile != "" && h > 6 && mitUnterzeile {
			pdf.SetXY(x+1, y+h*0.56)
			pdf.CellFormat(w-2, h*0.3, passendKlein(pdf, unterzeile, w-3, math.Max(5, groesse*0.55), 4), "", 0, "C", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
	}

	radius := math.Max(1.4, math.Min(2.6, 14*faktor))
	for _, o := range k.Objekte {
		x, y := px(float64(o.X)), py(float64(o.Y))
		zeichneObjekt(pdf, o.Art, x, y, radius)
		if o.Bez != "" {
			pdf.SetFont("lib", "", 6.5)
			pdf.SetTextColor(40, 40, 40)
			pdf.SetXY(x+radius+0.6, y-1.6)
			pdf.CellFormat(pdf.GetStringWidth(o.Bez)+1, 3.2, o.Bez, "", 0, "L", false, 0, "")
		}
	}
	pdf.SetTextColor(0, 0, 0)

	hoehe := (maxY - minY) * faktor
	if mitBild {
		// Quellenvermerk
		if k.Quelle != "" {
			pdf.SetFont("lib", "", 6)
			breite := pdf.GetStringWidth(k.Quelle) + 2
			pdf.SetFillColor(255, 255, 255)
			pdf.SetAlpha(0.8, "Normal")
			pdf.Rect(px(maxX)-breite, oben+hoehe-3.2, breite, 3.2, "F")
			pdf.SetAlpha(1, "Normal")
			pdf.SetTextColor(60, 60, 60)
			pdf.SetXY(px(maxX)-breite, oben+hoehe-3.2)
			pdf.CellFormat(breite, 3.2, k.Quelle, "", 0, "C", false, 0, "")
		}
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetLineWidth(0.2)
	pdf.SetDrawColor(0, 0, 0)
	return frei, hoehe
}

// lageplanPDF: Namen nur auf Wunsch und nur für den Admin (der Aushang ist sonst ohne Personenbezug).
func (a *App) lageplanMitNamen(r *http.Request) bool {
	return r.URL.Query().Get("namen") == "1" && a.isAdmin(r)
}

func (a *App) lageplanPDF(r *http.Request) ([]byte, error) {
	mitNamen := a.lageplanMitNamen(r)
	a.st.mu.Lock()
	lp := a.st.lagePlanLocked(mitNamen)
	s := a.st.d.Settings
	a.st.mu.Unlock()
	return buildLageplan(s, lp, mitNamen)
}

func (a *App) handleLageplanDruck(w http.ResponseWriter, r *http.Request) {
	pdf, err := a.lageplanPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="Lageplan.pdf"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleLageplanDruckOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, err := a.lageplanPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	full, err := a.st.schreibeDruckdatei("Lageplan.pdf", pdf)
	if err != nil {
		writeErr(w, err)
		return
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]string{"datei": filepath.ToSlash(filepath.Join(druckOrdner, filepath.Base(full)))})
}
