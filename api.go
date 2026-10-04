package main

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// appVersion wird beim Release-Build über -ldflags "-X main.appVersion=..." gesetzt.
var appVersion = "1.0"

// appAutor erscheint in der Fußzeile der Oberfläche und im Konsolenfenster.
const appAutor = "Derek"

// appName ist der Programmname (früher "Gartenabrechnung", seit das Programm
// mehr als reine Abrechnung kann: Lageplan, Kassenbericht, Kassenprüfer-Zugang
// usw.). appID ist die interne, versionsunabhängige Kennung für das
// Selbst-Erkennen beim Start (alreadyRunning) und die Update-Prüfung.
const (
	appName     = "Kleingarten-Manager"
	appID       = "kleingarten-manager"
	repoOwner   = "Magnethelm90"
	repoName    = "Kleingarten-Manager"
	repoURLBase = "https://github.com/" + repoOwner + "/" + repoName + "/"
)

type App struct {
	st   *Store
	port int

	smu      sync.Mutex
	sessions map[string]time.Time

	// Sitzungen des Kassenprüfer-Zugangs (eigenes, optionales Passwort, nur Lesezugriff).
	pmu       sync.Mutex
	psessions map[string]time.Time

	// imu sorgt dafür, dass Rechnungen nacheinander ausgestellt werden
	// (sonst könnten zwei Anfragen dieselbe Versionsnummer vergeben).
	imu sync.Mutex

	lmu       sync.Mutex
	failures  int
	strikes   uint
	lockUntil time.Time
}

// ---------------------------------------------------------------- Helfer

type apiError struct {
	Code int
	Msg  string
}

func (e apiError) Error() string { return e.Msg }

func bad(msg string) error      { return apiError{http.StatusBadRequest, msg} }
func notFound(msg string) error { return apiError{http.StatusNotFound, msg} }

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	var ae apiError
	if errors.As(err, &ae) {
		writeJSON(w, ae.Code, map[string]string{"error": ae.Msg})
		return
	}
	fmt.Fprintln(os.Stderr, "Interner Fehler:", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Interner Fehler. Einzelheiten stehen im Konsolenfenster des Programms."})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return bad("Ungültige Eingabe")
	}
	return nil
}

