package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// openPathFn öffnet eine Datei oder einen Ordner im Standardprogramm des Betriebssystems
// (in Tests ersetzbar, damit kein echtes Programm gestartet wird).
var openPathFn = openPath

// druckOrdner enthält Druckdateien, die nur zum Öffnen im PDF-Programm entstehen (Sammel-PDF der
// Postrechnungen, Ablesebogen). Sie lassen sich jederzeit neu erzeugen, enthalten aber Namen und
// Anschriften; deshalb wird der Ordner beim Programmstart und beim Löschen von Personen geleert.
const druckOrdner = "Druck"

func (s *Store) druckDir() string { return filepath.Join(s.dir, druckOrdner) }

// leereDruckOrdner entfernt alle gewöhnlichen Dateien im Druckordner (Verweise bleiben unangetastet).
func (s *Store) leereDruckOrdner() {
	entries, err := os.ReadDir(s.druckDir())
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type().IsRegular() {
			_ = os.Remove(filepath.Join(s.druckDir(), e.Name()))
		}
	}
}

// schreibeDruckdatei legt eine Druckdatei im Druckordner ab und gibt ihren Pfad zurück.
func (s *Store) schreibeDruckdatei(name string, data []byte) (string, error) {
	if err := os.MkdirAll(s.druckDir(), 0o700); err != nil {
		return "", err
	}
	full := filepath.Join(s.druckDir(), name)
	if fi, err := os.Lstat(full); err == nil && !fi.Mode().IsRegular() {
		return "", fmt.Errorf("%s ist keine gewöhnliche Datei", name)
	}
	if err := os.WriteFile(full, data, 0o600); err != nil {
		return "", err
	}
	return full, nil
}

// naturalLess sortiert Nummern wie »35-95« oder »2-1« so, wie man es erwartet (Zahlen nach Wert).
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		da, db := digitRun(a), digitRun(b)
		if da > 0 && db > 0 {
			na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[da:], b[db:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

// digitRun gibt die Länge der Ziffernfolge am Anfang von s zurück.
func digitRun(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

// druckItemsLocked sammelt die gültigen, ausgestellten Rechnungen eines Jahres für den Sammeldruck,
// sortiert nach Mitgliedsnummer. Nur Postversand, außer alle ist gesetzt. Der Aufrufer hält s.mu.
func (s *Store) druckItemsLocked(year int, alle bool) []invoiceItem {
	var recs []Rechnung
	for _, x := range s.d.Rechnungen {
		if x.Jahr != year || x.Status != statusGueltig || x.Bereinigt {
			continue
		}
		if !alle && !strings.EqualFold(strings.TrimSpace(x.Paechter.Versand), "Postversand") {
			continue
		}
		recs = append(recs, *x)
	}
	sort.SliceStable(recs, func(i, j int) bool { return naturalLess(recs[i].Paechter.Mitgliedsnr, recs[j].Paechter.Mitgliedsnr) })
	items := make([]invoiceItem, len(recs))
	for i, r := range recs {
		items[i] = invoiceItem{S: r.Settings, P: r.Paechter, A: r.Ablesung, R: r.Result}
	}
	return items
}

func (a *App) druckPDF(r *http.Request) ([]byte, int, int, error) {
	year := a.yearParam(r)
	alle := r.URL.Query().Get("versand") == "alle"
	a.st.mu.Lock()
	items := a.st.druckItemsLocked(year, alle)
	a.st.mu.Unlock()
	if len(items) == 0 {
		what := "mit Postversand"
		if alle {
			what = ""
		}
		return nil, year, 0, notFound(strings.TrimSpace(fmt.Sprintf("Für %d gibt es keine ausgestellten Rechnungen %s", year, what)))
	}
	pdf, err := buildInvoicesCombined(items, fmt.Sprintf("Rechnungen %d", year))
	if err != nil {
		return nil, year, 0, bad(err.Error())
	}
	return pdf, year, len(items), nil
}

// handleDruckPost liefert alle Postversand-Rechnungen eines Jahres als eine PDF-Datei zum Ausdrucken.
func (a *App) handleDruckPost(w http.ResponseWriter, r *http.Request) {
	pdf, year, _, err := a.druckPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="Rechnungen_%d_Postversand.pdf"`, year))
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

// handleDruckPostOeffnen legt die Sammel-PDF im Rechnungsordner des Jahres ab und öffnet sie im
// PDF-Programm des Rechners. Das funktioniert auch im Programmfenster, wo es keine Browser-Tabs gibt.
func (a *App) handleDruckPostOeffnen(w http.ResponseWriter, r *http.Request) {
	pdf, year, n, err := a.druckPDF(r)
	if err != nil {
		writeErr(w, err)
		return
	}
	full, err := a.st.schreibeDruckdatei(fmt.Sprintf("Rechnungen_%d_Postversand.pdf", year), pdf)
	if err != nil {
		writeErr(w, err)
		return
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]any{"anzahl": n, "datei": path.Join(druckOrdner, filepath.Base(full))})
}

// handleOpenInvoice öffnet die gespeicherte PDF einer ausgestellten Rechnung im PDF-Programm des
// Rechners (dort steht der volle Druckdialog zur Verfügung). Fehlt die Datei, wird sie aus den
// archivierten Werten neu erzeugt.
func (a *App) handleOpenInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var rec Rechnung
	found := false
	a.st.mu.Lock()
	for _, x := range a.st.d.Rechnungen {
		if x.ID == id {
			rec, found = *x, true
			break
		}
	}
	a.st.mu.Unlock()
	if !found {
		writeErr(w, notFound("Rechnung nicht im Archiv gefunden"))
		return
	}
	if rec.Bereinigt {
		writeErr(w, apiError{http.StatusGone, "Die Aufbewahrungsfrist dieser Rechnung ist abgelaufen, der Personenbezug wurde entfernt"})
		return
	}
	full, err := a.rechnungsDatei(rec.Datei)
	if err != nil {
		writeErr(w, bad(err.Error()))
		return
	}
	fi, statErr := os.Lstat(full)
	switch {
	case statErr == nil && !fi.Mode().IsRegular():
		// z. B. ein Verweis (Symlink): nie hindurchschreiben oder öffnen
		writeErr(w, bad("Die gespeicherte Rechnungsdatei ist keine gewöhnliche Datei"))
		return
	case statErr != nil && !os.IsNotExist(statErr):
		writeErr(w, statErr)
		return
	case statErr != nil:
		pdf, err := buildInvoice(rec.Settings, rec.Paechter, rec.Ablesung, rec.Result)
		if err != nil {
			writeErr(w, bad(err.Error()))
			return
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			writeErr(w, err)
			return
		}
		if err := os.WriteFile(full, pdf, 0o600); err != nil {
			writeErr(w, err)
			return
		}
	}
	openPathFn(full)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// rechnungsDatei wandelt den gespeicherten relativen Pfad einer Rechnung in einen absoluten um und
// stellt sicher, dass er innerhalb des Rechnungsordners liegt und eine PDF-Datei meint.
func (a *App) rechnungsDatei(rel string) (string, error) {
	rel = filepath.ToSlash(rel)
	if rel == "" || !strings.HasPrefix(rel, "Rechnungen/") || !strings.HasSuffix(strings.ToLower(rel), ".pdf") || strings.Contains(rel, "..") {
		return "", fmt.Errorf("Für diese Rechnung ist keine Datei gespeichert")
	}
	base := filepath.Join(a.st.dir, "Rechnungen")
	full := filepath.Join(a.st.dir, filepath.FromSlash(rel))
	if !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("Ungültiger Dateipfad")
	}
	return full, nil
}
