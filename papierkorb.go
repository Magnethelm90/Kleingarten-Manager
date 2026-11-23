package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

// Papierkorb: Gelöschte Mängel, Termine, Beschlüsse und Verträge bleiben 30 Tage lang wiederherstellbar. Der Eintrag
// wird unverändert als Abbild abgelegt und beim Wiederherstellen an seinen Platz zurückgestellt. Pächter haben ihren
// eigenen Papierkorb (Geloescht), die Warteliste bleibt bewusst draußen: Dort bedeutet »Löschen« Datenschutz.

const (
	papierkorbTage = 30
	papierkorbMax  = 500
)

// PapierkorbEintrag ist ein gelöschter Eintrag samt Abbild.
type PapierkorbEintrag struct {
	ID          string          `json:"id"`
	Art         string          `json:"art"` // mangel | termin | beschluss | vertrag
	Titel       string          `json:"titel"`
	GeloeschtAm string          `json:"geloeschtAm"` // JJJJ-MM-TT
	PaechterID  string          `json:"paechterId,omitempty"`
	Daten       json.RawMessage `json:"daten"`
}

var papierkorbArten = map[string]string{"mangel": "Mangel", "termin": "Termin", "beschluss": "Beschluss", "vertrag": "Vertrag"}

// papierkorbLegeLocked legt ein Abbild ab. Der Aufrufer hält s.mu und speichert danach.
// Liefert die ID des Eintrags (für »Rückgängig« direkt nach dem Löschen), leer, wenn nichts abgelegt wurde.
func (s *Store) papierkorbLegeLocked(art, titel, paechterID string, v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	id := newID()
	s.d.Papierkorb = append(s.d.Papierkorb, PapierkorbEintrag{ID: id, Art: art, Titel: titel,
		GeloeschtAm: time.Now().Format("2006-01-02"), PaechterID: paechterID, Daten: raw})
	if n := len(s.d.Papierkorb); n > papierkorbMax {
		s.d.Papierkorb = append([]PapierkorbEintrag(nil), s.d.Papierkorb[n-papierkorbMax:]...)
	}
	return id
}

// papierkorbAufraeumenLocked entfernt, was länger als 30 Tage liegt. Liefert true, wenn etwas entfernt wurde.
func (s *Store) papierkorbAufraeumenLocked(heute time.Time) bool {
	grenze := heute.AddDate(0, 0, -papierkorbTage).Format("2006-01-02")
	var neu []PapierkorbEintrag
	for _, e := range s.d.Papierkorb {
		if e.GeloeschtAm >= grenze {
			neu = append(neu, e)
		}
	}
	if len(neu) == len(s.d.Papierkorb) {
		return false
	}
	s.d.Papierkorb = neu
	return true
}

// papierkorbPersonEntfernen löscht Abbilder, die an der Person hängen (Datenschutz).
func (d *Data) papierkorbPersonEntfernen(id string) bool {
	var neu []PapierkorbEintrag
	for _, e := range d.Papierkorb {
		if e.PaechterID == "" || e.PaechterID != id {
			neu = append(neu, e)
		}
	}
	if len(neu) == len(d.Papierkorb) {
		return false
	}
	d.Papierkorb = neu
	return true
}

type papierkorbZeile struct {
	ID          string `json:"id"`
	Art         string `json:"art"`
	ArtText     string `json:"artText"`
	Titel       string `json:"titel"`
	GeloeschtAm string `json:"geloeschtAm"`
	NochTage    int    `json:"nochTage"`
}

func (a *App) handlePapierkorb(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	heute := time.Now()
	if a.st.papierkorbAufraeumenLocked(heute) {
		_ = a.st.saveLocked()
	}
	out := []papierkorbZeile{}
	for _, e := range a.st.d.Papierkorb {
		rest := papierkorbTage + tageBis(heute, e.GeloeschtAm)
		out = append(out, papierkorbZeile{e.ID, e.Art, papierkorbArten[e.Art], e.Titel, e.GeloeschtAm, max(rest, 0)})
	}
	a.st.mu.Unlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].GeloeschtAm > out[j].GeloeschtAm })
	writeJSON(w, 200, out)
}

func (a *App) handlePapierkorbWiederherstellen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	d := a.st.d
	for i, e := range d.Papierkorb {
		if e.ID != id {
			continue
		}
		var err error
		switch e.Art {
		case "mangel":
			var m Mangel
			if err = json.Unmarshal(e.Daten, &m); err == nil {
				d.Maengel = append(d.Maengel, m)
			}
		case "termin":
			var t Termin
			if err = json.Unmarshal(e.Daten, &t); err == nil {
				d.Termine = append(d.Termine, t)
				sortiereTermine(d.Termine)
			}
		case "beschluss":
			var b Beschluss
			if err = json.Unmarshal(e.Daten, &b); err == nil {
				for _, x := range d.Beschluesse {
					if x.Nummer == b.Nummer {
						b.Nummer = a.st.beschlussNummerLocked(b) // Nummer inzwischen anderweitig vergeben
						break
					}
				}
				d.Beschluesse = append(d.Beschluesse, b)
			}
		case "vertrag":
			var v Vertrag
			if err = json.Unmarshal(e.Daten, &v); err == nil {
				d.Vertraege = append(d.Vertraege, v)
			}
		default:
			err = bad("Unbekannte Art")
		}
		if err != nil {
			writeErr(w, bad("Der Eintrag lässt sich nicht wiederherstellen"))
			return
		}
		d.Papierkorb = append(d.Papierkorb[:i], d.Papierkorb[i+1:]...)
		a.st.audit("Aus dem Papierkorb wiederhergestellt: %s %s", papierkorbArten[e.Art], e.Titel)
		a.speichernUndAntworten(w, map[string]bool{"ok": true})
		return
	}
	writeErr(w, notFound("Eintrag nicht im Papierkorb"))
}

func (a *App) handlePapierkorbLeeren(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	n := len(a.st.d.Papierkorb)
	a.st.d.Papierkorb = nil
	a.st.fotosAufraeumenLocked()
	a.st.audit("Papierkorb geleert (%d Einträge)", n)
	a.speichernUndAntworten(w, map[string]int{"entfernt": n})
}
