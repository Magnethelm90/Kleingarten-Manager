package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Wahlen-Assistent: Wahlzettel drucken, Stimmen auszählen, das Ergebnis ins Beschlussbuch übernehmen und auf Wunsch
// den Amtswechsel (oder die Wiederwahl) in den Ämtern eintragen. Der fertige Protokollabsatz kommt als Text zurück.

type wahlKandidat struct {
	Name    string `json:"name"`
	Stimmen int    `json:"stimmen"`
}

type wahlEingabe struct {
	Amt         string         `json:"amt"`   // Bezeichnung des Amts
	AmtID       string         `json:"amtId"` // bisheriges Amt (leer = neu besetzen)
	Datum       string         `json:"datum"` // Tag der Wahl
	Gremium     string         `json:"gremium"`
	Kandidaten  []wahlKandidat `json:"kandidaten"`
	Nein        int            `json:"nein"` // nur bei einem Kandidaten
	Enthaltung  int            `json:"enthaltung"`
	Ungueltig   int            `json:"ungueltig"`
	Absolut     bool           `json:"absolut"`     // absolute Mehrheit der gültigen Stimmen nötig (laut Satzung)
	Uebernehmen bool           `json:"uebernehmen"` // Amt eintragen
	Amtszeit    int            `json:"amtszeit"`    // Jahre, 0 = unbefristet
}

type wahlErgebnis struct {
	Gewaehlt   string `json:"gewaehlt"`
	Ok         bool   `json:"ok"`
	Hinweis    string `json:"hinweis,omitempty"`
	Protokoll  string `json:"protokoll"`
	Beschluss  string `json:"beschluss,omitempty"` // Nummer im Beschlussbuch
	AmtEingetr bool   `json:"amtEingetragen"`
}

// wahlAuswerten entscheidet nach den Stimmen. Ein Kandidat: Ja muss die Neinstimmen übersteigen. Mehrere: die
// meisten Stimmen gewinnen, bei Gleichstand ist eine Stichwahl nötig. Absolut: mehr als die Hälfte der gültigen Stimmen.
func wahlAuswerten(in wahlEingabe) (name string, ja int, ok bool, hinweis string) {
	if len(in.Kandidaten) == 0 {
		return "", 0, false, "Es ist kein Kandidat eingetragen."
	}
	gueltig := in.Nein + in.Enthaltung
	best, bestI, gleich := -1, -1, false
	for i, k := range in.Kandidaten {
		gueltig += k.Stimmen
		switch {
		case k.Stimmen > best:
			best, bestI, gleich = k.Stimmen, i, false
		case k.Stimmen == best:
			gleich = true
		}
	}
	if gueltig == 0 {
		return "", 0, false, "Es sind keine gültigen Stimmen eingetragen."
	}
	if len(in.Kandidaten) == 1 {
		k := in.Kandidaten[0]
		if k.Stimmen <= in.Nein {
			return k.Name, k.Stimmen, false, "Die Ja-Stimmen übersteigen die Nein-Stimmen nicht."
		}
		if in.Absolut && k.Stimmen*2 <= gueltig {
			return k.Name, k.Stimmen, false, "Die absolute Mehrheit der gültigen Stimmen (einschließlich Enthaltungen) wurde nicht erreicht."
		}
		return k.Name, k.Stimmen, true, ""
	}
	if gleich {
		return "", 0, false, "Stimmengleichheit: Eine Stichwahl ist nötig."
	}
	if in.Absolut && best*2 <= gueltig {
		return in.Kandidaten[bestI].Name, best, false, "Die absolute Mehrheit wurde nicht erreicht: Stichwahl der beiden Bestplatzierten."
	}
	return in.Kandidaten[bestI].Name, best, true, ""
}

