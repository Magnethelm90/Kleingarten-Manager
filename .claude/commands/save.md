---
description: PROGRESS.md aus der aktuellen Session aktualisieren, committen und pushen
---

Analysiere unsere aktuelle Session und aktualisiere die Datei `PROGRESS.md` nach der vorgegebenen Struktur.

Regeln:
- Fasse dich kurz (nur Stichpunkte).
- Liste exakt auf, welche `.go`-Dateien erstellt oder geändert wurden (`git status --short` und `git diff --name-only --diff-filter=A|M <letzter Stand> HEAD -- '*.go'`).
- Dokumentiere bekannte Bugs, Compiler-Fehler oder fehlschlagende `go test`-Läufe.
- Überschreibe veraltete Punkte unter »Zuletzt erledigt«.

Struktur von `PROGRESS.md`:
1. Aktuelles Feature / Modul
2. Zuletzt erledigt (Stichpunkte, betroffene Dateien)
3. Offene Bugs / Fehlgeschlagene Tests
4. Nächster Schritt

Außerdem: Neue oder geänderte Funktionen in `FEATURES.md` eintragen (nichts weglassen, nichts löschen, was noch gilt). `CLAUDE.md` nur bei neuen Regeln oder Fallen ergänzen. Danach committen und pushen.
