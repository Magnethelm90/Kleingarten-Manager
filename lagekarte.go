package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // PNG-Hintergründe lesen
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Karte unter dem Lageplan: ein eigenes Bild (eingescannter Vereinsplan, Foto vom Aushang), dazu Umrisse der
// Parzellen sowie Leitungen und Einrichtungen (Wasser, Strom, Schächte, Absperrhähne, Tore ...). Das Bild liegt als Datei im Datenordner (Ordner »Lageplan«), alles andere in der
// Datendatei. Personenbezogene Daten gibt es hier nicht.

const (
	karteOrdner       = "Lageplan"
	karteDateiPraefix = "karte-"
	karteDateiTyp     = ".jpg"
	maxKarteUpload    = 25 << 20
	maxKartePixel     = 60_000_000 // größere Bilder würden beim Einlesen zu viel Speicher brauchen
	karteMaxKante     = 3000       // gespeichert wird höchstens so groß, das reicht für den Druck auf A4/A3
	maxObjekte        = 500
	maxLeitungen      = 200
	maxLinienPunkte   = 200
	maxUmrissPunkte   = 80
	maxKartenKoord    = 20000
)

// Punkt ist ein Punkt auf der Karte in Bildpunkten des Lageplans.
type Punkt struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// LageKarte: Hintergrund, Leitungen und Einrichtungen des Lageplans.
type LageKarte struct {
	// Bild: Dateiname im Ordner »Lageplan« (leer = kein Hintergrund); BildBreite/BildHoehe in Bildpunkten
	Bild       string `json:"bild,omitempty"`
	BildBreite int    `json:"bildBreite,omitempty"`
	BildHoehe  int    `json:"bildHoehe,omitempty"`
	// Massstab: wie groß das Bild auf der Karte erscheint (Kartenpunkte je Bildpunkt); Deckkraft 0,1 bis 1
	Massstab  float64 `json:"massstab,omitempty"`
	Deckkraft float64 `json:"deckkraft,omitempty"`
	// Quelle: Quellenvermerk, steht klein auf Plan und Ausdruck (z. B. »Plan: Vorstand 2024«)
	Quelle    string       `json:"quelle,omitempty"`
	Objekte   []LageObjekt `json:"objekte,omitempty"`
	Leitungen []LageLinie  `json:"leitungen,omitempty"`
}

// LageObjekt ist eine Einrichtung auf dem Gelände (Wasserzähler, Absperrhahn, Tor ...).
type LageObjekt struct {
	Art string `json:"art"`
	X   int    `json:"x"`
	Y   int    `json:"y"`
	Bez string `json:"bez,omitempty"`
}

// LageLinie ist eine Leitung (Wasser, Strom, Abwasser) oder ein Weg als Linienzug.
type LageLinie struct {
	Art    string  `json:"art"`
	Punkte []Punkt `json:"punkte"`
	Bez    string  `json:"bez,omitempty"`
}

// objektArten: Kürzel, Name und Farbe der Einrichtungen (Kürzel steht im Kreis auf der Karte und im Ausdruck).
var objektArten = map[string]struct {
	Kurz, Name string
	R, G, B    int
}{
	"wasserzaehler": {"W", "Wasserzähler", 29, 110, 196},
	"absperr":       {"A", "Absperrhahn", 14, 70, 140},
	"schacht":       {"S", "Schacht", 110, 110, 110},
	"strom":         {"E", "Stromverteiler", 214, 140, 0},
	"tor":           {"T", "Tor / Eingang", 125, 80, 40},
	"vereinsheim":   {"V", "Vereinsheim", 47, 111, 62},
	"kompost":       {"K", "Kompost / Container", 95, 120, 40},
	"parkplatz":     {"P", "Parkplatz", 60, 60, 160},
	"sonstiges":     {"•", "Sonstiges", 150, 60, 120},
}

var linienArten = map[string]struct {
	Name     string
	R, G, B  int
	Strichel bool
}{
	"wasser":   {"Wasserleitung", 29, 110, 196, false},
	"strom":    {"Stromleitung", 214, 140, 0, false},
	"abwasser": {"Abwasser", 120, 80, 40, true},
	"weg":      {"Weg", 150, 150, 150, true},
}