func wahlProtokoll(in wahlEingabe, name string, ok bool, s Settings) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Wahl: %s. Die Wahl fand am %s statt (%s). ", in.Amt, germanDate(in.Datum), in.Gremium)
	teile := []string{}
	for _, k := range in.Kandidaten {
		teile = append(teile, fmt.Sprintf("%s: %d", k.Name, k.Stimmen))
	}
	if len(in.Kandidaten) == 1 {
		teile = append(teile, fmt.Sprintf("Nein: %d", in.Nein))
	}
	teile = append(teile, fmt.Sprintf("Enthaltungen: %d", in.Enthaltung), fmt.Sprintf("ungültig: %d", in.Ungueltig))
	b.WriteString("Ergebnis: " + strings.Join(teile, ", ") + ". ")
	if ok {
		fmt.Fprintf(&b, "%s ist damit zum Amt %s gewählt. Die Wahl wurde angenommen.", name, in.Amt)
	} else {
		b.WriteString("Es kam keine Wahl zustande.")
	}
	return b.String()
}

func (a *App) handleWahl(w http.ResponseWriter, r *http.Request) {
	var in wahlEingabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in.Amt = trim(in.Amt, 60)
	if in.Amt == "" || !gueltigesDatum(in.Datum, false) {
		writeErr(w, bad("Bitte Amt und Tag der Wahl eintragen"))
		return
	}
	if in.Gremium != "Vorstand" {
		in.Gremium = "Mitgliederversammlung"
	}
	if len(in.Kandidaten) == 0 || len(in.Kandidaten) > 12 {
		writeErr(w, bad("Bitte einen bis zwölf Kandidaten eintragen"))
		return
	}
	for i := range in.Kandidaten {
		k := &in.Kandidaten[i]
		k.Name = trim(k.Name, 100)
		if k.Name == "" || k.Stimmen < 0 || k.Stimmen > 100000 {
			writeErr(w, bad("Bitte die Namen und Stimmen prüfen"))
			return
		}
	}
	if in.Nein < 0 || in.Enthaltung < 0 || in.Ungueltig < 0 || in.Nein > 100000 || in.Enthaltung > 100000 || in.Ungueltig > 100000 || in.Amtszeit < 0 || in.Amtszeit > 10 {
		writeErr(w, bad("Bitte die Zahlen prüfen"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	name, ja, ok, hinweis := wahlAuswerten(in)
	res := wahlErgebnis{Gewaehlt: name, Ok: ok, Hinweis: hinweis, Protokoll: wahlProtokoll(in, name, ok, a.st.d.Settings)}
	if name == "" && !ok {
		writeJSON(w, 200, res) // nichts auszuwerten: nichts speichern
		return
	}
	titel := "Wahl " + in.Amt + ": " + name
	text := res.Protokoll
	if !ok {
		titel = "Wahl " + in.Amt + ": ohne Ergebnis"
	}
	b := Beschluss{Datum: in.Datum, Gremium: in.Gremium, Titel: titel, Text: text, Ja: ja, Nein: in.Nein, Enthaltung: in.Enthaltung, Ergebnis: "angenommen", Stichworte: "Wahl"}
	if !ok {
		b.Ergebnis = "abgelehnt"
	}
	if len(a.st.d.Beschluesse) >= maxVerwaltung {
		writeErr(w, bad("Zu viele Beschlüsse"))
		return
	}
	b.Nummer, b.ID = a.st.beschlussNummerLocked(b), newID()
	// Amt eintragen, bevor das Beschlussbuch ergänzt wird: schlägt es fehl, bleibt alles unverändert.
	if ok && in.Uebernehmen {
		bis := ""
		if in.Amtszeit > 0 {
			if d, err := parseDate(in.Datum); err == nil {
				bis = d.AddDate(in.Amtszeit, 0, 0).Format("2006-01-02")
			}
		}
		neu := Amt{Amt: in.Amt, Name: name, Seit: in.Datum, Bis: bis}
		var err error
		if i := a.amtIndex(in.AmtID); i >= 0 && strings.EqualFold(strings.TrimSpace(a.st.d.Aemter[i].Name), name) && !a.st.d.Aemter[i].Ehemalig {
			a.st.d.Aemter[i].Bis = bis // Wiederwahl: gleiche Person, neue Amtszeit, keine Übergabe nötig
			a.st.audit("Wiederwahl %s: %s", in.Amt, name)
		} else if in.AmtID != "" {
			err = a.amtWechselLocked(in.AmtID, neu)
		} else if neu, err = cleanAmt(neu); err == nil {
			if len(a.st.d.Aemter) >= maxVerwaltung {
				err = bad("Zu viele Ämter")
			} else {
				neu.ID = newID()
				a.st.d.Aemter = append(a.st.d.Aemter, neu)
				a.st.audit("Amt eingetragen: %s (%s)", neu.Amt, neu.Name)
			}
		}
		if err != nil {
			writeErr(w, err)
			return
		}
		res.AmtEingetr = true
	}
	a.st.d.Beschluesse = append(a.st.d.Beschluesse, b)
	a.st.audit("Wahl %s ausgezählt: %s (%s)", in.Amt, titelOderLeer(name), b.Nummer)
	res.Beschluss = b.Nummer
	a.speichernUndAntworten(w, res)
}

func titelOderLeer(s string) string {
	if s == "" {
		return "ohne Ergebnis"
	}
	return s
}

// wahlzettelPDF: Stimmzettel, vier je Seite zum Zuschneiden. Anzahl und Kandidaten kommen per Abfrage.
func (a *App) wahlzettelPDF(r *http.Request) ([]byte, string, error) {
	q := r.URL.Query()
	amt := trim(q.Get("amt"), 60)
	kand := []string{}
	for _, k := range q["kandidat"] {
		if k = trim(k, 100); k != "" {
			kand = append(kand, k)
		}
	}
	anzahl := 0
	fmt.Sscanf(q.Get("anzahl"), "%d", &anzahl)
	if amt == "" || len(kand) == 0 || len(kand) > 12 || anzahl < 1 || anzahl > 400 {
		return nil, "", bad("Bitte Amt, Kandidaten und die Zahl der Stimmzettel (1 bis 400) angeben")
	}
	a.st.mu.Lock()
	s := a.st.d.Settings
	a.st.mu.Unlock()
	pdf := neuesPDF(s, "Stimmzettel "+amt, false)
	ew, eh := 190.0, 66.0
	for n := 0; n < anzahl; n++ {
		if n%4 == 0 {
			pdf.AddPage()
		}
		y0 := 10 + float64(n%4)*(eh+4)
		pdf.SetDrawColor(150, 150, 150)
		pdf.SetDashPattern([]float64{2, 2}, 0)
		pdf.Rect(10, y0, ew, eh, "D")
		pdf.SetDashPattern([]float64{}, 0)
		pdf.SetFont("lib", "B", 12)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(14, y0+3)
		pdf.CellFormat(ew-8, 6, "Stimmzettel – Wahl: "+amt, "", 0, "L", false, 0, "")
		pdf.SetFont("lib", "", 9)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(14, y0+9.5)
		hinweis := "Bitte höchstens ein Feld ankreuzen. Mehrere Kreuze machen den Stimmzettel ungültig."
		pdf.CellFormat(ew-8, 5, hinweis, "", 0, "L", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		zeilen := append(append([]string{}, kand...), "Enthaltung")
		if len(kand) == 1 {
			zeilen = []string{kand[0] + " – Ja", kand[0] + " – Nein", "Enthaltung"}
		}
		sz := min(6.5, (eh-20)/float64(len(zeilen)))
		for i, z := range zeilen {
			y := y0 + 17 + float64(i)*sz
			pdf.SetDrawColor(0, 0, 0)
			pdf.Rect(16, y, 4.2, 4.2, "D")
			pdf.SetFont("lib", "", 11)
			pdf.SetXY(24, y-0.7)
			pdf.CellFormat(ew-24, 5.6, z, "", 0, "L", false, 0, "")
		}
		if n%4 == 3 || n == anzahl-1 {
			mappeFuss(pdf, s, s.VereinName+" · Stimmzettel "+amt+" · "+time.Now().Format("02.01.2006"))
		}
	}
	out, err := pdfBytes(pdf)
	return out, "Stimmzettel.pdf", err
}
