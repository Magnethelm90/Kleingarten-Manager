package main

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// Jahresplaner: Termine des Vereins (Arbeitseinsätze, Versammlungen, Ablesung, Feste). Jeder kann sie
// ansehen, als Kalenderdatei mitnehmen und als Aushang drucken; ändern darf sie nur der Admin. Termine
// enthalten keine Personendaten (Hinweis in der Oberfläche).

// Termin ist ein Eintrag im Jahresplaner.
type Termin struct {
	ID    string `json:"id"`
	Datum string `json:"datum"`         // JJJJ-MM-TT
	Von   string `json:"von,omitempty"` // HH:MM, leer = ganztägig
	Bis   string `json:"bis,omitempty"` // HH:MM, optional
	Titel string `json:"titel"`
	Art   string `json:"art"`
	Ort   string `json:"ort,omitempty"`
	Notiz string `json:"notiz,omitempty"`
}

var terminArten = []string{"Arbeitseinsatz", "Versammlung", "Ablesung", "Fest", "Vorstand", "Sonstiges"}

func uhrzeitGueltig(s string) bool {
	if len(s) != 5 || s[2] != ':' {
		return false
	}
	h, e1 := strconv.Atoi(s[:2])
	m, e2 := strconv.Atoi(s[3:])
	return e1 == nil && e2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60
}

func cleanTermin(t Termin) (Termin, error) {
	einzeilig := func(x string) string { return strings.Join(strings.Fields(x), " ") }
	t.Titel = trim(einzeilig(t.Titel), 100)
	t.Ort = trim(einzeilig(t.Ort), 100)
	t.Notiz = trim(t.Notiz, 300)
	t.Von, t.Bis = strings.TrimSpace(t.Von), strings.TrimSpace(t.Bis)
	if t.Titel == "" {
		return t, bad("Bitte einen Titel eintragen, z. B. »Frühjahrsarbeitseinsatz«")
	}
	d, err := parseDate(t.Datum)
	if err != nil || d.Year() < 2000 || d.Year() > 2100 {
		return t, bad("Bitte ein gültiges Datum eintragen")
	}
	if t.Von != "" && !uhrzeitGueltig(t.Von) || t.Bis != "" && !uhrzeitGueltig(t.Bis) {
		return t, bad("Uhrzeiten bitte als SS:MM eintragen, z. B. 09:00")
	}
	if t.Von == "" {
		t.Bis = ""
	}
	if t.Bis != "" && t.Bis <= t.Von {
		return t, bad("Das Ende muss nach dem Beginn liegen")
	}
	gueltig := false
	for _, a := range terminArten {
		if a == t.Art {
			gueltig = true
		}
	}
	if !gueltig {
		t.Art = "Sonstiges"
	}
	return t, nil
}

func sortiereTermine(l []Termin) {
	sort.SliceStable(l, func(i, j int) bool {
		if l[i].Datum != l[j].Datum {
			return l[i].Datum < l[j].Datum
		}
		return l[i].Von < l[j].Von
	})
}

const maxTermine = 2000