// handleKarteHochladen übernimmt ein eigenes Bild als Hintergrund.
func (a *App) handleKarteHochladen(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxKarteUpload+1<<16)
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte ein Bild wählen (PNG oder JPG, höchstens 25 MB)"))
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxKarteUpload+1))
	if err != nil || len(raw) > maxKarteUpload {
		writeErr(w, bad("Das Bild ist zu groß (höchstens 25 MB)"))
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") {
		writeErr(w, bad("Bitte ein Bild im Format PNG oder JPG wählen"))
		return
	}
	if cfg.Width < 100 || cfg.Height < 100 {
		writeErr(w, bad("Das Bild ist zu klein (mindestens 100 × 100 Bildpunkte)"))
		return
	}
	if cfg.Width*cfg.Height > maxKartePixel || cfg.Width > 20000 || cfg.Height > 20000 {
		writeErr(w, bad("Das Bild hat zu viele Bildpunkte. Bitte vorher verkleinern"))
		return
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		writeErr(w, bad("Das Bild lässt sich nicht lesen"))
		return
	}
	a.karteSpeichern(w, img)
}

// karteSpeichern verkleinert das Bild, speichert es neu als JPEG (ohne eingebettete Zusatzdaten) und setzt es als
// Hintergrund. Das vorige Bild wird gelöscht. Größe und Deckkraft bleiben, wenn schon ein Bild da war, sonst wird
// das Bild so groß gezeigt, dass es etwa die Fläche der vorhandenen Anordnung einnimmt.
func (a *App) karteSpeichern(w http.ResponseWriter, img image.Image) {
	img, _ = verkleinern(img, karteMaxKante)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		writeErr(w, err)
		return
	}
	sum := sha256.Sum256(buf.Bytes())
	name := karteDateiPraefix + hex.EncodeToString(sum[:])[:12] + karteDateiTyp
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	dir := filepath.Join(a.st.dir, karteOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	full := filepath.Join(dir, name)
	if fi, err := os.Lstat(full); err == nil && !fi.Mode().IsRegular() {
		writeErr(w, bad("Im Ordner »Lageplan« liegt an dieser Stelle keine gewöhnliche Datei"))
		return
	}
	if err := schreibeAtomar(full, buf.Bytes(), 0o600); err != nil {
		writeErr(w, err)
		return
	}
	k := &a.st.d.LageKarte
	alt := k.Bild
	b := img.Bounds()
	if alt == "" || k.Massstab <= 0 {
		k.Massstab = startMassstab(a.st.d.Lageplan, b.Dx())
	}
	if k.Deckkraft <= 0 {
		k.Deckkraft = 0.85
	}
	k.Bild, k.BildBreite, k.BildHoehe = name, b.Dx(), b.Dy()
	a.st.audit("Hintergrundbild für den Lageplan hochgeladen")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	if alt != "" && alt != name {
		a.st.karteDateiLoeschen(alt)
	}
	writeJSON(w, 200, k)
}

func round4(x float64) float64 { return math.Round(x*10000) / 10000 }

// startMassstab: ohne Anordnung 1600 Kartenpunkte breit, sonst so breit wie die angeordneten Gärten plus Rand.
func startMassstab(pos map[string]LagePos, bildBreite int) float64 {
	ziel := 1600.0
	if len(pos) > 0 {
		maxX := 0
		for _, p := range pos {
			maxX = max(maxX, p.X+lpBreite)
		}
		ziel = math.Max(800, float64(maxX)+200)
	}
	return math.Round(ziel/float64(bildBreite)*1000) / 1000
}

// verkleinern skaliert ein Bild so, dass die längere Kante höchstens maxKante Bildpunkte hat (Mittelwert über die
// zusammengefassten Bildpunkte, damit nichts flimmert). Liefert auch den Faktor (alte je neue Bildpunkte).
func verkleinern(src image.Image, maxKante int) (image.Image, float64) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	faktor := math.Max(float64(w), float64(h)) / float64(maxKante)
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
	if faktor <= 1 {
		return rgba, 1
	}
	nw, nh := max(1, int(float64(w)/faktor)), max(1, int(float64(h)/faktor))
	out := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		y0, y1 := y*h/nh, max(y*h/nh+1, (y+1)*h/nh)
		for x := 0; x < nw; x++ {
			x0, x1 := x*w/nw, max(x*w/nw+1, (x+1)*w/nw)
			var r, g, bl, n int
			for yy := y0; yy < y1; yy++ {
				i := rgba.PixOffset(x0, yy)
				for xx := x0; xx < x1; xx++ {
					r += int(rgba.Pix[i])
					g += int(rgba.Pix[i+1])
					bl += int(rgba.Pix[i+2])
					i += 4
					n++
				}
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255})
		}
	}
	return out, float64(w) / float64(nw)
}

func karteNameGueltig(name string) bool {
	rest, ok := strings.CutPrefix(name, karteDateiPraefix)
	if !ok {
		return false
	}
	rest, ok = strings.CutSuffix(rest, karteDateiTyp)
	if !ok || len(rest) != 12 {
		return false
	}
	_, err := hex.DecodeString(rest)
	return err == nil
}

