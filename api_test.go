package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const testPort = 8765

func newTestApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	st, err := openStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	app := &App{st: st, port: testPort, sessions: map[string]time.Time{}}
	return app, app.routes(http.NotFoundHandler())
}

func do(h http.Handler, method, path string, body any, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var rdr *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		rdr = bytes.NewReader(raw)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-GA-Request", "1")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("keine gültige Antwort (%d): %s", rec.Code, rec.Body.String())
	}
	return v
}

func TestZugriffsschutz(t *testing.T) {
	_, h := newTestApp(t)

	// richtiger Host: erlaubt
	if rec := do(h, "GET", "/api/ping", nil); rec.Code != 200 {
		t.Fatalf("ping: %d", rec.Code)
	}
	// falscher Host (DNS-Rebinding): verboten
	req := httptest.NewRequest("GET", "/api/state", nil)
	req.Host = "evil.example.com:8765"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("fremder Host: %d, erwartet 403", rec.Code)
	}
	// fremde Herkunft: verboten
	req = httptest.NewRequest("GET", "/api/state", nil)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("Origin", "https://evil.example.com")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("fremde Herkunft: %d, erwartet 403", rec.Code)
	}
	// Schreibzugriff ohne Sonderkopfzeile (typisch für Formular-Angriffe): verboten
	req = httptest.NewRequest("POST", "/api/quit", nil)
	req.Host = "127.0.0.1:8765"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST ohne Kopfzeile: %d, erwartet 403", rec.Code)
	}
	// Sicherheits-Kopfzeilen
	rec = do(h, "GET", "/api/ping", nil)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" || rec.Header().Get("Content-Security-Policy") == "" {
		t.Error("Sicherheits-Kopfzeilen fehlen")
	}
}

func TestAdminPasswort(t *testing.T) {
	_, h := newTestApp(t)
	p := map[string]any{"mitgliedsnr": "1", "name": "A", "gartengroesse": 100}

	// ohne Passwort ist der Admin-Bereich offen
	if rec := do(h, "POST", "/api/admin/paechter", p); rec.Code != 200 {
		t.Fatalf("anlegen ohne Passwort: %d %s", rec.Code, rec.Body)
	}
	// Passwort setzen
	rec := do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec.Code != 200 {
		t.Fatalf("Passwort setzen: %d %s", rec.Code, rec.Body)
	}
	// ab jetzt: ohne Anmeldung gesperrt
	for _, path := range []string{"/api/admin/paechter", "/api/admin/jahreswechsel", "/api/admin/import/apply"} {
		if rec := do(h, "POST", path, map[string]any{}); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s ohne Anmeldung: %d, erwartet 401", path, rec.Code)
		}
	}
	if rec := do(h, "PUT", "/api/admin/settings", defaultSettings()); rec.Code != http.StatusUnauthorized {
		t.Errorf("Einstellungen ohne Anmeldung: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/backup", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("Backup ohne Anmeldung: %d", rec.Code)
	}
	// falsches Passwort
	if rec := do(h, "POST", "/api/admin/login", map[string]string{"password": "falsch"}); rec.Code != http.StatusUnauthorized {
		t.Errorf("falsches Passwort: %d", rec.Code)
	}
	// richtiges Passwort
	rec = do(h, "POST", "/api/admin/login", map[string]string{"password": "geheim123"})
	if rec.Code != 200 || len(rec.Result().Cookies()) == 0 {
		t.Fatalf("Login: %d", rec.Code)
	}
	c := rec.Result().Cookies()[0]
	if !c.HttpOnly || c.SameSite != http.SameSiteStrictMode {
		t.Error("Cookie ist nicht HttpOnly/SameSite=Strict")
	}
	if rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "B", "gartengroesse": 50}, c); rec.Code != 200 {
		t.Errorf("anlegen nach Login: %d %s", rec.Code, rec.Body)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state", nil, c))
	if !st.HasPassword || !st.LoggedIn {
		t.Errorf("Zustand: hasPassword=%v loggedIn=%v", st.HasPassword, st.LoggedIn)
	}
}

func TestPasswortNichtImKlartext(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]string{"new": "ganzgeheim99"})
	raw, err := os.ReadFile(app.st.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ganzgeheim99") {
		t.Error("Passwort steht im Klartext in der Datendatei")
	}
	if !strings.Contains(string(raw), `"hash"`) {
		t.Error("Passwort-Hash fehlt")
	}
}

func TestJahreswechsel(t *testing.T) {
	app, h := newTestApp(t)
	rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "35-95", "name": "Test", "gartengroesse": 300})
	pid := decode[Paechter](t, rec).ID
	abl := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20), Abschlag: 150}
	if rec := do(h, "PUT", "/api/ablesung/"+pid+"?year=2025", abl); rec.Code != 200 {
		t.Fatalf("Ablesung: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "POST", "/api/admin/jahreswechsel", nil); rec.Code != 200 {
		t.Fatalf("Jahreswechsel: %d %s", rec.Code, rec.Body)
	}
	// Einstellungen: neues Jahr, Datum eine Jahr weiter
	if app.st.d.Settings.Jahr != 2026 || app.st.d.Settings.Rechnungsdatum != "2027-01-10" || app.st.d.Settings.Zahlungsziel != "2027-02-15" {
		t.Errorf("Einstellungen nach Wechsel: %+v", app.st.d.Settings)
	}
	// altes Jahr: eingefroren und schreibgeschützt
	old := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if !old.ReadOnly || old.Settings.Jahr != 2025 || old.Settings.Rechnungsdatum != "2026-01-10" {
		t.Errorf("altes Jahr: readOnly=%v %+v", old.ReadOnly, old.Settings.Rechnungsdatum)
	}
	if rec := do(h, "PUT", "/api/ablesung/"+pid+"?year=2025", abl); rec.Code != http.StatusBadRequest {
		t.Errorf("Schreiben ins alte Jahr: %d, erwartet 400", rec.Code)
	}
	// altes Jahr: Rechnung lässt sich weiter erzeugen (mit den alten Preisen)
	if rec := do(h, "GET", "/api/invoice/"+pid+".pdf?year=2025", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Errorf("Rechnung altes Jahr: %d", rec.Code)
	}
	// neues Jahr: Vorjahresstände übernommen, Rest leer
	cur := decode[stateResp](t, do(h, "GET", "/api/state", nil))
	a := cur.Ablesungen[pid]
	if cur.Year != 2026 || cur.ReadOnly || a.WasserVJ == nil || *a.WasserVJ != 191 || *a.StromVJ != 2302 || a.WasserAkt != nil || a.Stunden != nil || a.Abschlag != 0 {
		t.Errorf("neues Jahr: year=%d ablesung=%+v", cur.Year, a)
	}
	// Preisänderung im neuen Jahr verändert das alte Jahr nicht
	s := cur.Settings
	s.WasserPreis = 9.99
	if rec := do(h, "PUT", "/api/admin/settings", s); rec.Code != 200 {
		t.Fatalf("Einstellungen: %d %s", rec.Code, rec.Body)
	}
	old = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if old.Settings.WasserPreis != 2.04 {
		t.Errorf("altes Jahr hat neuen Preis: %v", old.Settings.WasserPreis)
	}
	// Sicherung vor dem Wechsel wurde angelegt
	entries, _ := os.ReadDir(app.st.backupDir())
	found := false
	for _, e := range entries {
		if strings.Contains(e.Name(), "vor-Jahreswechsel") {
			found = true
		}
	}
	if !found {
		t.Error("keine Sicherung vor dem Jahreswechsel")
	}
}