func (a *App) yearParam(r *http.Request) int {
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil {
		return y
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	return a.st.d.Settings.Jahr
}

// ---------------------------------------------------------------- Schutz

func (a *App) allowedHosts() map[string]bool {
	p := strconv.Itoa(a.port)
	return map[string]bool{"127.0.0.1:" + p: true, "localhost:" + p: true, "[::1]:" + p: true}
}

// guard schützt die lokale Oberfläche vor fremden Webseiten (DNS-Rebinding, CSRF).
func (a *App) guard(next http.Handler) http.Handler {
	hosts := a.allowedHosts()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hosts[r.Host] {
			http.Error(w, "Zugriff nur über 127.0.0.1", http.StatusForbidden)
			return
		}
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if err != nil || !hosts[u.Host] {
				http.Error(w, "Fremde Herkunft nicht erlaubt", http.StatusForbidden)
				return
			}
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if r.Header.Get("X-GA-Request") != "1" {
				http.Error(w, "Anfrage abgelehnt", http.StatusForbidden)
				return
			}
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		if !strings.HasSuffix(r.URL.Path, ".pdf") {
			h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-src 'self' blob:; object-src 'self'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- Admin-Anmeldung

func hashPassword(pw string, salt []byte, iter int) string {
	k, err := pbkdf2.Key(sha256.New, pw, salt, iter, 32)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(k)
}

func (a *App) hasPassword() bool {
	return a.st.d.Admin.Hash != ""
}

func (a *App) sessionValid(r *http.Request) bool {
	c, err := r.Cookie("ga_session")
	if err != nil {
		return false
	}
	a.smu.Lock()
	defer a.smu.Unlock()
	exp, ok := a.sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(a.sessions, c.Value)
		return false
	}
	a.sessions[c.Value] = time.Now().Add(4 * time.Hour)
	return true
}

// ---------------------------------------------------------------- Kassenprüfer-Anmeldung
//
// Eigener, optionaler Zugang mit Lesezugriff auf den Kassenbericht (inkl. des
// Geprüft-Hakens), aber ohne die übrigen Admin-Rechte. Ist kein Kassenprüfer-
// Passwort eingerichtet (Standard), gibt es diesen Zugang schlicht nicht.

func (a *App) hasPruefPassword() bool { return a.st.d.PruefAuth.Hash != "" }

func (a *App) pruefSessionValid(r *http.Request) bool {
	c, err := r.Cookie("ga_pruef_session")
	if err != nil {
		return false
	}
	a.pmu.Lock()
	defer a.pmu.Unlock()
	if a.psessions == nil {
		a.psessions = map[string]time.Time{}
	}
	exp, ok := a.psessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(a.psessions, c.Value)
		return false
	}
	a.psessions[c.Value] = time.Now().Add(4 * time.Hour)
	return true
}

func (a *App) isPruef(r *http.Request) bool {
	a.st.mu.Lock()
	has := a.hasPruefPassword()
	a.st.mu.Unlock()
	return has && a.pruefSessionValid(r)
}

func (a *App) startPruefSession(w http.ResponseWriter) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	a.pmu.Lock()
	if a.psessions == nil {
		a.psessions = map[string]time.Time{}
	}
	a.psessions[tok] = time.Now().Add(4 * time.Hour)
	a.pmu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "ga_pruef_session", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

// adminOrPruef lässt sowohl den Admin als auch den (falls eingerichteten)
// Kassenprüfer-Zugang zu, z. B. für den lesenden Zugriff auf den Kassenbericht.
func (a *App) adminOrPruef(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.isAdmin(r) || a.isPruef(r) {
			h(w, r)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte anmelden"})
	}
}

func (a *App) handlePruefLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	has := a.hasPruefPassword()
	a.lmu.Lock()
	if time.Now().Before(a.lockUntil) {
		wait := time.Until(a.lockUntil).Round(time.Second)
		a.lmu.Unlock()
		a.st.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": fmt.Sprintf("Zu viele Fehlversuche. Bitte %s warten.", wait)})
		return
	}
	a.lmu.Unlock()
	if !has {
		a.st.mu.Unlock()
		writeErr(w, notFound("Der Kassenprüfer-Zugang ist nicht eingerichtet"))
		return
	}
	ok := checkPasswordAgainst(a.st.d.PruefAuth, in.Password)
	a.st.mu.Unlock()
	if !ok {
		a.lmu.Lock()
		a.failures++
		if a.failures >= 5 {
			d := 30 * time.Second << a.strikes
			if d > 15*time.Minute {
				d = 15 * time.Minute
			}
			if a.strikes < 10 {
				a.strikes++
			}
			a.lockUntil = time.Now().Add(d)
			a.failures = 0
		}
		a.lmu.Unlock()
		time.Sleep(700 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Falsches Passwort"})
		return
	}
	a.lmu.Lock()
	a.failures, a.strikes = 0, 0
	a.lmu.Unlock()
	a.startPruefSession(w)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) handlePruefLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("ga_pruef_session"); err == nil {
		a.pmu.Lock()
		delete(a.psessions, c.Value)
		a.pmu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "ga_pruef_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// handlePruefPassword setzt, ändert oder entfernt das Kassenprüfer-Passwort.
// Das verwaltet ausschließlich der Admin, ein bisheriges Prüfer-Passwort wird
// dafür nicht benötigt.
func (a *App) handlePruefPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		New string `json:"new"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if in.New == "" {
		a.st.d.PruefAuth = AdminAuth{}
		a.st.audit("Kassenprüfer-Passwort entfernt")
	} else {
		if len([]rune(in.New)) < 8 {
			writeErr(w, bad("Das Passwort muss mindestens 8 Zeichen haben"))
			return
		}
		if len(in.New) > 200 {
			writeErr(w, bad("Das Passwort ist zu lang"))
			return
		}
		salt := make([]byte, 16)
		_, _ = rand.Read(salt)
		const iter = 600000
		a.st.d.PruefAuth = AdminAuth{Salt: hex.EncodeToString(salt), Iter: iter, Hash: hashPassword(in.New, salt, iter)}
		a.st.audit("Kassenprüfer-Passwort gesetzt oder geändert")
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	// Bestehende Kassenprüfer-Sitzungen sofort ungültig machen, nicht erst nach
	// Ablauf der 4 Stunden (z. B. wenn der Zugang einer ausgeschiedenen Person
	// entzogen werden soll).
	a.pmu.Lock()
	a.psessions = map[string]time.Time{}
	a.pmu.Unlock()
	writeJSON(w, 200, map[string]bool{"ok": true, "hasPruefPassword": in.New != ""})
}

func (a *App) isAdmin(r *http.Request) bool {
	a.st.mu.Lock()
	has := a.hasPassword()
	a.st.mu.Unlock()
	if !has {
		return true // kein Passwort gesetzt: Admin-Bereich ist offen
	}
	return a.sessionValid(r)
}

func (a *App) admin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !a.isAdmin(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte im Admin-Bereich anmelden"})
			return
		}
		h(w, r)
	}
}

func checkPasswordAgainst(auth AdminAuth, pw string) bool {
	salt, err := hex.DecodeString(auth.Salt)
	if err != nil || auth.Hash == "" {
		return false
	}
	got := hashPassword(pw, salt, auth.Iter)
	return subtle.ConstantTimeCompare([]byte(got), []byte(auth.Hash)) == 1
}

func (a *App) checkPassword(pw string) bool { return checkPasswordAgainst(a.st.d.Admin, pw) }

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Password string `json:"password"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	// Sperre und Prüfung laufen unter einer gemeinsamen Sperre. So kann niemand mit vielen
	// gleichzeitigen Anfragen die Fehlversuchs-Sperre umgehen.
	a.st.mu.Lock()
	has := a.hasPassword()
	a.lmu.Lock()
	if time.Now().Before(a.lockUntil) {
		wait := time.Until(a.lockUntil).Round(time.Second)
		a.lmu.Unlock()
		a.st.mu.Unlock()
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": fmt.Sprintf("Zu viele Fehlversuche. Bitte %s warten.", wait)})
		return
	}
	a.lmu.Unlock()
	ok := has && a.checkPassword(in.Password)
	if has && !ok {
		a.lmu.Lock()
		a.failures++
		if a.failures >= 5 {
			// Sperrzeit verdoppelt sich mit jeder Serie: 30 s, 1 min, 2 min ... höchstens 15 min
			d := 30 * time.Second << a.strikes
			if d > 15*time.Minute {
				d = 15 * time.Minute
			}
			if a.strikes < 10 {
				a.strikes++
			}
			a.lockUntil = time.Now().Add(d)
			a.failures = 0
		}
		a.lmu.Unlock()
		a.st.mu.Unlock()
		time.Sleep(700 * time.Millisecond)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Falsches Passwort"})
		return
	}
	a.st.mu.Unlock()
	a.lmu.Lock()
	a.failures, a.strikes = 0, 0
	a.lmu.Unlock()
	a.startSession(w)
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) startSession(w http.ResponseWriter) {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	tok := hex.EncodeToString(b)
	a.smu.Lock()
	a.sessions[tok] = time.Now().Add(4 * time.Hour)
	a.smu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "ga_session", Value: tok, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("ga_session"); err == nil {
		a.smu.Lock()
		delete(a.sessions, c.Value)
		a.smu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "ga_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]bool{"ok": true})
}

// handlePassword setzt, ändert oder entfernt das Admin-Passwort.
func (a *App) handlePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if a.hasPassword() {
		if !a.sessionValid(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Bitte im Admin-Bereich anmelden"})
			return
		}
		if !a.checkPassword(in.Old) {
			time.Sleep(700 * time.Millisecond)
			writeErr(w, bad("Das bisherige Passwort stimmt nicht"))
			return
		}
	}
	if in.New == "" {
		a.st.d.Admin = AdminAuth{}
	} else {
		if len([]rune(in.New)) < 8 {
			writeErr(w, bad("Das Passwort muss mindestens 8 Zeichen haben"))
			return
		}
		if len(in.New) > 200 {
			writeErr(w, bad("Das Passwort ist zu lang"))
			return
		}
		salt := make([]byte, 16)
		_, _ = rand.Read(salt)
		const iter = 600000
		a.st.d.Admin = AdminAuth{Salt: hex.EncodeToString(salt), Iter: iter, Hash: hashPassword(in.New, salt, iter)}
	}
	if in.New == "" {
		a.st.audit("Admin-Passwort entfernt")
	} else {
		a.st.audit("Admin-Passwort geändert")
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	// Alle bestehenden Admin-Sitzungen (auch auf anderen Geräten) werden mit der
	// Passwortänderung sofort ungültig, nicht erst nach Ablauf der 4 Stunden.
	a.smu.Lock()
	a.sessions = map[string]time.Time{}
	a.smu.Unlock()
	if in.New != "" {
		a.startSession(w)
	}
	writeJSON(w, 200, map[string]bool{"ok": true, "hasPassword": in.New != ""})
}

// ---------------------------------------------------------------- Zustand

type stateResp struct {
	Autor       string              `json:"autor"`
	Version     string              `json:"version"`
	DataDir     string              `json:"dataDir"`
	CurrentYear int                 `json:"currentYear"`
	Years       []int               `json:"years"`
	Year        int                 `json:"year"`
	ReadOnly    bool                `json:"readOnly"`
	Settings    Settings            `json:"settings"`
	Paechter    []Paechter          `json:"paechter"`
	Ablesungen  map[string]Ablesung `json:"ablesungen"`
	Results     map[string]Result   `json:"results"`
	HasPassword bool                `json:"hasPassword"`
	LoggedIn    bool                `json:"loggedIn"`
	// Kassenprüfer-Zugang: eigenes, optionales Passwort mit Lesezugriff auf den
	// Kassenbericht, getrennt vom Admin-Passwort.
	HasPruefPassword bool                   `json:"hasPruefPassword"`
	PruefLoggedIn    bool                   `json:"pruefLoggedIn"`
	Issued           map[string]issuedInfo  `json:"issued"`
	Archive          []archiveEntry         `json:"archive"`
	History          map[string][]histEntry `json:"history"`
	Hinweise         map[string][]string    `json:"hinweise"`
	// LetzteSicherung: JJJJ-MM-TT der jüngsten Sicherungsdatei, leer wenn keine existiert.
	LetzteSicherung string `json:"letzteSicherung,omitempty"`
	// SicherungFehler: Hinweistext, falls die jüngste Sicherung beschädigt ist (leer = alles gut).
	SicherungFehler string `json:"sicherungFehler,omitempty"`
	TutorialGesehen bool   `json:"tutorialGesehen"`
	// OffeneVorjahre: noch nicht (voll) bezahlte, gültige Rechnungen aus allen anderen Jahren als dem
	// angezeigten, damit sie nach einem Jahreswechsel nicht aus dem Blick geraten.
	OffeneVorjahre []archiveEntry `json:"offeneVorjahre"`
}

func (a *App) handleState(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	has := a.hasPassword()
	hasPruef := a.hasPruefPassword()
	a.st.mu.Unlock()
	loggedIn := has && a.sessionValid(r)
	pruefLoggedIn := hasPruef && a.pruefSessionValid(r)

	a.st.mu.Lock()
	v, ok := a.st.viewLocked(year)
	if !ok {
		a.st.mu.Unlock()
		writeErr(w, notFound("Dieses Jahr gibt es nicht"))
		return
	}
	res := stateResp{
		Autor: appAutor, Version: appVersion, DataDir: a.st.dir, CurrentYear: a.st.d.Settings.Jahr, Years: a.st.years(),
		Year: year, ReadOnly: v.ReadOnly, Settings: v.Settings, Paechter: v.Paechter,
		Ablesungen: v.Ablesungen, Results: map[string]Result{}, HasPassword: has, LoggedIn: loggedIn,
		HasPruefPassword: hasPruef, PruefLoggedIn: pruefLoggedIn,
	}
	if res.Paechter == nil {
		res.Paechter = []Paechter{}
	}
	for _, p := range v.Paechter {
		res.Results[p.ID] = calculate(v.Settings, p, v.Ablesungen[p.ID])
	}
	res.Issued, res.Archive = a.st.archiveViewLocked(v)
	res.History = a.st.historyLocked()
	res.Hinweise = plausiLocked(v, res.Results, res.History)
	if t := a.st.letzteSicherung(); !t.IsZero() {
		res.LetzteSicherung = t.Format("2006-01-02")
	}
	res.SicherungFehler = a.st.pruefeSicherung()
	res.TutorialGesehen = a.st.d.TutorialGesehen
	res.OffeneVorjahre = a.st.offeneAndererJahreLocked(year)
	// JSON innerhalb der Sperre erzeugen, weil die Maps geteilt sind
	raw, err := json.Marshal(res)
	a.st.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(raw)
}

// ---------------------------------------------------------------- Ablesungen

func cleanAblesung(in Ablesung) (Ablesung, error) {
	for _, p := range []*float64{in.WasserVJ, in.WasserAkt, in.StromVJ, in.StromAkt, in.Stunden} {
		if !validNumPtr(p) {
			return in, bad("Zählerstände und Stunden müssen Zahlen ab 0 sein")
		}
	}
	for _, x := range []float64{in.Versicherung, in.Grundsteuer, in.Auslagen, in.Abschlag} {
		if !validNum(x) {
			return in, bad("Beträge müssen Zahlen ab 0 sein")
		}
	}
	in.Hinweis = trim(in.Hinweis, 300)
	var err error
	if in.WasserWechsel, err = cleanZaehlerWechsel(in.WasserWechsel); err != nil {
		return in, err
	}
	if in.StromWechsel, err = cleanZaehlerWechsel(in.StromWechsel); err != nil {
		return in, err
	}
	return in, nil
}

func cleanZaehlerWechsel(w *ZaehlerWechsel) (*ZaehlerWechsel, error) {
	if w == nil {
		return nil, nil
	}
	if !validNumPtr(w.AltEnde) || !validNumPtr(w.NeuStart) {
		return nil, bad("Die Zählerstände beim Zählerwechsel müssen Zahlen ab 0 sein")
	}
	if w.AltEnde == nil && w.NeuStart == nil && strings.TrimSpace(w.NeueNr) == "" {
		return nil, nil // leer eingegeben: kein Wechsel
	}
	w.NeueNr = trim(w.NeueNr, 40)
	return w, nil
}

func (a *App) handleAblesung(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	year := a.yearParam(r)
	var in Ablesung
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanAblesung(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if year != a.st.d.Settings.Jahr {
		writeErr(w, bad("Abgeschlossene Jahre können nicht mehr geändert werden"))
		return
	}
	var pa *Paechter
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == id && !a.st.d.Paechter[i].Geloescht {
			pa = &a.st.d.Paechter[i]
		}
	}
	if pa == nil {
		writeErr(w, notFound("Pächter nicht gefunden"))
		return
	}
	// Bei einem Zählerwechsel wandert die neue Nummer automatisch in die Stammdaten.
	if in.WasserWechsel != nil && in.WasserWechsel.NeueNr != "" {
		pa.WasserzaehlerNr = in.WasserWechsel.NeueNr
	}
	if in.StromWechsel != nil && in.StromWechsel.NeueNr != "" {
		pa.StromzaehlerNr = in.StromWechsel.NeueNr
	}
	j := a.st.ensureYear(year)
	j.Ablesungen[id] = in
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, a.st.ablesungResultLocked(year, id))
}

// ablesungResp ist das Ergebnis einer Eingabe: die Berechnung und die
// Plausibilitätshinweise zum Verbrauch.
type ablesungResp struct {
	Result
	Hinweise []string `json:"hinweise"`
}

func (s *Store) ablesungResultLocked(year int, id string) ablesungResp {
	v, _ := s.viewLocked(year)
	results := map[string]Result{}
	for _, p := range v.Paechter {
		results[p.ID] = calculate(v.Settings, p, v.Ablesungen[p.ID])
	}
	h := plausiLocked(v, results, s.historyLocked())[id]
	if h == nil {
		h = []string{}
	}
	return ablesungResp{Result: results[id], Hinweise: h}
}

// handleZaehler ändert nur die Zählernummern eines Pächters (auch für den Vorstand ohne Admin-Rechte).
func (a *App) handleZaehler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in struct {
		WasserzaehlerNr string `json:"wasserzaehlerNr"`
		StromzaehlerNr  string `json:"stromzaehlerNr"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == id && !a.st.d.Paechter[i].Geloescht {
			a.st.d.Paechter[i].WasserzaehlerNr = trim(in.WasserzaehlerNr, 40)
			a.st.d.Paechter[i].StromzaehlerNr = trim(in.StromzaehlerNr, 40)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, a.st.d.Paechter[i])
			return
		}
	}
	writeErr(w, notFound("Pächter nicht gefunden"))
}

// ---------------------------------------------------------------- Pächter (Admin)

func cleanPaechter(p Paechter) (Paechter, error) {
	p.Mitgliedsnr = trim(p.Mitgliedsnr, 20)
	p.Gartennr = trim(p.Gartennr, 20)
	p.Anrede = trim(p.Anrede, 30)
	p.Name = trim(p.Name, 100)
	p.Strasse = trim(p.Strasse, 100)
	p.PLZOrt = trim(p.PLZOrt, 100)
	p.Versand = trim(p.Versand, 30)
	p.WasserzaehlerNr = trim(p.WasserzaehlerNr, 40)
	p.StromzaehlerNr = trim(p.StromzaehlerNr, 40)
	p.Notiz = trim(p.Notiz, 300)
	if p.Mitgliedsnr == "" {
		return p, bad("Bitte eine Mitgliedsnummer eintragen")
	}
	if p.Name == "" {
		return p, bad("Bitte einen Namen eintragen")
	}
	if !validNum(p.Gartengroesse) {
		return p, bad("Die Gartengröße muss eine Zahl ab 0 sein")
	}
	if p.UmlageAbweichend != nil && !validNum(*p.UmlageAbweichend) {
		return p, bad("Die Umlage muss eine Zahl ab 0 sein")
	}
	return p, nil
}

// nrTaken prüft die Mitgliedsnummer nur unter den aktiven Pächtern: die
// Nummer eines Pächters im Papierkorb ist wieder frei.
func (s *Store) nrTaken(nr, exceptID string) bool {
	for _, q := range s.d.Paechter {
		if !q.Geloescht && q.ID != exceptID && strings.EqualFold(strings.TrimSpace(q.Mitgliedsnr), nr) {
			return true
		}
	}
	return false
}

func (a *App) handlePaechterCreate(w http.ResponseWriter, r *http.Request) {
	var in Paechter
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanPaechter(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if a.st.nrTaken(in.Mitgliedsnr, "") {
		writeErr(w, bad("Diese Mitgliedsnummer gibt es schon"))
		return
	}
	in.ID = newID()
	a.st.d.Paechter = append(a.st.d.Paechter, in)
	a.st.ensureYear(a.st.d.Settings.Jahr).Ablesungen[in.ID] = Ablesung{}
	a.st.gartenOeffnen(in.ID, in.Gartennr, in.Mitgliedsnr, in.Name)
	a.st.audit("Pächter angelegt: %s %s", in.Mitgliedsnr, in.Name)
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

func (a *App) handlePaechterUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in Paechter
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	in, err := cleanPaechter(in)
	if err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if a.st.nrTaken(in.Mitgliedsnr, id) {
		writeErr(w, bad("Diese Mitgliedsnummer gibt es schon"))
		return
	}
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == id && !a.st.d.Paechter[i].Geloescht {
			alt := a.st.d.Paechter[i]
			in.ID = id
			a.st.d.Paechter[i] = in
			a.st.gartenWechsel(id, alt.Gartennr, in.Gartennr, in.Mitgliedsnr, in.Name)
			if alt.Gartengroesse != in.Gartengroesse {
				a.st.audit("Gartengröße geändert: Garten %s (%s %s): %s m² → %s m²",
					in.Gartennr, in.Mitgliedsnr, in.Name, fmtFlex(alt.Gartengroesse), fmtFlex(in.Gartengroesse))
			}
			a.st.audit("Pächter geändert: %s %s", in.Mitgliedsnr, in.Name)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, in)
			return
		}
	}
	writeErr(w, notFound("Pächter nicht gefunden"))
}

// handlePaechterDelete legt einen Pächter in den Papierkorb: er verschwindet
// aus allen aktiven Ansichten, Zählerstände und Rechnungen bleiben aber
// erhalten und die Löschung lässt sich rückgängig machen.
func (a *App) handlePaechterDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == id && !a.st.d.Paechter[i].Geloescht {
			a.st.snapshotBackup("vor-Loeschen")
			a.st.d.Paechter[i].Geloescht = true
			a.st.d.Paechter[i].GeloeschtAm = time.Now().Format("2006-01-02")
			a.st.gartenSchliessen(id, a.st.d.Paechter[i].GeloeschtAm)
			a.st.audit("Pächter in den Papierkorb gelegt: %s %s", a.st.d.Paechter[i].Mitgliedsnr, a.st.d.Paechter[i].Name)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
	}
	writeErr(w, notFound("Pächter nicht gefunden"))
}

// handlePaechterPapierkorb listet die Pächter im Papierkorb, neueste Löschung zuerst.
func (a *App) handlePaechterPapierkorb(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	out := []Paechter{}
	for _, p := range a.st.d.Paechter {
		if p.Geloescht {
			out = append(out, p)
		}
	}
	a.st.mu.Unlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].GeloeschtAm > out[j].GeloeschtAm })
	writeJSON(w, 200, out)
}

// handlePaechterWiederherstellen holt einen Pächter aus dem Papierkorb zurück.
func (a *App) handlePaechterWiederherstellen(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID == id && a.st.d.Paechter[i].Geloescht {
			if a.st.nrTaken(a.st.d.Paechter[i].Mitgliedsnr, id) {
				writeErr(w, bad("Die Mitgliedsnummer "+a.st.d.Paechter[i].Mitgliedsnr+
					" ist inzwischen an einen anderen Pächter vergeben. Bitte zuerst dort die Nummer ändern."))
				return
			}
			bis := a.st.d.Paechter[i].GeloeschtAm
			a.st.d.Paechter[i].Geloescht = false
			a.st.d.Paechter[i].GeloeschtAm = ""
			a.st.gartenWiederOeffnen(id, bis)
			a.st.audit("Pächter aus dem Papierkorb wiederhergestellt: %s %s", a.st.d.Paechter[i].Mitgliedsnr, a.st.d.Paechter[i].Name)
			if err := a.st.saveLocked(); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, a.st.d.Paechter[i])
			return
		}
	}
	writeErr(w, notFound("Nicht im Papierkorb gefunden"))
}

// handlePaechterEndgueltig löscht einen Pächter aus dem Papierkorb endgültig und entfernt
// seinen Personenbezug (Art. 17 DSGVO): Stammdaten, offene Zählerstände, Notizen sowie Namen
// in Garten-Historie, Änderungsprotokoll und allen Sicherungen. Rechnungen und die
// Jahresunterlagen abgeschlossener Jahre bleiben bis zum Ablauf der Aufbewahrungsfrist
// bestehen und verlieren ihren Personenbezug erst danach (Datenschutz → Bereinigen).
// Bewusst keine Sicherung vor dem Löschen: sie würde die Daten wieder enthalten.
func (a *App) handlePaechterEndgueltig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	for i := range a.st.d.Paechter {
		if a.st.d.Paechter[i].ID != id {
			continue
		}
		if !a.st.d.Paechter[i].Geloescht {
			writeErr(w, bad("Nur Pächter im Papierkorb können endgültig gelöscht werden"))
			return
		}
		a.st.d.entfernePerson(id)
		// ohne Namen protokollieren, sonst entstünde die Spur sofort neu
		a.st.audit("Pächter endgültig gelöscht, Personenbezug entfernt")
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
		a.st.leereDruckOrdner() // abgeleitete Druckdateien (z. B. Sammel-PDF) enthalten den Namen noch
		sicherungen := a.st.bereinigeSicherungen(func(d *Data) bool { return d.entfernePerson(id) })
		writeJSON(w, 200, map[string]int{"sicherungenBereinigt": sicherungen})
		return
	}
	writeErr(w, notFound("Pächter nicht gefunden"))
}

// handleGartenHistorie liefert die Belegungshistorie eines Gartens (wer hatte
// ihn wann), unabhängig vom Abrechnungsjahr. Ohne ?gartennr= kommt die
// komplette Historie aller Gärten zurück.
func (a *App) handleGartenHistorie(w http.ResponseWriter, r *http.Request) {
	nr := strings.TrimSpace(r.URL.Query().Get("gartennr"))
	a.st.mu.Lock()
	out := []GartenEintrag{}
	for _, e := range a.st.d.GartenHistorie {
		if nr == "" || e.Gartennr == nr {
			out = append(out, e)
		}
	}
	a.st.mu.Unlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seit < out[j].Seit })
	writeJSON(w, 200, out)
}

// ---------------------------------------------------------------- Einstellungen (Admin)

func cleanSettings(in, cur Settings) (Settings, error) {
	in.Jahr = cur.Jahr // wird nur über den Jahreswechsel geändert
	in.VereinName = trim(in.VereinName, 120)
	in.Ort = trim(in.Ort, 80)
	in.Absenderzeile = trim(in.Absenderzeile, 250)
	in.RechnungsnrPraefix = trim(in.RechnungsnrPraefix, 20)
	in.BankName = trim(in.BankName, 80)
	in.IBAN = trim(in.IBAN, 42)
	in.BIC = trim(in.BIC, 15)
	in.ZweiteSicherung = trim(in.ZweiteSicherung, 250)
	if in.VereinName == "" {
		return in, bad("Bitte einen Vereinsnamen eintragen")
	}
	if _, err := parseDate(in.Rechnungsdatum); err != nil {
		return in, bad("Das Rechnungsdatum ist ungültig")
	}
	if _, err := parseDate(in.Zahlungsziel); err != nil {
		return in, bad("Das Zahlungsziel ist ungültig")
	}
	if in.EinspruchTage < 0 || in.EinspruchTage > 365 {
		return in, bad("Einspruchsfrist: bitte 0 bis 365 Tage")
	}
	for name, x := range map[string]float64{
		"Wasser Grundpreis": in.WasserGrundpreis, "Wasser Preis": in.WasserPreis, "Energie Grundpreis": in.EnergieGrundpreis,
		"Energie Preis": in.EnergiePreis, "Pflichtstunden": in.Pflichtstunden, "Stundengrenze": in.StundenObergrenze,
		"Vergütung je Stunde": in.VerguetungJeStd, "Nachzahlung je Stunde": in.NachzahlungJeStd, "Pacht je m²": in.PachtJeQm,
		"Vereinsfläche": in.VereinsflaecheQm, "Freie Gärten": in.FreieGaertenQm, "Vereinsbeitrag": in.Vereinsbeitrag,
		"Territorialverband": in.Territorialverband, "Umlage": in.UmlageStandard,
	} {
		if !validNum(x) {
			return in, bad(name + ": bitte eine Zahl ab 0 eingeben")
		}
	}
	return in, nil
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	var in Settings
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	in, err := cleanSettings(in, a.st.d.Settings)
	if err != nil {
		writeErr(w, err)
		return
	}
	if in.ZweiteSicherung != "" && in.ZweiteSicherung != a.st.d.Settings.ZweiteSicherung {
		if err := checkWritableDir(in.ZweiteSicherung); err != nil {
			writeErr(w, bad(err.Error()))
			return
		}
	}
	a.st.d.Settings = in
	a.st.audit("Preise und Einstellungen geändert")
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, in)
}

// ---------------------------------------------------------------- Jahreswechsel (Admin)

func clone[T any](v T) T {
	raw, _ := json.Marshal(v)
	var out T
	_ = json.Unmarshal(raw, &out)
	return out
}

func carry(akt, vj *float64) *float64 {
	if akt != nil {
		v := *akt
		return &v
	}
	if vj != nil {
		v := *vj
		return &v
	}
	return nil
}

func addYear(date string) string {
	t, err := parseDate(date)
	if err != nil {
		return date
	}
	return t.AddDate(1, 0, 0).Format("2006-01-02")
}

func (a *App) handleNextYear(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	d := a.st.d
	cur := d.Settings.Jahr
	next := cur + 1
	if _, exists := d.Jahre[yearKey(next)]; exists {
		writeErr(w, bad("Das Folgejahr existiert schon"))
		return
	}
	a.st.snapshotBackup("vor-Jahreswechsel")
	old := a.st.ensureYear(cur)
	cs := clone(d.Settings)
	old.Settings = &cs
	old.Paechter = clone(d.Paechter)
	old.Abgeschlossen = true

	// Der Kassenbestand am Ende des abgeschlossenen Jahres wird automatisch zum
	// Anfangsbestand des Folgejahres. Von Hand im Kassenbericht anpassbar, falls
	// sich danach noch etwas ändert (z. B. eine spät eingehende Zahlung).
	closingView, _ := a.st.viewLocked(cur)
	anfangsbestand := a.st.kassenberichtLocked(closingView).Kassenbestand

	nj := &Jahr{Ablesungen: map[string]Ablesung{}, Anfangsbestand: &anfangsbestand}
	for _, p := range d.Paechter {
		o := old.Ablesungen[p.ID]
		nj.Ablesungen[p.ID] = Ablesung{
			WasserVJ: carry(o.WasserAkt, o.WasserVJ),
			StromVJ:  carry(o.StromAkt, o.StromVJ),
		}
	}
	// Als wiederkehrend markierte Ausgaben (z. B. Kontoführungsgebühren) werden
	// als Vorschlag fürs neue Jahr übernommen: gleiche Beschreibung, Kategorie
	// und Betrag, aber ungeprüft und ohne den alten Beleg, damit nichts blind
	// durchgewunken wird und der Vorstand jede noch von Hand bestätigt.
	wiederkehrend := 0
	for _, x := range old.Ausgaben {
		if !x.Wiederkehrend {
			continue
		}
		nj.Ausgaben = append(nj.Ausgaben, Ausgabe{
			ID: newID(), Datum: addYear(x.Datum), Beschreibung: x.Beschreibung, Kategorie: x.Kategorie,
			Betrag: x.Betrag, Wiederkehrend: true,
		})
		wiederkehrend++
	}
	d.Jahre[yearKey(next)] = nj
	d.Settings.Jahr = next
	d.Settings.Rechnungsdatum = addYear(d.Settings.Rechnungsdatum)
	d.Settings.Zahlungsziel = addYear(d.Settings.Zahlungsziel)
	if wiederkehrend > 0 {
		a.st.audit("Jahreswechsel: %d abgeschlossen, %d begonnen (%d wiederkehrende Ausgabe(n) übernommen)", cur, next, wiederkehrend)
	} else {
		a.st.audit("Jahreswechsel: %d abgeschlossen, %d begonnen", cur, next)
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"jahr": next})
}

// ---------------------------------------------------------------- Import (Admin)

func (a *App) handleImportPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<20)
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeErr(w, bad("Bitte eine Datei auswählen"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		writeErr(w, bad("Datei konnte nicht gelesen werden"))
		return
	}
	var table [][]string
	raw := false
	name := strings.ToLower(hdr.Filename)
	switch {
	case strings.HasSuffix(name, ".xlsx"):
		table, err = readXLSX(data, "Mitglieder")
		raw = true
	case strings.HasSuffix(name, ".csv"), strings.HasSuffix(name, ".txt"):
		table, err = parseCSV(data)
	default:
		err = errors.New("Bitte eine .xlsx- oder .csv-Datei wählen")
	}
	if err != nil {
		writeErr(w, bad(err.Error()))
		return
	}
	a.st.mu.Lock()
	settings := a.st.d.Settings
	existing := aktivePaechter(a.st.d.Paechter)
	a.st.mu.Unlock()
	rows, warn, err := parseImport(table, raw, settings, existing)
	if err != nil {
		writeErr(w, bad(err.Error()))
		return
	}
	writeJSON(w, 200, map[string]any{"rows": rows, "warnings": warn})
}

func (a *App) handleImportApply(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Rows []ImportRow `json:"rows"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	// Erst alle Zeilen prüfen, damit ein Fehler in einer späteren Zeile nicht
	// einen halben Import im Speicher zurücklässt.
	for i, row := range in.Rows {
		p, err := cleanPaechter(row.Paechter)
		if err != nil {
			writeErr(w, bad(fmt.Sprintf("Zeile %d: %s", row.Zeile, err.Error())))
			return
		}
		in.Rows[i].Paechter = p
		if row.Ablesung != nil {
			ab, err := cleanAblesung(*row.Ablesung)
			if err != nil {
				writeErr(w, bad(fmt.Sprintf("Zeile %d: %s", row.Zeile, err.Error())))
				return
			}
			in.Rows[i].Ablesung = &ab
		}
	}
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	a.st.snapshotBackup("vor-Import")
	j := a.st.ensureYear(a.st.d.Settings.Jahr)
	added, updated := 0, 0
	for _, row := range in.Rows {
		p := row.Paechter
		idx := -1
		for i, q := range a.st.d.Paechter {
			if !q.Geloescht && strings.EqualFold(strings.TrimSpace(q.Mitgliedsnr), p.Mitgliedsnr) {
				idx = i
			}
		}
		if idx >= 0 {
			old := a.st.d.Paechter[idx]
			p.ID = old.ID
			// Felder, die in der Datei fehlten, bleiben unverändert
			has := map[string]bool{}
			for _, f := range row.Felder {
				has[f] = true
			}
			if !has["Gartennr."] {
				p.Gartennr = old.Gartennr
			}
			if !has["Anrede"] {
				p.Anrede = old.Anrede
			}
			if !has["Straße"] {
				p.Strasse = old.Strasse
			}
			if !has["PLZ Ort"] {
				p.PLZOrt = old.PLZOrt
			}
			if !has["Versandart"] {
				p.Versand = old.Versand
			}
			if !has["Gartengröße"] {
				p.Gartengroesse = old.Gartengroesse
			}
			if !has["Umlage abweichend"] && !has["Umlage"] {
				p.UmlageAbweichend = old.UmlageAbweichend
			}
			if !has["Wasserzähler-Nr."] {
				p.WasserzaehlerNr = old.WasserzaehlerNr
			}
			if !has["Stromzähler-Nr."] {
				p.StromzaehlerNr = old.StromzaehlerNr
			}
			a.st.d.Paechter[idx] = p
			updated++
		} else {
			p.ID = newID()
			a.st.d.Paechter = append(a.st.d.Paechter, p)
			added++
		}
		if row.Ablesung != nil {
			ab := *row.Ablesung
			cur := j.Ablesungen[p.ID]
			// nur die Felder überschreiben, die die Datei enthielt
			if ab.WasserVJ != nil {
				cur.WasserVJ = ab.WasserVJ
			}
			if ab.WasserAkt != nil {
				cur.WasserAkt = ab.WasserAkt
			}
			if ab.StromVJ != nil {
				cur.StromVJ = ab.StromVJ
			}
			if ab.StromAkt != nil {
				cur.StromAkt = ab.StromAkt
			}
			if ab.Stunden != nil {
				cur.Stunden = ab.Stunden
			}
			if ab.Versicherung != 0 {
				cur.Versicherung = ab.Versicherung
			}
			if ab.Grundsteuer != 0 {
				cur.Grundsteuer = ab.Grundsteuer
			}
			if ab.Auslagen != 0 {
				cur.Auslagen = ab.Auslagen
			}
			if ab.Abschlag != 0 {
				cur.Abschlag = ab.Abschlag
			}
			if ab.Hinweis != "" {
				cur.Hinweis = ab.Hinweis
			}
			j.Ablesungen[p.ID] = cur
		} else if _, ok := j.Ablesungen[p.ID]; !ok {
			j.Ablesungen[p.ID] = Ablesung{}
		}
	}
	if err := a.st.saveLocked(); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, 200, map[string]int{"neu": added, "aktualisiert": updated})
}

