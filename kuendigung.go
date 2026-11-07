package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Kündigung mit Fahrplan: Eingang und Pachtende erfassen, die Frist nach der Vereinsregel prüfen und die
// Schritte bis zur Übergabe abhaken. Einige Schritte erkennt das Programm selbst (Wertermittlung angelegt,
// Schlüssel zurück, Pächterwechsel durchgeführt). Das Cockpit erinnert an alles, was noch offen ist.

// KuendigungsRegel: Pachtende (MM-TT) und bis wann die Kündigung eingegangen sein muss (MM-TT).
type KuendigungsRegel struct {
	Zum string `json:"zum"`
	Bis string `json:"bis"`
}

func (r KuendigungsRegel) mitStandard() KuendigungsRegel {
	if !monatTagGueltig(r.Zum) {
		r.Zum = "11-30" // § 9 BKleingG: Kündigung zum 30. November
	}
	if !monatTagGueltig(r.Bis) {
		r.Bis = "08-31"
	}
	return r
}

func monatTagGueltig(s string) bool {
	_, err := time.Parse("2006-01-02", "2000-"+s)
	return len(s) == 5 && err == nil
}

// fristgerecht: frühestes Pachtende für eine Kündigung, die am eingang eingeht.
func (r KuendigungsRegel) fristgerecht(eingang string) string {
	t, err := parseDate(eingang)
	if err != nil {
		return ""
	}
	y := t.Year()
	if eingang > fmt.Sprintf("%d-%s", y, r.Bis) {
		y++
	}
	ende := fmt.Sprintf("%d-%s", y, r.Zum)
	if ende < eingang {
		ende = fmt.Sprintf("%d-%s", y+1, r.Zum)
	}
	return ende
}

type Kuendigung struct {
	ID         string `json:"id"`
	PaechterID string `json:"paechterId"`
	Gartennr   string `json:"gartennr"`
	Eingang    string `json:"eingang"`
	Zum        string `json:"zum"`
	// Von: »paechter«, »verein« oder »einvernehmlich«
	Von   string `json:"von"`
	Grund string `json:"grund,omitempty"`
	Notiz string `json:"notiz,omitempty"`
	// Erledigt: Schritt → Datum (von Hand abgehakt)
	Erledigt      map[string]string `json:"erledigt,omitempty"`
	Abgeschlossen string            `json:"abgeschlossen,omitempty"`
}

var kuendigungsSchritte = []struct{ Key, Titel, Hilfe string }{
	{"bestaetigung", "Kündigung schriftlich bestätigt", "Bestätigung als Brief drucken – der Haken wird dabei gesetzt."},
	{"warteliste", "Nachfolger angefragt", "Interessenten von der Warteliste zur Besichtigung einladen."},
	{"wertermittlung", "Wertermittlung gemacht", "Erkennt das Programm, sobald für den Garten eine Wertermittlung angelegt ist."},
	{"raeumung", "Garten abgenommen", "Begehung mit dem Pächter: Mängel, Müll, nicht erlaubte Bauten."},
	{"schluessel", "Schlüssel zurück", "Erkennt das Programm, sobald der Pächter keinen Schlüssel und kein Gerät mehr hat."},
	{"uebergabe", "Garten übergeben (Zählerstände, Schlussrechnung)", "Erkennt das Programm am »Pächterwechsel …« beim Pächter."},
}

type kuendigungSchritt struct {
	Key      string `json:"key"`
	Titel    string `json:"titel"`
	Hilfe    string `json:"hilfe"`
	Erledigt string `json:"erledigt,omitempty"` // Datum oder »automatisch«
	Auto     bool   `json:"auto,omitempty"`
}

type kuendigungAnsicht struct {
	Kuendigung
	Name         string              `json:"name"`
	Fristgemaess string              `json:"fristgemaess"` // frühestes Pachtende nach der Regel
	ZuFrueh      bool                `json:"zuFrueh,omitempty"`
	Schritte     []kuendigungSchritt `json:"schritte"`
	Offen        int                 `json:"offen"`
}