func TestPaechterVerwaltung(t *testing.T) {
	app, h := newTestApp(t)
	mk := func(nr, name string) *httptest.ResponseRecorder {
		return do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": nr, "name": name, "gartengroesse": 100})
	}
	if rec := mk("1", "A"); rec.Code != 200 {
		t.Fatal(rec.Body)
	}
	if rec := mk(" 1 ", "Doppelt"); rec.Code != http.StatusBadRequest {
		t.Errorf("doppelte Nummer: %d", rec.Code)
	}
	if rec := mk("", "Ohne"); rec.Code != http.StatusBadRequest {
		t.Errorf("ohne Nummer: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "3", "name": "X", "gartengroesse": -5}); rec.Code != http.StatusBadRequest {
		t.Errorf("negative Größe: %d", rec.Code)
	}
	id := decode[Paechter](t, mk("2", "B")).ID
	// Zählernummern ohne Admin ändern
	if rec := do(h, "PUT", "/api/zaehler/"+id, map[string]string{"wasserzaehlerNr": "W1", "stromzaehlerNr": "S1"}); rec.Code != 200 {
		t.Errorf("Zähler: %d", rec.Code)
	}
	if app.st.d.Paechter[1].WasserzaehlerNr != "W1" {
		t.Error("Zählernummer nicht gespeichert")
	}
	// Ablesung mit ungültigen Werten
	if rec := do(h, "PUT", "/api/ablesung/"+id+"?year=2025", map[string]any{"wasserVJ": -1}); rec.Code != http.StatusBadRequest {
		t.Errorf("negativer Zählerstand: %d", rec.Code)
	}
	// löschen
	if rec := do(h, "DELETE", "/api/admin/paechter/"+id, nil); rec.Code != 200 {
		t.Errorf("löschen: %d", rec.Code)
	}
	if len(app.st.d.Paechter) != 2 {
		t.Errorf("Papierkorb: Pächter sollte nur als gelöscht markiert werden, nicht verschwinden: %d", len(app.st.d.Paechter))
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if len(st.Paechter) != 1 {
		t.Errorf("gelöschter Pächter sollte aus der aktiven Ansicht verschwinden: %d", len(st.Paechter))
	}
	if rec := do(h, "DELETE", "/api/admin/paechter/"+id, nil); rec.Code != http.StatusNotFound {
		t.Errorf("erneut löschen: %d", rec.Code)
	}
}

func TestDatenBleibenErhalten(t *testing.T) {
	dir := t.TempDir()
	st, _ := openStore(dir)
	app := &App{st: st, port: testPort, sessions: map[string]time.Time{}}
	h := app.routes(http.NotFoundHandler())
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "7", "name": "Bleibt", "gartengroesse": 10})

	st2, err := openStore(dir) // Neustart
	if err != nil {
		t.Fatal(err)
	}
	if len(st2.d.Paechter) != 1 || st2.d.Paechter[0].Name != "Bleibt" {
		t.Errorf("nach Neustart: %+v", st2.d.Paechter)
	}
	// beschädigte Datei: verständlicher Fehler statt Datenverlust
	if err := os.WriteFile(filepath.Join(dir, dataFileName), []byte("{kaputt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := openStore(dir); err == nil || !strings.Contains(err.Error(), "beschädigt") {
		t.Errorf("beschädigte Datei: %v", err)
	}
}

func TestImportCSVDeutsch(t *testing.T) {
	csvText := "\xef\xbb\xbfMitgliedsnr.;Name;Straße;Gartengröße (m²);Wasser Stand Vorjahr;Wasser Stand aktuell;Erbrachte Stunden\r\n" +
		"35-95;Mustermann, Max;Musterstr. 15;1.234,5;148;191,5;20\r\n" +
		"40-01;Müller;Hauptstr. 1;300;10;20;\r\n" +
		"35-95;Doppelt;;1;;;\r\n" +
		";Ohne Nummer;;;;;\r\n"
	table, err := parseCSV([]byte(csvText))
	if err != nil {
		t.Fatal(err)
	}
	rows, warn, err := parseImport(table, false, defaultSettings(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(warn) != 2 {
		t.Fatalf("rows=%d warn=%v", len(rows), warn)
	}
	r := rows[0]
	if r.Paechter.Name != "Mustermann, Max" || r.Paechter.Gartengroesse != 1234.5 || r.Ablesung == nil ||
		*r.Ablesung.WasserAkt != 191.5 || *r.Ablesung.Stunden != 20 {
		t.Errorf("Zeile 1: %+v %+v", r.Paechter, r.Ablesung)
	}
	if rows[1].Ablesung == nil || rows[1].Ablesung.Stunden != nil {
		t.Errorf("leere Stunden müssen leer bleiben: %+v", rows[1].Ablesung)
	}
	// Windows-1252-Datei (ä als Byte 0xE4)
	table, err = parseCSV([]byte("Mitgliedsnr;Name\r\n1;M\xfcller\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	rows, _, err = parseImport(table, false, defaultSettings(), nil)
	if err != nil || rows[0].Paechter.Name != "Müller" {
		t.Errorf("cp1252: %v %+v", err, rows)
	}
	// ohne Pflichtspalten
	table, _ = parseCSV([]byte("Foo;Bar\r\n1;2\r\n"))
	if _, _, err := parseImport(table, false, defaultSettings(), nil); err == nil {
		t.Error("Datei ohne Kopfzeile muss abgelehnt werden")
	}
}

func TestExcelRundreise(t *testing.T) {
	s := defaultSettings()
	s.Pflichtstunden = 14
	p := Paechter{ID: "a", Mitgliedsnr: "35-95", Gartennr: "35", Anrede: "Herr", Name: "Muster <&> \"Test\"", Strasse: "Str. 1", PLZOrt: "12345 Musterstadt",
		Versand: "Emailsendung", Gartengroesse: 300, WasserzaehlerNr: "W1", StromzaehlerNr: "S1"}
	a := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20), Abschlag: 150, Hinweis: "Zeile1\nZeile2"}
	v := yearView{Jahr: 2025, Settings: s, Paechter: []Paechter{p}, Ablesungen: map[string]Ablesung{"a": a}}
	data, err := exportOverview(v)
	if err != nil {
		t.Fatal(err)
	}
	table, err := readXLSX(data, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 2 || table[1][3] != p.Name || table[1][18] != a.Hinweis {
		t.Fatalf("Rundreise: %d Zeilen, %q, %q", len(table), table[1][3], table[1][18])
	}
	// Export wieder importieren
	rows, warn, err := parseImport(table, true, s, nil)
	if err != nil || len(rows) != 1 {
		t.Fatalf("Import des Exports: %v %v", err, warn)
	}
	r := rows[0]
	if r.Paechter.Gartengroesse != 300 || r.Paechter.WasserzaehlerNr != "W1" || r.Ablesung == nil || *r.Ablesung.StromAkt != 2302 || r.Ablesung.Abschlag != 150 {
		t.Errorf("importierte Zeile: %+v %+v", r.Paechter, r.Ablesung)
	}
	// Gesamtbetrag im Export = 129,83
	if got := table[1][34]; got != "129.83" {
		t.Errorf("Gesamtbetrag im Export: %q", got)
	}
	// Datei, die keine Excel-Datei ist
	if _, err := readXLSX([]byte("kein zip"), ""); err == nil {
		t.Error("Nicht-Excel-Datei muss abgelehnt werden")
	}
}

func TestEinstellungenPruefung(t *testing.T) {
	app, h := newTestApp(t)
	s := defaultSettings()

	bad := s
	bad.Rechnungsdatum = "31.12.2025"
	if rec := do(h, "PUT", "/api/admin/settings", bad); rec.Code != http.StatusBadRequest {
		t.Errorf("ungültiges Datum: %d", rec.Code)
	}
	bad = s
	bad.WasserPreis = -1
	if rec := do(h, "PUT", "/api/admin/settings", bad); rec.Code != http.StatusBadRequest {
		t.Errorf("negativer Preis: %d", rec.Code)
	}
	bad = s
	bad.VereinName = "   "
	if rec := do(h, "PUT", "/api/admin/settings", bad); rec.Code != http.StatusBadRequest {
		t.Errorf("leerer Vereinsname: %d", rec.Code)
	}
	// Das Abrechnungsjahr lässt sich nur über den Jahreswechsel ändern
	ok := s
	ok.Jahr = 1999
	ok.WasserPreis = 2.5
	if rec := do(h, "PUT", "/api/admin/settings", ok); rec.Code != 200 {
		t.Fatalf("gültige Einstellungen: %d %s", rec.Code, rec.Body)
	}
	if app.st.d.Settings.Jahr != 2025 || app.st.d.Settings.WasserPreis != 2.5 {
		t.Errorf("Einstellungen: %+v", app.st.d.Settings)
	}
}

func TestRechnungsArchiv(t *testing.T) {
	app, h := newTestApp(t)
	mk := func(nr, name string) string {
		rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": nr, "name": name, "gartengroesse": 300})
		return decode[Paechter](t, rec).ID
	}
	p1 := mk("35-95", "Voll")
	mk("35-96", "Leer")
	abl := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20), Abschlag: 150}
	if rec := do(h, "PUT", "/api/ablesung/"+p1+"?year=2025", abl); rec.Code != 200 {
		t.Fatalf("Ablesung: %d", rec.Code)
	}
	type issueResp struct {
		Created []string    `json:"created"`
		Skipped []issueSkip `json:"skipped"`
	}
	// alle offenen ausstellen: nur der vollständige Pächter
	r := decode[issueResp](t, do(h, "POST", "/api/invoices/issue?year=2025", issueReq{}))
	if len(r.Created) != 1 || len(r.Skipped) != 1 || !strings.Contains(r.Skipped[0].Name, "Leer") {
		t.Fatalf("Ausstellen: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(app.st.dir, "Rechnungen", "2025", r.Created[0])); err != nil {
		t.Errorf("PDF-Datei fehlt: %v", err)
	}
	// zweites Mal: nichts Neues
	if r2 := decode[issueResp](t, do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})); len(r2.Created) != 0 {
		t.Errorf("doppelt ausgestellt: %+v", r2)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	inf, ok := st.Issued[p1]
	if !ok || inf.Geaendert || inf.Version != 1 || len(st.Archive) != 1 {
		t.Fatalf("Zustand nach Ausstellung: %+v archiv=%d", inf, len(st.Archive))
	}
	first := do(h, "GET", "/api/archive/"+inf.ID+".pdf", nil)
	if first.Code != 200 || !bytes.HasPrefix(first.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("Archiv-PDF: %d", first.Code)
	}
	// Preis und Name ändern: Archiv bleibt, Warnung erscheint
	s := st.Settings
	s.WasserPreis = 9.99
	do(h, "PUT", "/api/admin/settings", s)
	p := st.Paechter[0]
	p.Name = "Neuer Name"
	do(h, "PUT", "/api/admin/paechter/"+p1, p)
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if !st.Issued[p1].Geaendert {
		t.Error("Änderung nach Ausstellung wurde nicht erkannt")
	}
	if st.Archive[0].Name != "Voll" || st.Archive[0].Gesamt != inf.Gesamt {
		t.Errorf("Archiv hat sich verändert: %+v", st.Archive[0])
	}
	if again := do(h, "GET", "/api/archive/"+inf.ID+".pdf", nil); again.Code != 200 {
		t.Errorf("Archiv-PDF später: %d", again.Code)
	}
	// Ersetzen: neue Version, alte bleibt als »ersetzt«
	r3 := decode[issueResp](t, do(h, "POST", "/api/invoices/issue?year=2025", issueReq{IDs: []string{p1}, Replace: true}))
	if len(r3.Created) != 1 || !strings.Contains(r3.Created[0], "_v2") {
		t.Fatalf("Ersetzen: %+v", r3)
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if len(st.Archive) != 2 || st.Archive[0].Status != statusErsetzt || st.Archive[1].Status != statusGueltig || st.Issued[p1].Version != 2 || st.Issued[p1].Geaendert {
		t.Errorf("nach Ersetzen: %+v", st.Archive)
	}
	// gelöschter Pächter: Rechnung bleibt im Archiv abrufbar
	do(h, "DELETE", "/api/admin/paechter/"+p1, nil)
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if len(st.Archive) != 2 {
		t.Errorf("Archiv nach Löschen: %d", len(st.Archive))
	}
	if rec := do(h, "GET", "/api/archive/"+st.Archive[1].ID+".pdf", nil); rec.Code != 200 {
		t.Errorf("PDF nach Löschen: %d", rec.Code)
	}
	// Archiv überlebt Neustart
	st2, err := openStore(app.st.dir)
	if err != nil || len(st2.d.Rechnungen) != 2 {
		t.Errorf("Archiv nach Neustart: %v", err)
	}
}

func TestZahlungen(t *testing.T) {
	_, h := newTestApp(t)
	rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "35-95", "name": "Zahler", "gartengroesse": 300})
	pid := decode[Paechter](t, rec).ID
	abl := Ablesung{WasserVJ: fp(148), WasserAkt: fp(191), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20), Abschlag: 150}
	do(h, "PUT", "/api/ablesung/"+pid+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	id := st.Issued[pid].ID
	gesamt := st.Issued[pid].Gesamt
	if st.Archive[0].BezahltAm != "" || st.Archive[0].Faellig != "2026-02-15" {
		t.Fatalf("Ausgangszustand: %+v", st.Archive[0])
	}
	// Teilzahlung
	if rec := do(h, "PUT", "/api/payment/"+id, paymentReq{BezahltAm: "2026-01-20", BezahltBetrag: fp(50), Notiz: "Rate 1"}); rec.Code != 200 {
		t.Fatalf("Zahlung: %d %s", rec.Code, rec.Body)
	}
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	e := st.Archive[0]
	if e.BezahltAm != "2026-01-20" || e.BezahltBetrag == nil || *e.BezahltBetrag != 50 || e.Notiz != "Rate 1" {
		t.Errorf("Teilzahlung: %+v", e)
	}
	// volle Zahlung: Betrag wird nicht extra gespeichert
	do(h, "PUT", "/api/payment/"+id, paymentReq{BezahltAm: "2026-01-25", BezahltBetrag: fp(gesamt)})
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.Archive[0].BezahltBetrag != nil {
		t.Errorf("Vollzahlung speichert Betrag: %v", *st.Archive[0].BezahltBetrag)
	}
	// Fehleingaben
	if rec := do(h, "PUT", "/api/payment/"+id, paymentReq{BezahltAm: "gestern"}); rec.Code != 400 {
		t.Errorf("ungültiges Datum: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/payment/"+id, paymentReq{BezahltAm: "2026-01-25", BezahltBetrag: fp(-5)}); rec.Code != 400 {
		t.Errorf("negativer Betrag: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/payment/gibtesnicht", paymentReq{}); rec.Code != 404 {
		t.Errorf("unbekannte Rechnung: %d", rec.Code)
	}
	// Ersetzen übernimmt den Zahlungsvermerk; Rücknahme möglich
	do(h, "PUT", "/api/ablesung/"+pid+"?year=2025", Ablesung{WasserVJ: fp(148), WasserAkt: fp(195), StromVJ: fp(2136), StromAkt: fp(2302), Stunden: fp(20), Abschlag: 150})
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{IDs: []string{pid}, Replace: true})
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.Archive[1].BezahltAm != "2026-01-25" {
		t.Errorf("Zahlungsvermerk nicht übernommen: %+v", st.Archive[1])
	}
	do(h, "PUT", "/api/payment/"+st.Archive[1].ID, paymentReq{})
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.Archive[1].BezahltAm != "" {
		t.Error("Rücknahme der Zahlung fehlgeschlagen")
	}
	// Excel-Export
	if rec := do(h, "GET", "/api/export-payments?year=2025&nur=offen", nil); rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
		t.Errorf("Export: %d", rec.Code)
	}
}

func TestPasswortRichtlinieUndSperre(t *testing.T) {
	app, h := newTestApp(t)
	if rec := do(h, "POST", "/api/admin/password", map[string]string{"new": "kurz123"}); rec.Code != http.StatusBadRequest {
		t.Errorf("zu kurzes Passwort akzeptiert: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/password", map[string]string{"new": "langgenug1"}); rec.Code != 200 {
		t.Fatalf("Passwort setzen: %d", rec.Code)
	}
	// 40 gleichzeitige falsche Versuche: höchstens 5 dürfen überhaupt geprüft werden
	var mu sync.Mutex
	checked, locked := 0, 0
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := do(h, "POST", "/api/admin/login", map[string]string{"password": "falsch"})
			mu.Lock()
			defer mu.Unlock()
			switch rec.Code {
			case http.StatusUnauthorized:
				checked++
			case http.StatusTooManyRequests:
				locked++
			}
		}()
	}
	wg.Wait()
	if checked != 5 || locked != 35 {
		t.Errorf("Sperre umgangen: geprüft=%d gesperrt=%d (erwartet 5/35)", checked, locked)
	}
	// während der Sperre wird auch das richtige Passwort abgewiesen
	if rec := do(h, "POST", "/api/admin/login", map[string]string{"password": "langgenug1"}); rec.Code != http.StatusTooManyRequests {
		t.Errorf("richtiges Passwort während der Sperre: %d", rec.Code)
	}
	// die zweite Serie sperrt doppelt so lange
	app.lmu.Lock()
	first := time.Until(app.lockUntil)
	app.lockUntil = time.Now().Add(-time.Second)
	app.lmu.Unlock()
	for i := 0; i < 5; i++ {
		do(h, "POST", "/api/admin/login", map[string]string{"password": "falsch"})
	}
	app.lmu.Lock()
	second := time.Until(app.lockUntil)
	app.lmu.Unlock()
	if second < first+20*time.Second {
		t.Errorf("Sperrzeit steigt nicht: %v -> %v", first, second)
	}
}