// ---------------------------------------------------------------- Rechnungen, Export

func safeName(s string, max int) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x20, strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(strings.TrimSpace(b.String()), ".")
	rs := []rune(out)
	if len(rs) > max {
		rs = rs[:max]
	}
	return strings.TrimSpace(string(rs))
}

func invoiceFileName(s Settings, p Paechter) string {
	return safeName("Rechnung_"+invoiceNumber(s, p)+"_"+p.Name, 80) + ".pdf"
}

func findPaechter(v yearView, id string) (Paechter, bool) {
	for _, p := range v.Paechter {
		if p.ID == id {
			return p, true
		}
	}
	return Paechter{}, false
}

func (a *App) handleInvoice(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("id"), ".pdf")
	year := a.yearParam(r)
	a.st.mu.Lock()
	v, ok := a.st.viewLocked(year)
	var (
		p     Paechter
		found bool
		ab    Ablesung
	)
	if ok {
		p, found = findPaechter(v, id)
		ab = v.Ablesungen[id]
	}
	settings := v.Settings
	a.st.mu.Unlock()
	if !ok || !found {
		writeErr(w, notFound("Pächter oder Jahr nicht gefunden"))
		return
	}
	res := calculate(settings, p, ab)
	pdf, err := buildInvoice(settings, p, ab, res)
	if err != nil {
		writeErr(w, bad(err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+invoiceFileName(settings, p)+`"`)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(pdf)
}

func (a *App) handleExport(w http.ResponseWriter, r *http.Request) {
	year := a.yearParam(r)
	a.st.mu.Lock()
	v, ok := a.st.viewLocked(year)
	var data []byte
	var err error
	if ok {
		data, err = exportOverview(v)
	}
	a.st.mu.Unlock()
	if !ok {
		writeErr(w, notFound("Jahr nicht gefunden"))
		return
	}
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="Jahresuebersicht_%d.xlsx"`, year))
	_, _ = w.Write(data)
}

func (a *App) handleBackup(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	raw, err := os.ReadFile(a.st.path)
	a.st.mu.Unlock()
	if err != nil {
		writeErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+appID+`-sicherung-`+time.Now().Format("2006-01-02")+`.json"`)
	_, _ = w.Write(raw)
}

// handleAuditLog liefert das Änderungsprotokoll, neueste Einträge zuerst.
func (a *App) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	out := make([]AuditEntry, len(a.st.d.AuditLog))
	copy(out, a.st.d.AuditLog)
	a.st.mu.Unlock()
	sort.SliceStable(out, func(i, j int) bool { return out[i].Zeit > out[j].Zeit })
	writeJSON(w, 200, out)
}