// karteDaten liest das gespeicherte Hintergrundbild. Der Aufrufer hält s.mu nicht zwingend (nur Lesen der Datei).
func (s *Store) karteDaten(name string) ([]byte, bool) {
	if s.dir == "" || !karteNameGueltig(name) {
		return nil, false
	}
	full := filepath.Join(s.dir, karteOrdner, name)
	if fi, err := os.Lstat(full); err != nil || !fi.Mode().IsRegular() {
		return nil, false
	}
	raw, err := os.ReadFile(full)
	return raw, err == nil
}

func (s *Store) karteDateiLoeschen(name string) {
	if s.dir == "" || !karteNameGueltig(name) {
		return
	}
	_ = os.Remove(filepath.Join(s.dir, karteOrdner, name))
}

// handleKarteBild liefert das Hintergrundbild für die Karte im Programm.
func (a *App) handleKarteBild(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	name := a.st.d.LageKarte.Bild
	a.st.mu.Unlock()
	raw, ok := a.st.karteDaten(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(raw)
}

func (a *App) handleKarteEntfernen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	k := &a.st.d.LageKarte
	alt := k.Bild
	k.Bild, k.BildBreite, k.BildHoehe, k.Quelle = "", 0, 0, ""
	a.st.audit("Hintergrundbild des Lageplans entfernt")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.karteDateiLoeschen(alt)
	writeJSON(w, 200, k)
}

// ------------------------------------------------------------------ Prüfen beim Speichern

func koordOK(x, y int) bool { return x >= 0 && y >= 0 && x <= maxKartenKoord && y <= maxKartenKoord }

// bereinigeKarte prüft Leitungen und Einrichtungen aus dem Programm.
func bereinigeKarte(objekte []LageObjekt, leitungen []LageLinie) ([]LageObjekt, []LageLinie, error) {
	if len(objekte) > maxObjekte || len(leitungen) > maxLeitungen {
		return nil, nil, bad("Zu viele Einträge im Lageplan")
	}
	var obj []LageObjekt
	for _, o := range objekte {
		if _, ok := objektArten[o.Art]; !ok || !koordOK(o.X, o.Y) {
			return nil, nil, bad("Ungültige Einrichtung im Lageplan")
		}
		o.Bez = trim(o.Bez, 60)
		obj = append(obj, o)
	}
	var lin []LageLinie
	for _, l := range leitungen {
		if _, ok := linienArten[l.Art]; !ok || len(l.Punkte) < 2 || len(l.Punkte) > maxLinienPunkte {
			return nil, nil, bad("Ungültige Leitung im Lageplan")
		}
		for _, p := range l.Punkte {
			if !koordOK(p.X, p.Y) {
				return nil, nil, bad("Ungültige Leitung im Lageplan")
			}
		}
		l.Bez = trim(l.Bez, 60)
		lin = append(lin, l)
	}
	return obj, lin, nil
}

// bereinigeUmriss prüft den Umriss einer Parzelle und setzt die Position auf die linke obere Ecke des Umrisses.
func bereinigeUmriss(p LagePos) (LagePos, error) {
	if len(p.Umriss) == 0 {
		p.Umriss = nil
		return p, nil
	}
	if len(p.Umriss) < 3 || len(p.Umriss) > maxUmrissPunkte {
		return p, bad("Ein Umriss braucht mindestens 3 Punkte")
	}
	minX, minY := maxKartenKoord, maxKartenKoord
	for _, q := range p.Umriss {
		if !koordOK(q.X, q.Y) {
			return p, bad("Ungültiger Umriss im Lageplan")
		}
		minX, minY = min(minX, q.X), min(minY, q.Y)
	}
	p.X, p.Y = minX, minY
	return p, nil
}

// umrissMitte: Schwerpunkt der Fläche (für die Beschriftung), bei entarteten Umrissen der Mittelwert der Ecken.
func umrissMitte(u []Punkt) (float64, float64) {
	var a, cx, cy float64
	for i := range u {
		j := (i + 1) % len(u)
		k := float64(u[i].X*u[j].Y - u[j].X*u[i].Y)
		a += k
		cx += float64(u[i].X+u[j].X) * k
		cy += float64(u[i].Y+u[j].Y) * k
	}
	if math.Abs(a) < 1 {
		var sx, sy float64
		for _, p := range u {
			sx += float64(p.X)
			sy += float64(p.Y)
		}
		return sx / float64(len(u)), sy / float64(len(u))
	}
	return cx / (3 * a), cy / (3 * a)
}