func TestSicherheitsHeader(t *testing.T) {
	_, h := newTestApp(t)
	rec := do(h, "GET", "/api/ping", nil)
	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff", "X-Frame-Options": "SAMEORIGIN", "Referrer-Policy": "no-referrer",
		"Cross-Origin-Resource-Policy": "same-origin",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, erwartet %q", k, got, want)
		}
	}
	csp := rec.Header().Get("Content-Security-Policy")
	for _, must := range []string{"default-src 'self'", "base-uri 'none'", "frame-ancestors 'self'"} {
		if !strings.Contains(csp, must) {
			t.Errorf("CSP ohne %q: %s", must, csp)
		}
	}
	if strings.Contains(csp, "unsafe-eval") || strings.Contains(csp, "script-src 'unsafe-inline'") {
		t.Errorf("CSP erlaubt Skript-Injektion: %s", csp)
	}
}

func TestImportFehlerAendertNichts(t *testing.T) {
	app, h := newTestApp(t)
	rows := []ImportRow{
		{Zeile: 2, Paechter: Paechter{Mitgliedsnr: "1", Name: "Gültig"}},
		{Zeile: 3, Paechter: Paechter{Mitgliedsnr: "2", Name: ""}},
	}
	if rec := do(h, "POST", "/api/admin/import/apply", map[string]any{"rows": rows}); rec.Code != http.StatusBadRequest {
		t.Fatalf("ungültige Zeile muss abgelehnt werden: %d %s", rec.Code, rec.Body.String())
	}
	app.st.mu.Lock()
	n := len(app.st.d.Paechter)
	app.st.mu.Unlock()
	if n != 0 {
		t.Fatalf("abgelehnter Import darf nichts übernehmen, %d Pächter vorhanden", n)
	}

	rows[1].Paechter.Name = "Auch gültig"
	bad := -1.0
	rows[1].Ablesung = &Ablesung{Stunden: &bad}
	if rec := do(h, "POST", "/api/admin/import/apply", map[string]any{"rows": rows}); rec.Code != http.StatusBadRequest {
		t.Fatalf("ungültige Ablesung muss abgelehnt werden: %d %s", rec.Code, rec.Body.String())
	}
	app.st.mu.Lock()
	n = len(app.st.d.Paechter)
	app.st.mu.Unlock()
	if n != 0 {
		t.Fatalf("abgelehnter Import darf nichts übernehmen, %d Pächter vorhanden", n)
	}
}