// kuendigungAnsichtLocked: Schritte samt automatisch erkannten. Der Aufrufer hält s.mu.
func (s *Store) kuendigungAnsichtLocked(k Kuendigung) kuendigungAnsicht {
	regel := s.d.KuendigungsRegel.mitStandard()
	v := kuendigungAnsicht{Kuendigung: k, Fristgemaess: regel.fristgerecht(k.Eingang)}
	v.ZuFrueh = k.Von == "paechter" && k.Zum < v.Fristgemaess
	var p *Paechter
	for i := range s.d.Paechter {
		if s.d.Paechter[i].ID == k.PaechterID {
			p = &s.d.Paechter[i]
		}
	}
	if p != nil {
		v.Name = p.Name
	} else {
		v.Name = geloeschtePerson
	}
	auto := map[string]bool{}
	for _, w := range s.d.Wertermittlungen {
		if w.Gartennr == k.Gartennr && w.Datum >= k.Eingang {
			auto["wertermittlung"] = true
		}
	}
	if p == nil || p.AusgeschiedenAm != "" {
		auto["uebergabe"] = true
	}
	hatNoch := false
	for _, x := range s.d.Schluessel {
		hatNoch = hatNoch || (x.PaechterID != "" && x.PaechterID == k.PaechterID)
	}
	for _, g := range s.d.Geraete {
		hatNoch = hatNoch || (g.PaechterID != "" && g.PaechterID == k.PaechterID)
	}
	auto["schluessel"] = !hatNoch
	for _, sch := range kuendigungsSchritte {
		x := kuendigungSchritt{Key: sch.Key, Titel: sch.Titel, Hilfe: sch.Hilfe, Erledigt: k.Erledigt[sch.Key]}
		if x.Erledigt == "" && auto[sch.Key] {
			x.Erledigt, x.Auto = "automatisch", true
		}
		if x.Erledigt == "" {
			v.Offen++
		}
		v.Schritte = append(v.Schritte, x)
	}
	return v
}

func (a *App) handleKuendigungen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	out := []kuendigungAnsicht{}
	for _, k := range a.st.d.Kuendigungen {
		out = append(out, a.st.kuendigungAnsichtLocked(k))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Abgeschlossen == "") != (out[j].Abgeschlossen == "") {
			return out[i].Abgeschlossen == ""
		}
		return out[i].Zum < out[j].Zum
	})
	writeJSON(w, 200, map[string]any{"regel": a.st.d.KuendigungsRegel.mitStandard(), "kuendigungen": out})
}

func (a *App) handleKuendigungSpeichern(w http.ResponseWriter, r *http.Request) {
	var in Kuendigung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if !gueltigesDatum(in.Eingang, false) || !gueltigesDatum(in.Zum, false) {
		writeErr(w, bad("Bitte Eingang der Kündigung und Pachtende angeben"))
		return
	}
	if in.Zum < in.Eingang {
		writeErr(w, bad("Das Pachtende liegt vor dem Eingang der Kündigung"))
		return
	}
	if in.Von != "verein" && in.Von != "einvernehmlich" {
		in.Von = "paechter"
	}
	in.Grund, in.Notiz = trim(in.Grund, 200), trim(in.Notiz, 1000)
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Kuendigungen {
		k := &a.st.d.Kuendigungen[i]
		if in.ID != "" && k.ID == in.ID {
			k.Eingang, k.Zum, k.Von, k.Grund, k.Notiz = in.Eingang, in.Zum, in.Von, in.Grund, in.Notiz
			a.st.audit("Kündigung Garten %s geändert", k.Gartennr)
			a.speichernUndAntworten(w, a.st.kuendigungAnsichtLocked(*k))
			return
		}
	}
	var p *Paechter
	for i := range a.st.d.Paechter {
		if x := &a.st.d.Paechter[i]; x.ID == in.PaechterID && !x.Geloescht && x.AusgeschiedenAm == "" {
			p = x
		}
	}
	if p == nil {
		writeErr(w, bad("Pächter nicht gefunden"))
		return
	}
	for _, k := range a.st.d.Kuendigungen {
		if k.PaechterID == p.ID && k.Abgeschlossen == "" {
			writeErr(w, bad("Für "+p.Name+" ist schon eine Kündigung erfasst"))
			return
		}
	}
	k := Kuendigung{ID: newID(), PaechterID: p.ID, Gartennr: p.Gartennr, Eingang: in.Eingang, Zum: in.Zum, Von: in.Von, Grund: in.Grund, Notiz: in.Notiz}
	a.st.d.Kuendigungen = append(a.st.d.Kuendigungen, k)
	a.st.audit("Kündigung Garten %s erfasst (zum %s)", k.Gartennr, germanDate(k.Zum))
	a.speichernUndAntworten(w, a.st.kuendigungAnsichtLocked(k))
}

