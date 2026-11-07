package main

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Gartenakte: alles zu einem Garten an einer Stelle – wer ihn wann hatte, Übergaben, Wertermittlungen und
// Dokumente (Bauantrag, Genehmigung, Fotos, Pachtvertrag). Die Akte hängt am Garten, nicht an der Person:
// ein Foto der Laube bleibt beim Garten, wenn der Pächter wechselt. Dokumente, die zur Person gehören
// (Pachtvertrag, Schriftverkehr), werden beim endgültigen Löschen des Pächters mit entfernt.

// GartenDokument ist eine Datei in der Gartenakte.
type GartenDokument struct {
	ID       string `json:"id"`
	Gartennr string `json:"gartennr"`
	Titel    string `json:"titel"`
	Art      string `json:"art"`
	Datum    string `json:"datum"` // Datum des Dokuments (JJJJ-MM-TT)
	Datei    string `json:"datei"` // relativ zum Datenordner, immer unter »Gartenakte/«
	Groesse  int    `json:"groesse"`
	// PaechterID: gehört zur Person (z. B. Pachtvertrag) und wird mit ihr gelöscht; leer = gehört zum Garten
	PaechterID  string `json:"paechterId,omitempty"`
	Hochgeladen string `json:"hochgeladen"`
}

var dokumentArten = []string{"Bauantrag", "Genehmigung", "Foto", "Pachtvertrag", "Schriftverkehr", "Protokoll", "Sonstiges"}

const (
	gartenakteOrdner    = "Gartenakte"
	maxDokumenteJeAkte  = 200
	maxDokumentGroesse  = 12 << 20
	maxWertPositionen   = 100
	maxWertermittlungen = 2000
)

// dokumentDatei prüft einen gespeicherten Pfad wie belegDatei: nur unter »Gartenakte/«, kein »..«, erlaubte Endung.
func (s *Store) dokumentDatei(rel string) (string, error) {
	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, gartenakteOrdner+"/") || strings.Contains(rel, "..") || !belegExtensions[strings.ToLower(path.Ext(rel))] {
		return "", errors.New("ungültiger Dokumentpfad")
	}
	base := filepath.Join(s.dir, gartenakteOrdner) + string(filepath.Separator)
	full := filepath.Join(s.dir, filepath.FromSlash(rel))
	if !strings.HasPrefix(full, base) {
		return "", errors.New("ungültiger Dokumentpfad")
	}
	return full, nil
}

func (s *Store) entferneDokumentDatei(rel string) {
	full, err := s.dokumentDatei(rel)
	if err != nil {
		return
	}
	if fi, err := os.Lstat(full); err == nil && fi.Mode().IsRegular() {
		_ = os.Remove(full)
	}
}

// aktiverPaechterLocked liefert den Pächter, der den Garten gerade hat. Der Aufrufer hält s.mu.
func (s *Store) aktiverPaechterLocked(gartennr string) (Paechter, bool) {
	p, ok := s.belegteGaertenLocked()[gartennr]
	return p, ok
}