func TestRechnungenGleichzeitigErsetzen(t *testing.T) {
	app, h := newTestApp(t)
	rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Parallel", "gartengroesse": 300})
	id := decode[Paechter](t, rec).ID
	abl := Ablesung{WasserVJ: fp(1), WasserAkt: fp(2), StromVJ: fp(1), StromAkt: fp(2), Stunden: fp(12)}
	do(h, "PUT", "/api/ablesung/"+id+"?year=2025", abl)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			do(h, "POST", "/api/invoices/issue?year=2025", issueReq{IDs: []string{id}, Replace: true})
		}()
	}
	wg.Wait()
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	versions, gueltig := map[int]bool{}, 0
	for _, r := range app.st.d.Rechnungen {
		if versions[r.Version] {
			t.Errorf("Version %d doppelt vergeben", r.Version)
		}
		versions[r.Version] = true
		if r.Status == statusGueltig {
			gueltig++
		}
	}
	if len(versions) != 5 || gueltig != 1 {
		t.Errorf("Versionen=%v gültig=%d", versions, gueltig)
	}
}

func TestSicherungenGetrenntAufgeraeumt(t *testing.T) {
	dir := t.TempDir()
	for d := 0; d < 65; d++ {
		name := fmt.Sprintf("gartenabrechnung-daten-%s.json", time.Date(2025, 1, 1+d, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
		_ = os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600)
	}
	for i := 0; i < 70; i++ {
		name := fmt.Sprintf("gartenabrechnung-daten-2025-06-01_%06d-vor-Import.json", i)
		_ = os.WriteFile(filepath.Join(dir, name), []byte("{}"), 0o600)
	}
	pruneBackups(dir, isDailyBackup, 60)
	daily, events := 0, 0
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if isDailyBackup(e.Name()) {
			daily++
		} else {
			events++
		}
	}
	if daily != 60 || events != 70 {
		t.Errorf("Tagessicherungen=%d (erwartet 60), Ereignissicherungen=%d (erwartet 70 unverändert)", daily, events)
	}
	if _, err := os.Stat(filepath.Join(dir, "gartenabrechnung-daten-2025-01-01.json")); err == nil {
		t.Error("älteste Tagessicherung hätte entfernt werden müssen")
	}
}

func TestAusgabenUndKassenbericht(t *testing.T) {
	app, h := newTestApp(t)
	// Admin-Passwort setzen, damit die Admin-Routen geprüft werden können
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	loginRec := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"})
	cookie := loginRec.Result().Cookies()[0]

	// ohne Anmeldung: verboten
	if rec := do(h, "GET", "/api/admin/kassenbericht?year=2025", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("Kassenbericht ohne Anmeldung: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "Rasenmäher", Betrag: 50}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("Ausgabe anlegen ohne Anmeldung: %d", rec.Code)
	}

	// Pächter mit Rechnung anlegen, damit die Abrechnungssummen nicht leer sind
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Zahler", "gartengroesse": 300}, cookie))
	abl := Ablesung{WasserVJ: fp(100), WasserAkt: fp(150), StromVJ: fp(1000), StromAkt: fp(1200), Stunden: fp(20)}
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", abl)
	issued := decode[struct {
		Created []string `json:"created"`
	}](t, do(h, "POST", "/api/invoices/issue?year=2025", issueReq{}))
	if len(issued.Created) != 1 {
		t.Fatalf("Rechnung nicht ausgestellt: %+v", issued)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	rechnung := st.Archive[0]
	do(h, "PUT", "/api/payment/"+rechnung.ID, map[string]any{"bezahltAm": "2025-03-15"})

	// ungültige Ausgabe: abgelehnt
	if rec := do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "", Betrag: 50}, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("leere Beschreibung muss abgelehnt werden: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "X", Betrag: 0}, cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("Betrag 0 muss abgelehnt werden: %d", rec.Code)
	}

	a1 := decode[Ausgabe](t, do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "Rasenmäher", Betrag: 50}, cookie))
	a2 := decode[Ausgabe](t, do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-04-01", Beschreibung: "Kettensäge Reparatur", Betrag: 20}, cookie))
	if a1.ID == "" || a2.ID == "" || a1.ID == a2.ID {
		t.Fatalf("Ausgaben-IDs: %+v %+v", a1, a2)
	}

	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	if len(k.Ausgaben) != 2 || k.AusgabenSumme != 70 {
		t.Fatalf("Kassenbericht Ausgaben: %+v", k)
	}
	if k.EinnahmenBezahlt != rechnung.Gesamt {
		t.Errorf("EinnahmenBezahlt: %v erwartet %v", k.EinnahmenBezahlt, rechnung.Gesamt)
	}
	if k.Saldo != round2(k.EinnahmenBezahlt-k.AusgabenSumme) {
		t.Errorf("Saldo: %v", k.Saldo)
	}
	if k.Ausgestellt != 1 || k.Paechter != 1 {
		t.Errorf("Pächter-Zählung: %+v", k)
	}

	// Ausgabe ändern
	if rec := do(h, "PUT", "/api/admin/ausgaben/"+a1.ID+"?year=2025", Ausgabe{Datum: "2025-03-02", Beschreibung: "Rasenmäher (neu)", Betrag: 60}, cookie); rec.Code != 200 {
		t.Fatalf("Ausgabe ändern: %d %s", rec.Code, rec.Body.String())
	}
	k = decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	if k.AusgabenSumme != 80 {
		t.Fatalf("Summe nach Änderung: %v", k.AusgabenSumme)
	}

	// Ausgabe löschen
	if rec := do(h, "DELETE", "/api/admin/ausgaben/"+a2.ID+"?year=2025", nil, cookie); rec.Code != 200 {
		t.Fatalf("Ausgabe löschen: %d", rec.Code)
	}
	k = decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	if len(k.Ausgaben) != 1 || k.AusgabenSumme != 60 {
		t.Fatalf("Kassenbericht nach Löschen: %+v", k)
	}

	// Versorgerwerte speichern und im Bericht wiederfinden
	vr := decode[kassenbericht](t, do(h, "PUT", "/api/admin/versorger?year=2025", Versorger{WasserM3: fp(500), StromKWh: fp(3000)}, cookie))
	if vr.Versorger.WasserM3 == nil || *vr.Versorger.WasserM3 != 500 {
		t.Fatalf("Versorger nicht gespeichert: %+v", vr.Versorger)
	}

	// Export als Excel funktioniert (nur mit Anmeldung)
	if rec := do(h, "GET", "/api/admin/export-kassenbericht?year=2025", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("Export ohne Anmeldung: %d", rec.Code)
	}
	if rec := do(h, "GET", "/api/admin/export-kassenbericht?year=2025", nil, cookie); rec.Code != 200 || rec.Body.Len() == 0 {
		t.Fatalf("Export: %d, %d Bytes", rec.Code, rec.Body.Len())
	}
	_ = app
}

