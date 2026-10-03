package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2", "10", true},
		{"10", "2", false},
		{"35-95", "35-100", true},
		{"1-2", "1-10", true},
		{"a1", "a2", true},
		{"", "1", true},
		{"1", "1", false},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v, erwartet %v", c.a, c.b, got, c.want)
		}
	}
	nr := []string{"35-100", "2", "10", "35-95", "9"}
	sort.SliceStable(nr, func(i, j int) bool { return naturalLess(nr[i], nr[j]) })
	if got := nr[0] + " " + nr[1] + " " + nr[2] + " " + nr[3] + " " + nr[4]; got != "2 9 10 35-95 35-100" {
		t.Errorf("Reihenfolge: %s", got)
	}
}

var bilderRegexp = regexp.MustCompile(`/Subtype /Image`)

type druckFixture struct {
	app    *App
	h      http.Handler
	ids    map[string]string // Mitgliedsnr -> Pächter-ID
	geoeff []string          // Pfade, die im Test »geöffnet« wurden
}

// newDruckFixture legt drei Pächter an (Nr. 10 und 2 mit Postversand, Nr. 5 per E-Mail), mit
// unterschiedlichen Beträgen, und stellt alle Rechnungen aus.
func newDruckFixture(t *testing.T) *druckFixture {
	t.Helper()
	app, h := newTestApp(t)
	f := &druckFixture{app: app, h: h, ids: map[string]string{}}
	orig := openPathFn
	openPathFn = func(p string) { f.geoeff = append(f.geoeff, p) }
	t.Cleanup(func() { openPathFn = orig })
	for i, p := range []struct{ nr, versand string }{{"10", "Postversand"}, {"5", "Emailsendung"}, {"2", "Postversand"}} {
		id := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{
			"mitgliedsnr": p.nr, "name": "Pächter " + p.nr, "versand": p.versand, "gartengroesse": 300})).ID
		f.ids[p.nr] = id
		abl := Ablesung{WasserVJ: fp(100), WasserAkt: fp(150), StromVJ: fp(2000), StromAkt: fp(2200 + float64(i)*100), Stunden: fp(20)}
		if rec := do(h, "PUT", "/api/ablesung/"+id+"?year=2025", abl); rec.Code != 200 {
			t.Fatalf("Ablesung: %d", rec.Code)
		}
	}
	res := decode[struct {
		Created []string `json:"created"`
	}](t, do(h, "POST", "/api/invoices/issue?year=2025", issueReq{}))
	if len(res.Created) != 3 {
		t.Fatalf("Rechnungen ausgestellt: %+v", res)
	}
	return f
}

func (f *druckFixture) rechnung(t *testing.T, nr string) *Rechnung {
	t.Helper()
	f.app.st.mu.Lock()
	defer f.app.st.mu.Unlock()
	for _, r := range f.app.st.d.Rechnungen {
		if r.PaechterID == f.ids[nr] {
			return r
		}
	}
	t.Fatalf("keine Rechnung für %s", nr)
	return nil
}

