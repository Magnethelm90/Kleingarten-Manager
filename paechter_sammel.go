package main

import (
	"net/http"
	"strings"
)

// Sammeländerung: mehrere Pächter auf einmal auf eine Versandart oder eine Umlage setzen.
// Nur diese zwei Angaben – alles andere ist je Pächter verschieden und bleibt im Einzelformular.

type paechterSammelEingabe struct {
	IDs           []string `json:"ids"`
	Versand       string   `json:"versand"`       // "" = nicht ändern
	UmlageAendern bool     `json:"umlageAendern"` // false = nicht ändern
	Umlage        *float64 `json:"umlage"`        // nil = Standard-Umlage
}

type paechterSammelErgebnis struct {
	Geaendert     int      `json:"geaendert"`
	Uebersprungen []string `json:"uebersprungen"` // Namen, bei denen »Emailsendung« ohne E-Mail-Adresse nicht ging
}

func (a *App) handlePaechterSammel(w http.ResponseWriter, r *http.Request) {
	var in paechterSammelEingabe
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if len(in.IDs) == 0 {
		writeErr(w, bad("Bitte mindestens einen Pächter auswählen"))
		return
	}
	if in.Versand != "" && in.Versand != "Emailsendung" && in.Versand != "Postversand" {
		writeErr(w, bad("Unbekannte Versandart"))
		return
	}
	if in.Umlage != nil && (!validNum(*in.Umlage) || *in.Umlage > 100000) {
		writeErr(w, bad("Die Umlage muss eine Zahl ab 0 sein"))
		return
	}
	if in.Versand == "" && !in.UmlageAendern {
		writeErr(w, bad("Bitte angeben, was geändert werden soll"))
		return
	}
	ids := map[string]bool{}
	for _, id := range in.IDs {
		ids[id] = true
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	erg := paechterSammelErgebnis{Uebersprungen: []string{}}
	var namen []string
	for i := range a.st.d.Paechter {
		p := &a.st.d.Paechter[i]
		if !ids[p.ID] || p.Geloescht {
			continue
		}
		geaendert := false
		if in.Versand != "" && p.Versand != in.Versand {
			if in.Versand == "Emailsendung" && strings.TrimSpace(p.Email) == "" {
				erg.Uebersprungen = append(erg.Uebersprungen, p.Name)
			} else {
				p.Versand = in.Versand
				geaendert = true
			}
		}
		if in.UmlageAendern && !gleicheUmlage(p.UmlageAbweichend, in.Umlage) {
			if in.Umlage == nil {
				p.UmlageAbweichend = nil
			} else {
				u := *in.Umlage
				p.UmlageAbweichend = &u
			}
			geaendert = true
		}
		if geaendert {
			erg.Geaendert++
			namen = append(namen, p.Mitgliedsnr)
		}
	}
	if erg.Geaendert > 0 {
		was := []string{}
		if in.Versand != "" {
			was = append(was, "Versandart "+in.Versand)
		}
		if in.UmlageAendern {
			if in.Umlage == nil {
				was = append(was, "Umlage Standard")
			} else {
				was = append(was, "Umlage "+fmtEUR(*in.Umlage))
			}
		}
		a.st.audit("Sammeländerung (%s) bei %d Pächtern: %s", strings.Join(was, ", "), erg.Geaendert, strings.Join(namen, ", "))
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
	}
	writeJSON(w, 200, erg)
}

func gleicheUmlage(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