func TestPlausibilitaetHinweise(t *testing.T) {
	_, h := newTestApp(t)
	mk := func(nr string) string {
		return decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": nr, "name": "P" + nr, "gartengroesse": 300})).ID
	}
	// fünf normale Pächter für einen aussagekräftigen Median
	for i := 1; i <= 5; i++ {
		id := mk(fmt.Sprintf("%d", i))
		do(h, "PUT", fmt.Sprintf("/api/ablesung/%s?year=2025", id), Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(400), Stunden: fp(12)})
	}
	// ein Ausreißer, deutlich über dem Median
	out := mk("9")
	res := decode[ablesungResp](t, do(h, "PUT", "/api/ablesung/"+out+"?year=2025", Ablesung{WasserVJ: fp(0), WasserAkt: fp(500), StromVJ: fp(0), StromAkt: fp(400), Stunden: fp(12)}))
	if len(res.Hinweise) == 0 || !strings.Contains(res.Hinweise[0], "Wasser") {
		t.Fatalf("Ausreißer ohne Vorjahr sollte auffallen: %+v", res.Hinweise)
	}

	// Jahreswechsel, dann im neuen Jahr ein Ausreißer gegenüber dem eigenen Vorjahr
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	carried := decode[stateResp](t, do(h, "GET", "/api/state?year=2026", nil)).Ablesungen[out]
	res2 := decode[ablesungResp](t, do(h, "PUT", "/api/ablesung/"+out+"?year=2026", Ablesung{WasserVJ: carried.WasserVJ, WasserAkt: fp(2000), StromVJ: carried.StromVJ, StromAkt: fp(900), Stunden: fp(12)}))
	if len(res2.Hinweise) == 0 {
		t.Fatalf("Abweichung vom eigenen Vorjahr sollte auffallen: %+v", res2.Hinweise)
	}

	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if len(st.Hinweise[out]) == 0 {
		t.Errorf("Hinweis muss auch im Zustand des Jahres stehen")
	}
}

func TestJahresverlauf(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Verlauf", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)})
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	carried := decode[stateResp](t, do(h, "GET", "/api/state?year=2026", nil)).Ablesungen[p.ID]
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2026", Ablesung{WasserVJ: carried.WasserVJ, WasserAkt: fp(55), StromVJ: carried.StromVJ, StromAkt: fp(340), Stunden: fp(12)})

	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2026", nil))
	hist := st.History[p.ID]
	if len(hist) != 2 || hist[0].Jahr != 2026 || hist[1].Jahr != 2025 {
		t.Fatalf("Verlauf: %+v", hist)
	}
	if !hist[1].Ausgestellt || hist[1].Wasser == nil || *hist[1].Wasser != 40 {
		t.Errorf("2025 sollte aus der ausgestellten Rechnung stammen: %+v", hist[1])
	}
	if hist[0].Ausgestellt || hist[0].Wasser == nil || *hist[0].Wasser != 15 {
		t.Errorf("2026 sollte aus der aktuellen Berechnung stammen: %+v", hist[0])
	}
}

func TestKassenberichtFuerAbgeschlossenesJahr(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	cookie := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"}).Result().Cookies()[0]
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	// jetzt ist 2025 abgeschlossen, 2026 aktuell
	if rec := do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-11-01", Beschreibung: "Nachzügler-Rechnung", Betrag: 30}, cookie); rec.Code != 200 {
		t.Fatalf("Ausgabe für abgeschlossenes Jahr: %d %s", rec.Code, rec.Body.String())
	}
	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	if k.Jahr != 2025 || k.AusgabenSumme != 30 {
		t.Fatalf("Kassenbericht 2025: %+v", k)
	}
	if rec := do(h, "GET", "/api/admin/export-kassenbericht?year=2025", nil, cookie); rec.Code != 200 {
		t.Fatalf("Export für abgeschlossenes Jahr: %d", rec.Code)
	}
	_ = app
}

func TestAusgabenKategorien(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	cookie := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"}).Result().Cookies()[0]

	// ohne Kategorie: Standardkategorie wird gesetzt
	a1 := decode[Ausgabe](t, do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "Rasenmäher", Betrag: 50}, cookie))
	if a1.Kategorie != kategorieStandard {
		t.Fatalf("Standardkategorie: %+v", a1)
	}
	do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-04-01", Beschreibung: "Kettensäge Reparatur", Kategorie: "Instandhaltung", Betrag: 20}, cookie)
	do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-04-15", Beschreibung: "Rasenmäher Wartung", Kategorie: "Instandhaltung", Betrag: 15}, cookie)

	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	byKat := map[string]float64{}
	for _, kat := range k.AusgabenKategorie {
		byKat[kat.Kategorie] = kat.Summe
	}
	if byKat["Instandhaltung"] != 35 || byKat[kategorieStandard] != 50 {
		t.Fatalf("Kategorien-Summen: %+v", byKat)
	}
	_ = app
}

func TestAnfangsbestandUndJahreswechsel(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	cookie := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"}).Result().Cookies()[0]

	// Anfangsbestand von Hand setzen
	k := decode[kassenbericht](t, do(h, "PUT", "/api/admin/anfangsbestand?year=2025", map[string]any{"betrag": 500}, cookie))
	if k.Anfangsbestand != 500 || k.Kassenbestand != 500 {
		t.Fatalf("Anfangsbestand: %+v", k)
	}

	// ohne Anmeldung verboten
	if rec := do(h, "PUT", "/api/admin/anfangsbestand?year=2025", map[string]any{"betrag": 999}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("Anfangsbestand ohne Anmeldung: %d", rec.Code)
	}

	// Pächter mit bezahlter Rechnung, dazu eine Ausgabe über 50 €
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Kasse", "gartengroesse": 100}, cookie))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(0), WasserAkt: fp(0), StromVJ: fp(0), StromAkt: fp(0), Stunden: fp(12)})
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	rechnung := st.Archive[0]
	do(h, "PUT", "/api/payment/"+rechnung.ID, map[string]any{"bezahltAm": "2025-05-01"})
	do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-06-01", Beschreibung: "Reparatur", Betrag: 50}, cookie)

	k = decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	wantKassenbestand := round2(500 + k.EinnahmenBezahlt - k.GuthabenAusgezahlt - 50)
	if k.Kassenbestand != wantKassenbestand {
		t.Fatalf("Kassenbestand: %v erwartet %v (einnahmen=%v guthaben=%v)", k.Kassenbestand, wantKassenbestand, k.EinnahmenBezahlt, k.GuthabenAusgezahlt)
	}

	// Jahreswechsel: der Kassenbestand von 2025 wird zum Anfangsbestand von 2026
	if rec := do(h, "POST", "/api/admin/jahreswechsel", nil, cookie); rec.Code != 200 {
		t.Fatalf("Jahreswechsel: %d %s", rec.Code, rec.Body.String())
	}
	k2026 := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2026", nil, cookie))
	if k2026.Anfangsbestand != k.Kassenbestand {
		t.Fatalf("Anfangsbestand 2026: %v erwartet %v", k2026.Anfangsbestand, k.Kassenbestand)
	}
	if k2026.Kassenbestand != k2026.Anfangsbestand {
		t.Fatalf("Kassenbestand 2026 ohne weitere Buchungen sollte gleich Anfangsbestand sein: %+v", k2026)
	}
	_ = app
}