func TestSammelDruck(t *testing.T) {
	f := newDruckFixture(t)

	rec := do(f.h, "GET", "/api/admin/rechnungen-druck?year=2025", nil)
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatalf("Sammeldruck: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	pdf := rec.Body.Bytes()
	if n := len(seitenRegexp.FindAll(pdf, -1)); n != 2 {
		t.Errorf("nur die 2 Postsendungen erwartet, Seiten: %d", n)
	}
	// jede Seite braucht ihren eigenen QR-Code, sonst trüge sie den Betrag einer anderen Rechnung
	if n := len(bilderRegexp.FindAll(pdf, -1)); n != 2 {
		t.Errorf("jede Seite braucht ihr eigenes GiroCode-Bild, gefunden: %d", n)
	}

	all := do(f.h, "GET", "/api/admin/rechnungen-druck?year=2025&versand=alle", nil).Body.Bytes()
	if n := len(seitenRegexp.FindAll(all, -1)); n != 3 {
		t.Errorf("mit allen: 3 Seiten erwartet, %d", n)
	}
	if n := len(bilderRegexp.FindAll(all, -1)); n != 3 {
		t.Errorf("mit allen: 3 GiroCode-Bilder erwartet, %d", n)
	}

	if rec := do(f.h, "GET", "/api/admin/rechnungen-druck?year=2024", nil); rec.Code != 404 {
		t.Errorf("Jahr ohne Rechnungen: %d, erwartet 404", rec.Code)
	}
}

func TestSammelDruckNurMitAdminZugang(t *testing.T) {
	f := newDruckFixture(t)
	do(f.h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(f.h, "GET", "/api/admin/rechnungen-druck?year=2025", nil); rec.Code != 401 {
		t.Errorf("Sammeldruck ohne Anmeldung: %d, erwartet 401", rec.Code)
	}
	if rec := do(f.h, "POST", "/api/admin/rechnungen-druck/oeffnen?year=2025", nil); rec.Code != 401 {
		t.Errorf("Öffnen ohne Anmeldung: %d, erwartet 401", rec.Code)
	}
	if len(f.geoeff) != 0 {
		t.Errorf("ohne Anmeldung darf nichts geöffnet werden: %v", f.geoeff)
	}
}

func TestSammelDruckOeffnen(t *testing.T) {
	f := newDruckFixture(t)
	rec := do(f.h, "POST", "/api/admin/rechnungen-druck/oeffnen?year=2025", nil)
	if rec.Code != 200 {
		t.Fatalf("öffnen: %d %s", rec.Code, rec.Body)
	}
	want := filepath.Join(f.app.st.dir, "Rechnungen", "2025", sammelDateiName)
	if len(f.geoeff) != 1 || f.geoeff[0] != want {
		t.Fatalf("geöffnet: %v, erwartet %s", f.geoeff, want)
	}
	fi, err := os.Stat(want)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("Sammeldatei fehlt: %v", err)
	}
	if fi.Mode().Perm() != 0o600 && filepath.Separator == '/' {
		t.Errorf("Sammeldatei darf nur für den Benutzer lesbar sein, Rechte: %v", fi.Mode().Perm())
	}
}

func TestRechnungImPdfProgrammOeffnen(t *testing.T) {
	f := newDruckFixture(t)
	r := f.rechnung(t, "10")
	url := "/api/open-invoice/" + r.ID
	voll := filepath.Join(f.app.st.dir, filepath.FromSlash(r.Datei))

	if rec := do(f.h, "POST", url, nil); rec.Code != 200 {
		t.Fatalf("öffnen: %d %s", rec.Code, rec.Body)
	}
	if len(f.geoeff) != 1 || f.geoeff[0] != voll {
		t.Fatalf("geöffnet: %v, erwartet %s", f.geoeff, voll)
	}

	// Datei versehentlich gelöscht: wird aus den archivierten Werten neu erzeugt
	if err := os.Remove(voll); err != nil {
		t.Fatal(err)
	}
	if rec := do(f.h, "POST", url, nil); rec.Code != 200 {
		t.Fatalf("öffnen nach Löschen: %d %s", rec.Code, rec.Body)
	}
	if fi, err := os.Stat(voll); err != nil || fi.Size() == 0 {
		t.Errorf("Rechnungsdatei wurde nicht neu erzeugt: %v", err)
	}

	if rec := do(f.h, "POST", "/api/open-invoice/gibtsnicht", nil); rec.Code != 404 {
		t.Errorf("unbekannte Rechnung: %d, erwartet 404", rec.Code)
	}
}

// Der Pfad kommt aus den gespeicherten Daten; auch eine manipulierte Datendatei darf nicht dazu
// führen, dass beliebige Dateien geöffnet oder überschrieben werden.
func TestRechnungOeffnenLehntFremdePfadeAb(t *testing.T) {
	f := newDruckFixture(t)
	r := f.rechnung(t, "10")
	url := "/api/open-invoice/" + r.ID
	fremd := filepath.Join(t.TempDir(), "fremd.pdf")
	if err := os.WriteFile(fremd, []byte("ORIGINAL"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, datei := range []string{"", "../fremd.pdf", "Rechnungen/../../fremd.pdf", "/etc/passwd", "Rechnungen/2025/programm.exe", "Sicherungen/x.pdf", fremd} {
		f.app.st.mu.Lock()
		r.Datei = datei
		f.app.st.mu.Unlock()
		if rec := do(f.h, "POST", url, nil); rec.Code != 400 {
			t.Errorf("Datei %q: %d, erwartet 400", datei, rec.Code)
		}
	}
	if len(f.geoeff) != 0 {
		t.Errorf("es darf nichts geöffnet werden: %v", f.geoeff)
	}

	// Symlink an Stelle der Rechnungsdatei: weder folgen noch das Ziel überschreiben
	link := filepath.Join(f.app.st.dir, "Rechnungen", "2025", "link.pdf")
	if err := os.Symlink(fremd, link); err != nil {
		t.Skip("Symlinks nicht verfügbar:", err)
	}
	f.app.st.mu.Lock()
	r.Datei = "Rechnungen/2025/link.pdf"
	f.app.st.mu.Unlock()
	if rec := do(f.h, "POST", url, nil); rec.Code != 400 {
		t.Errorf("Symlink: %d, erwartet 400", rec.Code)
	}
	if b, _ := os.ReadFile(fremd); string(b) != "ORIGINAL" {
		t.Error("das Ziel des Symlinks wurde überschrieben")
	}
	if len(f.geoeff) != 0 {
		t.Errorf("Symlink darf nicht geöffnet werden: %v", f.geoeff)
	}
}

func TestBereinigenLoeschtSammelDatei(t *testing.T) {
	f := newDruckFixture(t)
	do(f.h, "POST", "/api/admin/rechnungen-druck/oeffnen?year=2025", nil)
	sammel := filepath.Join(f.app.st.dir, "Rechnungen", "2025", sammelDateiName)
	if _, err := os.Stat(sammel); err != nil {
		t.Fatal(err)
	}
	f.app.st.mu.Lock()
	for _, r := range f.app.st.d.Rechnungen {
		r.Ausgestellt = "2010-01-05T10:00:00+01:00"
	}
	f.app.st.mu.Unlock()
	if rec := do(f.h, "POST", "/api/admin/datenschutz/bereinigen", nil); rec.Code != 200 {
		t.Fatalf("bereinigen: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(sammel); !os.IsNotExist(err) {
		t.Error("die Sammel-Druckdatei mit Namen und Anschriften muss mit bereinigt werden")
	}
	if rec := do(f.h, "POST", "/api/open-invoice/"+f.rechnung(t, "10").ID, nil); rec.Code != 410 {
		t.Errorf("bereinigte Rechnung öffnen: %d, erwartet 410", rec.Code)
	}
}
