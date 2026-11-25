# Projektstand: Kleingarten-Manager

Stand: 25.11.2026 · Version 1.6 (Release `v1.6` veröffentlicht, PR #15 gemergt) · Branch `claude/github-improvements-pi59o5`

## Aktuelles Feature / Modul
- Navigation: Zurück-Knopf, Klick auf Vereinsnamen = Übersicht, Einrichtungs-Knopf verschwindet nach der Einrichtung (noch nicht committet).

## Zuletzt erledigt
- Zurück-Knopf in der Kopfzeile (merkt bis zu 30 Seiten im Speicher, am Handy nur Symbol); Logo und Vereinsname führen zur Übersicht (`web/app.js`, `web/style.css`).
- Karte »Wie möchtest du anfangen?« mit »Programm einrichten« und der automatische Assistent beim ersten Start entfallen, sobald die Einrichtung einmal gespeichert wurde; in den Einstellungen bleibt der Assistent (`web/admin.js`, `web/uebersicht.js`).
- Neu: `FEATURES.md` (vollständige Funktionsliste), `CLAUDE.md` neu gegliedert, `/save`-Befehl (`.claude/commands/save.md`); `prozess.md` und `save.md` entfallen.
- Betroffene Go-Dateien: neu `einrichtung_fertig_test.go`; geändert `api.go`, `model.go`, `neuverein.go`.
- Geprüft: `go vet`, `go test ./...` grün; Browsertest (1366 px und 390 px): Zurück, Brand-Klick, Karte vor/nach der Einrichtung ohne Fehler.
- Davor: Version 1.6 veröffentlicht (Merge `09e9d61`, Release mit `.exe` und Mac-Zip).

## Offene Bugs / Fehlgeschlagene Tests
- Keine bekannten Fehler; keine Compiler-Fehler, keine fehlschlagenden Tests.
- Cockpit-Warnung »Wasserschwund« öffnet die eingeklappte Karte `kb-schwund` nicht selbst.
- Lageplan am Handy breiter als der Bildschirm (gewollt). Klick-Zufallstest öffnet das Handy-Menü nicht.
- Tag-Push aus der Sitzung ist gesperrt (403); Releases legt der Nutzer an.

## Nächster Schritt
- Änderungen committen und pushen; auf Wunsch neue Version (1.6.1) mit eigenem `NEUERUNGEN`-Eintrag.
- Optional: Cockpit-Ziel »Schwund« öffnet `kb-schwund` (`zielOeffnen()`).
