package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Amtsübergabe: Wechselt der Vorstand oder Kassenwart, geht oft Wissen verloren. Die Vereinsakte ist ein ZIP mit
// dem kompletten Datenordner (Daten, Rechnungen, Gartenakten, Belege, Logo) und einem PDF »Lies mich zuerst«:
// wie man startet, die Zahlen des Vereins, Preise und Regeln, das Jahr mit seinen Terminen, die Angaben zur
// Gemeinnützigkeit und was gerade ansteht. Passwörter stehen nirgends im Klartext.

const vereinsakteOrdner = "Vereinsakte"

// vereinsakteAuslassen: Ordner und Dateien, die nicht in die Akte gehören (Zwischenstände, Sperre, alte Akten).
func vereinsakteAuslassen(rel string, dir bool) bool {
	erster := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	if dir {
		return erster == "Sicherungen" || erster == druckOrdner || erster == vereinsakteOrdner
	}
	base := filepath.Base(rel)
	return base == sperrDateiName || strings.HasPrefix(base, ".schreibtest") || strings.HasSuffix(base, ".tmp")
}

type uebergabeDaten struct {
	S        Settings
	Info     Vereinsinfo
	G        Gemeinnuetzigkeit
	K        kassenbericht
	Paechter int
	Gaerten  int
	Wartende int
	Termine  []Termin
	Aufgaben []aufgabe
	Jahre    []int
	// Inventar: Schlüssel (wer hat welchen) und Geräte, je eine Zeile
	Inventar [][2]string
}

func (s *Store) uebergabeLocked(heute time.Time) uebergabeDaten {
	d := uebergabeDaten{S: s.d.Settings, Info: s.d.Vereinsinfo, G: s.d.Gemeinnuetzigkeit, Wartende: len(s.d.Warteliste), Aufgaben: s.aufgabenLocked(heute)}
	if v, ok := s.viewLocked(s.d.Settings.Jahr); ok {
		d.K = s.kassenberichtLocked(v)
	}
	for _, p := range s.d.Paechter {
		if !p.Geloescht && p.AusgeschiedenAm == "" {
			d.Paechter++
		}
	}
	d.Gaerten = len(s.belegteGaertenLocked())
	for k := range s.d.Jahre {
		if y, err := strconv.Atoi(k); err == nil {
			d.Jahre = append(d.Jahre, y)
		}
	}
	for _, k := range s.d.Schluessel {
		wer, _ := s.inhaberName(k.PaechterID, k.Inhaber)
		if wer == "" {
			wer = "im Schrank"
		}
		d.Inventar = append(d.Inventar, [2]string{"Schlüssel " + k.Art + " " + k.Nummer, wer})
	}
	for _, g := range s.d.Geraete {
		wo := g.Standort
		if wo == "" {
			wo = "–"
		}
		if wer, _ := s.inhaberName(g.PaechterID, g.Entleiher); wer != "" {
			wo = "verliehen an " + wer
		}
		d.Inventar = append(d.Inventar, [2]string{g.Name, wo})
	}
	ab := heute.Format("2006-01-02")
	for _, t := range s.d.Termine {
		if t.Datum >= ab && len(d.Termine) < 16 {
			d.Termine = append(d.Termine, t)
		}
	}
	return d
}