func (a *App) handleTerminCreate(w http.ResponseWriter, r *http.Request) {
	var in Termin
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanTermin(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if len(a.st.d.Termine) >= maxTermine {
		writeErr(w, bad("Es sind schon sehr viele Termine eingetragen. Bitte zuerst alte löschen"))
		return
	}
	in.ID = newID()
	a.st.d.Termine = append(a.st.d.Termine, in)
	sortiereTermine(a.st.d.Termine)
	a.st.audit("Termin angelegt: %s am %s", in.Titel, germanDate(in.Datum))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handleTerminUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in Termin
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanTermin(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Termine {
		if a.st.d.Termine[i].ID == id {
			in.ID = id
			a.st.d.Termine[i] = in
			sortiereTermine(a.st.d.Termine)
			a.st.audit("Termin geändert: %s am %s", in.Titel, germanDate(in.Datum))
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, in)
			return
		}
	}
	writeErr(w, notFound("Termin nicht gefunden"))
}

func (a *App) handleTerminDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, t := range a.st.d.Termine {
		if t.ID == id {
			a.st.d.Termine = append(a.st.d.Termine[:i], a.st.d.Termine[i+1:]...)
			a.st.audit("Termin gelöscht: %s am %s", t.Titel, germanDate(t.Datum))
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Termin nicht gefunden"))
}

// gleicherWochentag verschiebt ein Datum ins Zieljahr auf denselben Wochentag, höchstens drei Tage vom
// gleichen Kalendertag entfernt. Vereinstermine hängen meist am Wochentag (»erster Samstag im April«).
func gleicherWochentag(d time.Time, zielJahr int) time.Time {
	ziel := time.Date(zielJahr, d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	versatz := (int(d.Weekday()) - int(ziel.Weekday()) + 7) % 7
	if versatz > 3 {
		versatz -= 7
	}
	n := ziel.AddDate(0, 0, versatz)
	if n.Year() < zielJahr {
		n = n.AddDate(0, 0, 7)
	} else if n.Year() > zielJahr {
		n = n.AddDate(0, 0, -7)
	}
	return n
}

// handleTermineUebernehmen kopiert die Termine eines Jahres ins nächste, jeweils auf denselben Wochentag.
// Termine, die es im Zieljahr mit gleichem Titel schon gibt, werden nicht doppelt angelegt.
func (a *App) handleTermineUebernehmen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Von  int `json:"von"`
		Nach int `json:"nach"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Von < 2000 || in.Von > 2100 || in.Nach < 2000 || in.Nach > 2100 || in.Von == in.Nach {
		writeErr(w, bad("Ungültige Jahre"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	vorhanden := map[string]bool{}
	for _, t := range a.st.d.Termine {
		if strings.HasPrefix(t.Datum, strconv.Itoa(in.Nach)+"-") {
			vorhanden[strings.ToLower(t.Titel)] = true
		}
	}
	neu := 0
	for _, t := range append([]Termin{}, a.st.d.Termine...) {
		d, err := parseDate(t.Datum)
		if err != nil || d.Year() != in.Von || vorhanden[strings.ToLower(t.Titel)] {
			continue
		}
		if len(a.st.d.Termine) >= maxTermine {
			break
		}
		t.ID = newID()
		t.Datum = gleicherWochentag(d, in.Nach).Format("2006-01-02")
		a.st.d.Termine = append(a.st.d.Termine, t)
		neu++
	}
	if neu == 0 {
		writeJSON(w, 200, map[string]int{"uebernommen": 0})
		return
	}
	sortiereTermine(a.st.d.Termine)
	a.st.audit("Jahresplaner: %d Termin(e) aus %d nach %d übernommen", neu, in.Von, in.Nach)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"uebernommen": neu})
}

func (a *App) termineDesJahres(jahr int) ([]Termin, Settings) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	var out []Termin
	for _, t := range a.st.d.Termine {
		if jahr == 0 || strings.HasPrefix(t.Datum, strconv.Itoa(jahr)+"-") {
			out = append(out, t)
		}
	}
	return out, a.st.d.Settings
}

func jahrParam(r *http.Request) int {
	j, err := strconv.Atoi(r.URL.Query().Get("jahr"))
	if err != nil || j < 2000 || j > 2100 {
		return time.Now().Year()
	}
	return j
}

// handleTermineICS liefert die Termine (ab Jahresbeginn des Vorjahres) als Kalenderdatei.
func (a *App) handleTermineICS(w http.ResponseWriter, r *http.Request) {
	alle, s := a.termineDesJahres(0)
	ab := strconv.Itoa(time.Now().Year()-1) + "-01-01"
	var b strings.Builder
	fmt.Fprintf(&b, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//%s//DE\r\nCALSCALE:GREGORIAN\r\nX-WR-CALNAME:%s\r\n", appName, icsEscape(s.VereinName))
	stamp := time.Now().UTC().Format("20060102T150405Z")
	for _, t := range alle {
		if t.Datum < ab {
			continue
		}
		tag := strings.ReplaceAll(t.Datum, "-", "")
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:termin-%s@%s\r\n", t.ID, appID)
		fmt.Fprintf(&b, "DTSTAMP:%s\r\n", stamp)
		if t.Von == "" {
			d, _ := parseDate(t.Datum)
			fmt.Fprintf(&b, "DTSTART;VALUE=DATE:%s\r\nDTEND;VALUE=DATE:%s\r\n", tag, d.AddDate(0, 0, 1).Format("20060102"))
		} else {
			// »schwebende« Ortszeit ohne Zeitzone: der Termin gilt zur angegebenen Uhrzeit vor Ort
			fmt.Fprintf(&b, "DTSTART:%sT%s00\r\n", tag, strings.ReplaceAll(t.Von, ":", ""))
			bis := t.Bis
			if bis == "" {
				h, _ := strconv.Atoi(t.Von[:2])
				if h < 22 {
					bis = fmt.Sprintf("%02d%s", h+2, t.Von[2:])
				} else {
					bis = "23:59"
				}
			}
			fmt.Fprintf(&b, "DTEND:%sT%s00\r\n", tag, strings.ReplaceAll(bis, ":", ""))
		}
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(t.Titel))
		if t.Ort != "" {
			fmt.Fprintf(&b, "LOCATION:%s\r\n", icsEscape(t.Ort))
		}
		desc := t.Art
		if t.Notiz != "" {
			desc += "\n" + t.Notiz
		}
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", icsEscape(desc))
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Vereinstermine.ics"`)
	_, _ = w.Write([]byte(b.String()))
}

var monatsnamen = []string{"Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}
var wochentage = []string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}

// buildJahresplan erzeugt den Jahresplaner als Aushang (A4 hoch): Termine nach Monaten gruppiert.
func buildJahresplan(s Settings, jahr int, ts []Termin) ([]byte, error) {
	if len(ts) == 0 {
		return nil, bad(fmt.Sprintf("Für %d sind noch keine Termine eingetragen", jahr))
	}
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(fmt.Sprintf("Jahresplaner %d", jahr), true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	mr, mg, mb := markenfarbe(s.Farbe)
	fr, fg, fb := aufgehellt(mr, mg, mb, 0.9)
	var y float64
	seite := func() {
		pdf.AddPage()
		zeichneKopf(pdf, s)
		pdf.SetFont("lib", "B", 17)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(leftX, 34)
		pdf.CellFormat(rightX-leftX, 8, fmt.Sprintf("Vereinstermine %d", jahr), "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y = 47
	}
	seite()
	monat := -1
	for _, t := range ts {
		d, err := parseDate(t.Datum)
		if err != nil {
			continue
		}
		zusatz := strings.Join(nonEmpty(t.Ort, t.Notiz), " · ")
		hoehe := 7.0
		if zusatz != "" {
			hoehe += 4.5
		}
		neuerMonat := int(d.Month()) != monat
		if neuerMonat {
			hoehe += 10
		}
		if y+hoehe > 280 {
			seite()
			neuerMonat = true
		}
		if neuerMonat {
			monat = int(d.Month())
			pdf.SetFillColor(fr, fg, fb)
			pdf.Rect(leftX, y, rightX-leftX, 7, "F")
			pdf.SetFont("lib", "B", 10.5)
			pdf.SetTextColor(mr, mg, mb)
			pdf.SetXY(leftX+2, y)
			pdf.CellFormat(rightX-leftX-4, 7, monatsnamen[monat-1], "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			y += 9
		}
		pdf.SetFont("lib", "B", 10.5)
		pdf.SetXY(leftX+2, y)
		pdf.CellFormat(22, 6, wochentage[d.Weekday()]+" "+d.Format("02.01."), "", 0, "L", false, 0, "")
		zeit := "ganztägig"
		if t.Von != "" {
			zeit = t.Von
			if t.Bis != "" {
				zeit += "–" + t.Bis
			}
			zeit += " Uhr"
		}
		pdf.SetFont("lib", "", 9.5)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetXY(leftX+24, y)
		pdf.CellFormat(28, 6, zeit, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		x := leftX + 54
		pdf.SetXY(x, y)
		pdf.CellFormat(rightX-x-30, 6, passendKleinStil(pdf, "B", t.Titel, rightX-x-30, 10.5, 8), "", 0, "L", false, 0, "")
		pdf.SetFont("lib", "", 8.5)
		pdf.SetTextColor(mr, mg, mb)
		pdf.SetXY(rightX-30, y)
		pdf.CellFormat(30, 6, t.Art, "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		y += 6
		if zusatz != "" {
			pdf.SetFont("lib", "", 9)
			pdf.SetTextColor(70, 70, 70)
			pdf.SetXY(x, y-0.5)
			pdf.CellFormat(rightX-x, 4.5, passendKlein(pdf, zusatz, rightX-x, 9, 7), "", 0, "L", false, 0, "")
			pdf.SetTextColor(0, 0, 0)
			y += 4.5
		}
		pdf.SetDrawColor(225, 228, 222)
		pdf.SetLineWidth(0.2)
		pdf.Line(leftX, y+0.5, rightX, y+0.5)
		y += 1.5
	}
	pdf.SetFont("lib", "", 7.5)
	pdf.SetTextColor(110, 110, 110)
	pdf.SetXY(leftX, 287)
	pdf.CellFormat(rightX-leftX, 4, fmt.Sprintf("%s · Stand %s", s.VereinName, time.Now().Format("02.01.2006")), "", 0, "C", false, 0, "")
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) jahresplanPDF(r *http.Request) ([]byte, string, error) {
	jahr := jahrParam(r)
	ts, s := a.termineDesJahres(jahr)
	pdf, err := buildJahresplan(s, jahr, ts)
	return pdf, fmt.Sprintf("Vereinstermine_%d.pdf", jahr), err
}

func (a *App) handleJahresplan(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.jahresplanPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleJahresplanOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, name, err := a.jahresplanPDF(r)
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
