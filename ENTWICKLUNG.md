# Entwicklung

Für alle, die den Kleingarten-Manager selbst bauen, ändern oder eine neue Version veröffentlichen wollen.
Wer das Programm nur nutzt, braucht nichts davon – die fertigen Dateien gibt es unter
[Releases](https://github.com/Magnethelm90/Kleingarten-Manager/releases/latest).

## Aufbau

- **Go** (Server, Berechnung, PDF, Excel) – ein einziges Programm ohne Datenbank; alle Daten stehen in einer
  JSON-Datei im Datenordner.
- **Oberfläche** in `web/` (HTML, CSS, JavaScript ohne Framework), wird ins Programm eingebettet und in einem eigenen
  Fenster angezeigt (Windows: WebView2, macOS: WebKit, Paket `webview/webview_go`).
- Der eingebaute Webserver lauscht nur auf `127.0.0.1`.
- Die Gartenillustration in `web/bilder/garten.webp` ist mit Blender gerendert; das Skript dazu liegt in
  `docs/blender/garten.py`.

## Bauen

Benötigt [Go](https://go.dev/dl/) in der Version aus `go.mod` (derzeit 1.26; ältere Go-Versionen laden die
passende automatisch nach). Das Programmfenster nutzt CGO und braucht deshalb einen C-Compiler. Programme mit
Fenster lassen sich nicht über Betriebssystemgrenzen hinweg bauen: Windows-Builds entstehen unter Windows,
macOS-Builds unter macOS. Unter Linux (zum Entwickeln) öffnet sich das Programm statt im eigenen Fenster im
Standardbrowser.

**Windows** (PowerShell, C-Compiler z. B. über [MSYS2/MinGW](https://www.msys2.org/)):

```
go test ./...
go build -trimpath -ldflags "-s -w -H windowsgui" -o Kleingarten-Manager.exe .
```

`-H windowsgui` unterdrückt das Konsolenfenster. Zum Testen lässt es sich weglassen, dann bleibt ein
Konsolenfenster mit den Log-Ausgaben offen.

**macOS** (der mitgelieferte `clang` baut Apple Silicon und Intel):

```
VERSION=1.5.1
CGO_ENABLED=1 GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.appVersion=$VERSION" -o Kleingarten-Manager-arm64 .
CGO_ENABLED=1 GOARCH=amd64 CC="clang -arch x86_64" go build -trimpath -ldflags "-s -w -X main.appVersion=$VERSION" -o Kleingarten-Manager-amd64 .
```

Beide Dateien kommen in `Kleingarten-Manager.app/Contents/MacOS/`, ein kleines Startskript wählt per `uname -m`
die passende aus. Wie das Bündel genau aussieht, steht in `.github/workflows/release.yml`.

**Zum Ausprobieren ohne Fenster** (jedes System): `go run . --no-browser --data ./testdaten`, dann im Browser
`http://127.0.0.1:8765/` öffnen. Der Ordner `testdaten` hält eure echten Daten getrennt.

## Prüfungen

Bei jedem Push auf `main` und bei jedem Pull Request prüft GitHub Actions:

- Formatierung (`gofmt`), `go vet` und alle Tests (`go test ./...`)
- bekannte Sicherheitslücken in Go und den verwendeten Paketen mit
  [govulncheck](https://go.dev/doc/security/vuln/), dem Prüfprogramm des Go-Teams
- den Bau des Windows- und des macOS-Programms; die Dateien liegen beim jeweiligen Lauf unter »Artifacts«

Lokal reicht meist `go vet ./... && go test ./...`. Für die Eingabeprüfung gibt es zusätzlich Fuzz-Tests, z. B.
`go test -run '^$' -fuzz FuzzTerminKalenderUndAushang -fuzztime 30s`.

## Neue Version veröffentlichen

Versionsnummern werden fortlaufend vergeben (1.5, 1.5.1, 1.5.2, …); die Update-Prüfung vergleicht sie Stelle für Stelle.

```
git tag v1.5.1
git push origin v1.5.1
```

Alternativ auf der GitHub-Seite unter »Releases« einen neuen Release mit neuem Tag anlegen. In beiden Fällen
baut GitHub `.exe` und `.app` mit dieser Versionsnummer und hängt sie an den Release an. Die Windows-Datei
erscheint sofort nach ihrem Bau, die Mac-Datei kommt dazu, sobald GitHub einen Mac-Rechner frei hat (das kann
dauern). Bleibt ein Bau-Job hängen oder wird abgebrochen, unter »Actions« den Lauf öffnen und »Re-run failed jobs«
wählen; schon angehängte Dateien bleiben. Das Programm zeigt die
Versionsnummer unten an und findet neue Versionen über »Nach Updates suchen«.

## Sicherheit im Detail

- **Nur lokal erreichbar:** Der Server lauscht auf `127.0.0.1`. Gegen Angriffe fremder Webseiten über den Browser
  schützen eine Prüfung von Host und Herkunft (Origin), ein Pflicht-Header bei jeder Änderung, Cookies mit
  `SameSite=Strict` und eine Content-Security-Policy ohne Inline-Skripte.
- **Passwörter:** Admin- und Kassenprüfer-Passwort sind optional (mindestens 8 Zeichen) und werden als
  PBKDF2-SHA256-Hash mit Salt und 600.000 Runden gespeichert. Nach 5 Fehlversuchen wird gesperrt; die Sperrzeit
  verdoppelt sich mit jeder Serie (30 Sekunden bis 15 Minuten).
- **Keine Code-Einschleusung:** Die Oberfläche fügt Daten nie als HTML ein (kein `innerHTML`); Kalenderdateien,
  Excel-Dateien und PDFs maskieren Sonderzeichen.
- **Hochgeladene Dateien** (Belege, Gartenakte, Logo): nur PDF und Fotos, der Inhalt muss zur Endung passen,
  höchstens 12 MB; gespeichert unter selbst vergebenen Namen. Logos werden neu als PNG gespeichert. Pfade aus der
  Datendatei werden vor jedem Zugriff geprüft, damit ein manipulierter Eintrag keine fremden Dateien öffnet oder löscht.
  Bilder werden in einer Sandbox ausgeliefert.
- **Kein Internet:** keine Telemetrie, keine Schlüssel oder Zugangsdaten im Code. Die einzige Verbindung nach außen
  ist »Nach Updates suchen«, und die nur auf Knopfdruck.
- Die Datendatei enthält personenbezogene Daten und gehört nicht in ein Repository (`.gitignore` schließt sie aus).

Sicherheitslücken bitte nicht öffentlich als Issue melden, sondern direkt an den Autor.
