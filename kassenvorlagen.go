package main

import "net/http"

// Kassenbuch-Vorlagen: wiederkehrende Ausgaben (Versicherung, Strom Vereinsheim, Kontoführung) einmal als Vorlage
// speichern und danach im Ausgaben-Dialog mit einem Klick einsetzen. Die Vorlage enthält nur Beschreibung, Kategorie und
// Betrag – Datum und Beleg kommen bei jeder Buchung neu, und gebucht wird erst mit »Speichern«.

type KassenVorlage struct {
	ID           string  `json:"id"`
	Beschreibung string  `json:"beschreibung"`
	Kategorie    string  `json:"kategorie"`
	Betrag       float64 `json:"betrag"`
}

const maxKassenVorlagen = 30

func (a *App) handleKassenVorlagenGet(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	v := append([]KassenVorlage{}, a.st.d.KassenVorlagen...)
	a.st.mu.Unlock()
	writeJSON(w, 200, v)
}

// handleKassenVorlagenPut ersetzt die ganze Liste (Hinzufügen und Löschen im Dialog).
func (a *App) handleKassenVorlagenPut(w http.ResponseWriter, r *http.Request) {
	var in []KassenVorlage
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if len(in) > maxKassenVorlagen {
		writeErr(w, bad("Es sind höchstens 30 Vorlagen möglich."))
		return
	}
	out := make([]KassenVorlage, 0, len(in))
	for _, v := range in {
		v.Beschreibung = trim(v.Beschreibung, 120)
		v.Kategorie = trim(v.Kategorie, 40)
		if v.Beschreibung == "" {
			writeErr(w, bad("Eine Vorlage braucht eine Beschreibung."))
			return
		}
		if !validNum(v.Betrag) || v.Betrag < 0 {
			writeErr(w, bad("Der Betrag einer Vorlage muss eine Zahl ab 0 sein."))
			return
		}
		if v.ID == "" {
			v.ID = newID()
		}
		out = append(out, v)
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.KassenVorlagen = out
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, out)
}
