package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed web
var webFS embed.FS

const defaultPort = 8765

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err == nil {
		go func() { _ = cmd.Wait() }()
	}
}

// externalURLAllowed legt fest, welche Adressen aus dem Programmfenster heraus
// im Systembrowser geöffnet werden dürfen: nur die eigene GitHub-Projektseite
// (Release-Link der Update-Prüfung). Alles andere wird abgelehnt, damit die
// Brücke vom Fenster zum Betriebssystem nicht für beliebige Adressen oder
// Protokolle (file:, ms-…:, Programmaufrufe) missbraucht werden kann.
func externalURLAllowed(raw string) bool {
	if !strings.HasPrefix(raw, repoURLBase) {
		return false
	}
	for _, c := range raw {
		if c < 0x21 || c == 0x7f || c == '"' || c == '\\' {
			return false
		}
	}
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "github.com" && u.User == nil
}

// dataDir bestimmt, wo die Daten liegen: neben der .exe, sonst im Benutzerordner.
func dataDir(flagDir string) string {
	if flagDir != "" {
		return flagDir
	}
	// KLEINGARTEN_MANAGER_DATEN ist der aktuelle Name; GARTENABRECHNUNG_DATEN wird
	// aus der Zeit vor der Umbenennung weiterhin unterstützt.
	if env := os.Getenv("KLEINGARTEN_MANAGER_DATEN"); env != "" {
		return env
	}
	if env := os.Getenv("GARTENABRECHNUNG_DATEN"); env != "" {
		return env
	}
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		probe := filepath.Join(dir, ".schreibtest")
		if werr := os.WriteFile(probe, []byte("x"), 0o644); werr == nil {
			_ = os.Remove(probe)
			return dir
		}
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		return filepath.Join(cfg, appName)
	}
	return "."
}

// alreadyRunning prüft, ob unter der Adresse schon unser Programm läuft.
func alreadyRunning(port int) bool {
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/ping", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var m map[string]string
	if json.NewDecoder(resp.Body).Decode(&m) != nil {
		return false
	}
	return m["app"] == appID
}

func main() {
	dirFlag := flag.String("data", "", "Ordner für Daten, Sicherungen und Rechnungen (Standard: neben der .exe)")
	noBrowser := flag.Bool("no-browser", false, "Browser nicht automatisch öffnen")
	port := flag.Int("port", defaultPort, "Port auf 127.0.0.1")
	resetAdmin := flag.Bool("reset-admin", false, "Admin-Passwort entfernen und beenden")
	flag.Parse()
	setupConsole()

	dir := dataDir(*dirFlag)
	st, err := openStore(dir)
	if err != nil {
		fail(err.Error())
	}
	if *resetAdmin {
		st.mu.Lock()
		st.d.Admin = AdminAuth{}
		err := st.saveLocked()
		st.mu.Unlock()
		if err != nil {
			fail(err.Error())
		}
		fmt.Println("Das Admin-Passwort wurde entfernt.")
		return
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		if alreadyRunning(*port) {
			fmt.Println(appName, "laeuft bereits - oeffne das Fenster.")
			openStart(fmt.Sprintf("http://127.0.0.1:%d/", *port), *noBrowser)
			return
		}
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fail("Kein freier Port gefunden: " + err.Error())
		}
	}
	actual := ln.Addr().(*net.TCPAddr).Port

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	static := http.FileServerFS(sub)
	noCache := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		static.ServeHTTP(w, r)
	})
	app := &App{st: st, port: actual, sessions: map[string]time.Time{}, psessions: map[string]time.Time{}}
	srv := &http.Server{
		Handler:           app.routes(noCache),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/", actual)
	fmt.Println("==============================================================")
	fmt.Println(" "+appName, appVersion, "- Copyright (c)", time.Now().Year(), appAutor)
	fmt.Println("==============================================================")
	fmt.Println(" Daten liegen in:", dir)
	fmt.Println(" Adresse:        ", url)
	if *noBrowser || !hasNativeWindow {
		fmt.Println(" Das Programm laeuft. Dieses Fenster bitte offen lassen.")
		fmt.Println(" Beenden: im Programm oben rechts auf \"Beenden\" klicken")
		fmt.Println(" oder dieses Fenster schliessen.")
	}
	fmt.Println("==============================================================")

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			fail(err.Error())
		}
	}()

	openStart(url, *noBrowser)
	// Läuft das Programm als eigenes Fenster, endet es, sobald das Fenster
	// geschlossen wird. Sonst (Browser oder --no-browser) läuft der Server
	// im Vordergrund weiter, bis das Programm über /api/quit beendet wird.
	if *noBrowser || !hasNativeWindow {
		select {}
	}
}

// openStart öffnet die Oberfläche: als eigenes Programmfenster, wenn die
// Plattform das unterstützt (Windows/macOS), sonst im Standardbrowser.
func openStart(url string, noBrowser bool) {
	if noBrowser {
		return
	}
	if hasNativeWindow {
		runNativeWindow(url)
		return
	}
	openBrowser(url)
}

// fail meldet einen Startfehler (Konsole, bei Windows zusätzlich als
// Meldungsfenster, da das Programmfenster keine sichtbare Konsole mehr hat)
// und beendet das Programm.
func fail(msg string) {
	fmt.Println("FEHLER:", msg)
	fatalBox(msg)
	os.Exit(1)
}
