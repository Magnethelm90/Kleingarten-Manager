package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"unicode/utf8"
)

// Verschlüsselte Sicherung für den USB-Stick: der Datenbestand als eine Datei, die sich nur mit dem Passwort
// öffnen lässt (AES-256-GCM, Schlüssel per PBKDF2-SHA256 aus dem Passwort). Die Datei landet im Ordner
// »Verschlüsselte Sicherungen« des Datenordners. Zum Einspielen wird sie entschlüsselt und als gewöhnliche
// Sicherung abgelegt – ansehen und wiederherstellen laufen dann über die vorhandene Liste.
//
// Aufbau der Datei: Kennung, 16 Byte Salz, 12 Byte Nonce, verschlüsselter Inhalt mit Prüfsumme. Die Kennung
// geht als Zusatzdaten in die Prüfsumme ein.

const (
	tresorOrdner      = "Verschlüsselte Sicherungen"
	tresorEndung      = ".kgsicherung"
	tresorKennung     = "KLEINGARTEN-MANAGER-SICHERUNG-1\n"
	tresorDurchlaufe  = 600_000
	tresorMinPasswort = 10
)

var errTresorPasswort = errors.New("Das Passwort stimmt nicht oder die Datei ist beschädigt")

func tresorSchluessel(passwort string, salz []byte) ([]byte, error) {
	return pbkdf2.Key(sha256.New, passwort, salz, tresorDurchlaufe, 32)
}

func tresorGCM(passwort string, salz []byte) (cipher.AEAD, error) {
	key, err := tresorSchluessel(passwort, salz)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func tresorVerschluesseln(klartext []byte, passwort string) ([]byte, error) {
	salz := make([]byte, 16)
	if _, err := rand.Read(salz); err != nil {
		return nil, err
	}
	gcm, err := tresorGCM(passwort, salz)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	out := append([]byte(tresorKennung), salz...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, klartext, []byte(tresorKennung)), nil
}

func tresorEntschluesseln(datei []byte, passwort string) ([]byte, error) {
	if !bytes.HasPrefix(datei, []byte(tresorKennung)) {
		return nil, bad("Das ist keine verschlüsselte Sicherung des " + appName)
	}
	rest := datei[len(tresorKennung):]
	if len(rest) < 16+12+16 {
		return nil, errTresorPasswort
	}
	salz, rest := rest[:16], rest[16:]
	gcm, err := tresorGCM(passwort, salz)
	if err != nil {
		return nil, err
	}
	nonce, geheim := rest[:gcm.NonceSize()], rest[gcm.NonceSize():]
	klar, err := gcm.Open(nil, nonce, geheim, []byte(tresorKennung))
	if err != nil {
		return nil, errTresorPasswort
	}
	return klar, nil
}

// handleTresorErstellen schreibt den aktuellen Stand verschlüsselt in den Ordner und öffnet ihn.
func (a *App) handleTresorErstellen(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Passwort string `json:"passwort"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	if utf8.RuneCountInString(in.Passwort) < tresorMinPasswort {
		writeErr(w, bad(fmt.Sprintf("Das Passwort braucht mindestens %d Zeichen", tresorMinPasswort)))
		return
	}
	a.st.mu.Lock()
	err := a.st.saveLocked() // aktueller Stand liegt sicher auf der Platte
	var raw []byte
	if err == nil {
		raw, err = os.ReadFile(a.st.path)
	}
	a.st.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	// das Ableiten des Schlüssels dauert absichtlich einen Moment – ohne die Datensperre zu halten
	geheim, err := tresorVerschluesseln(raw, in.Passwort)
	if err != nil {
		writeErr(w, err)
		return
	}
	dir := filepath.Join(a.st.dir, tresorOrdner)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	name := appID + "-sicherung-" + time.Now().Format("2006-01-02_150405") + tresorEndung
	full := filepath.Join(dir, name)
	if err := schreibeAtomar(full, geheim, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	a.st.audit("Verschlüsselte Sicherung erstellt")
	err = a.st.saveLocked()
	a.st.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	revealPathFn(full)
	writeJSON(w, 200, map[string]any{"datei": name, "groesse": len(geheim)})
}

// handleTresorOeffnen entschlüsselt eine hochgeladene Datei und legt sie als gewöhnliche Sicherung ab.
// Am heutigen Stand ändert das nichts; eingespielt wird erst über »Wiederherstellen« in der Liste.
func (a *App) handleTresorOeffnen(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte eine Datei auswählen"))
		return
	}
	defer file.Close()
	datei, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}
	klar, err := tresorEntschluesseln(datei, r.FormValue("passwort"))
	if errors.Is(err, errTresorPasswort) {
		writeErr(w, bad(err.Error()))
		return
	} else if err != nil {
		writeErr(w, err)
		return
	}
	d, err := parseDaten(klar)
	switch {
	case errors.Is(err, errNeuereVersion):
		writeErr(w, bad("Diese Sicherung stammt aus einer neueren Programmversion und lässt sich hier nicht einspielen"))
		return
	case err != nil:
		writeErr(w, bad("Der Inhalt der Sicherung ist beschädigt"))
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	dir := a.st.backupDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	name := fmt.Sprintf("%s%s-entschluesselt.json", backupPrefix, time.Now().Format("2006-01-02_150405"))
	if err := schreibeAtomar(filepath.Join(dir, name), klar, 0o600); err != nil {
		writeErr(w, err)
		return
	}
	a.st.audit("Verschlüsselte Sicherung geöffnet und als Sicherung abgelegt")
	a.speichernUndAntworten(w, map[string]any{"name": name, "paechter": len(d.Paechter), "rechnungen": len(d.Rechnungen)})
}

// entferneTresore löscht alle verschlüsselten Sicherungen im Datenordner: sie lassen sich nicht bereinigen,
// wenn eine Person endgültig gelöscht wird. Kopien auf dem USB-Stick muss der Verein selbst ersetzen.
func (s *Store) entferneTresore() int {
	dir := filepath.Join(s.dir, tresorOrdner)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.Type().IsRegular() && filepath.Ext(e.Name()) == tresorEndung {
			if os.Remove(filepath.Join(dir, e.Name())) == nil {
				n++
			}
		}
	}
	return n
}