func (a *App) handleDokumentUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxDokumentGroesse+1<<16)
	if err := r.ParseMultipartForm(maxDokumentGroesse); err != nil {
		writeErr(w, bad("Die Datei ist zu groß oder konnte nicht gelesen werden (höchstens 12 MB)"))
		return
	}
	gartennr := trim(r.FormValue("gartennr"), 20)
	titel := trim(strings.Join(strings.Fields(r.FormValue("titel")), " "), 120)
	art := r.FormValue("art")
	datum := strings.TrimSpace(r.FormValue("datum"))
	zurPerson := r.FormValue("zurPerson") == "1"
	if gartennr == "" {
		writeErr(w, bad("Gartennummer fehlt"))
		return
	}
	gueltig := false
	for _, x := range dokumentArten {
		gueltig = gueltig || x == art
	}
	if !gueltig {
		art = "Sonstiges"
	}
	if datum == "" {
		datum = time.Now().Format("2006-01-02")
	}
	if _, err := parseDate(datum); err != nil {
		writeErr(w, bad("Das Datum ist ungültig"))
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte eine Datei auswählen"))
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	if !belegExtensions[ext] {
		writeErr(w, bad("Bitte ein Foto (JPG, PNG, WebP) oder eine PDF-Datei auswählen"))
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxDokumentGroesse+1))
	if err != nil || len(data) > maxDokumentGroesse {
		writeErr(w, bad("Die Datei ist zu groß (höchstens 12 MB)"))
		return
	}
	if !belegInhaltPasst(ext, data) {
		writeErr(w, bad("Der Inhalt der Datei passt nicht zur Dateiendung. Bitte ein echtes Foto oder eine PDF-Datei wählen"))
		return
	}
	if titel == "" {
		titel = strings.TrimSuffix(trim(filepath.Base(hdr.Filename), 120), filepath.Ext(hdr.Filename))
	}

	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	n := 0
	for _, d := range a.st.d.Dokumente {
		if d.Gartennr == gartennr {
			n++
		}
	}
	if n >= maxDokumenteJeAkte {
		writeErr(w, bad(fmt.Sprintf("Höchstens %d Dokumente je Garten", maxDokumenteJeAkte)))
		return
	}
	doc := GartenDokument{ID: newID(), Gartennr: gartennr, Titel: titel, Art: art, Datum: datum, Groesse: len(data), Hochgeladen: time.Now().Format(time.RFC3339)}
	if zurPerson {
		p, ok := a.st.aktiverPaechterLocked(gartennr)
		if !ok {
			writeErr(w, bad("Garten "+gartennr+" hat gerade keinen Pächter, dem das Dokument gehören könnte"))
			return
		}
		doc.PaechterID = p.ID
	}
	dir := filepath.Join(a.st.dir, gartenakteOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	name := doc.ID + ext
	if err := schreibeAtomar(filepath.Join(dir, name), data, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	doc.Datei = gartenakteOrdner + "/" + name
	a.st.d.Dokumente = append(a.st.d.Dokumente, doc)
	a.st.audit("Gartenakte %s: Dokument hinzugefügt (%s)", gartennr, art)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, doc)
}

func (a *App) handleDokumentDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, d := range a.st.d.Dokumente {
		if d.ID == id {
			a.st.d.Dokumente = append(a.st.d.Dokumente[:i], a.st.d.Dokumente[i+1:]...)
			a.st.entferneDokumentDatei(d.Datei)
			a.st.audit("Gartenakte %s: Dokument gelöscht (%s)", d.Gartennr, d.Art)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Dokument nicht gefunden"))
}

// handleDokumentServe liefert ein Dokument aus. Hochgeladene Dateien gelten als nicht vertrauenswürdig:
// sie laufen in einer Sandbox ohne Skripte und ohne Zugriff auf das Programm.
func (a *App) handleDokumentServe(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	var rel string
	for _, d := range a.st.d.Dokumente {
		if d.ID == id {
			rel = d.Datei
		}
	}
	a.st.mu.Unlock()
	if rel == "" {
		writeErr(w, notFound("Dokument nicht gefunden"))
		return
	}
	full, err := a.st.dokumentDatei(rel)
	if err != nil {
		writeErr(w, bad("Der gespeicherte Dokumentpfad ist ungültig"))
		return
	}
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		writeErr(w, notFound("Die Datei fehlt"))
		return
	}
	f, err := os.Open(full)
	if err != nil {
		writeErr(w, notFound("Die Datei fehlt"))
		return
	}
	defer f.Close()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if strings.ToLower(path.Ext(full)) != ".pdf" {
		// Bilder in einer Sandbox ausliefern; bei PDF-Dateien würde die Sandbox den PDF-Betrachter des Browsers
		// blockieren – dort schützt wie bei den Belegen die Prüfung der Dateisignatur beim Hochladen
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	}
	http.ServeContent(w, r, path.Base(full), fi.ModTime(), f)
}

// ---------------------------------------------------------------- Wertermittlung

// WertPosition ist eine Zeile der Wertermittlung, z. B. »Laube, Holz, 12 m²«.
type WertPosition struct {
	Bezeichnung string  `json:"bezeichnung"`
	Kategorie   string  `json:"kategorie"` // Baulichkeiten | Anpflanzungen | Einrichtungen | Abzug
	Menge       float64 `json:"menge"`
	Einheit     string  `json:"einheit"`
	Einzelwert  float64 `json:"einzelwert"`
	Abschlag    float64 `json:"abschlag"` // Prozent für Alter und Zustand
}

func (p WertPosition) wert() float64 {
	w := round2(p.Menge * p.Einzelwert * (1 - p.Abschlag/100))
	if p.Kategorie == "Abzug" {
		return -w
	}
	return w
}

