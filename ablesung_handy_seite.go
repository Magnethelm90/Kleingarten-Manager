package main

// handySeite ist die eigenständige HTML-Seite für die Ablesung auf dem Handy: ohne Internet, ohne Programm, alle
// Eingaben bleiben im Browser des Handys. Das Platzhalter-Token wird durch die Daten ersetzt. Texte werden nur über
// textContent gesetzt, nie als HTML.
const handySeite = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Zählerablesung</title>
<style>
:root { --bg:#f6f7f5; --card:#fff; --text:#1f2a24; --muted:#5d6b63; --line:#d9dfda; --marke:#2f6b4f; --warn:#8a5a00; --warn-bg:#fff4d6; }
* { box-sizing:border-box; }
body { margin:0; font:16px/1.4 system-ui,-apple-system,Segoe UI,Roboto,sans-serif; background:var(--bg); color:var(--text); }
header { position:sticky; top:0; background:var(--marke); color:#fff; padding:12px 16px; z-index:2; }
header h1 { margin:0; font-size:18px; }
header div { font-size:13px; opacity:.9; }
main { padding:12px 16px 90px; max-width:640px; margin:0 auto; }
input[type=search] { width:100%; padding:12px; font-size:16px; border:1px solid var(--line); border-radius:8px; margin:6px 0 12px; }
.z { background:var(--card); border:1px solid var(--line); border-radius:10px; padding:12px; margin-bottom:10px; }
.z.fertig { border-left:5px solid var(--marke); }
.z h2 { margin:0 0 2px; font-size:16px; }
.z .nr { color:var(--muted); font-size:13px; margin-bottom:8px; }
.feld { display:flex; align-items:center; gap:8px; margin:6px 0; }
.feld label { width:74px; font-size:14px; }
.feld input { flex:1; min-width:0; padding:10px; font-size:18px; border:1px solid var(--line); border-radius:8px; }
.feld .vj { width:96px; text-align:right; color:var(--muted); font-size:13px; }
.warn { background:var(--warn-bg); color:var(--warn); border-radius:6px; padding:4px 8px; font-size:13px; margin-top:4px; }
footer { position:fixed; left:0; right:0; bottom:0; background:#fff; border-top:1px solid var(--line); padding:10px 16px; display:flex; gap:8px; align-items:center; }
footer .stand { flex:1; font-size:14px; color:var(--muted); }
button { font:inherit; padding:10px 14px; border-radius:8px; border:1px solid var(--marke); background:var(--marke); color:#fff; }
button.zweit { background:#fff; color:var(--marke); }
.hinweis { font-size:13px; color:var(--muted); margin:0 0 10px; }
textarea { width:100%; height:6em; font:12px monospace; }
</style>
</head>
<body>
<header><h1>Zählerablesung <span id="jahr"></span></h1><div id="verein"></div></header>
<main>
<p class="hinweis">Diese Datei funktioniert ohne Internet. Deine Eingaben bleiben auf diesem Handy gespeichert. Zum Schluss speicherst du das Ergebnis und liest es im Kleingarten-Manager wieder ein. Die Datei enthält Namen und ist vertraulich.</p>
<input type="search" id="suche" placeholder="Garten oder Name suchen" autocomplete="off">
<div id="liste"></div>
<div id="export" hidden><p class="hinweis">Falls das Speichern auf diesem Handy nicht klappt: Text kopieren, in einer Datei mit der Endung .json ablegen und im Programm auswählen.</p><textarea id="text" readonly></textarea></div>
</main>
<footer><span class="stand" id="stand"></span><button class="zweit" id="alsText" type="button">Als Text</button><button id="speichern" type="button">Ergebnis speichern</button></footer>
<script>
(function () {
  'use strict';
  var D = /*DATEN*/null;
  if (!D) { document.body.textContent = 'Keine Daten.'; return; }
  var KEY = 'km-ablesung-' + D.jahr + '-' + D.zeilen.length + '-' + (D.zeilen[0] ? D.zeilen[0].id : '');
  var eingaben = {};
  try { eingaben = JSON.parse(localStorage.getItem(KEY) || '{}') || {}; } catch (e) { eingaben = {}; }
  function sichern() { try { localStorage.setItem(KEY, JSON.stringify(eingaben)); } catch (e) { /* ohne Speicher weiterarbeiten */ } }
  function zahl(s) {
    s = String(s == null ? '' : s).trim().replace(',', '.');
    if (s === '') return null;
    var n = Number(s);
    return (n >= 0 && n < 1e9) ? n : -1;
  }
  function fmt(n) { return n == null ? '–' : String(n).replace('.', ','); }
  function el(tag, cls, text) { var e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; }
  document.getElementById('jahr').textContent = D.jahr;
  document.getElementById('verein').textContent = D.verein;
  var liste = document.getElementById('liste');
  var karten = [];
  function wert(z, art) {
    var e = eingaben[z.id] || {};
    return art in e ? e[art] : (art === 'wasser' ? z.wasserAkt : z.stromAkt);
  }
  function stand() {
    var fertig = 0;
    D.zeilen.forEach(function (z) {
      var w = wert(z, 'wasser'), s = wert(z, 'strom');
      var braucht = (z.wasserNr || z.wasserVJ != null) || (z.stromNr || z.stromVJ != null);
      var ok = braucht ? (((z.wasserNr || z.wasserVJ != null) ? w != null && w !== '' : true) && ((z.stromNr || z.stromVJ != null) ? s != null && s !== '' : true)) : false;
      if (ok) fertig++;
    });
    document.getElementById('stand').textContent = fertig + ' von ' + D.zeilen.length + ' Gärten erfasst';
  }
  function feld(z, art, titel, nr, vj, karte) {
    var zeile = el('div', 'feld');
    var l = el('label', null, titel); zeile.appendChild(l);
    var i = document.createElement('input');
    i.type = 'text'; i.inputMode = 'decimal'; i.autocomplete = 'off';
    var v = wert(z, art); i.value = v == null ? '' : String(v).replace('.', ',');
    var warn = el('div', 'warn'); warn.hidden = true;
    i.addEventListener('input', function () {
      var n = zahl(i.value);
      var e = eingaben[z.id] || (eingaben[z.id] = {});
      if (n !== null && n < 0) { warn.textContent = 'Bitte nur eine Zahl eintragen.'; warn.hidden = false; return; }
      e[art] = n; sichern();
      if (n != null && vj != null && n < vj) { warn.textContent = 'Kleiner als der Vorjahresstand – Zählerwechsel oder Tippfehler?'; warn.hidden = false; } else { warn.hidden = true; }
      karte.classList.toggle('fertig', kartenFertig(z));
      stand();
    });
    zeile.appendChild(i);
    zeile.appendChild(el('span', 'vj', 'Vorjahr ' + fmt(vj)));
    var box = el('div'); box.appendChild(zeile); box.appendChild(warn);
    return box;
  }
  function kartenFertig(z) {
    var w = wert(z, 'wasser'), s = wert(z, 'strom');
    var hatW = z.wasserNr || z.wasserVJ != null, hatS = z.stromNr || z.stromVJ != null;
    return !!(hatW || hatS) && (!hatW || (w != null && w !== '')) && (!hatS || (s != null && s !== ''));
  }
  D.zeilen.forEach(function (z) {
    var k = el('div', 'z' + (kartenFertig(z) ? ' fertig' : ''));
    k.appendChild(el('h2', null, 'Garten ' + z.garten + ' · ' + z.name));
    var nummern = [];
    if (z.wasserNr) nummern.push('Wasserzähler ' + z.wasserNr);
    if (z.stromNr) nummern.push('Stromzähler ' + z.stromNr);
    k.appendChild(el('div', 'nr', nummern.join(' · ') || 'Keine Zählernummer eingetragen'));
    k.appendChild(feld(z, 'wasser', 'Wasser', z.wasserNr, z.wasserVJ, k));
    k.appendChild(feld(z, 'strom', 'Strom', z.stromNr, z.stromVJ, k));
    k.dataset.suche = (z.garten + ' ' + z.name).toLowerCase();
    liste.appendChild(k); karten.push(k);
  });
  document.getElementById('suche').addEventListener('input', function (e) {
    var q = e.target.value.trim().toLowerCase();
    karten.forEach(function (k) { k.hidden = q !== '' && k.dataset.suche.indexOf(q) < 0; });
  });
  function ergebnis() {
    var out = { version: 1, jahr: D.jahr, staende: [] };
    D.zeilen.forEach(function (z) {
      var e = eingaben[z.id];
      if (!e) return;
      var w = typeof e.wasser === 'number' ? e.wasser : null, s = typeof e.strom === 'number' ? e.strom : null;
      if (w != null || s != null) out.staende.push({ id: z.id, wasser: w, strom: s });
    });
    return JSON.stringify(out);
  }
  document.getElementById('speichern').addEventListener('click', function () {
    var blob = new Blob([ergebnis()], { type: 'application/json' });
    var a = document.createElement('a');
    a.href = URL.createObjectURL(blob); a.download = 'Ablesung_Ergebnis_' + D.jahr + '.json';
    document.body.appendChild(a); a.click(); a.remove();
  });
  document.getElementById('alsText').addEventListener('click', function () {
    var b = document.getElementById('export'); b.hidden = !b.hidden;
    var t = document.getElementById('text'); t.value = ergebnis(); if (!b.hidden) { t.focus(); t.select(); }
  });
  stand();
})();
</script>
</body>
</html>`
