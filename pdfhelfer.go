package main

import (
	"bytes"
	"net/http"
	"path/filepath"

	"github.com/go-pdf/fpdf"
)

// Gemeinsame Hilfen für PDF-Antworten: Anzeige im Browser und »im PDF-Programm öffnen« (Druck im
// Programmfenster). Der Erzeuger liefert PDF, Dateinamen und ggf. einen Fehler.

type pdfErzeuger func(r *http.Request) ([]byte, string, error)

func (a *App) pdfAnzeigen(bau pdfErzeuger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pdf, name, err := bau(r)
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", `inline; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(pdf)
	}
}

func (a *App) pdfOeffnen(bau pdfErzeuger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pdf, name, err := bau(r)
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
}

// neuesPDF legt ein A4-Dokument mit den Schriften des Programms an.
func neuesPDF(s Settings, titel string, quer bool) *fpdf.Fpdf {
	ori := "P"
	if quer {
		ori = "L"
	}
	pdf := fpdf.New(ori, "mm", "A4", "")
	pdf.AddUTF8FontFromBytes("lib", "", fontRegular)
	pdf.AddUTF8FontFromBytes("lib", "B", fontBold)
	pdf.SetMargins(leftX, 12, 20)
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetTitle(titel, true)
	pdf.SetAuthor(s.VereinName, true)
	pdf.SetCreator(appName, true)
	return pdf
}

func pdfBytes(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var langeWochentage = []string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}