// Wertermittlung schätzt den Wert von Laube, Anpflanzungen und Einrichtungen eines Gartens, z. B. als
// Grundlage für die Ablösesumme beim Pächterwechsel. Die verbindliche Bewertung nach den Richtlinien des
// Landesverbands nimmt eine geschulte Wertermittlerin oder ein Wertermittler vor; hier wird sie festgehalten.
type Wertermittlung struct {
	ID            string         `json:"id"`
	Gartennr      string         `json:"gartennr"`
	Datum         string         `json:"datum"`
	Anlass        string         `json:"anlass"`
	Wertermittler string         `json:"wertermittler,omitempty"`
	Positionen    []WertPosition `json:"positionen"`
	Bemerkung     string         `json:"bemerkung,omitempty"`
	Summe         float64        `json:"summe"`
}

var wertKategorien = []string{"Baulichkeiten", "Anpflanzungen", "Einrichtungen", "Abzug"}

func cleanWertermittlung(in Wertermittlung) (Wertermittlung, error) {
	in.Gartennr = trim(in.Gartennr, 20)
	in.Anlass = trim(in.Anlass, 60)
	in.Wertermittler = trim(in.Wertermittler, 100)
	in.Bemerkung = trim(in.Bemerkung, 1500)
	if in.Gartennr == "" {
		return in, bad("Gartennummer fehlt")
	}
	if _, err := parseDate(in.Datum); err != nil {
		return in, bad("Bitte das Datum der Wertermittlung eintragen")
	}
	if in.Anlass == "" {
		in.Anlass = "Pächterwechsel"
	}
	if len(in.Positionen) > maxWertPositionen {
		return in, bad(fmt.Sprintf("Höchstens %d Positionen", maxWertPositionen))
	}
	var pos []WertPosition
	summe := 0.0
	for _, p := range in.Positionen {
		p.Bezeichnung = trim(strings.Join(strings.Fields(p.Bezeichnung), " "), 120)
		p.Einheit = trim(p.Einheit, 15)
		if p.Bezeichnung == "" && p.Einzelwert == 0 {
			continue // leere Zeile
		}
		if p.Bezeichnung == "" {
			return in, bad("Jede Position braucht eine Bezeichnung")
		}
		if !validNum(p.Menge) || !validNum(p.Einzelwert) || !validNum(p.Abschlag) || p.Abschlag > 100 {
			return in, bad("Menge und Wert müssen Zahlen ab 0 sein, der Abschlag zwischen 0 und 100 %")
		}
		if p.Menge == 0 {
			p.Menge = 1
		}
		ok := false
		for _, k := range wertKategorien {
			ok = ok || k == p.Kategorie
		}
		if !ok {
			p.Kategorie = "Einrichtungen"
		}
		summe += p.wert()
		pos = append(pos, p)
	}
	if len(pos) == 0 {
		return in, bad("Bitte mindestens eine Position eintragen, z. B. die Laube")
	}
	in.Positionen = pos
	in.Summe = round2(math.Max(0, summe))
	return in, nil
}

