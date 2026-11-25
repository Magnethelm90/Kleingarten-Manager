# Projekt: Kleingarten-Manager (Go)

Programm für Kleingartenvereine (Windows/macOS): Pächter, Zählerstände, Rechnungen als PDF, Zahlungen, Kasse, Vorstandsarbeit. Alle Daten bleiben lokal.

## Dateien für den Überblick (zuerst lesen)
- `FEATURES.md` – **vollständige Liste aller Funktionen** mit Fundstelle im Code. Vor jedem Umbau nachsehen, damit nichts verloren geht; neue Funktionen dort eintragen.
- `PROGRESS.md` – aktueller Stand (Feature, zuletzt erledigt, offene Bugs, nächster Schritt). Wird mit `/save` aktualisiert.
- `README.md` (Außenansicht), `ENTWICKLUNG.md` (Release-Ablauf), `DATENSCHUTZ.md`, `git log`.

## Tech-Stack & Befehle
- Go (Version siehe `go.mod`), `net/http`, JSON-Store; Oberfläche in Vanilla-JS unter `web/`, per `//go:embed web` eingebettet.
- Tests: `go vet ./... && go test ./...` (~35 s). Starten: `go run . -data <ordner> -no-browser -port 8080` (Datenordner immer frisch mit `mktemp -d`).
- Vor größeren Pushes zusätzlich: `staticcheck -checks "all,-ST1005,-ST1000,-ST1003,-ST1021" ./...` (deutsche Fehlertexte sind gewollt), `deadcode .` (`externalURLAllowed` nur unter Build-Tag genutzt), `govulncheck ./...`, `go test -race ./...` (~4 min), ESLint 9 (`@eslint/js` recommended + `globals.browser`) über die zusammengehängten `web/*.js`, Zufallstest `KM_FUZZ=1 KM_FUZZ_N=60 KM_FUZZ_SEED=n go test -run TestApiZufall`.
- UI mit Playwright (Chromium vorinstalliert): alle Seiten bei 1366 px und 390 px, auch auf »null«/»NaN« im Text prüfen. Nur der Lageplan darf am Handy breiter sein.
- Version: `var appVersion` in `api.go`; Release = Tag `vX.Y.Z` (siehe `ENTWICKLUNG.md`). Aktuell **1.6**. Neue Version nur auf Wunsch des Nutzers.

## Code-Regeln
- Idiomatisches Go (`gofmt`, Fehler mit Kontext). Keine neuen externen Bibliotheken ohne Rücksprache; das Programm lädt nichts von außen (CSP `default-src 'self'; img-src 'self' data:`), nur der Update-Check geht ins Internet.
- Für neue Logik gleich Tests schreiben (`*_test.go`); neue Routen laufen durch `vollschutz`/`zugriff_test.go`.
- Server: Mutex `a.st.mu`, `saveLocked()`, `speichernUndAntworten(w,v)` (Aufrufer hält den Mutex); Wrapper `admin()` / `adminOrPruef()`; Fehler `bad()` / `notFound()`; `readJSON`, `writeJSON`/`writeErr`.
- PDF mit fpdf: `neuesPDF`, `zeichneKopf`, `briefAnfang`, `briefText`; **immer mit `pdfBytes(pdf)` beenden**. Schrift »lib« hat keinen Kursivschnitt; nur Zeichen bis U+FFFF (`trim()`/`texteBereinigen` entfernen Emoji).
- Frontend: klassische Skripte mit **einem gemeinsamen globalen Scope** (Reihenfolge in `web/index.html`): `app.js`, `uebersicht`, `zaehlerstaende`, `lageplan`, `zahlungen`, `verwaltung`, `admin`, `paechter`, `termine`, `anleitung`, `start`, `tabellen`. Neue Seiten in die passende Datei, neue Dateien in `index.html` eintragen. Helfer: `h()`, `modal`, `breiterDialog`, `api`, `leer`, `confirmBox`, `klappKarte`, `reiterKopf/reiterLeiste`, `druckKnopf`, `anz(n, eins, viele)`, `feldFehler`, `geloeschtToast`, `navGeh`, `seiteZurueck`.
- 3D-Lageplan `web/lageplan3d.js` (three.js 0.170.0 lokal, `web/vendor/three/`). Bilder: SVG in `web/bilder/` (Kopien in `docs/bilder/`). Projektseite `docs/index.html`.
- Demo-Daten in `demo.go`, `datenbestandLeer` mitpflegen. `defaultSettings`/`newData` **nicht ändern** (Testbasis).

