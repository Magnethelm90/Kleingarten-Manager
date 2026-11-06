package main

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Vorauszahlungen per Überweisung: Pächter zahlen monatlich einen festen Betrag (meist per Dauerauftrag) auf das
// Vereinskonto. Je Pächter steht der Monatsbetrag (Soll); jede eingegangene Zahlung wird als Vorauszahlung für
// ein Abrechnungsjahr erfasst – von Hand oder beim Einlesen des Kontoauszugs – und auf den Abschlag dieses Jahres
// gebucht, den die Jahresrechnung abzieht. Kommt eine Zahlung für ein Jahr, das im Programm noch nicht begonnen
// hat (Überweisung im Januar vor dem Jahreswechsel), wird sie vorgemerkt und beim Jahreswechsel gebucht.

// Vorauszahlung ist eine eingegangene Zahlung auf die monatliche Vorauszahlung.
type Vorauszahlung struct {
	ID         string  `json:"id"`
	PaechterID string  `json:"paechterId"`
	Jahr       int     `json:"jahr"` // Abrechnungsjahr, für das gezahlt wurde
	Datum      string  `json:"datum"`
	Betrag     float64 `json:"betrag"`
	Gebucht    bool    `json:"gebucht"` // auf den Abschlag des Abrechnungsjahres gebucht (sonst vorgemerkt)
	Quelle     string  `json:"quelle"`  // hand | kontoauszug
	Notiz      string  `json:"notiz,omitempty"`
}

const maxVorauszahlungen = 50000

// vorauszahlungBuchen bucht eine Zahlung auf den Abschlag (richtung +1) oder nimmt sie zurück (−1). Ist das
// Abrechnungsjahr noch nicht angelegt, bleibt die Zahlung vorgemerkt.
func (d *Data) vorauszahlungBuchen(v *Vorauszahlung, richtung float64) {
	j := d.Jahre[yearKey(v.Jahr)]
	if j == nil || j.Abgeschlossen {
		return
	}
	if (richtung > 0 && v.Gebucht) || (richtung < 0 && !v.Gebucht) {
		return
	}
	if j.Ablesungen == nil {
		j.Ablesungen = map[string]Ablesung{}
	}
	ab := j.Ablesungen[v.PaechterID]
	ab.Abschlag = math.Max(0, round2(ab.Abschlag+richtung*v.Betrag))
	j.Ablesungen[v.PaechterID] = ab
	v.Gebucht = richtung > 0
}

// vorgemerkteBuchen bucht beim Jahreswechsel die Zahlungen, die schon für das neue Jahr eingegangen sind.
func (d *Data) vorgemerkteBuchen(jahr int) int {
	n := 0
	for i := range d.Vorauszahlungen {
		if v := &d.Vorauszahlungen[i]; v.Jahr == jahr && !v.Gebucht && v.PaechterID != "" {
			d.vorauszahlungBuchen(v, +1)
			if v.Gebucht {
				n++
			}
		}
	}
	return n
}

// faelligeMonate: Wie viele Monatsraten eines Jahres sind bis heute fällig? Der laufende Monat zählt ab dem 10.
func faelligeMonate(jahr int, heute time.Time) int {
	switch {
	case jahr < heute.Year():
		return 12
	case jahr > heute.Year():
		return 0
	}
	n := int(heute.Month()) - 1
	if heute.Day() >= 10 {
		n++
	}
	return n
}

type vzZeile struct {
	PaechterID  string          `json:"paechterId"`
	Gartennr    string          `json:"gartennr"`
	Mitgliedsnr string          `json:"mitgliedsnr"`
	Name        string          `json:"name"`
	Soll        float64         `json:"soll"`
	Summe       float64         `json:"summe"`
	Faellig     float64         `json:"faellig"` // bis heute fällig
	Rueckstand  float64         `json:"rueckstand"`
	Monate      []float64       `json:"monate"` // je Monat: 1 = gedeckt, 0–1 = teilweise, 0 = offen (aus der Summe gerechnet)
	Zahlungen   []Vorauszahlung `json:"zahlungen"`
}

type vzAnsicht struct {
	Jahr        int       `json:"jahr"`
	Zeilen      []vzZeile `json:"zeilen"`
	Summe       float64   `json:"summe"`
	Rueckstand  float64   `json:"rueckstand"`
	Vorgemerkt  int       `json:"vorgemerkt"`
	JahrOffen   bool      `json:"jahrOffen"` // Abrechnungsjahr angelegt und nicht abgeschlossen
	FaelligeMon int       `json:"faelligeMonate"`
}

