package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // Logos dürfen als JPEG hochgeladen werden
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// ---------------------------------------------------------------- Vereinsfarbe

const standardFarbe = "#2f6f3e"

func farbeGueltig(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	_, err := strconv.ParseUint(s[1:], 16, 32)
	return err == nil
}

func farbeRGB(s string) (r, g, b float64) {
	if !farbeGueltig(s) {
		s = standardFarbe
	}
	v, _ := strconv.ParseUint(s[1:], 16, 32)
	return float64(v >> 16 & 0xff), float64(v >> 8 & 0xff), float64(v & 0xff)
}

// relHelligkeit ist die relative Leuchtdichte nach WCAG.
func relHelligkeit(r, g, b float64) float64 {
	lin := func(c float64) float64 {
		c /= 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// markenfarbe liefert die Vereinsfarbe so abgedunkelt, dass weiße Schrift darauf und die Farbe als
// Schrift auf Weiß gut lesbar bleiben (Kontrast mindestens 4,5:1). Eine zu helle Wahl (z. B. Gelb)
// wird also automatisch kräftiger, statt die Rechnung unleserlich zu machen.
func markenfarbe(s string) (int, int, int) {
	r, g, b := farbeRGB(s)
	for i := 0; i < 40 && (1.05)/(relHelligkeit(r, g, b)+0.05) < 4.5; i++ {
		r, g, b = r*0.92, g*0.92, b*0.92
	}
	return int(math.Round(r)), int(math.Round(g)), int(math.Round(b))
}

// aufgehellt mischt eine Farbe mit Weiß (anteil 0 = Farbe, 1 = Weiß), z. B. für hinterlegte Flächen.
func aufgehellt(r, g, b int, anteil float64) (int, int, int) {
	m := func(c int) int { return int(math.Round(float64(c) + (255-float64(c))*anteil)) }
	return m(r), m(g), m(b)
}

// ---------------------------------------------------------------- Vereinslogo

const (
	logoOrdner     = "Logo"
	maxLogoBytes   = 3 << 20
	maxLogoPixel   = 2500
	minLogoPixel   = 16
	logoPraefix    = "logo-"
	logoDateiTyp   = ".png"
	logoSchluessel = 12 // Länge des Prüfsummen-Teils im Dateinamen
)

// logoQuelle liefert die Bilddaten eines gespeicherten Logos für die PDF-Erzeugung. Wird beim Öffnen des
// Datenordners gesetzt; ohne Datenordner (z. B. in Tests) gibt es kein Logo.
var (
	logoMu     sync.RWMutex
	logoQuelle func(name string) ([]byte, bool)
)

func setzeLogoQuelle(dir string) {
	logoMu.Lock()
	defer logoMu.Unlock()
	logoQuelle = func(name string) ([]byte, bool) {
		if !logoNameGueltig(name) {
			return nil, false
		}
		full := filepath.Join(dir, logoOrdner, name)
		if fi, err := os.Lstat(full); err != nil || !fi.Mode().IsRegular() {
			return nil, false
		}
		raw, err := os.ReadFile(full)
		return raw, err == nil
	}
}

func logoDaten(name string) ([]byte, bool) {
	logoMu.RLock()
	q := logoQuelle
	logoMu.RUnlock()
	if q == nil || name == "" {
		return nil, false
	}
	return q(name)
}

// logoNameGueltig erlaubt nur die selbst vergebenen Namen (logo-<hex>.png), nie einen Pfad.
func logoNameGueltig(name string) bool {
	hexTeil, ok := strings.CutPrefix(name, logoPraefix)
	if !ok {
		return false
	}
	hexTeil, ok = strings.CutSuffix(hexTeil, logoDateiTyp)
	if !ok || len(hexTeil) != logoSchluessel {
		return false
	}
	_, err := hex.DecodeString(hexTeil)
	return err == nil
}

// pruefeLogo liest ein hochgeladenes Bild (PNG oder JPEG), prüft Größe und Abmessungen und speichert es
// neu als PNG. Das Neuspeichern entfernt eingebettete Zusatzdaten und stellt sicher, dass nur ein echtes
// Bild abgelegt wird. Gibt die PNG-Daten zurück.
func pruefeLogo(raw []byte) ([]byte, error) {
	if len(raw) > maxLogoBytes {
		return nil, errors.New("Das Bild ist zu groß (höchstens 3 MB)")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || (format != "png" && format != "jpeg") {
		return nil, errors.New("Bitte ein Bild im Format PNG oder JPG wählen")
	}
	if cfg.Width < minLogoPixel || cfg.Height < minLogoPixel {
		return nil, errors.New("Das Bild ist zu klein")
	}
	if cfg.Width > maxLogoPixel || cfg.Height > maxLogoPixel {
		return nil, fmt.Errorf("Das Bild ist zu groß (höchstens %d × %d Bildpunkte). Bitte vorher verkleinern", maxLogoPixel, maxLogoPixel)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("Das Bild lässt sich nicht lesen")
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (a *App) handleLogoHochladen(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLogoBytes+1<<16)
	f, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte eine Bilddatei wählen (höchstens 3 MB)"))
		return
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxLogoBytes+1))
	if err != nil {
		writeErr(w, bad("Die Datei ließ sich nicht lesen"))
		return
	}
	pngDaten, err := pruefeLogo(raw)
	if err != nil {
		writeErr(w, bad(err.Error()))
		return
	}
	sum := sha256.Sum256(pngDaten)
	name := logoPraefix + hex.EncodeToString(sum[:])[:logoSchluessel] + logoDateiTyp
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	dir := filepath.Join(a.st.dir, logoOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	full := filepath.Join(dir, name)
	if fi, err := os.Lstat(full); err == nil && !fi.Mode().IsRegular() {
		writeErr(w, bad("Im Logo-Ordner liegt an dieser Stelle keine gewöhnliche Datei"))
		return
	}
	if err := schreibeAtomar(full, pngDaten, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	a.st.d.Settings.Logo = name
	a.st.audit("Vereinslogo geändert")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]string{"logo": name})
}

// handleLogoEntfernen nimmt das Logo aus den aktuellen Einstellungen. Die Datei bleibt liegen, weil
// abgeschlossene Jahre und ausgestellte Rechnungen sie weiter verwenden.
func (a *App) handleLogoEntfernen(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.d.Settings.Logo = ""
	a.st.audit("Vereinslogo entfernt")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// handleLogo liefert das aktuelle Logo (für Kopfzeile und Vorschau im Programm).
func (a *App) handleLogo(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	name := a.st.d.Settings.Logo
	a.st.mu.Unlock()
	raw, ok := logoDaten(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(raw)
}