// doMultipart schickt eine Datei als multipart/form-data an das Testprogramm.
func doMultipart(h http.Handler, path, filename string, content []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		panic(err)
	}
	if _, err := fw.Write(content); err != nil {
		panic(err)
	}
	if err := mw.Close(); err != nil {
		panic(err)
	}
	req := httptest.NewRequest("POST", path, &buf)
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-GA-Request", "1")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestBelegAnhang(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	cookie := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"}).Result().Cookies()[0]
	a1 := decode[Ausgabe](t, do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "Rasenmäher", Betrag: 50}, cookie))

	// falscher Dateityp wird abgelehnt
	if rec := doMultipart(h, "/api/admin/ausgaben/"+a1.ID+"/beleg?year=2025", "quittung.exe", []byte("xx"), cookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("falscher Dateityp: %d", rec.Code)
	}
	// ohne Anmeldung verboten
	if rec := doMultipart(h, "/api/admin/ausgaben/"+a1.ID+"/beleg?year=2025", "quittung.pdf", []byte("%PDF-1.4 Inhalt")); rec.Code != http.StatusUnauthorized {
		t.Fatalf("Upload ohne Anmeldung: %d", rec.Code)
	}

	rec := doMultipart(h, "/api/admin/ausgaben/"+a1.ID+"/beleg?year=2025", "quittung.pdf", []byte("%PDF-1.4 Inhalt"), cookie)
	if rec.Code != 200 {
		t.Fatalf("Beleg hochladen: %d %s", rec.Code, rec.Body.String())
	}
	updated := decode[Ausgabe](t, rec)
	if updated.Beleg == "" {
		t.Fatal("Beleg-Pfad fehlt nach Upload")
	}
	if _, err := os.Stat(filepath.Join(app.st.dir, updated.Beleg)); err != nil {
		t.Fatalf("Beleg-Datei fehlt auf der Platte: %v", err)
	}

	// Beleg abrufen
	if rec := do(h, "GET", "/api/admin/beleg/"+a1.ID+"?year=2025", nil, cookie); rec.Code != 200 || rec.Body.String() != "%PDF-1.4 Inhalt" {
		t.Fatalf("Beleg abrufen: %d %q", rec.Code, rec.Body.String())
	}

	// Ausgabe bearbeiten (ohne Beleg-Feld): Beleg bleibt erhalten
	do(h, "PUT", "/api/admin/ausgaben/"+a1.ID+"?year=2025", Ausgabe{Datum: "2025-03-02", Beschreibung: "Rasenmäher (neu)", Betrag: 55}, cookie)
	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	if k.Ausgaben[0].Beleg == "" {
		t.Fatalf("Beleg nach Bearbeiten verloren: %+v", k.Ausgaben[0])
	}

	// Beleg löschen
	if rec := do(h, "DELETE", "/api/admin/ausgaben/"+a1.ID+"/beleg?year=2025", nil, cookie); rec.Code != 200 {
		t.Fatalf("Beleg löschen: %d", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(app.st.dir, updated.Beleg)); err == nil {
		t.Fatal("Beleg-Datei sollte nach dem Löschen weg sein")
	}
	if rec := do(h, "GET", "/api/admin/beleg/"+a1.ID+"?year=2025", nil, cookie); rec.Code != http.StatusNotFound {
		t.Fatalf("Beleg nach Löschen sollte fehlen: %d", rec.Code)
	}

	// Ausgabe löschen: verbleibender Beleg wird mit entfernt
	rec2 := doMultipart(h, "/api/admin/ausgaben/"+a1.ID+"/beleg?year=2025", "quittung2.jpg", []byte("bild"), cookie)
	updated2 := decode[Ausgabe](t, rec2)
	do(h, "DELETE", "/api/admin/ausgaben/"+a1.ID+"?year=2025", nil, cookie)
	if _, err := os.Stat(filepath.Join(app.st.dir, updated2.Beleg)); err == nil {
		t.Fatal("Beleg-Datei sollte nach dem Löschen der Ausgabe weg sein")
	}
}

func TestZaehlerwechselAPI(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Wechsel", "gartengroesse": 300, "wasserzaehlerNr": "ALT-1"}))
	ab := Ablesung{
		WasserVJ: fp(100), WasserAkt: fp(30), StromVJ: fp(0), StromAkt: fp(0), Stunden: fp(12),
		WasserWechsel: &ZaehlerWechsel{AltEnde: fp(180), NeueNr: "NEU-2", NeuStart: fp(0)},
	}
	res := decode[ablesungResp](t, do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", ab))
	if res.WasserVerbrauch == nil || *res.WasserVerbrauch != 110 {
		t.Fatalf("Verbrauch: %+v", res.WasserVerbrauch)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	var updated Paechter
	for _, x := range st.Paechter {
		if x.ID == p.ID {
			updated = x
		}
	}
	if updated.WasserzaehlerNr != "NEU-2" {
		t.Fatalf("Zählernummer wurde nicht übernommen: %+v", updated)
	}
}

func TestKassenpruefer(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	cookie := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"}).Result().Cookies()[0]
	a1 := decode[Ausgabe](t, do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "Rasenmäher", Betrag: 50}, cookie))
	if a1.Geprueft {
		t.Fatal("neue Ausgabe sollte nicht geprüft sein")
	}

	if rec := do(h, "PUT", "/api/admin/ausgaben/"+a1.ID+"/geprueft?year=2025", map[string]any{"geprueft": true}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	updated := decode[Ausgabe](t, do(h, "PUT", "/api/admin/ausgaben/"+a1.ID+"/geprueft?year=2025", map[string]any{"geprueft": true}, cookie))
	if !updated.Geprueft || updated.GeprueftAm == "" {
		t.Fatalf("geprüft nicht gesetzt: %+v", updated)
	}

	// Bearbeiten der Ausgabe darf den Haken nicht zurücksetzen
	do(h, "PUT", "/api/admin/ausgaben/"+a1.ID+"?year=2025", Ausgabe{Datum: "2025-03-02", Beschreibung: "Rasenmäher (neu)", Betrag: 55}, cookie)
	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, cookie))
	if !k.Ausgaben[0].Geprueft {
		t.Fatalf("Haken nach Bearbeiten verloren: %+v", k.Ausgaben[0])
	}

	// Zurücknehmen
	back := decode[Ausgabe](t, do(h, "PUT", "/api/admin/ausgaben/"+a1.ID+"/geprueft?year=2025", map[string]any{"geprueft": false}, cookie))
	if back.Geprueft || back.GeprueftAm != "" {
		t.Fatalf("Haken nicht zurückgenommen: %+v", back)
	}
}

func TestKassenberichtVerlauf(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]any{"new": "geheim123"})
	cookie := do(h, "POST", "/api/admin/login", map[string]any{"password": "geheim123"}).Result().Cookies()[0]
	do(h, "PUT", "/api/admin/anfangsbestand?year=2025", map[string]any{"betrag": 200}, cookie)
	do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2025-03-01", Beschreibung: "X", Betrag: 40}, cookie)
	do(h, "POST", "/api/admin/jahreswechsel", nil, cookie)
	do(h, "POST", "/api/admin/ausgaben?year=2026", Ausgabe{Datum: "2026-03-01", Beschreibung: "Y", Betrag: 10}, cookie)

	if rec := do(h, "GET", "/api/admin/kassenbericht-verlauf", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ohne Anmeldung: %d", rec.Code)
	}
	verlauf := decode[[]kassenberichtJahr](t, do(h, "GET", "/api/admin/kassenbericht-verlauf", nil, cookie))
	if len(verlauf) != 2 || verlauf[0].Jahr != 2026 || verlauf[1].Jahr != 2025 {
		t.Fatalf("Verlauf: %+v", verlauf)
	}
	if verlauf[1].AusgabenSumme != 40 || verlauf[0].AusgabenSumme != 10 {
		t.Fatalf("Ausgabensummen: %+v", verlauf)
	}
	if verlauf[0].Anfangsbestand != verlauf[1].Kassenbestand {
		t.Fatalf("Anfangsbestand 2026 sollte Kassenbestand 2025 sein: %+v", verlauf)
	}
}