## Arbeitsweise mit dem Nutzer
- Deutsch, knapp. Programmtexte per **»du«**, in einfachen Worten, keine Emoji in der Oberfläche.
- Branch `claude/github-improvements-pi59o5` (Repo Magnethelm90/Kleingarten-Manager, MCP-Name `gartenabrechnung`). Nach jeder Arbeit committen und pushen. Keine PR anlegen und nicht mergen, bis der Nutzer es sagt. Tags kann ich nicht pushen (403) – Releases legt der Nutzer auf GitHub an.
- Design: hochwertig, aber **professionell** für den Vereinsvorstand – sachliche Strichzeichnungen, gedeckte Farben, kein Spielzeug.
- **Keine Funktion entfernen**; vereinfachen = zusammenlegen, einklappen, Doppeltes streichen. Neue Seiten als Reiter in bestehende `NAV`-Einträge hängen, Menü nicht verlängern.
- Abgelehnt, nicht wieder bauen: Luftbild/WMS, Gartenschild/Mitgliedsausweis, Tastenkürzel, Kautions-Feld.
- CI-Prüfungen nie abschwächen (kein `continue-on-error`) ohne ausdrückliche Zustimmung.
- Nach jedem Paket: `NEUERUNGEN` (`web/start.js`) ergänzen, `FEATURES.md` ergänzen, hier nur bei neuen Regeln/Fallen, dann `/save` (aktualisiert `PROGRESS.md`).

## Muster, die man immer braucht
- **Personendaten:** neue Art in `auskunftLocked` aufnehmen, in `entfernePerson` per `d.xxxPersonEntfernen(id)` löschen, `DATENSCHUTZ.md` ergänzen.
- **Cockpit:** `aufgabenLocked` (`aufgaben.go`) ruft die `…AufgabeLocked`-Funktionen; Ziel-Format `admin:<tab>[:<unterseite>]`; neue Ziele in `COCKPIT_BEREICHE`/`cockpitBereich()` einordnen; `TestZielFormat` prüft sie.
- **Bereiche ein-/ausblenden:** `module.go` (`Data.AusModule` = Liste der AUSgeschalteten; leer = alles an); Frontend `modulAn()`, Cockpit `s.modulAn()`.
- **Löschen:** `papierkorbLegeLocked` im Handler, Antwort mit `papierkorbId` → `geloeschtToast` mit »Rückgängig«; bei Personenlöschung `papierkorbPersonEntfernen`.
- **Briefe:** Texte über `s.Brieftexte.text(key, werte)`; neue Briefe als Vorlage in `brieftextVorlagen` + Feld in `Brieftexte`.
- **Berichte:** Kachel in die passende der 4 Gruppen; Einstellungen: Seltenes als `klappKarte`.
- **Einfache Dialoge:** `xxxDialog` = Schleife um `xxxDialogEinmal` (»Speichern und nächste«).
- **Zurück-Knopf / Kopfzeile:** `verlaufFuehren()` in `render()` merkt die Seiten (nur Speicher, max. 30); Klick auf den Vereinsnamen = `navGeh({tab:'uebersicht'})`.
- **Merker pro Installation** (z. B. `TutorialGesehen`, `EinrichtungFertig`): Feld in `Data` (`omitempty`), im Zustand ausliefern, kleiner POST-Handler; `demo.go` entscheidet, ob er beim Entfernen der Beispieldaten bleibt.

## Fallen (hier ist schon etwas schiefgegangen)
- Server-Felder **nie aus der Eingabe** übernehmen (Belege, Foto, Stufen, Geprüft, Bereinigt, Geloescht, AusgeschiedenAm, Verschickt, ZuletztGenutzt, PostAm …): beim Anlegen/Ändern leeren oder vom alten Eintrag holen.
- Leere Listen immer als `[]` liefern, nie `nil` (`leer_test.go` prüft auf `null`). Zahlen nie als »-0« anzeigen (`fmtNum`/`nf2`/`nfFlex`/`nfIn`).
- Monate rechnen mit `monateDazu(t, n)`, nie `AddDate(0, n, 0)`; Verlängerungen immer vom Ursprungsdatum.
- JS: `replaceChildren(..., null)` schreibt »null« (leeres Element übergeben); `tr.append(array)` flacht nicht ab (Spread); kein »NaN« im Text (sonst schlägt `TestAlleLeseRouten` an); Dialog-`cancel` nur behandeln, wenn `e.target === dlg`.
- CSS: `animation-fill-mode: backwards` (nicht `both`); `prefers-reduced-motion` beachten; `[hidden] { display:none !important }` steht ganz oben.
- Jahre vor `Settings.Jahr` gibt es nicht (Tests starten im Startjahr). Verbrauch eines Jahres wird im Folgejahr bezahlt; Zahlungen zählen im Rechnungsjahr.
- Schreibt ein Test in ein Datenverzeichnis: immer `mktemp -d`, nie alte Ordner im Scratchpad (veraltete Daten, Mehrplatz-Sperre).
- Nach einem Squash-Merge ist der alte Branch überholt: nicht force-pushen (wird abgelehnt), sondern `origin/<branch>` per Merge einbringen.
