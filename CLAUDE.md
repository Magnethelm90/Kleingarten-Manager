# Notizen für Claude (nur das Wichtigste)

Was alles gebaut wurde: `prozess.md` (Überblick), `PROGRESS.md` (aktueller Stand), `git log`. Hier stehen nur Regeln und Fallen.

## Arbeitsweise mit dem Nutzer
- Deutsch, knapp. Programmtexte per **»du«**, in einfachen Worten (Kinder-Niveau), keine Emoji in der Oberfläche.
- Branch `claude/github-improvements-pi59o5`, Sammel-PR Magnethelm90/Kleingarten-Manager#15 (MCP-Reponame `gartenabrechnung`). Aktuelle Version: **1.6** (Release = Tag `v1.6`, siehe `ENTWICKLUNG.md`).
- **Nicht mergen**, keine neue PR, bis der Nutzer es sagt. Nach jeder Arbeit committen und pushen.
- Design: hochwertig, aber **professionell** für den Vereinsvorstand – sachliche Strichzeichnungen, gedeckte Farben, kein Spielzeug, keine Knet-/3D-Optik.
- **Keine Funktion entfernen**; vereinfachen = zusammenlegen, einklappen, Doppeltes streichen. Neue Seiten als Reiter in bestehende `NAV`-Einträge hängen, Menü nicht verlängern.
- Abgelehnt, nicht wieder bauen: Luftbild/WMS, Gartenschild/Mitgliedsausweis, Tastenkürzel. Kein Kautions-Feld.
- CI-Prüfungen nie abschwächen (kein `continue-on-error`) ohne ausdrückliche Zustimmung.
- Nach jedem Paket: diese Datei nur bei neuen Regeln/Fallen ergänzen, `NEUERUNGEN` (`web/start.js`) ergänzen, `PROGRESS.md` aktualisieren (Anweisung: `save.md`).

## Technik
- Go `net/http`, JSON-Store `Data` (`model.go`), Mutex `a.st.mu`, `saveLocked()`, `speichernUndAntworten(w,v)` (Aufrufer hält den Mutex).
- Wrapper `admin()` / `adminOrPruef()`; Fehler `bad()` / `notFound()`; `readJSON`, `writeJSON`/`writeErr`. Neue offene `/api`-Routen sind durch `vollschutz` automatisch gesperrt.
- PDF mit fpdf: `neuesPDF`, `zeichneKopf`, `briefAnfang`, `briefText`; **immer mit `pdfBytes(pdf)` beenden**. Schrift »lib« hat keinen Kursivschnitt; nur Zeichen bis U+FFFF (`trim()`/`texteBereinigen` entfernen Emoji).
- Frontend: Vanilla-JS, klassische Skripte mit **einem gemeinsamen globalen Scope** (Reihenfolge in `web/index.html`): `app.js`, `uebersicht`, `zaehlerstaende`, `lageplan`, `zahlungen`, `verwaltung`, `admin`, `paechter`, `termine`, `anleitung`, `start`, `tabellen`. Neue Seiten in die passende Datei, neue Dateien in `index.html` eintragen. Helfer: `h()`, `modal`, `breiterDialog`, `api`, `leer`, `confirmBox`, `klappKarte`, `reiterKopf/reiterLeiste`, `druckKnopf`, `anz(n, eins, viele)`, `feldFehler`, `geloeschtToast`.
- Assets per `//go:embed web`. CSP `default-src 'self'; img-src 'self' data:` – nichts von außen laden. Das Programm geht nur für den Update-Check ins Internet.
- 3D-Lageplan `web/lageplan3d.js` (three.js 0.170.0 lokal, `web/vendor/three/`). Bilder: SVG in `web/bilder/` (Kopien in `docs/bilder/`). Projektseite `docs/index.html` (Pages erst nach Merge).
- Demo-Daten in `demo.go`, `datenbestandLeer` mitpflegen. `defaultSettings`/`newData` **nicht ändern** (Testbasis).

