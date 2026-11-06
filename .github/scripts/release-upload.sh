#!/bin/bash
# Hängt eine Datei an den Release des aktuellen Tags an. Gibt es den Release noch nicht (z. B. weil der andere
# Job ihn gerade erst anlegt), wird er neu angelegt; wurde er schon auf der Webseite mit eigenem Text angelegt,
# bleibt der Text unverändert.
set -u
datei="$1"
for versuch in 1 2 3 4 5; do
  if gh release view "$GITHUB_REF_NAME" >/dev/null 2>&1; then
    gh release upload "$GITHUB_REF_NAME" "$datei" --clobber && exit 0
  else
    gh release create "$GITHUB_REF_NAME" "$datei" --title "Kleingarten-Manager $GITHUB_REF_NAME" --generate-notes && exit 0
  fi
  echo "Versuch $versuch fehlgeschlagen, neuer Versuch in 15 s"
  sleep 15
done
exit 1