func buildUebergabe(d uebergabeDaten, heute time.Time) ([]byte, error) {
	s := d.S
	pdf := neuesPDF(s, "Übergabe der Vereinsverwaltung", false)
	pdf.AddPage()
	zeichneKopf(pdf, s)
	mr, mg, mb := markenfarbe(s.Farbe)
	y := 38.0
	pdf.SetFont("lib", "B", 20)
	pdf.SetTextColor(mr, mg, mb)
	pdf.SetXY(leftX, y)
	pdf.CellFormat(rightX-leftX, 10, "Lies mich zuerst", "", 0, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	y += 12
	y = mappeText(pdf, y, fmt.Sprintf("Übergabe der Vereinsverwaltung des %s, Stand %s. In diesem Ordner steckt alles, was der Kleingarten-Manager über "+
		"den Verein weiß: Pächter, Zählerstände, alle Rechnungen, Kasse, Gartenakten, Termine und Spenden.", s.VereinName, heute.Format("02.01.2006")), 10.5)

	y = mappeAbschnitt(pdf, s, y, "So geht es weiter")
	for i, z := range []string{
		"Den Kleingarten-Manager herunterladen: github.com/Magnethelm90/Kleingarten-Manager → Releases (Windows: .exe, Mac: .zip).",
		"Den Inhalt des Ordners »Daten« aus dieser Akte in den Ordner kopieren, in dem die .exe liegt (die Datei kleingarten-manager-daten.json gehört direkt neben die .exe).",
		"Das Programm starten: Alles ist wie beim Vorgänger da. Das Admin-Passwort steht aus gutem Grund nirgends in dieser Akte – bitte persönlich übergeben. Ist es verloren, lässt es sich mit »--reset-admin« entfernen.",
		"Unter »Einstellungen« Bankverbindung, Ansprechpartner und den zweiten Sicherungsordner prüfen.",
	} {
		y = mappeText(pdf, y, fmt.Sprintf("%d.  %s", i+1, z), 10) - 1
	}
	y += 2

	y = mappeAbschnitt(pdf, s, y, "Der Verein in Zahlen")
	jahre := ""
	if len(d.Jahre) > 0 {
		min, max := d.Jahre[0], d.Jahre[0]
		for _, j := range d.Jahre {
			if j < min {
				min = j
			}
			if j > max {
				max = j
			}
		}
		jahre = fmt.Sprintf("%d bis %d", min, max)
	}
	y = mappeTabelle(pdf, y, [][2]string{
		{"Pächter / belegte Gärten", fmt.Sprintf("%d / %d", d.Paechter, d.Gaerten)},
		{"Abrechnungsjahr", strconv.Itoa(s.Jahr)}, {"Daten aus den Jahren", jahre},
		{"Kassenbestand " + strconv.Itoa(s.Jahr), fmtEUR(d.K.Kassenbestand)},
		{"Warteliste", mehrzahl(d.Wartende, "Interessent", "Interessenten")},
	})

	y = mappeAbschnitt(pdf, s, y, "Preise und Regeln "+strconv.Itoa(s.Jahr))
	y = mappeTabelle(pdf, y, [][2]string{
		{"Wasser", fmt.Sprintf("%s je m³, Grundpreis %s", fmtEUR(s.WasserPreis), fmtEUR(s.WasserGrundpreis))},
		{"Strom", fmt.Sprintf("%s je kWh, Grundpreis %s", fmtEUR(s.EnergiePreis), fmtEUR(s.EnergieGrundpreis))},
		{"Pacht", fmtEUR(s.PachtJeQm) + " je m²"},
		{"Beiträge", fmt.Sprintf("Verein %s, Verband %s, Umlage %s", fmtEUR(s.Vereinsbeitrag), fmtEUR(s.Territorialverband), fmtEUR(s.UmlageStandard))},
		{"Gemeinschaftsarbeit", fmt.Sprintf("%s Stunden, je fehlende Stunde %s", fmtFlex(s.Pflichtstunden), fmtEUR(s.NachzahlungJeStd))},
		{"Bankverbindung", strings.TrimSpace(s.BankName + " " + s.IBAN)},
	})

	if d.G.Steuernummer != "" {
		y = mappeAbschnitt(pdf, s, y, "Gemeinnützigkeit")
		gueltig := ""
		if b, err := parseDate(d.G.BescheidDatum); err == nil {
			j := freistellungGueltig
			if d.G.Bescheid == "feststellung" {
				j = feststellungGueltig
			}
			gueltig = b.AddDate(j, 0, 0).Format("02.01.2006")
		}
		y = mappeTabelle(pdf, y, [][2]string{{"Finanzamt / StNr.", d.G.Finanzamt + " / " + d.G.Steuernummer}, {"Bescheid vom", germanDate(d.G.BescheidDatum)},
			{"Zuwendungsbestätigungen bis", gueltig}})
	}

	if len(d.Aufgaben) > 0 {
		if y > 230 {
			pdf.AddPage()
			y = 20
		}
		y = mappeAbschnitt(pdf, s, y, "Was gerade ansteht")
		for _, a := range d.Aufgaben {
			if y > 275 {
				pdf.AddPage()
				y = 20
			}
			y = mappeText(pdf, y, "•  "+a.Titel+func() string {
				if a.Text != "" {
					return " – " + a.Text
				}
				return ""
			}(), 9.5) - 1.5
		}
		y += 2
	}
	if len(d.Termine) > 0 {
		if y > 230 {
			pdf.AddPage()
			y = 20
		}
		y = mappeAbschnitt(pdf, s, y, "Die nächsten Termine")
		for _, t := range d.Termine {
			if y > 278 {
				pdf.AddPage()
				y = 20
			}
			y = mappeText(pdf, y, germanDate(t.Datum)+"  "+t.Titel+" ("+t.Art+")", 9.5) - 1.5
		}
		y += 2
	}
	if len(d.Inventar) > 0 {
		if y > 230 {
			pdf.AddPage()
			y = 20
		}
		y = mappeAbschnitt(pdf, s, y, "Schlüssel und Geräte")
		for i := 0; i < len(d.Inventar); i += 12 {
			if y > 250 {
				pdf.AddPage()
				y = 20
			}
			y = mappeTabelle(pdf, y, d.Inventar[i:min(i+12, len(d.Inventar))])
		}
	}
	if d.Info.Ansprechpartner != "" || d.Info.Hinweise != "" {
		if y > 230 {
			pdf.AddPage()
			y = 20
		}
		y = mappeAbschnitt(pdf, s, y, "Ansprechpartner und Hinweise")
		mappeText(pdf, y, strings.TrimSpace(d.Info.Ansprechpartner+"\n\n"+d.Info.Hinweise), 9.5)
	}
	mappeFuss(pdf, s, s.VereinName+" · Vereinsakte vom "+heute.Format("02.01.2006")+" · enthält personenbezogene Daten – vertraulich behandeln")
	return pdfBytes(pdf)
}

// schreibeVereinsakte legt das ZIP im Ordner »Vereinsakte« des Datenordners an. Der Aufrufer hält s.mu, damit
// während des Packens niemand speichert.
func (s *Store) schreibeVereinsakteLocked(heute time.Time) (string, int, error) {
	lies, err := buildUebergabe(s.uebergabeLocked(heute), heute)
	if err != nil {
		return "", 0, err
	}
	ziel := filepath.Join(s.dir, vereinsakteOrdner)
	if err := os.MkdirAll(ziel, 0o700); err != nil {
		return "", 0, err
	}
	name := safeName("Vereinsakte_"+s.d.Settings.VereinName+"_"+heute.Format("2006-01-02"), 100) + ".zip"
	full := filepath.Join(ziel, name)
	tmp := full + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", 0, err
	}
	zw := zip.NewWriter(f)
	n := 0
	schreibe := func(name string, r io.Reader) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: heute})
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		n++
		return err
	}
	err = schreibe("Lies mich zuerst.pdf", bytes.NewReader(lies))
	if err == nil {
		err = filepath.WalkDir(s.dir, func(p string, e fs.DirEntry, werr error) error {
			if werr != nil {
				return nil // unlesbare Teile überspringen, der Rest zählt
			}
			rel, rerr := filepath.Rel(s.dir, p)
			if rerr != nil || rel == "." {
				return nil
			}
			if e.IsDir() {
				if vereinsakteAuslassen(rel, true) {
					return filepath.SkipDir
				}
				return nil
			}
			if !e.Type().IsRegular() || vereinsakteAuslassen(rel, false) {
				return nil // keine Verknüpfungen oder Sonderdateien
			}
			src, oerr := os.Open(p)
			if oerr != nil {
				return nil
			}
			defer src.Close()
			return schreibe("Daten/"+filepath.ToSlash(rel), src)
		})
	}
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	s.entferneVereinsakten() // nur die neueste behalten
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return "", 0, err
	}
	return full, n, nil
}

// entferneVereinsakten löscht alle früheren Vereinsakten im Datenordner: Jede ist eine Kopie aller Personendaten.
// Beim Erstellen bleibt so nur die neue übrig (sie liegt in diesem Moment noch als .tmp vor), beim endgültigen
// Löschen einer Person verschwinden alle, weil sie deren Daten noch enthalten.
func (s *Store) entferneVereinsakten() {
	dir := filepath.Join(s.dir, vereinsakteOrdner)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".zip") {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func (a *App) handleVereinsakte(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	full, n, err := a.st.schreibeVereinsakteLocked(time.Now())
	if err == nil {
		a.st.audit("Vereinsakte für die Amtsübergabe erstellt (%d Dateien)", n)
		err = a.st.saveLocked()
	}
	a.st.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	revealPathFn(full)
	fi, _ := os.Stat(full)
	groesse := int64(0)
	if fi != nil {
		groesse = fi.Size()
	}
	writeJSON(w, 200, map[string]any{"datei": filepath.Base(full), "ordner": filepath.Dir(full), "dateien": n, "groesse": groesse})
}