func TestPaechterNotiz(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Notiz", "gartengroesse": 300, "notiz": "Tochter kümmert sich, Tel. 0123"}))
	if p.Notiz != "Tochter kümmert sich, Tel. 0123" {
		t.Fatalf("Notiz beim Anlegen: %+v", p)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.Paechter[0].Notiz == "" {
		t.Fatal("Notiz fehlt im Zustand")
	}
	// zu lange Notiz wird gekürzt, nicht abgelehnt
	long := strings.Repeat("a", 400)
	p2 := decode[Paechter](t, do(h, "PUT", "/api/admin/paechter/"+p.ID, map[string]any{"mitgliedsnr": "1", "name": "Notiz", "gartengroesse": 300, "notiz": long}))
	if len([]rune(p2.Notiz)) != 300 {
		t.Fatalf("Notiz sollte auf 300 Zeichen gekürzt werden, hat %d", len([]rune(p2.Notiz)))
	}
}

func TestPapierkorb(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Weg", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)})
	do(h, "DELETE", "/api/admin/paechter/"+p.ID, nil)

	// Nummer ist wieder frei
	if rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Neu", "gartengroesse": 100}); rec.Code != 200 {
		t.Fatalf("Nummer sollte nach Löschen wieder frei sein: %d %s", rec.Code, rec.Body.String())
	}

	// im Papierkorb sichtbar
	korb := decode[[]Paechter](t, do(h, "GET", "/api/admin/paechter-papierkorb", nil))
	if len(korb) != 1 || korb[0].ID != p.ID {
		t.Fatalf("Papierkorb: %+v", korb)
	}
	// aber nicht mehr bearbeitbar oder in Zählerständen erreichbar
	if rec := do(h, "PUT", "/api/admin/paechter/"+p.ID, map[string]any{"mitgliedsnr": "99", "name": "Weg2", "gartengroesse": 300}); rec.Code != http.StatusNotFound {
		t.Fatalf("Bearbeiten im Papierkorb sollte scheitern: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{Stunden: fp(1)}); rec.Code != http.StatusNotFound {
		t.Fatalf("Ablesung im Papierkorb sollte scheitern: %d", rec.Code)
	}

	// Wiederherstellen scheitert, solange die Nummer vergeben ist
	if rec := do(h, "POST", "/api/admin/paechter/"+p.ID+"/wiederherstellen", nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("Wiederherstellen mit doppelter Nummer sollte scheitern: %d", rec.Code)
	}

	// endgültig löschen
	if rec := do(h, "DELETE", "/api/admin/paechter/"+p.ID+"/endgueltig", nil); rec.Code != 200 {
		t.Fatalf("endgültig löschen: %d %s", rec.Code, rec.Body.String())
	}
	korb2 := decode[[]Paechter](t, do(h, "GET", "/api/admin/paechter-papierkorb", nil))
	if len(korb2) != 0 {
		t.Fatalf("Papierkorb sollte leer sein: %+v", korb2)
	}
	if rec := do(h, "DELETE", "/api/admin/paechter/"+p.ID+"/endgueltig", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("nochmal endgültig löschen: %d", rec.Code)
	}
}

func TestPapierkorbWiederherstellen(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Weg", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)})
	do(h, "DELETE", "/api/admin/paechter/"+p.ID, nil)

	restored := decode[Paechter](t, do(h, "POST", "/api/admin/paechter/"+p.ID+"/wiederherstellen", nil))
	if restored.Geloescht || restored.GeloeschtAm != "" {
		t.Fatalf("wiederhergestellt: %+v", restored)
	}
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if len(st.Paechter) != 1 {
		t.Fatalf("Pächter sollte wieder aktiv sein: %d", len(st.Paechter))
	}
	if ab := st.Ablesungen[p.ID]; ab.WasserAkt == nil || *ab.WasserAkt != 40 {
		t.Errorf("Zählerstände sollten den Papierkorb überstanden haben: %+v", ab)
	}
	// Mitgliedsnummer ist wieder belegt
	if rec := do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Kollision", "gartengroesse": 50}); rec.Code != http.StatusBadRequest {
		t.Fatalf("Nummer sollte wieder belegt sein: %d", rec.Code)
	}
}

func TestArchivAlleJahre(t *testing.T) {
	_, h := newTestApp(t)
	p := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Verlauf", "gartengroesse": 300}))
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2025", Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)})
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	do(h, "POST", "/api/admin/jahreswechsel", nil)
	do(h, "PUT", "/api/ablesung/"+p.ID+"?year=2026", Ablesung{WasserVJ: fp(40), WasserAkt: fp(55), StromVJ: fp(300), StromAkt: fp(340), Stunden: fp(12)})
	do(h, "POST", "/api/invoices/issue?year=2026", issueReq{})

	all := decode[[]archiveEntry](t, do(h, "GET", "/api/archiv-alle", nil))
	if len(all) != 2 {
		t.Fatalf("Archiv über alle Jahre: %d Einträge, erwartet 2: %+v", len(all), all)
	}
	if all[0].Jahr != 2026 || all[1].Jahr != 2025 {
		t.Fatalf("Reihenfolge (neuestes zuerst): %+v", all)
	}
	for _, e := range all {
		if e.Mitgliedsnr != "1" || e.Name != "Verlauf" {
			t.Errorf("Eintrag: %+v", e)
		}
	}
}

func TestLetzteSicherung(t *testing.T) {
	_, h := newTestApp(t)
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.LetzteSicherung != "" {
		t.Fatalf("ganz frisch angelegt: sollte noch keine Sicherung existieren: %q", st.LetzteSicherung)
	}
	// eine zweite Speicherung legt die erste Tagessicherung an (die erste sichert das Vorherige, nicht sich selbst)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "X", "gartengroesse": 100})
	st = decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	if st.LetzteSicherung == "" {
		t.Fatal("nach der zweiten Speicherung sollte eine Tagessicherung existieren")
	}
}

func TestZweiteSicherung(t *testing.T) {
	_, h := newTestApp(t)
	secondary := t.TempDir() + "/spiegel"
	cur := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil)).Settings
	cur.ZweiteSicherung = secondary
	if rec := do(h, "PUT", "/api/admin/settings", cur); rec.Code != 200 {
		t.Fatalf("Settings mit zweitem Sicherungsordner: %d %s", rec.Code, rec.Body)
	}
	// nächste Änderung soll sowohl im Hauptordner als auch gespiegelt ankommen
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "X", "gartengroesse": 100})
	entries, err := os.ReadDir(secondary)
	if err != nil || len(entries) == 0 {
		t.Fatalf("zweiter Sicherungsordner sollte eine Datei enthalten: %v, %v", err, entries)
	}
	// ein nicht erreichbarer Pfad wird abgelehnt, statt still zu versagen: eine
	// gewöhnliche Datei kann kein Verzeichnis-Bestandteil sein
	blocker := filepath.Join(t.TempDir(), "datei.txt")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cur.ZweiteSicherung = filepath.Join(blocker, "unterordner")
	if rec := do(h, "PUT", "/api/admin/settings", cur); rec.Code != http.StatusBadRequest {
		t.Errorf("nicht beschreibbarer Sicherungsordner sollte abgelehnt werden: %d", rec.Code)
	}
}

func TestAenderungsprotokoll(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Neu", "gartengroesse": 100})
	cur := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil)).Settings
	cur.WasserPreis = 3.5
	do(h, "PUT", "/api/admin/settings", cur)

	log := decode[[]AuditEntry](t, do(h, "GET", "/api/admin/audit-log", nil))
	if len(log) < 2 {
		t.Fatalf("Protokoll sollte mindestens 2 Einträge haben: %+v", log)
	}
	found := map[string]bool{}
	for _, e := range log {
		found[e.Aktion] = true
	}
	if !found["Pächter angelegt: 1 Neu"] {
		t.Errorf("Pächter-Anlage fehlt im Protokoll: %+v", log)
	}
	hasSettings := false
	for a := range found {
		if a == "Preise und Einstellungen geändert" {
			hasSettings = true
		}
	}
	if !hasSettings {
		t.Errorf("Einstellungsänderung fehlt im Protokoll: %+v", log)
	}
}