## Muster, die man immer braucht
- **Personendaten:** neue Art in `auskunftLocked` aufnehmen, in `entfernePerson` per `d.xxxPersonEntfernen(id)` löschen, `DATENSCHUTZ.md` ergänzen.
- **Cockpit:** `aufgabenLocked` (`aufgaben.go`) ruft die `…AufgabeLocked`-Funktionen; Ziel-Format `admin:<tab>[:<unterseite>]`; neue Ziele in `COCKPIT_BEREICHE`/`cockpitBereich()` einordnen; `TestZielFormat` prüft sie.
- **Bereiche ein-/ausblenden:** `module.go` (`Data.AusModule` = Liste der AUSgeschalteten; leer = alles an); Frontend `modulAn()`, Cockpit `s.modulAn()`.
- **Löschen:** `papierkorbLegeLocked` im Handler, Antwort mit `papierkorbId` → `geloeschtToast` mit »Rückgängig«; bei Personenlöschung `papierkorbPersonEntfernen`.
- **Briefe:** Texte über `s.Brieftexte.text(key, werte)`; neue Briefe als Vorlage in `brieftextVorlagen` + Feld in `Brieftexte`.
- **Berichte:** Kachel in die passende der 4 Gruppen; Einstellungen: Seltenes als `klappKarte`.
- **Einfache Dialoge:** `xxxDialog` = Schleife um `xxxDialogEinmal` (»Speichern und nächste«).

## Fallen (hier ist schon etwas schiefgegangen)
- Server-Felder **nie aus der Eingabe** übernehmen (Belege, Foto, Stufen, Geprüft, Bereinigt, Geloescht, AusgeschiedenAm, Verschickt, ZuletztGenutzt, PostAm …): beim Anlegen/Ändern leeren oder vom alten Eintrag holen.
- Leere Listen immer als `[]` liefern, nie `nil` (`leer_test.go` prüft auf `null`). Zahlen nie als »-0« anzeigen (`fmtNum`/`nf2`/`nfFlex`/`nfIn`).
- Monate rechnen mit `monateDazu(t, n)`, nie `AddDate(0, n, 0)`; Verlängerungen immer vom Ursprungsdatum.
- JS: `replaceChildren(..., null)` schreibt »null« (leeres Element übergeben); `tr.append(array)` flacht nicht ab (Spread); kein »NaN« im Text (sonst schlägt `TestAlleLeseRouten` an); Dialog-`cancel` nur behandeln, wenn `e.target === dlg`.
- CSS: `animation-fill-mode: backwards` (nicht `both`); `prefers-reduced-motion` beachten; `[hidden] { display:none !important }` steht ganz oben.
- Jahre vor `Settings.Jahr` gibt es nicht (Tests starten im Startjahr). Verbrauch eines Jahres wird im Folgejahr bezahlt; Zahlungen zählen im Rechnungsjahr.
- Schreibt ein Test in ein Datenverzeichnis: immer `mktemp -d`, nie alte Ordner im Scratchpad (veraltete Daten, Mehrplatz-Sperre).

## Prüfen vor dem Push
- `go vet ./... && go test ./...` (~35 s).
- Bei größeren Änderungen zusätzlich: `staticcheck -checks "all,-ST1005,-ST1000,-ST1003,-ST1021" ./...` (deutsche Fehlertexte sind gewollt), `deadcode .` (`externalURLAllowed` ist nur unter Build-Tag genutzt), `govulncheck ./...`, `go test -race ./...`, ESLint 9 (`@eslint/js` recommended + `globals.browser`) über die zusammengehängten `web/*.js`, Zufallstest `KM_FUZZ=1 KM_FUZZ_N=60 KM_FUZZ_SEED=n go test -run TestApiZufall`.
- UI mit Playwright (Chromium vorinstalliert, frisches Datenverzeichnis): alle Seiten am Rechner (1366 px) und am Handy (390 px); prüft auch »null«/»NaN« im Text. Nur der Lageplan darf am Handy breiter sein.