// ---------------------------------------------------------------- Update-Prüfung
//
// Rein manuell (Admin klickt auf »Nach Updates suchen«), nie automatisch beim
// Start: das Programm ruft sonst nie nach Hause. Es wird ausschließlich die
// öffentliche, feste GitHub-Release-Adresse dieses Projekts abgefragt, die
// Antwort wird in der Größe begrenzt und nur zwei einfache Textfelder werden
// ausgelesen. Es wird nichts heruntergeladen oder ausgeführt, nur ein Link
// zur Release-Seite angezeigt, den die Person selbst anklicken kann.
const updateCheckURL = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"

type updateInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	URL       string `json:"url,omitempty"`
	Hinweis   string `json:"hinweis,omitempty"`
}

// versionNewer vergleicht zwei Versionsnummern der Form "1.2.3" (fehlende
// Teile zählen als 0). Ungültige Teile werden wie 0 behandelt.
func versionNewer(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	num := func(parts []string, i int) int {
		if i >= len(parts) {
			return 0
		}
		v, _ := strconv.Atoi(strings.TrimSpace(parts[i]))
		return v
	}
	for i := 0; i < n; i++ {
		if x, y := num(pa, i), num(pb, i); x != y {
			return x > y
		}
	}
	return false
}

func (a *App) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 6 * time.Second}
	req, err := http.NewRequest(http.MethodGet, updateCheckURL, nil)
	if err != nil {
		writeJSON(w, 200, updateInfo{Hinweis: "Update-Prüfung konnte nicht gestartet werden"})
		return
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", appName+"/"+appVersion)
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 200, updateInfo{Hinweis: "Keine Verbindung zu GitHub möglich"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeJSON(w, 200, updateInfo{Hinweis: "Keine Release-Information verfügbar"})
		return
	}
	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&rel); err != nil {
		writeJSON(w, 200, updateInfo{Hinweis: "Antwort von GitHub konnte nicht gelesen werden"})
		return
	}
	latest := strings.TrimPrefix(strings.TrimSpace(rel.TagName), "v")
	if latest == "" || !strings.HasPrefix(rel.HTMLURL, repoURLBase) {
		writeJSON(w, 200, updateInfo{Hinweis: "Keine Release-Information verfügbar"})
		return
	}
	if versionNewer(latest, appVersion) {
		writeJSON(w, 200, updateInfo{Available: true, Version: latest, URL: rel.HTMLURL})
		return
	}
	writeJSON(w, 200, updateInfo{Available: false})
}