func TestKassenpruferZugang(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]string{"new": "admingeheim1"})
	// ohne eingerichteten Prüfer-Zugang: Login schlägt fehl
	if rec := do(h, "POST", "/api/pruef/login", map[string]string{"password": "irgendwas"}); rec.Code != http.StatusNotFound {
		t.Fatalf("Login ohne eingerichteten Zugang: %d", rec.Code)
	}
	// Kassenbericht ist für niemanden ohne Anmeldung erreichbar
	if rec := do(h, "GET", "/api/admin/kassenbericht?year=2025", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("Kassenbericht ohne Anmeldung: %d", rec.Code)
	}
	// Admin richtet den Prüfer-Zugang ein (braucht dafür eine Admin-Sitzung)
	loginRec := do(h, "POST", "/api/admin/login", map[string]string{"password": "admingeheim1"})
	var adminCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "ga_session" {
			adminCookie = c
		}
	}
	if adminCookie == nil {
		t.Fatal("keine Admin-Sitzung erhalten")
	}
	if rec := do(h, "POST", "/api/admin/pruef-password", map[string]string{"new": "pruefgeheim1"}, adminCookie); rec.Code != 200 {
		t.Fatalf("Prüfer-Passwort setzen: %d %s", rec.Code, rec.Body)
	}
	// Admin selbst darf weiterhin alles (Ausgabe anlegen)
	ausg := decode[Ausgabe](t, do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2026-01-05", Beschreibung: "Rasenmäher", Betrag: 50}, adminCookie))

	// falsches Prüfer-Passwort
	if rec := do(h, "POST", "/api/pruef/login", map[string]string{"password": "falsch"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("falsches Prüfer-Passwort: %d", rec.Code)
	}
	loginRec = do(h, "POST", "/api/pruef/login", map[string]string{"password": "pruefgeheim1"})
	if loginRec.Code != 200 {
		t.Fatalf("Prüfer-Login: %d %s", loginRec.Code, loginRec.Body)
	}
	var pruefCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "ga_pruef_session" {
			pruefCookie = c
		}
	}
	if pruefCookie == nil {
		t.Fatal("keine Prüfer-Sitzung erhalten")
	}
	// Prüfer darf den Kassenbericht lesen und den Geprüft-Haken setzen
	k := decode[kassenbericht](t, do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, pruefCookie))
	if len(k.Ausgaben) != 1 {
		t.Fatalf("Kassenbericht für Prüfer: %+v", k)
	}
	if rec := do(h, "PUT", "/api/admin/ausgaben/"+ausg.ID+"/geprueft?year=2025", map[string]bool{"geprueft": true}, pruefCookie); rec.Code != 200 {
		t.Fatalf("Geprüft-Haken durch Prüfer: %d %s", rec.Code, rec.Body)
	}
	// Prüfer darf aber keine Ausgaben anlegen oder Pächter ändern (bleibt Admin-only)
	if rec := do(h, "POST", "/api/admin/ausgaben?year=2025", Ausgabe{Datum: "2026-01-05", Beschreibung: "X", Betrag: 1}, pruefCookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("Prüfer sollte keine Ausgaben anlegen dürfen: %d", rec.Code)
	}
	if rec := do(h, "PUT", "/api/admin/settings", Settings{}, pruefCookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("Prüfer sollte keine Einstellungen ändern dürfen: %d", rec.Code)
	}
	// Logout beendet den Lesezugriff wieder
	do(h, "POST", "/api/pruef/logout", nil, pruefCookie)
	if rec := do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, pruefCookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("nach Logout sollte der Zugriff verweigert werden: %d", rec.Code)
	}
}

func TestMahnung(t *testing.T) {
	_, h := newTestApp(t)
	p1 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Erster", "gartengroesse": 300}))
	p2 := decode[Paechter](t, do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Zweiter", "gartengroesse": 300}))
	abl := Ablesung{WasserVJ: fp(0), WasserAkt: fp(40), StromVJ: fp(0), StromAkt: fp(300), Stunden: fp(12)}
	do(h, "PUT", "/api/ablesung/"+p1.ID+"?year=2025", abl)
	do(h, "PUT", "/api/ablesung/"+p2.ID+"?year=2025", abl)
	do(h, "POST", "/api/invoices/issue?year=2025", issueReq{})
	st := decode[stateResp](t, do(h, "GET", "/api/state?year=2025", nil))
	id1, id2 := st.Issued[p1.ID].ID, st.Issued[p2.ID].ID

	// leere Auswahl
	if rec := do(h, "POST", "/api/admin/mahnung", map[string]any{"ids": []string{}}); rec.Code != http.StatusBadRequest {
		t.Errorf("leere Auswahl: %d", rec.Code)
	}
	// eine ausgewählte offene Rechnung: einzelnes PDF
	rec := do(h, "POST", "/api/admin/mahnung", map[string]any{"ids": []string{id1}})
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")) {
		t.Fatalf("einzelne Mahnung: %d, Anfang %q", rec.Code, rec.Body.Bytes()[:min(20, rec.Body.Len())])
	}
	// zwei ausgewählte: ZIP
	rec = do(h, "POST", "/api/admin/mahnung", map[string]any{"ids": []string{id1, id2}})
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("PK")) {
		t.Fatalf("mehrere Mahnungen (ZIP): %d", rec.Code)
	}
	// bereits vollständig bezahlt: keine Mahnung nötig
	do(h, "PUT", "/api/payment/"+id1, paymentReq{BezahltAm: "2026-01-20"})
	if rec := do(h, "POST", "/api/admin/mahnung", map[string]any{"ids": []string{id1}}); rec.Code != http.StatusBadRequest {
		t.Errorf("bezahlte Rechnung sollte abgelehnt werden: %d", rec.Code)
	}
}

func TestSessionInvalidierungBeiPasswortaenderung(t *testing.T) {
	_, h := newTestApp(t)
	do(h, "POST", "/api/admin/password", map[string]string{"new": "altespasswort1"})
	loginRec := do(h, "POST", "/api/admin/login", map[string]string{"password": "altespasswort1"})
	var oldCookie *http.Cookie
	for _, c := range loginRec.Result().Cookies() {
		if c.Name == "ga_session" {
			oldCookie = c
		}
	}
	if oldCookie == nil {
		t.Fatal("keine Admin-Sitzung erhalten")
	}
	if rec := do(h, "GET", "/api/admin/audit-log", nil, oldCookie); rec.Code != 200 {
		t.Fatalf("alte Sitzung sollte vor der Änderung noch gültig sein: %d", rec.Code)
	}
	// Passwort ändern: die alte Sitzung (von einem anderen Gerät/Browser) muss sofort ungültig werden
	if rec := do(h, "POST", "/api/admin/password", map[string]string{"old": "altespasswort1", "new": "neuespasswort1"}, oldCookie); rec.Code != 200 {
		t.Fatalf("Passwort ändern: %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, "GET", "/api/admin/audit-log", nil, oldCookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("alte Sitzung sollte nach Passwortänderung ungültig sein: %d", rec.Code)
	}

	// dasselbe für den Kassenprüfer-Zugang
	adminRec := do(h, "POST", "/api/admin/login", map[string]string{"password": "neuespasswort1"})
	var adminCookie *http.Cookie
	for _, c := range adminRec.Result().Cookies() {
		if c.Name == "ga_session" {
			adminCookie = c
		}
	}
	do(h, "POST", "/api/admin/pruef-password", map[string]string{"new": "pruefalt1234"}, adminCookie)
	pruefRec := do(h, "POST", "/api/pruef/login", map[string]string{"password": "pruefalt1234"})
	var pruefCookie *http.Cookie
	for _, c := range pruefRec.Result().Cookies() {
		if c.Name == "ga_pruef_session" {
			pruefCookie = c
		}
	}
	if pruefCookie == nil {
		t.Fatal("keine Kassenprüfer-Sitzung erhalten")
	}
	if rec := do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, pruefCookie); rec.Code != 200 {
		t.Fatalf("Prüfer-Sitzung sollte vor der Änderung gültig sein: %d", rec.Code)
	}
	do(h, "POST", "/api/admin/pruef-password", map[string]string{"new": "pruefneu1234"}, adminCookie)
	if rec := do(h, "GET", "/api/admin/kassenbericht?year=2025", nil, pruefCookie); rec.Code != http.StatusUnauthorized {
		t.Errorf("alte Kassenprüfer-Sitzung sollte nach Passwortänderung ungültig sein: %d", rec.Code)
	}
}

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"1.2", "1.1", true}, {"1.1", "1.2", false}, {"1.0", "1.0", false},
		{"1.10", "1.9", true}, {"2.0", "1.99", true}, {"1", "1.0.1", false}, {"1.0.1", "1", true},
	}
	for _, c := range cases {
		if got := versionNewer(c.a, c.b); got != c.want {
			t.Errorf("versionNewer(%q,%q) = %v, erwartet %v", c.a, c.b, got, c.want)
		}
	}
}
