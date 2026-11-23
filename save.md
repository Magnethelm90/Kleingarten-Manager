# save

Analysiere unsere aktuelle Session und aktualisiere die Datei `PROGRESS.md` nach der vorgegebenen Struktur.

Regeln:
- Fasse dich kurz (nur Stichpunkte).
- Liste exakt auf, welche `.go`-Dateien erstellt oder geändert wurden.
- Dokumentiere bekannte Bugs, Compiler-Fehler oder fehlschlagende `go test`-Läufe.
- Überschreibe veraltete Punkte unter »Zuletzt erledigt«.

Struktur von `PROGRESS.md`:
1. Aktuelles Feature / Modul
2. Zuletzt erledigt (Stichpunkte, Go-Dateien: neu / geändert)
3. Offene Bugs / Fehlgeschlagene Tests
4. Nächster Schritt

Dateiliste holen: `git diff --name-only --diff-filter=A <merge-base> HEAD -- '*.go'` (neu), `--diff-filter=M` (geändert).
Danach committen und pushen.