func (a *App) handleWertermittlungSpeichern(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id") // leer beim Anlegen
	var in Wertermittlung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanWertermittlung(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if id == "" {
		if len(a.st.d.Wertermittlungen) >= maxWertermittlungen {
			writeErr(w, bad("Zu viele Wertermittlungen"))
			return
		}
		in.ID = newID()
		a.st.d.Wertermittlungen = append(a.st.d.Wertermittlungen, in)
		a.st.audit("Wertermittlung Garten %s angelegt: %s", in.Gartennr, fmtEUR(in.Summe))
	} else {
		i := -1
		for k := range a.st.d.Wertermittlungen {
			if a.st.d.Wertermittlungen[k].ID == id {
				i = k
			}
		}
		if i < 0 {
			writeErr(w, notFound("Wertermittlung nicht gefunden"))
			return
		}
		in.ID = id
		a.st.d.Wertermittlungen[i] = in
		a.st.audit("Wertermittlung Garten %s geändert: %s", in.Gartennr, fmtEUR(in.Summe))
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handleWertermittlungDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, x := range a.st.d.Wertermittlungen {
		if x.ID == id {
			a.st.d.Wertermittlungen = append(a.st.d.Wertermittlungen[:i], a.st.d.Wertermittlungen[i+1:]...)
			a.st.audit("Wertermittlung Garten %s gelöscht", x.Gartennr)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Wertermittlung nicht gefunden"))
}

func buildWertermittlung(s Settings, x Wertermittlung) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle("Wertermittlung Garten "+x.Gartennr, true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	pdf.AliasNbPages("{nb}")
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	pdf.SetFooterFunc(func() {
		pdf.SetFont("lib", "", 7.5)
		pdf.SetTextColor(110, 110, 110)
		pdf.SetXY(leftX, 287)
		pdf.CellFormat(rightX-leftX, 4, fmt.Sprintf("%s · Wertermittlung Garten %s · Seite %d von {nb}", s.VereinName, x.Gartennr, pdf.PageNo()), "", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	cols := []struct {
		t     string
		w     float64
		align string
	}{{"Position", 62, "L"}, {"Menge", 22, "R"}, {"Einzelwert", 26, "R"}, {"Abschlag", 22, "R"}, {"Wert", 38, "R"}}
	y := 0.0
	kopf := func() {
		pdf.AddPage()
		zeichneKopf(pdf, s)
		y = 34
	}
	tabKopf := func() {
		pdf.SetFillColor(fr, fg, fb)
		pdf.SetFont("lib", "B", 9)
		xx := leftX
		for _, c := range cols {
			pdf.SetXY(xx, y)
			pdf.CellFormat(c.w, 7, " "+c.t+" ", "", 0, c.align, true, 0, "")
			xx += c.w
		}
		y += 8
	}
	kopf()
	pdf.SetFont("lib", "B", 16)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 8, "Wertermittlung Garten Nr. "+x.Gartennr, "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 9
	pdf.SetFont("lib", "", 10)
	info := germanDate(x.Datum) + " · Anlass: " + x.Anlass
	if x.Wertermittler != "" {
		info += " · Wertermittlung durch " + x.Wertermittler
	}
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 6, info, "", 0, "L", false, 0, "")
	y += 11

	for _, kat := range wertKategorien {
		var ps []WertPosition
		for _, p := range x.Positionen {
			if p.Kategorie == kat {
				ps = append(ps, p)
			}
		}
		if len(ps) == 0 {
			continue
		}
		if y+24 > 270 {
			kopf()
		}
		titel := kat
		if kat == "Abzug" {
			titel = "Abzüge (z. B. Beseitigung von Mängeln, Rückbau)"
		}
		pdf.SetFont("lib", "B", 11)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6, titel, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 7
		tabKopf()
		zw := 0.0
		for _, p := range ps {
			if y+7 > 272 {
				kopf()
				tabKopf()
			}
			menge := fmtFlex(p.Menge)
			if p.Einheit != "" {
				menge += " " + p.Einheit
			}
			abschlag := ""
			if p.Abschlag > 0 {
				abschlag = fmtFlex(p.Abschlag) + " %"
			}
			cells := []string{p.Bezeichnung, menge, fmtEUR(p.Einzelwert), abschlag, fmtEUR(p.wert())}
			xx := leftX
			pdf.SetFont("lib", "", 9.5)
			for i, c := range cols {
				pdf.SetXY(xx, y)
				txt := cells[i]
				if i == 0 {
					txt = passendKlein(pdf, txt, c.w-2, 9.5, 7)
				}
				pdf.CellFormat(c.w, 6.5, " "+txt+" ", "", 0, c.align, false, 0, "")
				xx += c.w
			}
			pdf.SetDrawColor(225, 228, 222)
			pdf.SetLineWidth(0.2)
			pdf.Line(leftX, y+6.7, rightX, y+6.7)
			y += 7
			zw += p.wert()
		}
		pdf.SetFont("lib", "B", 9.5)
		pdf.SetXY(leftX, y)
		pdf.CellFormat(rightX-leftX, 6.5, "Zwischensumme "+kat+": "+fmtEUR(zw)+" ", "", 0, "R", false, 0, "")
		y += 11
	}
	if y+30 > 275 {
		kopf()
	}
	betragsBox(pdf, s, leftX+70, y, "Ermittelter Wert:", fmtEUR(x.Summe))
	y += 14
	if x.Bemerkung != "" {
		pdf.SetFont("lib", "", 9.5)
		pdf.SetXY(leftX, y)
		pdf.MultiCell(rightX-leftX, 5, "Bemerkung: "+x.Bemerkung, "", "L", false)
		y = pdf.GetY() + 4
	}
	pdf.SetFont("lib", "", 8.5)
	pdf.SetTextColor(90, 90, 90)
	pdf.SetXY(leftX, y)
	pdf.MultiCell(rightX-leftX, 4.4, "Wert je Position = Menge × Einzelwert, vermindert um den Abschlag für Alter und Zustand. "+
		"Maßgeblich sind die Richtlinien des zuständigen Landesverbands für die Wertermittlung bei Pächterwechsel.", "", "L", false)
	pdf.SetTextColor(0, 0, 0)
	y = math.Max(pdf.GetY()+16, 255)
	if y > 268 {
		kopf()
		y = 60
	}
	sw := (rightX - leftX - 12) / 3
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.3)
	for i, t := range []string{"Wertermittlung", "Bisheriger Pächter", "Neuer Pächter"} {
		xx := leftX + float64(i)*(sw+6)
		pdf.Line(xx, y, xx+sw, y)
		pdf.SetFont("lib", "", 8.5)
		pdf.SetXY(xx, y+1)
		pdf.CellFormat(sw, 5, t, "", 0, "C", false, 0, "")
	}
	return pdfBytes(pdf)
}

func (a *App) wertermittlungPDF(id string) ([]byte, string, error) {
	a.st.mu.Lock()
	var x *Wertermittlung
	for i := range a.st.d.Wertermittlungen {
		if a.st.d.Wertermittlungen[i].ID == id {
			c := a.st.d.Wertermittlungen[i]
			x = &c
		}
	}
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if x == nil {
		return nil, "", notFound("Wertermittlung nicht gefunden")
	}
	pdf, err := buildWertermittlung(s, *x)
	return pdf, safeName("Wertermittlung_Garten_"+x.Gartennr+"_"+x.Datum, 80) + ".pdf", err
}

func (a *App) handleWertermittlungPDF(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.wertermittlungPDF(r.PathValue("id"))
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleWertermittlungPDFOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.wertermittlungPDF(r.PathValue("id"))
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

// ---------------------------------------------------------------- Akte eines Gartens

type gartenakte struct {
	Gartennr         string           `json:"gartennr"`
	Paechter         *Paechter        `json:"paechter,omitempty"` // wer ihn gerade hat
	Historie         []GartenEintrag  `json:"historie"`
	Uebergaben       []Uebergabe      `json:"uebergaben"`
	Wertermittlungen []Wertermittlung `json:"wertermittlungen"`
	Dokumente        []GartenDokument `json:"dokumente"`
	AufKarte         bool             `json:"aufKarte"`
}

func (a *App) handleGartenakte(w http.ResponseWriter, r *http.Request) {
	nr := trim(r.URL.Query().Get("gartennr"), 20)
	if nr == "" {
		writeErr(w, bad("Gartennummer fehlt"))
		return
	}
	a.st.mu.Lock()
	ak := gartenakte{Gartennr: nr, Historie: []GartenEintrag{}, Uebergaben: []Uebergabe{}, Wertermittlungen: []Wertermittlung{}, Dokumente: []GartenDokument{}}
	if p, ok := a.st.aktiverPaechterLocked(nr); ok {
		ak.Paechter = &p
	}
	for _, e := range a.st.d.GartenHistorie {
		if e.Gartennr == nr {
			ak.Historie = append(ak.Historie, e)
		}
	}
	for _, u := range a.st.d.Uebergaben {
		if u.Gartennr == nr {
			ak.Uebergaben = append(ak.Uebergaben, u)
		}
	}
	for _, x := range a.st.d.Wertermittlungen {
		if x.Gartennr == nr {
			ak.Wertermittlungen = append(ak.Wertermittlungen, x)
		}
	}
	for _, d := range a.st.d.Dokumente {
		if d.Gartennr == nr {
			ak.Dokumente = append(ak.Dokumente, d)
		}
	}
	_, ak.AufKarte = a.st.d.Lageplan[nr]
	a.st.mu.Unlock()
	sort.SliceStable(ak.Historie, func(i, j int) bool { return ak.Historie[i].Seit > ak.Historie[j].Seit })
	sort.SliceStable(ak.Uebergaben, func(i, j int) bool { return ak.Uebergaben[i].Datum > ak.Uebergaben[j].Datum })
	sort.SliceStable(ak.Wertermittlungen, func(i, j int) bool { return ak.Wertermittlungen[i].Datum > ak.Wertermittlungen[j].Datum })
	sort.SliceStable(ak.Dokumente, func(i, j int) bool { return ak.Dokumente[i].Datum > ak.Dokumente[j].Datum })
	writeJSON(w, 200, ak)
}