func (a *App) kuendigungLocked(id string) *Kuendigung {
	for i := range a.st.d.Kuendigungen {
		if a.st.d.Kuendigungen[i].ID == id {
			return &a.st.d.Kuendigungen[i]
		}
	}
	return nil
}

// handleKuendigungSchritt hakt einen Schritt ab ({schritt, erledigt}) oder schließt die Kündigung ab
// ({schritt: "abgeschlossen"}).
func (a *App) handleKuendigungSchritt(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Schritt  string `json:"schritt"`
		Erledigt bool   `json:"erledigt"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	k := a.kuendigungLocked(r.PathValue("id"))
	if k == nil {
		writeErr(w, notFound("Kündigung nicht gefunden"))
		return
	}
	heute := time.Now().Format("2006-01-02")
	if in.Schritt == "abgeschlossen" {
		k.Abgeschlossen = ""
		if in.Erledigt {
			k.Abgeschlossen = heute
		}
		a.st.audit("Kündigung Garten %s abgeschlossen: %v", k.Gartennr, in.Erledigt)
		a.speichernUndAntworten(w, a.st.kuendigungAnsichtLocked(*k))
		return
	}
	bekannt := false
	for _, s := range kuendigungsSchritte {
		bekannt = bekannt || s.Key == in.Schritt
	}
	if !bekannt {
		writeErr(w, bad("Unbekannter Schritt"))
		return
	}
	if k.Erledigt == nil {
		k.Erledigt = map[string]string{}
	}
	if in.Erledigt {
		k.Erledigt[in.Schritt] = heute
	} else {
		delete(k.Erledigt, in.Schritt)
	}
	a.speichernUndAntworten(w, a.st.kuendigungAnsichtLocked(*k))
}

func (a *App) handleKuendigungLoeschen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i, k := range a.st.d.Kuendigungen {
		if k.ID == r.PathValue("id") {
			a.st.d.Kuendigungen = append(a.st.d.Kuendigungen[:i], a.st.d.Kuendigungen[i+1:]...)
			a.st.audit("Kündigung Garten %s gelöscht (zurückgezogen)", k.Gartennr)
			a.speichernUndAntworten(w, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Kündigung nicht gefunden"))
}

func (a *App) handleKuendigungsRegel(w http.ResponseWriter, r *http.Request) {
	var in KuendigungsRegel
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if !monatTagGueltig(in.Zum) || !monatTagGueltig(in.Bis) {
		writeErr(w, bad("Bitte Tag und Monat angeben, z. B. 11-30"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.KuendigungsRegel = in
	a.st.audit("Kündigungsregel: bis %s zum %s", in.Bis, in.Zum)
	a.speichernUndAntworten(w, in)
}

// kuendigungBestaetigungPDF: Brief an den Pächter; setzt den Schritt »bestätigt«.
func (a *App) kuendigungBestaetigungPDF(r *http.Request) ([]byte, string, error) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	k := a.kuendigungLocked(r.PathValue("id"))
	if k == nil {
		return nil, "", notFound("Kündigung nicht gefunden")
	}
	var p *Paechter
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == k.PaechterID {
			p = &a.st.d.Paechter[i]
		}
	}
	if p == nil {
		return nil, "", bad("Der Pächter ist gelöscht")
	}
	var schluessel []string
	for _, x := range a.st.d.Schluessel {
		if x.PaechterID == p.ID {
			schluessel = append(schluessel, x.Art+"schlüssel Nr. "+x.Nummer)
		}
	}
	s := a.st.d.Settings
	heute := time.Now().Format("2006-01-02")
	pdf := neuesPDF(s, "Bestätigung der Kündigung – Garten "+k.Gartennr, false)
	y := briefAnfang(pdf, s, *p, heute)
	text := "Liebe Gartenfreundin, lieber Gartenfreund,\n\n"
	switch k.Von {
	case "verein":
		text += fmt.Sprintf("hiermit halten wir fest, dass der Verein das Pachtverhältnis über den Garten %s zum %s gekündigt hat.", k.Gartennr, germanDate(k.Zum))
	case "einvernehmlich":
		text += fmt.Sprintf("wie besprochen endet das Pachtverhältnis über den Garten %s im gegenseitigen Einvernehmen zum %s.", k.Gartennr, germanDate(k.Zum))
	default:
		text += fmt.Sprintf("wir bestätigen den Eingang Ihrer Kündigung vom %s. Das Pachtverhältnis über den Garten %s endet zum %s.", germanDate(k.Eingang), k.Gartennr, germanDate(k.Zum))
	}
	text += "\n\nBis zur Übergabe gilt Folgendes:\n" +
		"•  Der Garten wird bis dahin weiter gepflegt; Pacht, Beiträge und Verbrauch werden bis zur Übergabe abgerechnet.\n" +
		"•  Für Laube, Anpflanzungen und Einrichtungen findet eine Wertermittlung statt. Den Termin stimmen wir mit Ihnen ab.\n" +
		"•  Bei der Übergabe lesen wir gemeinsam die Zähler ab; danach erhalten Sie die Schlussrechnung.\n"
	if len(schluessel) > 0 {
		text += "•  Bitte geben Sie bei der Übergabe zurück: " + strings.Join(schluessel, ", ") + ".\n"
	}
	text += "\nGern helfen wir dabei, eine Nachfolgerin oder einen Nachfolger für den Garten zu finden. Bei Fragen melden Sie sich bitte beim Vorstand."
	briefText(pdf, s, y, "Bestätigung der Kündigung – Garten "+k.Gartennr, text)
	out, err := pdfBytes(pdf)
	if err != nil {
		return nil, "", err
	}
	if k.Erledigt == nil {
		k.Erledigt = map[string]string{}
	}
	if k.Erledigt["bestaetigung"] == "" {
		k.Erledigt["bestaetigung"] = heute
		_ = a.st.saveLocked()
	}
	return out, "Kuendigungsbestaetigung_Garten_" + safeName(k.Gartennr, 20) + ".pdf", nil
}

// kuendigungAufgabenLocked: offene Schritte laufender Kündigungen.
func (s *Store) kuendigungAufgabenLocked(heute time.Time, add func(stufe, titel, text, ziel, knopf string)) {
	for _, k := range s.d.Kuendigungen {
		if k.Abgeschlossen != "" {
			continue
		}
		v := s.kuendigungAnsichtLocked(k)
		if v.Offen == 0 {
			add("info", fmt.Sprintf("Kündigung Garten %s erledigt", k.Gartennr), "Alle Schritte sind abgehakt – die Kündigung kann abgeschlossen werden.", "admin:kuendigungen", "Kündigungen")
			continue
		}
		var offen []string
		for _, x := range v.Schritte {
			if x.Erledigt == "" {
				offen = append(offen, x.Titel)
			}
		}
		stufe := "info"
		if tageBis(heute, k.Zum) <= 30 {
			stufe = "warn"
		}
		add(stufe, fmt.Sprintf("Kündigung Garten %s zum %s: %s offen", k.Gartennr, germanDate(k.Zum), mehrzahl(v.Offen, "Schritt", "Schritte")),
			"Noch offen: "+strings.Join(offen, ", ")+".", "admin:kuendigungen", "Kündigungen")
	}
}

func (d *Data) kuendigungPersonEntfernen(id string) bool {
	behalten := d.Kuendigungen[:0]
	changed := false
	for _, k := range d.Kuendigungen {
		if k.PaechterID == id {
			changed = true
			continue
		}
		behalten = append(behalten, k)
	}
	d.Kuendigungen = behalten
	return changed
}