// vzAnsichtLocked: Monatsübersicht. Die Monate werden aus der Summe der Zahlungen gefüllt (Januar zuerst), so
// zählt auch eine Überweisung über mehrere Monate richtig. Der Aufrufer hält s.mu.
func (s *Store) vzAnsichtLocked(jahr int, heute time.Time) vzAnsicht {
	out := vzAnsicht{Jahr: jahr, Zeilen: []vzZeile{}, FaelligeMon: faelligeMonate(jahr, heute)}
	if j := s.d.Jahre[yearKey(jahr)]; j != nil && !j.Abgeschlossen {
		out.JahrOffen = true
	}
	zahl := map[string][]Vorauszahlung{}
	for _, v := range s.d.Vorauszahlungen {
		if v.Jahr == jahr {
			zahl[v.PaechterID] = append(zahl[v.PaechterID], v)
			if !v.Gebucht {
				out.Vorgemerkt++
			}
		}
	}
	for _, p := range s.d.Paechter {
		soll := s.d.VorauszahlungSoll[p.ID]
		aktiv := !p.Geloescht && p.AusgeschiedenAm == ""
		if !aktiv && len(zahl[p.ID]) == 0 {
			continue
		}
		z := vzZeile{PaechterID: p.ID, Gartennr: p.Gartennr, Mitgliedsnr: p.Mitgliedsnr, Name: p.Name, Soll: soll, Monate: make([]float64, 12), Zahlungen: zahl[p.ID]}
		if z.Zahlungen == nil {
			z.Zahlungen = []Vorauszahlung{}
		}
		sort.SliceStable(z.Zahlungen, func(i, k int) bool { return z.Zahlungen[i].Datum < z.Zahlungen[k].Datum })
		for _, v := range z.Zahlungen {
			z.Summe += v.Betrag
		}
		z.Summe = round2(z.Summe)
		if soll > 0 {
			for m := 0; m < 12; m++ {
				z.Monate[m] = math.Max(0, math.Min(1, (z.Summe-float64(m)*soll)/soll))
			}
			if aktiv {
				z.Faellig = round2(soll * float64(out.FaelligeMon))
				z.Rueckstand = math.Max(0, round2(z.Faellig-z.Summe))
			}
		}
		out.Summe += z.Summe
		out.Rueckstand += z.Rueckstand
		out.Zeilen = append(out.Zeilen, z)
	}
	out.Summe, out.Rueckstand = round2(out.Summe), round2(out.Rueckstand)
	sort.SliceStable(out.Zeilen, func(i, k int) bool {
		return naturalLess(out.Zeilen[i].Gartennr+" "+out.Zeilen[i].Mitgliedsnr, out.Zeilen[k].Gartennr+" "+out.Zeilen[k].Mitgliedsnr)
	})
	return out
}

func (a *App) handleVzAnsicht(w http.ResponseWriter, r *http.Request) {
	jahr := jahrParam(r)
	a.st.mu.Lock()
	v := a.st.vzAnsichtLocked(jahr, time.Now())
	a.st.mu.Unlock()
	writeJSON(w, 200, v)
}

func (s *Store) paechterAktivLocked(id string) (Paechter, bool) {
	for _, p := range s.d.Paechter {
		if p.ID == id && !p.Geloescht {
			return p, true
		}
	}
	return Paechter{}, false
}

