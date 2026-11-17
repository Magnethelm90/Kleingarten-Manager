package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Zuwendungsbestätigungen für ein ganzes Jahr: alle noch nicht bestätigten Spenden eines Kalenderjahres, je Spender
// eine (Sammel-)Bestätigung, als ZIP. Das Paket vermerkt nichts – erst nach dem Ausdrucken bestätigt der Vorstand im
// Programm, dass die Bestätigungen ausgestellt sind (POST /api/admin/spenden/bestaetigen je Spender).

type spendenGruppe struct {
	Name string   `json:"name"`
	IDs  []string `json:"ids"`
}

// offeneSpendenGruppenLocked: offene Spenden des Jahres je Spender (wie bei der Sammelbestätigung: Name+Anschrift);
// ohne Anschrift oder bereinigt landen sie in »fehlt«.
func (s *Store) offeneSpendenGruppenLocked(jahr int) (gruppen []spendenGruppe, fehlt []string) {
	idx := map[string]int{}
	j := strconv.Itoa(jahr)
	l := append([]Spende(nil), s.d.Spenden...)
	sort.SliceStable(l, func(a, b int) bool { return l[a].Datum < l[b].Datum })
	for _, x := range l {
		if jahrVon(x.Datum) != j || x.BestaetigtAm != "" || x.Bereinigt {
			continue
		}
		if x.Strasse == "" || x.PLZOrt == "" {
			fehlt = append(fehlt, fmt.Sprintf("%s (%s, %s): Anschrift fehlt", x.Name, germanDate(x.Datum), fmtEUR(x.Betrag)))
			continue
		}
		k := strings.ToLower(x.Name + "|" + x.Strasse + "|" + x.PLZOrt)
		i, ok := idx[k]
		if !ok {
			i = len(gruppen)
			idx[k] = i
			gruppen = append(gruppen, spendenGruppe{Name: x.Name})
		}
		gruppen[i].IDs = append(gruppen[i].IDs, x.ID)
	}
	return gruppen, fehlt
}

// GET /api/admin/spenden/jahr?jahr=JJJJ – Übersicht, was das Jahrespaket enthielte.
func (a *App) handleSpendenJahr(w http.ResponseWriter, r *http.Request) {
	jahr, err := strconv.Atoi(r.URL.Query().Get("jahr"))
	if err != nil || jahr < 1900 || jahr > 3000 {
		writeErr(w, bad("Ungültiges Jahr"))
		return
	}
	a.st.mu.Lock()
	g, f := a.st.offeneSpendenGruppenLocked(jahr)
	a.st.mu.Unlock()
	if g == nil {
		g = []spendenGruppe{}
	}
	if f == nil {
		f = []string{}
	}
	writeJSON(w, 200, map[string]any{"gruppen": g, "fehlt": f})
}

// GET /api/admin/spenden/jahrespaket?jahr=JJJJ – ZIP mit einer Bestätigung je Spender.
func (a *App) handleSpendenJahrespaket(w http.ResponseWriter, r *http.Request) {
	jahr, err := strconv.Atoi(r.URL.Query().Get("jahr"))
	if err != nil || jahr < 1900 || jahr > 3000 {
		writeErr(w, bad("Ungültiges Jahr"))
		return
	}
	heute := time.Now()
	a.st.mu.Lock()
	gruppen, fehlt := a.st.offeneSpendenGruppenLocked(jahr)
	var listen [][]Spende
	for _, g := range gruppen {
		// dieselben Prüfungen wie bei der Einzel-/Sammelbestätigung (Gemeinnützigkeit, Anschrift, ein Spender)
		l, err := a.st.spendenFuerBestaetigungLocked(g.IDs, heute)
		if err != nil {
			a.st.mu.Unlock()
			writeErr(w, err)
			return
		}
		listen = append(listen, l)
	}
	s, gm := a.st.d.Settings, a.st.d.Gemeinnuetzigkeit
	a.st.mu.Unlock()
	if len(listen) == 0 {
		writeErr(w, bad(fmt.Sprintf("Für %d gibt es keine offenen Spenden mit Anschrift", jahr)))
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, l := range listen {
		pdf, err := buildZuwendungsbestaetigung(s, gm, l, heute)
		if err != nil {
			writeErr(w, err)
			return
		}
		f, err := zw.Create(safeName(fmt.Sprintf("%02d_Zuwendungsbestaetigung_%s_%d", i+1, l[0].Name, jahr), 100) + ".pdf")
		if err != nil {
			writeErr(w, err)
			return
		}
		_, _ = f.Write(pdf)
	}
	if len(fehlt) > 0 {
		f, _ := zw.Create("Nicht_enthalten.txt")
		_, _ = f.Write([]byte("Diese Spenden fehlen im Paket – bitte die Anschrift ergänzen und einzeln bestätigen:\r\n\r\n" + strings.Join(fehlt, "\r\n") + "\r\n"))
	}
	if err := zw.Close(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	a.st.audit("Zuwendungsbestätigungen %d als Paket erstellt (%d Spender, noch nicht vermerkt)", jahr, len(listen))
	_ = a.st.saveLocked()
	a.st.mu.Unlock()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="Zuwendungsbestaetigungen_`+strconv.Itoa(jahr)+`.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}
