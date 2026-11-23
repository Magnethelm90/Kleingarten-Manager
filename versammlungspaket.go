package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"strings"
)

// Versammlungspaket: ein Knopf bündelt alles, was für eine Mitgliederversammlung gebraucht wird, als ZIP –
// Einladung mit Tagesordnung, Anwesenheitsliste, Jahresbericht, Bericht der Kassenprüfung (falls erfasst) und das
// vorbereitete Protokoll mit den Tagesordnungspunkten. Der Jahresbericht enthält keine Namen.

func (a *App) handleVersammlungspaket(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	t, ok := a.st.terminLocked(id)
	jahr := a.st.d.Settings.Jahr
	_, hatPruefung := a.st.d.Kassenpruefungen[yearKey(jahr)]
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Termin nicht gefunden"))
		return
	}
	if t.Art != "Versammlung" && t.Art != "Vorstand" {
		writeErr(w, bad("Das Paket gibt es nur für Versammlungen und Vorstandssitzungen"))
		return
	}
	// Die vorhandenen PDF-Erzeuger lesen Termin und Jahr aus der Anfrage.
	anfrage := func(jahrParam bool) *http.Request {
		u := "/"
		if jahrParam {
			u += fmt.Sprintf("?year=%d", jahr)
		}
		x, _ := http.NewRequest("GET", u, nil)
		x.SetPathValue("id", id)
		return x
	}
	teile := []struct {
		ordner string
		bau    pdfErzeuger
		jahr   bool
		wenn   bool
	}{
		{"1 Einladung", a.einladungPDF, false, true},
		{"2 Anwesenheitsliste", a.anwesenheitPDF, false, true},
		{"3 Jahresbericht", a.jahresberichtPDF, true, true},
		{"4 Bericht der Kassenpruefung", a.pruefberichtPDF, true, hatPruefung},
		{"5 Protokoll (vorbereitet)", a.sitzungPDF, false, true},
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	n := 0
	for _, tl := range teile {
		if !tl.wenn {
			continue
		}
		pdf, name, err := tl.bau(anfrage(tl.jahr))
		if err != nil {
			writeErr(w, err)
			return
		}
		f, err := zw.Create(safeName(strings.SplitN(tl.ordner, " ", 2)[0]+"_"+strings.TrimSuffix(name, ".pdf"), 100) + ".pdf")
		if err != nil {
			writeErr(w, err)
			return
		}
		_, _ = f.Write(pdf)
		n++
	}
	for i, bf := range a.befragungenImJahr(jahr) {
		a.st.mu.Lock()
		s := a.st.d.Settings
		a.st.mu.Unlock()
		pdf, err := buildBefragung(s, bf, "auswertung")
		if err != nil {
			writeErr(w, err)
			return
		}
		f, err := zw.Create(fmt.Sprintf("6_Befragung_%d_%s.pdf", i+1, safeName(bf.Titel, 60)))
		if err != nil {
			writeErr(w, err)
			return
		}
		_, _ = f.Write(pdf)
		n++
	}
	if err := zw.Close(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	a.st.audit("Versammlungspaket erstellt (%d Dokumente)", n)
	_ = a.st.saveLocked()
	a.st.mu.Unlock()
	name := safeName("Versammlungspaket_"+t.Titel+"_"+t.Datum, 100) + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}