func (a *App) handleVzSoll(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PaechterID string  `json:"paechterId"`
		Betrag     float64 `json:"betrag"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if !validNum(in.Betrag) || in.Betrag < 0 || in.Betrag > 10000 {
		writeErr(w, bad("Der Monatsbetrag muss zwischen 0 und 10.000 € liegen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	p, ok := a.st.paechterAktivLocked(in.PaechterID)
	if !ok {
		writeErr(w, notFound("Pächter nicht gefunden"))
		return
	}
	if a.st.d.VorauszahlungSoll == nil {
		a.st.d.VorauszahlungSoll = map[string]float64{}
	}
	if in.Betrag == 0 {
		delete(a.st.d.VorauszahlungSoll, p.ID)
	} else {
		a.st.d.VorauszahlungSoll[p.ID] = round2(in.Betrag)
	}
	a.st.audit("Monatliche Vorauszahlung %s %s: %s", p.Mitgliedsnr, p.Name, fmtEUR(in.Betrag))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]float64{"betrag": round2(in.Betrag)})
}

// neueVorauszahlungLocked prüft und bucht eine Zahlung. Der Aufrufer hält s.mu und speichert.
func (s *Store) neueVorauszahlungLocked(v Vorauszahlung) (Vorauszahlung, error) {
	if _, ok := s.paechterAktivLocked(v.PaechterID); !ok {
		return v, bad("Bitte einen Pächter auswählen")
	}
	d, err := parseDate(v.Datum)
	if err != nil || d.Year() < 2000 || d.Year() > 2100 {
		return v, bad("Bitte das Datum der Zahlung eintragen")
	}
	if v.Jahr == 0 {
		v.Jahr = d.Year()
	}
	if v.Jahr < d.Year()-1 || v.Jahr > d.Year()+1 {
		return v, bad("Das Abrechnungsjahr passt nicht zum Datum der Zahlung")
	}
	if j := s.d.Jahre[yearKey(v.Jahr)]; j != nil && j.Abgeschlossen {
		return v, bad(fmt.Sprintf("Das Abrechnungsjahr %d ist abgeschlossen", v.Jahr))
	}
	if !validNum(v.Betrag) || v.Betrag <= 0 || v.Betrag > 100000 {
		return v, bad("Bitte einen Betrag größer als 0 eintragen")
	}
	if len(s.d.Vorauszahlungen) >= maxVorauszahlungen {
		return v, bad("Es sind sehr viele Vorauszahlungen gespeichert")
	}
	v.ID, v.Betrag, v.Gebucht = newID(), round2(v.Betrag), false
	v.Notiz = trim(strings.Join(strings.Fields(v.Notiz), " "), 200)
	if v.Quelle != "kontoauszug" {
		v.Quelle = "hand"
	}
	s.d.Vorauszahlungen = append(s.d.Vorauszahlungen, v)
	s.d.vorauszahlungBuchen(&s.d.Vorauszahlungen[len(s.d.Vorauszahlungen)-1], +1)
	return s.d.Vorauszahlungen[len(s.d.Vorauszahlungen)-1], nil
}

func (a *App) handleVzNeu(w http.ResponseWriter, r *http.Request) {
	var in Vorauszahlung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Quelle = "hand"
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	v, err := a.st.neueVorauszahlungLocked(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.audit("Vorauszahlung erfasst: %s am %s", fmtEUR(v.Betrag), germanDate(v.Datum))
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, v)
}

func (a *App) handleVzLoeschen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Vorauszahlungen {
		v := &a.st.d.Vorauszahlungen[i]
		if v.ID != id {
			continue
		}
		if j := a.st.d.Jahre[yearKey(v.Jahr)]; v.Gebucht && (j == nil || j.Abgeschlossen) {
			writeErr(w, bad(fmt.Sprintf("Das Abrechnungsjahr %d ist abgeschlossen – die Zahlung lässt sich nicht mehr löschen", v.Jahr)))
			return
		}
		a.st.d.vorauszahlungBuchen(v, -1)
		x := *v
		a.st.d.Vorauszahlungen = append(a.st.d.Vorauszahlungen[:i], a.st.d.Vorauszahlungen[i+1:]...)
		a.st.audit("Vorauszahlung gelöscht: %s am %s", fmtEUR(x.Betrag), germanDate(x.Datum))
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
		return
	}
	writeErr(w, notFound("Zahlung nicht gefunden"))
}

// ------------------------------------------------------------------ Kontoauszug

var vzStichworte = []string{"VORAUSZAHLUNG", "ABSCHLAG", "DAUERAUFTRAG", "MONATSBEITRAG", "MONATLICH"}

// vorauszahlerLocked: Pächter mit Monatsbetrag (für die Zuordnung im Kontoauszug).
func (s *Store) vorauszahlerLocked() []Paechter {
	var l []Paechter
	for _, p := range s.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" && s.d.VorauszahlungSoll[p.ID] > 0 {
			l = append(l, p)
		}
	}
	sort.SliceStable(l, func(i, k int) bool { return naturalLess(l[i].Gartennr, l[k].Gartennr) })
	return l
}

// vzZuordnen sucht zu einem Eingang den Pächter: Name im Text und Betrag gleich einem Vielfachen des Monatsbetrags
// (bis zwölf Monate), eindeutig. stichwort: »Vorauszahlung«, »Abschlag« o. Ä. steht im Text.
func (s *Store) vzZuordnenLocked(text string, betrag float64, kandidaten []Paechter) (Paechter, bool, bool) {
	stichwort := false
	for _, w := range vzStichworte {
		stichwort = stichwort || strings.Contains(text, w)
	}
	var treffer []Paechter
	for _, p := range kandidaten {
		name := kompakt(p.Name)
		soll := s.d.VorauszahlungSoll[p.ID]
		if len([]rune(name)) < 4 || !strings.Contains(text, name) || soll <= 0 {
			continue
		}
		k := betrag / soll
		if k >= 0.999 && k <= 12.001 && math.Abs(k-math.Round(k)) < 0.001 {
			treffer = append(treffer, p)
		}
	}
	if len(treffer) == 1 {
		return treffer[0], true, stichwort
	}
	return Paechter{}, false, stichwort
}

// ------------------------------------------------------------------ Erinnerung

func buildVzErinnerung(s Settings, p Paechter, z vzZeile, jahr int, heute time.Time) ([]byte, error) {
	if z.Rueckstand <= 0 {
		return nil, bad("Bei diesem Pächter gibt es keinen Rückstand")
	}
	pdf := neuesPDF(s, "Erinnerung Vorauszahlung "+strconv.Itoa(jahr), false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
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
	pdf.CellFormat(rightX-leftX, 7, fmt.Sprintf("Erinnerung: monatliche Vorauszahlung %d", jahr), "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 10
	gedeckt := 0
	for _, m := range z.Monate {
		if m >= 0.999 {
			gedeckt++
		}
	}
	monatText := "noch keine Rate"
	if gedeckt > 0 {
		monatText = fmt.Sprintf("die Raten bis einschließlich %s", monatsnamen[gedeckt-1])
	}
	y = mappeText(pdf, y, fmt.Sprintf("Liebe Gartenfreundin, lieber Gartenfreund,\n\nfür Ihren Garten %s ist eine monatliche Vorauszahlung von %s vereinbart. "+
		"Bis heute sind %s fällig, eingegangen sind %s – damit sind %s bezahlt. Es fehlen noch %s.",
		p.Gartennr, fmtEUR(z.Soll), fmtEUR(z.Faellig), fmtEUR(z.Summe), monatText, fmtEUR(z.Rueckstand)), 10)
	y = mappeTabelle(pdf, y, [][2]string{
		{"Noch offen", fmtEUR(z.Rueckstand)},
		{"Bankverbindung", strings.TrimSpace(s.BankName + ", IBAN " + s.IBAN)},
		{"Verwendungszweck", fmt.Sprintf("Vorauszahlung %d Mitglied %s", jahr, p.Mitgliedsnr)},
	})
	y = mappeText(pdf, y, "Bitte überweisen Sie den fehlenden Betrag und prüfen Sie Ihren Dauerauftrag. Hat sich Ihre Zahlung mit diesem Schreiben überschnitten, "+
		"betrachten Sie es bitte als gegenstandslos. Die Vorauszahlungen werden mit der Jahresabrechnung verrechnet.", 10)
	pdf.SetFont("lib", "", 10)
	pdf.SetXY(leftX, y+4)
	pdf.CellFormat(rightX-leftX, 6, "Mit freundlichen Gartengrüßen", "", 0, "L", false, 0, "")
	pdf.SetXY(leftX, y+10)
	pdf.CellFormat(rightX-leftX, 6, "Der Vorstand", "", 0, "L", false, 0, "")
	return pdfBytes(pdf)
}

func (a *App) vzErinnerungPDF(r *http.Request) ([]byte, string, error) {
	jahr := jahrParam(r)
	pid := r.URL.Query().Get("paechter")
	heute := time.Now()
	a.st.mu.Lock()
	p, ok := a.st.paechterAktivLocked(pid)
	v := a.st.vzAnsichtLocked(jahr, heute)
	s := a.st.d.Settings
	a.st.mu.Unlock()
	if !ok {
		return nil, "", notFound("Pächter nicht gefunden")
	}
	for _, z := range v.Zeilen {
		if z.PaechterID == pid {
			pdf, err := buildVzErinnerung(s, p, z, jahr, heute)
			return pdf, safeName(fmt.Sprintf("Erinnerung_Vorauszahlung_%d_%s", jahr, p.Name), 80) + ".pdf", err
		}
	}
	return nil, "", notFound("Pächter nicht gefunden")
}

// ------------------------------------------------------------------ Datenschutz

// vzPersonEntfernen: Monatsbetrag löschen; die Zahlungen sind Kassenbelege und bleiben ohne Verknüpfung zur Person.
func (d *Data) vzPersonEntfernen(id string) bool {
	changed := false
	if _, ok := d.VorauszahlungSoll[id]; ok {
		delete(d.VorauszahlungSoll, id)
		changed = true
	}
	for i := range d.Vorauszahlungen {
		if d.Vorauszahlungen[i].PaechterID == id {
			d.Vorauszahlungen[i].PaechterID, d.Vorauszahlungen[i].Notiz, changed = "", "", true
		}
	}
	return changed
}
