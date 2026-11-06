package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	_ "image/png" // PNG-Hintergründe lesen
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Echte Karte unter dem Lageplan: ein eigenes Bild (Luftbild-Ausschnitt, eingescannter Vereinsplan, Foto vom
// Aushang) oder ein amtliches Luftbild (Orthophoto), das die Vermessungsverwaltungen der Länder kostenlos als
// Open Data anbieten. Dazu Umrisse der Parzellen sowie Leitungen und Einrichtungen (Wasser, Strom, Schächte,
// Absperrhähne, Tore ...). Das Bild liegt als Datei im Datenordner (Ordner »Lageplan«), alles andere in der
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
	// Quelle: Quellenvermerk, der bei amtlichen Luftbildern auf jedem Ausdruck stehen muss
	Quelle string `json:"quelle,omitempty"`
	// MeterJeBildpunkt: nur bei Luftbildern bekannt, ergibt die Maßstabsleiste im Ausdruck
	MeterJeBildpunkt float64 `json:"meterJeBildpunkt,omitempty"`
	// Luftbild: womit das Bild geladen wurde, damit es sich später neu laden lässt
	Luftbild  *LuftbildAuftrag `json:"luftbild,omitempty"`
	Objekte   []LageObjekt     `json:"objekte,omitempty"`
	Leitungen []LageLinie      `json:"leitungen,omitempty"`
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

// ------------------------------------------------------------------ amtliche Luftbilder

// luftbildDienst ist ein frei nutzbarer Kartendienst (WMS) mit Orthophotos eines Landes.
type luftbildDienst struct {
	Land, URL, Layer, Quelle string
	// MinMeter: feinste sinnvolle Auflösung in Metern je Bildpunkt
	MinMeter float64
}

// luftbildDienste: geprüft im November 2026, alle kostenfrei (Datenlizenz Deutschland oder CC BY 4.0, siehe
// Quellenvermerk). Hamburg fehlt, weil dessen Dienst nicht öffentlich erreichbar war; dafür gibt es »eigener Dienst«.
var luftbildDienste = map[string]luftbildDienst{
	"bw": {"Baden-Württemberg", "https://owsproxy.lgl-bw.de/owsproxy/ows/WMS_LGL-BW_ATKIS_DOP_20_C", "IMAGES_DOP_20_RGB", "© LGL-BW (www.lgl-bw.de), dl-de/by-2-0", 0.2},
	"by": {"Bayern", "https://geoservices.bayern.de/od/wms/dop/v1/dop40", "by_dop40c", "© Bayerische Vermessungsverwaltung (www.geodaten.bayern.de), CC BY 4.0", 0.4},
	"be": {"Berlin", "https://gdi.berlin.de/services/wms/truedop_2024", "truedop_2024", "© Geoportal Berlin / TrueOrthophotos 2024, dl-de/zero-2-0", 0.2},
	"bb": {"Brandenburg", "https://isk.geobasis-bb.de/mapproxy/dop20c/service/wms", "bebb_dop20c", "© GeoBasis-DE/LGB, dl-de/by-2-0", 0.2},
	"hb": {"Bremen", "https://geodienste.bremen.de/wms_dop20_2023", "DOP20_2023_HB", "© Landesamt GeoInformation Bremen, CC BY", 0.2},
	"he": {"Hessen", "https://www.gds-srv.hessen.de/cgi-bin/lika-services/ogc-free-images.ows", "he_dop20_rgb", "© HVBG Hessen", 0.2},
	"mv": {"Mecklenburg-Vorpommern", "https://www.geodaten-mv.de/dienste/adv_dop", "mv_dop", "© GeoBasis-DE/M-V", 0.2},
	"ni": {"Niedersachsen", "https://opendata.lgln.niedersachsen.de/doorman/noauth/dop_wms", "ni_dop20", "© LGLN, CC BY 4.0", 0.2},
	"nw": {"Nordrhein-Westfalen", "https://www.wms.nrw.de/geobasis/wms_nw_dop", "nw_dop_rgb", "© Geobasis NRW, dl-de/zero-2-0", 0.2},
	"rp": {"Rheinland-Pfalz", "https://geo4.service24.rlp.de/wms/rp_dop20.fcgi", "rp_dop20", "© GeoBasis-DE/LVermGeoRP, dl-de/by-2-0", 0.2},
	"sl": {"Saarland", "https://geoportal.saarland.de/freewms/dop2023", "sl_dop20_rgb", "© GeoBasis DE/LVGL-SL", 0.2},
	"sn": {"Sachsen", "https://geodienste.sachsen.de/wms_geosn_dop-rgb/guest", "sn_dop_020", "© GeoSN, dl-de/by-2-0", 0.2},
	"st": {"Sachsen-Anhalt", "https://www.geodatenportal.sachsen-anhalt.de/wss/service/ST_LVermGeo_DOP_WMS_OpenData/guest", "lsa_lvermgeo_dop20_2", "© GeoBasis-DE/LVermGeo ST, dl-de/by-2-0", 0.2},
	"sh": {"Schleswig-Holstein", "https://dienste.gdi-sh.de/WMS_SH_DOP20col_OpenGBD", "sh_dop20_rgb", "© GeoBasis-DE/LVermGeo SH, CC BY 4.0", 0.2},
	"th": {"Thüringen", "https://www.geoproxy.geoportal-th.de/geoproxy/services/DOP", "th_dop", "© GDI-Th, CC BY 4.0", 0.2},
}

// LuftbildAuftrag: welcher Ausschnitt geladen wurde (Mitte, Breite und Höhe in Metern).
type LuftbildAuftrag struct {
	Land   string  `json:"land"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	Breite float64 `json:"breite"`
	Hoehe  float64 `json:"hoehe"`
	// nur bei Land »eigen«: Adresse und Ebene eines anderen WMS-Dienstes
	URL   string `json:"url,omitempty"`
	Layer string `json:"layer,omitempty"`
}

// Für Tests austauschbar: Abruf von Luftbild und Ortssuche.
var (
	luftbildClient = &http.Client{Timeout: 90 * time.Second, CheckRedirect: nurOeffentlicheWeiterleitung}
	ortssucheURL   = "https://nominatim.openstreetmap.org/search"
)

// nurOeffentlicheWeiterleitung folgt Weiterleitungen nur zu öffentlichen https-Adressen.
func nurOeffentlicheWeiterleitung(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("zu viele Weiterleitungen")
	}
	if _, err := eigenerDienstPruefen(req.URL.String(), "ebene"); err != nil {
		return errors.New("Weiterleitung an eine nicht erlaubte Adresse")
	}
	return nil
}

// eigenerDienstPruefen lässt nur öffentliche https-Adressen zu (kein Zugriff auf Geräte im eigenen Netz).
func eigenerDienstPruefen(raw, layer string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, bad("Bitte die https-Adresse des Kartendienstes (WMS) angeben")
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil || !strings.Contains(host, ".") || strings.HasSuffix(host, ".local") || strings.EqualFold(host, "localhost") {
		return nil, bad("Bitte die Adresse eines öffentlichen Kartendienstes angeben (keine IP-Adresse)")
	}
	if layer == "" || len(layer) > 100 || strings.ContainsAny(layer, "&?#= \t\r\n") {
		return nil, bad("Bitte den Namen der Kartenebene (Layer) angeben")
	}
	u.RawQuery, u.Fragment = "", ""
	return u, nil
}

// luftbildURL baut die GetMap-Anfrage (WMS 1.3.0, Web-Mercator EPSG:3857) und liefert Breite/Höhe in Bildpunkten
// sowie die Auflösung in Metern je Bildpunkt.
func luftbildURL(dienst luftbildDienst, a LuftbildAuftrag) (string, int, int, float64) {
	const r = 6378137.0
	x := r * a.Lon * math.Pi / 180
	y := r * math.Log(math.Tan(math.Pi/4+a.Lat*math.Pi/360))
	// Web-Mercator streckt Entfernungen um 1/cos(Breite)
	k := 1 / math.Cos(a.Lat*math.Pi/180)
	mpp := math.Max(dienst.MinMeter, math.Max(a.Breite, a.Hoehe)/2400)
	w := int(math.Round(a.Breite / mpp))
	h := int(math.Round(a.Hoehe / mpp))
	hb, hh := a.Breite/2*k, a.Hoehe/2*k
	q := url.Values{}
	q.Set("SERVICE", "WMS")
	q.Set("VERSION", "1.3.0")
	q.Set("REQUEST", "GetMap")
	q.Set("LAYERS", dienst.Layer)
	q.Set("STYLES", "")
	q.Set("CRS", "EPSG:3857")
	q.Set("BBOX", fmt.Sprintf("%.2f,%.2f,%.2f,%.2f", x-hb, y-hh, x+hb, y+hh))
	q.Set("WIDTH", strconv.Itoa(w))
	q.Set("HEIGHT", strconv.Itoa(h))
	q.Set("FORMAT", "image/jpeg")
	trenner := "?"
	if strings.Contains(dienst.URL, "?") {
		trenner = "&"
	}
	return dienst.URL + trenner + q.Encode(), w, h, mpp
}

// handleLuftbild lädt den gewünschten Ausschnitt einmalig herunter und speichert ihn als Hintergrund.
func (a *App) handleLuftbild(w http.ResponseWriter, r *http.Request) {
	var in LuftbildAuftrag
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if in.Lat < 47 || in.Lat > 55.2 || in.Lon < 5.5 || in.Lon > 15.5 {
		writeErr(w, bad("Der Ort liegt nicht in Deutschland. Bitte zuerst den Ort der Anlage suchen"))
		return
	}
	if in.Breite < 50 || in.Breite > 2000 || in.Hoehe < 50 || in.Hoehe > 2000 {
		writeErr(w, bad("Der Ausschnitt muss zwischen 50 und 2000 Metern breit und hoch sein"))
		return
	}
	var dienst luftbildDienst
	if in.Land == "eigen" {
		u, err := eigenerDienstPruefen(in.URL, strings.TrimSpace(in.Layer))
		if err != nil {
			writeErr(w, err)
			return
		}
		in.URL, in.Layer = u.String(), strings.TrimSpace(in.Layer)
		dienst = luftbildDienst{Land: "eigener Dienst", URL: in.URL, Layer: in.Layer, Quelle: "Quelle: " + u.Hostname(), MinMeter: 0.1}
	} else {
		d, ok := luftbildDienste[in.Land]
		if !ok {
			writeErr(w, bad("Bitte das Bundesland wählen"))
			return
		}
		dienst, in.URL, in.Layer = d, "", ""
	}
	adresse, _, _, mpp := luftbildURL(dienst, in)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, adresse, nil)
	if err != nil {
		writeErr(w, bad("Der Kartendienst lässt sich nicht ansprechen"))
		return
	}
	req.Header.Set("User-Agent", appName+"/"+appVersion)
	resp, err := luftbildClient.Do(req)
	if err != nil {
		writeErr(w, bad("Keine Verbindung zum Kartendienst. Ist der Rechner mit dem Internet verbunden?"))
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxKarteUpload+1))
	if err != nil || resp.StatusCode != http.StatusOK || len(raw) > maxKarteUpload {
		writeErr(w, bad(fmt.Sprintf("Der Kartendienst von %s hat kein Bild geliefert. Bitte später noch einmal versuchen", dienst.Land)))
		return
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	var img image.Image
	if err == nil && cfg.Width*cfg.Height <= maxKartePixel {
		img, _, err = image.Decode(bytes.NewReader(raw))
	}
	if err != nil || img == nil {
		writeErr(w, bad(fmt.Sprintf("Der Kartendienst von %s hat kein Bild geliefert. Liegt der Ort wirklich in %s?", dienst.Land, dienst.Land)))
		return
	}
	if bildEinfarbig(img) {
		writeErr(w, bad(fmt.Sprintf("Für diesen Ort hat %s kein Luftbild. Stimmt das Bundesland?", dienst.Land)))
		return
	}
	quelle := dienst.Quelle
	if in.Land == "mv" || in.Land == "rp" {
		quelle += " " + strconv.Itoa(time.Now().Year())
	}
	a.karteSpeichern(w, img, quelle, mpp, &in, "Luftbild für den Lageplan geladen ("+dienst.Land+")")
}

// bildEinfarbig erkennt leere Antworten (weiße oder schwarze Fläche außerhalb des Landesgebiets).
func bildEinfarbig(img image.Image) bool {
	b := img.Bounds()
	if b.Dx() < 2 || b.Dy() < 2 {
		return true
	}
	var n, sum, sum2 float64
	for y := b.Min.Y; y < b.Max.Y; y += max(1, b.Dy()/60) {
		for x := b.Min.X; x < b.Max.X; x += max(1, b.Dx()/60) {
			r, g, bl, _ := img.At(x, y).RGBA()
			v := float64(r+g+bl) / 3 / 257
			n++
			sum += v
			sum2 += v * v
		}
	}
	mittel := sum / n
	return math.Sqrt(math.Max(0, sum2/n-mittel*mittel)) < 3
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
	a.karteSpeichern(w, img, "", 0, nil, "Hintergrundbild für den Lageplan hochgeladen")
}

// karteSpeichern verkleinert das Bild, speichert es neu als JPEG (ohne eingebettete Zusatzdaten) und setzt es als
// Hintergrund. Das vorige Bild wird gelöscht. Größe und Deckkraft bleiben, wenn schon ein Bild da war, sonst wird
// das Bild so groß gezeigt, dass es etwa die Fläche der vorhandenen Anordnung einnimmt.
func (a *App) karteSpeichern(w http.ResponseWriter, img image.Image, quelle string, mpp float64, auftrag *LuftbildAuftrag, aktion string) {
	img, faktor := verkleinern(img, karteMaxKante)
	if mpp > 0 {
		mpp *= faktor
	}
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
	k.Quelle, k.MeterJeBildpunkt, k.Luftbild = quelle, round4(mpp), auftrag
	a.st.audit("%s", aktion)
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
	k.Bild, k.BildBreite, k.BildHoehe, k.Quelle, k.MeterJeBildpunkt, k.Luftbild = "", 0, 0, "", 0, nil
	a.st.audit("Hintergrundbild des Lageplans entfernt")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	a.st.karteDateiLoeschen(alt)
	writeJSON(w, 200, k)
}

// ------------------------------------------------------------------ Ortssuche

type ortTreffer struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
	Land string  `json:"land,omitempty"` // Kürzel aus luftbildDienste, wenn erkannt
}

// koordinatenAusText erkennt »52.52, 13.40« (aus Google Maps per Rechtsklick kopiert) und Kartenlinks mit
// »@52.52,13.40« oder »mlat=52.52&mlon=13.40«.
func koordinatenAusText(s string) (float64, float64, bool) {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "mlat="); i >= 0 {
		rest := s[i+5:]
		lat, _, _ := strings.Cut(rest, "&")
		_, lon, ok := strings.Cut(s, "mlon=")
		lon, _, _ = strings.Cut(lon, "&")
		if ok {
			s = lat + "," + lon
		}
	}
	teile := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' || r == ';' || r == '/' })
	if len(teile) < 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(teile[0], 64)
	lon, err2 := strconv.ParseFloat(teile[1], 64)
	if err1 != nil || err2 != nil || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return 0, 0, false
	}
	return lat, lon, true
}

// handleOrtssuche sucht eine Anschrift über OpenStreetMap (Nominatim) – nur auf Knopfdruck, eine Anfrage je Suche.
func (a *App) handleOrtssuche(w http.ResponseWriter, r *http.Request) {
	q := trim(r.URL.Query().Get("q"), 200)
	if q == "" {
		writeErr(w, bad("Bitte eine Anschrift oder Koordinaten eingeben"))
		return
	}
	if lat, lon, ok := koordinatenAusText(q); ok {
		writeJSON(w, 200, []ortTreffer{{Name: fmt.Sprintf("Koordinaten %.5f, %.5f", lat, lon), Lat: lat, Lon: lon}})
		return
	}
	p := url.Values{}
	p.Set("format", "jsonv2")
	p.Set("q", q)
	p.Set("countrycodes", "de")
	p.Set("addressdetails", "1")
	p.Set("limit", "5")
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, ortssucheURL+"?"+p.Encode(), nil)
	if err != nil {
		writeErr(w, err)
		return
	}
	req.Header.Set("User-Agent", appName+"/"+appVersion+" (+"+repoURLBase+")")
	req.Header.Set("Accept-Language", "de")
	resp, err := luftbildClient.Do(req)
	if err != nil {
		writeErr(w, bad("Keine Verbindung zur Ortssuche. Ist der Rechner mit dem Internet verbunden?"))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeErr(w, bad("Die Ortssuche antwortet gerade nicht. Du kannst stattdessen Koordinaten eingeben, z. B. 52.52, 13.40"))
		return
	}
	var roh []struct {
		Name    string `json:"display_name"`
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
		Address struct {
			ISO string `json:"ISO3166-2-lvl4"`
		} `json:"address"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&roh); err != nil {
		writeErr(w, bad("Die Antwort der Ortssuche ließ sich nicht lesen"))
		return
	}
	out := []ortTreffer{}
	for _, t := range roh {
		lat, err1 := strconv.ParseFloat(t.Lat, 64)
		lon, err2 := strconv.ParseFloat(t.Lon, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		land := strings.ToLower(strings.TrimPrefix(t.Address.ISO, "DE-"))
		if _, ok := luftbildDienste[land]; !ok {
			land = ""
		}
		out = append(out, ortTreffer{Name: trim(t.Name, 200), Lat: lat, Lon: lon, Land: land})
	}
	writeJSON(w, 200, out)
}

// handleLuftbildDienste: Liste der Länder für die Auswahl im Programm.
func (a *App) handleLuftbildDienste(w http.ResponseWriter, r *http.Request) {
	type eintrag struct {
		Kurz   string `json:"kurz"`
		Land   string `json:"land"`
		Quelle string `json:"quelle"`
	}
	out := []eintrag{}
	for k, d := range luftbildDienste {
		out = append(out, eintrag{k, d.Land, d.Quelle})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Land < out[j].Land })
	writeJSON(w, 200, out)
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
