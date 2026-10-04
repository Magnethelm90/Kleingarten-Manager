package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func paechterNamen(app *App) []string {
	app.st.mu.Lock()
	defer app.st.mu.Unlock()
	var n []string
	for _, p := range app.st.d.Paechter {
		n = append(n, p.Name)
	}
	return n
}

func TestSicherungWiederherstellen(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Alt", "gartengroesse": 100})
	app.st.mu.Lock()
	app.st.snapshotBackup("meine-marke")
	app.st.mu.Unlock()
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "2", "name": "Neu", "gartengroesse": 100})

	list := decode[[]sicherungInfo](t, do(h, "GET", "/api/admin/sicherungen", nil))
	var marke string
	for _, s := range list {
		if strings.Contains(s.Name, "meine-marke") {
			marke = s.Name
			if !s.Lesbar || s.Paechter != 1 || s.Art != "meine marke" {
				t.Errorf("Eintrag der Sicherung: %+v", s)
			}
		}
	}
	if marke == "" {
		t.Fatalf("Sicherung fehlt in der Liste: %+v", list)
	}

	if rec := do(h, "POST", "/api/admin/sicherungen/wiederherstellen", map[string]string{"name": marke}); rec.Code != 200 {
		t.Fatalf("wiederherstellen: %d %s", rec.Code, rec.Body)
	}
	if got := strings.Join(paechterNamen(app), ","); got != "Alt" {
		t.Errorf("nach der Wiederherstellung nur »Alt« erwartet, bekommen: %s", got)
	}
	// der Stand vor der Wiederherstellung bleibt als Sicherung erhalten (also auch rückgängig machbar)
	var vorher string
	for _, n := range app.st.sicherungsNamen() {
		if strings.Contains(n, "vor-Wiederherstellung") {
			vorher = n
		}
	}
	if vorher == "" {
		t.Fatal("Sicherung »vor-Wiederherstellung« fehlt")
	}
	raw, _ := os.ReadFile(filepath.Join(app.st.backupDir(), vorher))
	if !strings.Contains(string(raw), "Neu") {
		t.Error("die Sicherung vor der Wiederherstellung muss den vorherigen Stand enthalten")
	}
	// ... und die Wiederherstellung ist protokolliert und überlebt einen Neustart
	re, err := openStore(app.st.dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(re.d.Paechter) != 1 || re.d.Paechter[0].Name != "Alt" {
		t.Errorf("nach Neustart: %+v", re.d.Paechter)
	}
	found := false
	for _, e := range re.d.AuditLog {
		found = found || strings.Contains(e.Aktion, "Sicherung wiederhergestellt")
	}
	if !found {
		t.Error("Wiederherstellung fehlt im Änderungsprotokoll")
	}
	// und den rückgängig zu machen funktioniert
	if rec := do(h, "POST", "/api/admin/sicherungen/wiederherstellen", map[string]string{"name": vorher}); rec.Code != 200 {
		t.Fatalf("Rückgängig: %d %s", rec.Code, rec.Body)
	}
	if got := strings.Join(paechterNamen(app), ","); got != "Alt,Neu" {
		t.Errorf("nach Rückgängig: %s", got)
	}
}

func TestSicherungWiederherstellenAbwehr(t *testing.T) {
	app, h := newTestApp(t)
	do(h, "POST", "/api/admin/paechter", map[string]any{"mitgliedsnr": "1", "name": "Bleibt", "gartengroesse": 100})
	dir := app.st.backupDir()
	os.MkdirAll(dir, 0o700)
	kaputt := backupPrefix + "2000-01-01.json"
	neuer := backupPrefix + "2000-01-02.json"
	os.WriteFile(filepath.Join(dir, kaputt), []byte("{kaputt"), 0o600)
	os.WriteFile(filepath.Join(dir, neuer), []byte(`{"version": 99}`), 0o600)
	fremd := filepath.Join(t.TempDir(), "fremd.json")
	os.WriteFile(fremd, []byte(`{"version":1,"paechter":[{"id":"x","name":"Eindringling"}]}`), 0o600)

	for name, want := range map[string]int{kaputt: 400, neuer: 400, "gibtsnicht.json": 404, "../" + filepath.Base(fremd): 404, fremd: 404, "": 404,
		backupPrefix + "../../etc/passwd.json": 404} {
		if rec := do(h, "POST", "/api/admin/sicherungen/wiederherstellen", map[string]string{"name": name}); rec.Code != want {
			t.Errorf("Name %q: %d, erwartet %d", name, rec.Code, want)
		}
	}
	if got := strings.Join(paechterNamen(app), ","); got != "Bleibt" {
		t.Errorf("abgelehnte Wiederherstellung darf nichts ändern, bekommen: %s", got)
	}
	// in der Liste werden die Problemfälle gekennzeichnet
	for _, s := range decode[[]sicherungInfo](t, do(h, "GET", "/api/admin/sicherungen", nil)) {
		if (s.Name == kaputt || s.Name == neuer) && (s.Lesbar || s.Hinweis == "") {
			t.Errorf("%s muss als nicht wiederherstellbar gekennzeichnet sein: %+v", s.Name, s)
		}
	}
	// nur für den Admin
	do(h, "POST", "/api/admin/password", map[string]string{"new": "geheim123"})
	if rec := do(h, "GET", "/api/admin/sicherungen", nil); rec.Code != 401 {
		t.Errorf("Liste ohne Anmeldung: %d", rec.Code)
	}
	if rec := do(h, "POST", "/api/admin/sicherungen/wiederherstellen", map[string]string{"name": kaputt}); rec.Code != 401 {
		t.Errorf("Wiederherstellen ohne Anmeldung: %d", rec.Code)
	}
}

// Ein älteres Programm darf eine Datendatei aus einer neueren Version nicht öffnen und dabei
// unbemerkt Felder verlieren.
func TestDatendateiNeuererVersionWirdAbgelehnt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, dataFileName), []byte(`{"version": 99, "zukunft": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := openStore(dir)
	if err == nil || !strings.Contains(err.Error(), "neueren Programmversion") {
		t.Fatalf("erwartet Ablehnung wegen neuerer Version, bekommen: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, dataFileName)); !strings.Contains(string(b), "zukunft") {
		t.Error("die Datendatei darf dabei nicht überschrieben werden")
	}
}

func TestSchreibeAtomar(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "daten.json")
	if err := schreibeAtomar(p, []byte("eins"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := schreibeAtomar(p, []byte("zwei"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); string(b) != "zwei" {
		t.Errorf("Inhalt: %q", b)
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Error("die temporäre Datei darf nicht liegen bleiben")
	}
	if fi, _ := os.Stat(p); filepath.Separator == '/' && fi.Mode().Perm() != 0o600 {
		t.Errorf("Rechte: %v", fi.Mode().Perm())
	}
	// Fehlerfall: Zielordner existiert nicht, es bleibt nichts zurück und der Fehler wird gemeldet
	if err := schreibeAtomar(filepath.Join(dir, "gibtsnicht", "x.json"), []byte("x"), 0o600); err == nil {
		t.Error("Schreiben in einen fehlenden Ordner muss einen Fehler melden")
	}
}