func openPath(p string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", p)
	case "darwin":
		cmd = exec.Command("open", p)
	default:
		cmd = exec.Command("xdg-open", p)
	}
	_ = cmd.Start()
	if cmd.Process != nil {
		go func() { _ = cmd.Wait() }()
	}
}

func (a *App) handleOpenFolder(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Which string `json:"which"`
		Year  int    `json:"year"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, err)
		return
	}
	dir := a.st.dir
	switch in.Which {
	case "rechnungen":
		dir = filepath.Join(a.st.dir, "Rechnungen")
		if in.Year > 0 {
			dir = filepath.Join(dir, strconv.Itoa(in.Year))
		}
	case "sicherungen":
		dir = a.st.backupDir()
	case "daten":
	default:
		writeErr(w, bad("Unbekannter Ordner"))
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		writeErr(w, err)
		return
	}
	openPath(dir)
	writeJSON(w, 200, map[string]string{"folder": dir})
}

// handleTutorial merkt sich, dass der Einführungsrundgang durchlaufen oder
// übersprungen wurde. Bewusst ohne Admin-Pflicht: die Einführung ist für alle
// Nutzer gedacht und enthält keine Daten.
func (a *App) handleTutorial(w http.ResponseWriter, r *http.Request) {
	a.st.mu.Lock()
	defer a.st.mu.Unlock()
	if !a.st.d.TutorialGesehen {
		a.st.d.TutorialGesehen = true
		if err := a.st.saveLocked(); err != nil {
			writeErr(w, err)
			return
		}
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
}

func (a *App) handleQuit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]bool{"ok": true})
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()
}

// ---------------------------------------------------------------- Routen

func (a *App) routes(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/ping", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"app": appID, "version": appVersion})
	})
	mux.HandleFunc("GET /api/state", a.handleState)
	mux.HandleFunc("PUT /api/ablesung/{id}", a.handleAblesung)
	mux.HandleFunc("PUT /api/zaehler/{id}", a.handleZaehler)
	mux.HandleFunc("GET /api/invoice/{id}", a.handleInvoice)
	mux.HandleFunc("POST /api/invoices/issue", a.handleIssue)
	mux.HandleFunc("POST /api/open-invoice/{id}", a.handleOpenInvoice)
	mux.HandleFunc("GET /api/ablesebogen", a.handleAblesebogen)
	mux.HandleFunc("POST /api/ablesebogen/oeffnen", a.handleAblesebogenOeffnen)
	mux.HandleFunc("GET /api/admin/rechnungen-druck", a.admin(a.handleDruckPost))
	mux.HandleFunc("POST /api/admin/rechnungen-druck/oeffnen", a.admin(a.handleDruckPostOeffnen))
	mux.HandleFunc("GET /api/archive/{id}", a.handleArchivePDF)
	mux.HandleFunc("GET /api/archiv-alle", a.handleArchivAlle)
	mux.HandleFunc("PUT /api/payment/{id}", a.handlePayment)
	mux.HandleFunc("GET /api/export-payments", a.handlePaymentsExport)
	mux.HandleFunc("GET /api/export-ics", a.handleICSExport)
	mux.HandleFunc("GET /api/garten-historie", a.handleGartenHistorie)
	mux.HandleFunc("GET /api/export", a.handleExport)
	mux.HandleFunc("POST /api/open-folder", a.handleOpenFolder)
	mux.HandleFunc("POST /api/quit", a.handleQuit)
	mux.HandleFunc("POST /api/tutorial", a.handleTutorial)

	mux.HandleFunc("POST /api/admin/login", a.handleLogin)
	mux.HandleFunc("POST /api/admin/logout", a.handleLogout)
	mux.HandleFunc("POST /api/admin/password", a.handlePassword)
	mux.HandleFunc("POST /api/pruef/login", a.handlePruefLogin)
	mux.HandleFunc("POST /api/pruef/logout", a.handlePruefLogout)
	mux.HandleFunc("POST /api/admin/pruef-password", a.admin(a.handlePruefPassword))
	mux.HandleFunc("POST /api/admin/paechter", a.admin(a.handlePaechterCreate))
	mux.HandleFunc("PUT /api/admin/paechter/{id}", a.admin(a.handlePaechterUpdate))
	mux.HandleFunc("DELETE /api/admin/paechter/{id}", a.admin(a.handlePaechterDelete))
	mux.HandleFunc("GET /api/admin/paechter-papierkorb", a.admin(a.handlePaechterPapierkorb))
	mux.HandleFunc("POST /api/admin/paechter/{id}/wiederherstellen", a.admin(a.handlePaechterWiederherstellen))
	mux.HandleFunc("GET /api/admin/paechter/{id}/auskunft", a.admin(a.handleAuskunft))
	mux.HandleFunc("GET /api/admin/sicherungen", a.admin(a.handleSicherungen))
	mux.HandleFunc("POST /api/admin/sicherungen/wiederherstellen", a.admin(a.handleSicherungWiederherstellen))
	mux.HandleFunc("GET /api/admin/datenschutz", a.admin(a.handleDatenschutz))
	mux.HandleFunc("POST /api/admin/datenschutz/bereinigen", a.admin(a.handleBereinigen))
	mux.HandleFunc("DELETE /api/admin/paechter/{id}/endgueltig", a.admin(a.handlePaechterEndgueltig))
	mux.HandleFunc("PUT /api/admin/settings", a.admin(a.handleSettings))
	mux.HandleFunc("POST /api/admin/jahreswechsel", a.admin(a.handleNextYear))
	mux.HandleFunc("POST /api/admin/import/preview", a.admin(a.handleImportPreview))
	mux.HandleFunc("POST /api/admin/import/apply", a.admin(a.handleImportApply))
	mux.HandleFunc("GET /api/admin/backup", a.admin(a.handleBackup))
	mux.HandleFunc("GET /api/admin/audit-log", a.admin(a.handleAuditLog))
	mux.HandleFunc("GET /api/admin/check-update", a.admin(a.handleCheckUpdate))
	mux.HandleFunc("GET /api/admin/kassenbericht", a.adminOrPruef(a.handleKassenbericht))
	mux.HandleFunc("PUT /api/admin/versorger", a.admin(a.handleVersorger))
	mux.HandleFunc("PUT /api/admin/anfangsbestand", a.admin(a.handleAnfangsbestand))
	mux.HandleFunc("GET /api/admin/export-kassenbericht", a.adminOrPruef(a.handleKassenberichtExport))
	mux.HandleFunc("GET /api/admin/kassenbericht-verlauf", a.adminOrPruef(a.handleKassenberichtVerlauf))
	mux.HandleFunc("POST /api/admin/ausgaben", a.admin(a.handleAusgabeCreate))
	mux.HandleFunc("PUT /api/admin/ausgaben/{id}", a.admin(a.handleAusgabeUpdate))
	mux.HandleFunc("DELETE /api/admin/ausgaben/{id}", a.admin(a.handleAusgabeDelete))
	mux.HandleFunc("PUT /api/admin/ausgaben/{id}/geprueft", a.adminOrPruef(a.handleAusgabeGeprueft))
	mux.HandleFunc("POST /api/admin/ausgaben/{id}/beleg", a.admin(a.handleBelegUpload))
	mux.HandleFunc("DELETE /api/admin/ausgaben/{id}/beleg", a.admin(a.handleBelegDelete))
	mux.HandleFunc("GET /api/admin/beleg/{id}", a.adminOrPruef(a.handleBelegServe))
	mux.HandleFunc("POST /api/admin/mahnung", a.admin(a.handleMahnung))
	mux.Handle("/", static)
	return a.guard(mux)
}
