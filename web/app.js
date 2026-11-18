'use strict';
/* Kleingarten-Manager – Oberfläche. Alle Texte aus Daten werden nur über textContent/DOM-Knoten
   eingefügt (kein innerHTML), damit Namen o. Ä. nie als Code ausgeführt werden können. */

const S = { state: null, year: null, tab: 'uebersicht', adminTab: 'paechter', filter: '', selInvoice: null, invTab: 'pruefen', invView: 'archiv', eingabeMode: 'schnell', quickId: null, quickQ: '', payFilter: 'offen', payQ: '', importPreview: null, kasse: null, kasseLoading: false, kasseYear: null, kasseNurOhneBeleg: false, kasseVerlauf: null, kasseVerlaufLoading: false, remindDismissed: false, nurUnvollstaendig: false, archivSuche: '', archivAlle: null, archivAlleLoading: false, papierkorb: null, papierkorbLoading: false, papierkorbOffen: false, dashKasse: null, dashKasseLoading: false,
  mahnSel: new Set(), pruefLoginMode: false, protokoll: null, protokollLoading: false, updateCheck: null, updateChecking: false,
  sicherungDismissed: false, abschlussCheck: null, abschlussLoading: false, jubilaeen: null, jubilaeenLoading: false, datenschutz: null, datenschutzLoading: false, sicherungen: null, sicherungenLoading: false, sicherungenZeigen: false,
  dashJubilaeen: null, dashJubilaeenLoading: false, jubilaeumDismissed: false, globalSearchQ: '', tour: null, tourAutoChecked: false };

// ------------------------------------------------------------------ Hilfsfunktionen

function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  let value;
  for (const [k, v] of Object.entries(props || {})) {
    if (v === false || v == null) continue;
    if (k === 'class') el.className = v;
    else if (k === 'value') value = v;
    else if (k === 'checked') el.checked = !!v;
    else if (k.startsWith('on') && typeof v === 'function') el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? '' : String(v));
  }
  for (const kid of kids.flat(Infinity)) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  if (value !== undefined) el.value = value;
  return el;
}

function svgEl(tag, attrs, ...kids) {
  const el = document.createElementNS('http://www.w3.org/2000/svg', tag);
  for (const [k, v] of Object.entries(attrs || {})) if (v != null) el.setAttribute(k, v);
  for (const kid of kids.flat(Infinity)) {
    if (kid == null || kid === false) continue;
    el.append(kid instanceof Node ? kid : document.createTextNode(String(kid)));
  }
  return el;
}

// Kleine Strich-Symbole (24er-Raster, Farbe über currentColor), damit Übersicht und Kopf ohne Bilddateien
// auskommen. Jedes Symbol ist eine Liste von Pfaden.
const ICONS = {
  blatt: ['M5 19c0-9 5-14 15-14 0 10-5 15-14 15z', 'M5 20l9-9'],
  zaehler: ['M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18z', 'M12 12l4-4', 'M7.5 15.5h9'],
  rechnung: ['M7 3h7l4 4v14H7z', 'M14 3v4h4', 'M9.5 12h6', 'M9.5 16h6'],
  euro: ['M17 6.5A6.5 6.5 0 1 0 17 17.5', 'M5 10.5h8', 'M5 13.5h8'],
  kasse: ['M4 7h16v12H4z', 'M4 11h16', 'M8 15h3'],
  sicherung: ['M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6z', 'M9 12l2 2 4-4'],
  personen: ['M9 11a3.5 3.5 0 1 0 0-7a3.5 3.5 0 0 0 0 7z', 'M3 20c0-3.5 2.7-6 6-6s6 2.5 6 6', 'M16 4.5a3 3 0 0 1 0 6', 'M21 20c0-3-1.8-5.2-4.5-5.8'],
  warnung: ['M12 4l9 16H3z', 'M12 10v4', 'M12 17.5v.5'],
  fertig: ['M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18z', 'M8 12.5l2.7 2.7L16 9.8'],
  karte: ['M3 6l6-2 6 2 6-2v14l-6 2-6-2-6 2z', 'M9 4v14', 'M15 6v14'],
  kalender: ['M4 6h16v14H4z', 'M4 10h16', 'M8 3v5', 'M16 3v5', 'M8 14h3'],
  haus: ['M3 11l9-7 9 7', 'M5 10v10h14V10', 'M10 20v-6h4v6'],
  warteliste: ['M9 6h11', 'M9 12h11', 'M9 18h7', 'M4.5 6h.01', 'M4.5 12h.01', 'M4.5 18h.01'],
  bericht: ['M7 3h7l4 4v14H7z', 'M14 3v4h4', 'M10 18v-3', 'M12.5 18v-6', 'M15 18v-4'],
  regler: ['M4 7h9', 'M17 7h3', 'M15 5v4', 'M4 17h3', 'M11 17h9', 'M9 15v4'],
  stern: ['M12 3.5l2.6 5.3 5.8.8-4.2 4.1 1 5.8-5.2-2.8-5.2 2.8 1-5.8-4.2-4.1 5.8-.8z'],
  uhr: ['M12 3a9 9 0 1 0 0 18a9 9 0 0 0 0-18z', 'M12 7.5V12l3 2'],
  schloss: ['M6 11h12v9H6z', 'M8.5 11V8a3.5 3.5 0 0 1 7 0v3'],
  schluessel: ['M7.5 16.5a3.5 3.5 0 1 1 0-7a3.5 3.5 0 0 1 0 7z', 'M11 13h10', 'M18 13v3', 'M15 13v2'],
  daten: ['M12 3v11', 'M7.5 9.5L12 14l4.5-4.5', 'M4 15v4h16v-4'],
  akte: ['M3 6.5h6l2 2h10V19H3z', 'M3 11h18'],
  waage: ['M12 4v16', 'M7 20h10', 'M5 7h14', 'M5 7l-3 6a3 3 0 0 0 6 0z', 'M19 7l-3 6a3 3 0 0 0 6 0z'],
  menue: ['M4 7h16', 'M4 12h16', 'M4 17h16'],
  brief: ['M3 6h18v12H3z', 'M3 7l9 6 9-6'],
  kuendigung: ['M5 21V4h9v17', 'M14 4l5 2v15', 'M11 13h.01', 'M3 21h18'],
  werkzeug: ['M14.7 6.3a4 4 0 0 0 5 5L12 19a2.1 2.1 0 0 1-3-3z', 'M14.7 6.3L17 4l3 3-2.3 2.3'],
  drucker: ['M7 9V3h10v6', 'M5 9h14a2 2 0 0 1 2 2v6h-4', 'M7 17H3v-6a2 2 0 0 1 2-2', 'M7 14h10v7H7z'],
};

// Leerer Zustand: Symbol, kurze Überschrift, Erklärung und – wo sinnvoll – der Knopf zum nächsten Schritt.
// Illustrationen (3D-Renderings aus Blender, siehe docs/blender) für leere Seiten, je nach Thema
const ILLU = { personen: 'laube', karte: 'laube', kuendigung: 'laube', schluessel: 'laube', kalender: 'liste', stern: 'liste',
  rechnung: 'rechnung', euro: 'rechnung', kasse: 'rechnung', werkzeug: 'zaehler', zaehler: 'zaehler', warnung: 'baum', brief: 'liste', akte: 'liste', warteliste: 'liste' };
function leer(ic, titel, text, knopf, onclick) {
  const bild = ILLU[ic] ? h('img', { class: 'leer-bild', src: `/bilder/${ILLU[ic]}.svg`, alt: '', onerror: (e) => e.target.replaceWith(h('span', { class: 'leer-ico' }, icon(ic, 30))) }) : null;
  return h('div', { class: 'leer' }, bild || h('span', { class: 'leer-ico' }, icon(ic, 30)), h('b', null, titel), h('p', null, text),
    knopf ? h('button', { class: 'btn primary', onclick }, knopf) : null);
}
function icon(name, size) {
  return svgEl('svg', { viewBox: '0 0 24 24', width: size || 20, height: size || 20, fill: 'none', stroke: 'currentColor',
    'stroke-width': 1.8, 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true', class: 'ico' },
  (ICONS[name] || []).map((d) => svgEl('path', { d })));
}

// Einfaches Balkendiagramm ohne externe Bibliothek (reines inline-SVG, keine
// Fremdskripte nötig). data: [{label, value}]. valueFmt formatiert die Zahl
// über den Balken, negative Werte werden rot dargestellt.
function barChart(data, opts) {
  const { height = 150, valueFmt = (v) => nfFlex.format(v) } = opts || {};
  if (!data.length) return h('p', { class: 'hint' }, 'Noch keine Werte vorhanden.');
  const width = Math.max(260, data.length * 70);
  const padL = 6, padB = 32, padT = 18, padR = 6;
  const w = width - padL - padR, hgt = height - padT - padB;
  const max = Math.max(1, ...data.map((d) => Math.abs(d.value)));
  const bw = w / data.length;
  const svg = svgEl('svg', { viewBox: `0 0 ${width} ${height}`, width: '100%', height, class: 'barchart' });
  svg.append(svgEl('line', { x1: padL, y1: padT + hgt, x2: padL + w, y2: padT + hgt, class: 'axis' }));
  data.forEach((d, i) => {
    const bh = (Math.abs(d.value) / max) * hgt;
    const x = padL + i * bw + bw * 0.18;
    const y = padT + hgt - bh;
    svg.append(svgEl('rect', { x, y, width: bw * 0.64, height: Math.max(bh, 1), rx: 2, class: 'bar' + (d.value < 0 ? ' neg' : '') }));
    svg.append(svgEl('text', { x: x + bw * 0.32, y: y - 4, class: 'barval' }, valueFmt(d.value)));
    svg.append(svgEl('text', { x: x + bw * 0.32, y: height - 10, class: 'barlabel' }, d.label));
  });
  return h('div', { class: 'chartwrap' }, svg);
}

const nf2 = new Intl.NumberFormat('de-DE', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const nfFlex = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 2 });
const nfIn = new Intl.NumberFormat('de-DE', { maximumFractionDigits: 4, useGrouping: false });
const eur = (x) => nf2.format(x) + ' €';
const numIn = (v) => (v == null ? '' : nfIn.format(v));

// Zahlen deutsch oder englisch schreibbar: "2,5" "2.5" "1.234,5"; leer -> null; Unsinn -> NaN
function parseNum(s) {
  s = String(s ?? '').trim().replace(/[€\s\u00a0]/g, '');
  if (s === '') return null;
  if (s.includes(',')) s = s.replace(/\./g, '').replace(',', '.');
  else if ((s.match(/\./g) || []).length > 1) s = s.replace(/\./g, '');
  if (!/^-?\d*\.?\d+$|^-?\d+\.$/.test(s)) return NaN;
  return Number(s);
}

const cmpNr = (a, b) => a.mitgliedsnr.localeCompare(b.mitgliedsnr, 'de', { numeric: true });

async function api(method, url, body, isForm) {
  const opt = { method, headers: { 'X-GA-Request': '1' } };
  if (isForm) opt.body = body;
  else if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  let r;
  try {
    r = await fetch(url, opt);
  } catch (e) {
    const err = new Error('Keine Verbindung zum Programm. Läuft das Fenster mit dem Kleingarten-Manager noch?');
    err.network = true;
    throw err;
  }
  const isJson = (r.headers.get('content-type') || '').includes('json');
  const data = isJson ? await r.json() : null;
  if (!r.ok) {
    const err = new Error((data && data.error) || `Fehler ${r.status}`);
    err.status = r.status;
    throw err;
  }
  return data;
}

// Kleine Erklärung zum Aufklappen: »?« neben einer Beschriftung. Es ist immer nur eine offen; Klick daneben oder Esc schließt.
function hilfe(text) {
  const box = h('span', { class: 'hilfe-box', role: 'note' }, text);
  const knopf = h('button', { type: 'button', class: 'hilfe-i', 'aria-label': 'Erklärung anzeigen', 'aria-expanded': 'false' }, '?');
  const w = h('span', { class: 'hilfe' }, knopf, box);
  knopf.addEventListener('click', (e) => {
    e.preventDefault(); e.stopPropagation();
    const offen = w.classList.contains('offen');
    hilfenSchliessen();
    if (!offen) { w.classList.add('offen'); knopf.setAttribute('aria-expanded', 'true'); }
  });
  return w;
}
// Kurzerklärung je Seite (»?« neben der Überschrift): wozu die Seite dient und womit man anfängt.
const SEITEN_HILFE = {
  eingabe: 'Hier trägst du jedes Jahr die Zählerstände und Arbeitsstunden ein. Fang mit der Schnelleingabe an: Nummer tippen, Enter drücken, Werte eintragen.',
  rechnungen: 'Hier prüfst und stellst du die Rechnungen aus. Erster Schritt: Zählerstände eintragen, dann »Alle offenen ausstellen«.',
  zahlungen: 'Hier siehst du, wer schon bezahlt hat. Haken setzen = bezahlt. Wer überfällig ist, bekommt auf Wunsch eine Mahnung (nie automatisch).',
  termine: 'Der Jahresplan des Vereins: Arbeitseinsätze, Versammlungen, Ablesetermine. Mit »+ Termin« legst du den ersten an.',
  lageplan: 'Eine Karte der Anlage: jede Kachel ist ein Garten, die Farbe zeigt den Stand der Abrechnung. Ein Klick öffnet die Zählerstände.',
  paechter: 'Die Stammdaten aller Pächter. Erster Schritt: »+ Pächter« oder die Liste aus Excel übernehmen (Einstellungen → Import).',
  warteliste: 'Interessenten in der Reihenfolge ihrer Anmeldung. Wird ein Garten frei, schlägt das Programm den Nächsten vor.',
  kuendigungen: 'Eine Kündigung erfassen, die Frist prüfen und die Schritte bis zur Übergabe abhaken. Das Cockpit erinnert an Offenes.',
  jubilaeen: 'Wer wie lange Mitglied ist, z. B. für Ehrungen in der Versammlung. Das Datum kommt aus der Gartenhistorie.',
  maengel: 'Beim Rundgang gefundene Mängel mit Frist. Erster Schritt: »Begehung durchführen« und Garten für Garten durchgehen.',
  inventar: 'Wer hat welchen Schlüssel, wo steht welches Gerät. Ausgabe und Rückgabe werden im Verlauf festgehalten.',
  rundschreiben: 'Ein Brief des Vorstands an alle Pächter oder eine Auswahl, als Brief-PDF, Aushang oder E-Mail-Paket.',
  verwaltung: 'Beschlussbuch, Verträge und Ämter des Vereins an einem Ort. Beschlüsse trägst du nach jeder Versammlung ein.',
  kassenbericht: 'Einnahmen, Ausgaben und Kassenbestand des Jahres. Erster Schritt: Anfangsbestand prüfen, Ausgaben mit Beleg erfassen.',
  haushalt: 'Plan und Ist je Posten für das Jahr. Das Ist kommt laufend aus dem Kassenbericht.',
  spenden: 'Geldspenden erfassen und die Zuwendungsbestätigung für das Finanzamt ausstellen. Zuerst die Gemeinnützigkeit eintragen.',
  vorauszahlung: 'Monatsbeträge, die Pächter vorab überweisen. Betrag je Pächter eintragen, Eingänge erkennt der Kontoauszug.',
  sepa: 'Lastschriften einziehen: Mandate erfassen, dann die Datei für das Online-Banking erstellen.',
  berichte: 'Alles, was der Vorstand im Jahr abgeben muss: Jahresmeldung, Jahresbericht, Versammlungspaket und mehr.',
  jahreswechsel: 'Am Jahresende schließt du das Jahr ab. Die Prüfung oben zeigt, was vorher noch fehlt; sie blockiert nichts.',
  einstellungen: 'Vereinsname, Bankverbindung, Preise und Rechnungsdaten. Das bleibt in allen Folgejahren erhalten.',
  daten: 'Pächter importieren, alles als Excel exportieren und Sicherungen anlegen oder zurückspielen.',
  sicherheit: 'Mit einem Passwort schützt du die Verwaltung. Ohne Passwort kann jeder am Rechner Preise und Pächter ändern.',
  datenschutz: 'Auskunft, Löschfristen und Aufbewahrung nach DSGVO an einem Ort.',
  protokoll: 'Wer hat wann was geändert – zum Nachvollziehen, nicht zum Bearbeiten.',
};
const seitenHilfe = (key) => (SEITEN_HILFE[key] ? hilfe(SEITEN_HILFE[key]) : null);

function hilfenSchliessen() {
  document.querySelectorAll('.hilfe.offen').forEach((x) => { x.classList.remove('offen'); x.querySelector('.hilfe-i').setAttribute('aria-expanded', 'false'); });
}
document.addEventListener('click', (e) => { if (!e.target.closest('.hilfe-box')) hilfenSchliessen(); });
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && document.querySelector('.hilfe.offen')) { e.preventDefault(); e.stopPropagation(); hilfenSchliessen(); } }, true);

function toast(msg, kind, aktion) {
  const box = document.getElementById('toasts');
  const t = h('div', { class: 'toast ' + (kind || '') }, msg,
    aktion ? h('button', { class: 'toast-aktion', onclick: () => { t.remove(); aktion.fn(); } }, aktion.label) : null);
  box.append(t);
  setTimeout(() => t.remove(), aktion ? 10000 : kind === 'err' ? 7000 : kind === 'warn' ? 9000 : 3500);
}

// Fehler an einem Eingabefeld: Feld rot markieren, hineinspringen und die Meldung zeigen. Die Markierung verschwindet beim Tippen.
function feldFehler(el, msg) {
  toast(msg, 'err');
  if (!el) return false;
  el.classList.add('err');
  const weg = () => { el.classList.remove('err'); el.removeEventListener('input', weg); el.removeEventListener('change', weg); };
  el.addEventListener('input', weg); el.addEventListener('change', weg);
  el.scrollIntoView({ block: 'center', behavior: 'smooth' });
  el.focus();
  return false;
}

// Ungewöhnlicher Verbrauch: gleich beim Eintippen sagen, was auffällt (vielleicht ein Tippfehler), einmal je Meldung.
function verbrauchToast(p, res) {
  const alt = ((S.state && S.state.hinweise) || {})[p.id] || [];
  for (const t of (res.hinweise || [])) {
    if (!alt.includes(t)) toast(`Bitte prüfen – ${t}. Stimmt die Zahl?`, 'warn');
  }
}

// Nach dem Löschen: kurze Meldung mit »Rückgängig« (holt den Eintrag aus dem Papierkorb zurück).
function geloeschtToast(r, was, neuLaden) {
  const id = r && r.papierkorbId;
  toast(`${was} gelöscht`, 'ok', id ? { label: 'Rückgängig', fn: async () => {
    try { await api('POST', `/api/admin/papierkorb/${encodeURIComponent(id)}/wiederherstellen`); toast('Wiederhergestellt', 'ok'); await neuLaden(); } catch (e) { handleErr(e); }
  } } : null);
}

// Mehrere Einträge auf einmal in den Papierkorb legen; ein Rückgängig holt alle zurück.
async function sammelLoeschen(pfade, was, neuLaden) {
  const ids = [];
  try { for (const pfad of pfade) { const r = await api('DELETE', pfad); if (r && r.papierkorbId) ids.push(r.papierkorbId); } } catch (e) { handleErr(e); }
  await neuLaden();
  toast(`${was} gelöscht`, 'ok', ids.length ? { label: 'Rückgängig', fn: async () => {
    try { for (const id of ids) await api('POST', `/api/admin/papierkorb/${encodeURIComponent(id)}/wiederherstellen`); toast('Wiederhergestellt', 'ok'); await neuLaden(); } catch (e) { handleErr(e); }
  } } : null);
}

// Fehlermeldungen, die auf eine fehlende Vorbereitung hinweisen, bekommen gleich den Weg dorthin.
const FEHLER_HILFE = [
  [/keine Pächter angelegt|noch keine Gärten|keine Gärten mit Gartennummer/i, 'Pächter anlegen', 'admin:paechter'],
  [/Admin-Passwort festlegen/i, 'Passwort festlegen', 'admin:sicherheit'],
  [/Gläubiger-Identifikationsnummer/i, 'Zur Vorauszahlung', 'admin:vorauszahlung'],
  [/Angaben zur Gemeinnützigkeit/i, 'Zu den Spenden', 'admin:spenden'],
  [/Termine/i, 'Zu den Terminen', 'tab:termine'],
  [/viele Mängel/i, 'Zu den Mängeln', 'admin:maengel'],
  [/Schlüssel ist noch ausgegeben|Gerät ist noch verliehen/i, 'Zu Schlüssel & Geräte', 'admin:inventar'],
];

function handleErr(e) {
  if (e.status === 401) {
    S.state.loggedIn = false;
    render();
  }
  const hilfe = !e.network && FEHLER_HILFE.find(([re]) => re.test(e.message || ''));
  toast(e.message, 'err', hilfe ? { label: hilfe[1], fn: () => zielOeffnen(hilfe[2]) } : null);
}

let dialogZaehler = 0;
// Zuletzt gewählte Werte in Dialogen merken (nur dieser Rechner, nur Auswahlen ohne personenbezogene Angaben).
const merkeLesen = (k, vorgabe) => { try { return localStorage.getItem('km-zuletzt-' + k) || vorgabe; } catch (e) { return vorgabe; } };
const merkeSchreiben = (k, v) => { try { localStorage.setItem('km-zuletzt-' + k, v); } catch (e) { /* ohne Speicher geht es auch */ } };
// Entwürfe: Wird ein Formular-Dialog mit Eingaben ohne Speichern geschlossen, bleiben die Eingaben bis zum Neuladen
// im Arbeitsspeicher (nicht auf der Festplatte – sie können personenbezogen sein). Beim nächsten Öffnen desselben
// Dialogs mit denselben Ausgangswerten wird angeboten, sie zurückzuholen.
const entwuerfe = new Map();
function modal(title, body, buttons) {
  return new Promise((resolve) => {
    const titelId = 'dlg-titel-' + (++dialogZaehler);
    const dlg = h('dialog', { 'aria-labelledby': titelId });
    const vorher = document.activeElement;
    let gespeichert = false;
    const close = (val) => {
      entwurfMerken();
      dlg.close(); dlg.remove(); resolve(val);
      // Fokus zurück an die Stelle, von der aus der Dialog geöffnet wurde (Tastaturbedienung).
      if (vorher && vorher.isConnected && typeof vorher.focus === 'function') vorher.focus();
    };
    const foot = h('div', { class: 'dfoot' });
    for (const b of buttons.filter(Boolean)) {
      foot.append(h('button', {
        class: 'btn ' + (b.cls || ''), type: 'button',
        onclick: async () => {
          if (b.action && (await b.action()) === false) return;
          if (b.action) gespeichert = true; // Knöpfe mit Aktion speichern oder verarbeiten die Eingaben
          close(b.value);
        },
      }, b.label));
    }
    dlg.append(h('div', { class: 'dhead', id: titelId }, title), h('div', { class: 'dbody' }, body), foot);
    // »cancel« kommt auch von Datei-Feldern im Dialog (Auswahl abgebrochen) – nur das eigene Esc schließt.
    // Formular-Dialoge (mit »primary«-Knopf): Wer Eingaben gemacht hat und mit Esc schließt, wird vorher gefragt.
    const istFormular = buttons.some((b) => b && /primary/.test(b.cls || ''));
    // Passwörter, Dateien und versteckte Felder werden weder verglichen noch als Entwurf gemerkt.
    const felder = () => [...dlg.querySelectorAll('.dbody input, .dbody select, .dbody textarea')].filter((f) => !['file', 'search', 'password', 'hidden'].includes(f.type));
    const feldWerte = () => felder().map((f) => (f.type === 'checkbox' || f.type === 'radio' ? f.checked : f.value)).join('\u0001');
    let anfang = '';
    const entwurfSchluessel = () => (typeof title === 'string' ? title : dlg.querySelector('.dhead').textContent) + '\u0002' + anfang;
    function entwurfMerken() {
      if (!istFormular || !anfang) return; // ausgewählte Dateien werden nicht gemerkt, nur Text und Auswahlen
      const k = entwurfSchluessel();
      if (gespeichert || feldWerte() === anfang) { if (gespeichert) entwuerfe.delete(k); return; }
      entwuerfe.set(k, felder().map((f) => (f.type === 'checkbox' || f.type === 'radio' ? f.checked : f.value)));
      if (entwuerfe.size > 20) entwuerfe.delete(entwuerfe.keys().next().value);
    }
    dlg.addEventListener('cancel', async (e) => {
      if (e.target !== dlg) return;
      e.preventDefault();
      if (istFormular && anfang !== feldWerte() && !dlg.dataset.frage) {
        dlg.dataset.frage = '1';
        const ok = await confirmBox('Die Eingaben in diesem Fenster sind noch nicht gespeichert. Wirklich schließen? Beim nächsten Öffnen lassen sie sich zurückholen, bis das Programm beendet wird.', 'Schließen', true);
        delete dlg.dataset.frage;
        if (!ok) return;
      }
      close(undefined);
    });
    document.body.append(dlg);
    dlg.showModal();
    anfang = feldWerte();
    const alt = istFormular && entwuerfe.get(entwurfSchluessel());
    if (alt) {
      const hinweis = h('div', { class: 'banner info entwurf' }, 'Hier gibt es ungespeicherte Eingaben vom letzten Öffnen. ',
        h('button', { class: 'btn small', type: 'button', onclick: () => {
          hinweis.remove();
          // Feld für Feld setzen und die üblichen Ereignisse auslösen: Viele Dialoge übernehmen Eingaben erst über
          // oninput/onchange in ihre Daten. Baut ein Dialog dabei Felder um, wird abgebrochen statt falsch zugeordnet.
          for (let i = 0; i < alt.length; i++) {
            const fs = felder();
            if (fs.length !== alt.length) { toast('Die Eingaben ließen sich nicht vollständig zurückholen – bitte prüfen.', 'err'); return; }
            const f = fs[i];
            const auswahl = f.type === 'checkbox' || f.type === 'radio';
            if (auswahl ? f.checked === alt[i] : f.value === alt[i]) continue;
            if (auswahl) f.checked = alt[i]; else f.value = alt[i];
            f.dispatchEvent(new Event('input', { bubbles: true }));
            f.dispatchEvent(new Event('change', { bubbles: true }));
          }
        } }, 'Zurückholen'),
        ' ', h('button', { class: 'btn small', type: 'button', onclick: () => { entwuerfe.delete(entwurfSchluessel()); hinweis.remove(); } }, 'Verwerfen'));
      dlg.querySelector('.dbody').prepend(hinweis);
    }
  });
}

// Im eigenen Programmfenster (Go stellt kgmOpenExternal bereit) gibt es keine Browser-Tabs:
// target=_blank und window.open würden dort je nach Plattform nichts tun oder das App-Fenster
// verlassen. Dokumente (PDF, Belege) werden deshalb in einem Vorschau-Dialog angezeigt.
const inNativeWindow = () => typeof window.kgmOpenExternal === 'function';

function openViewer(url, title) {
  const frame = h('iframe', { src: url, title: title || 'Vorschau' });
  const done = modal(title || 'Vorschau', frame, [{ label: 'Schließen', value: true }]);
  const dlg = document.querySelector('dialog:last-of-type');
  if (dlg) dlg.classList.add('viewer');
  return done;
}

function openDocument(url, title) {
  if (inNativeWindow()) return openViewer(url, title);
  window.open(url, '_blank');
  return Promise.resolve();
}

document.addEventListener('click', (e) => {
  if (!inNativeWindow() || e.defaultPrevented) return;
  const a = e.target.closest && e.target.closest('a[target="_blank"]');
  if (!a || a.hasAttribute('download')) return;
  const u = new URL(a.href, location.href);
  if (u.origin !== location.origin) return;
  e.preventDefault();
  openViewer(a.href, a.textContent.trim() || 'Vorschau');
});

const confirmBox = (msg, okLabel, danger) =>
  modal('Bitte bestätigen', h('p', null, msg), [
    { label: 'Abbrechen', value: false },
    { label: okLabel || 'OK', cls: danger ? 'solid-danger' : 'primary', value: true },
  ]);

// ------------------------------------------------------------------ Laden

async function load() {
  const q = S.year ? `?year=${S.year}` : '';
  S.state = await api('GET', '/api/state' + q);
  S.year = S.state.year;
}

async function reload() {
  S.aufgaben = null; // »Was steht an?« nach jeder Änderung neu bestimmen
  S.schwundEingabe = null;
  try { await load(); render(); } catch (e) { handleErr(e); }
}

// ------------------------------------------------------------------ Grundgerüst

// Vereinsfarbe: aus einer Grundfarbe alle Abstufungen berechnen. Zu helle Farben werden abgedunkelt,
// bis weiße Schrift darauf gut lesbar ist (wie auf der Rechnung, siehe markenfarbe in farbe.go).
function hexRGB(hex) { const v = parseInt(hex.slice(1), 16); return [(v >> 16) & 255, (v >> 8) & 255, v & 255]; }
function helligkeit([r, g, b]) {
  const lin = (c) => { c /= 255; return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4; };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}
function mische([r, g, b], [r2, g2, b2], t) { return [r + (r2 - r) * t, g + (g2 - g) * t, b + (b2 - b) * t].map(Math.round); }
const rgbStr = (c) => `rgb(${c.join(',')})`;
function farbeAnwenden(hex) {
  const root = document.documentElement.style;
  const namen = ['--green', '--green-dark', '--green-mid', '--green-soft', '--green-tint', '--green-line', '--green-faint', '--ring'];
  if (!/^#[0-9a-f]{6}$/i.test(hex || '')) { namen.forEach((n) => root.removeProperty(n)); return; }
  let c = hexRGB(hex);
  for (let i = 0; i < 40 && 1.05 / (helligkeit(c) + 0.05) < 4.5; i++) c = c.map((x) => x * 0.92);
  c = c.map(Math.round);
  const weiss = [255, 255, 255], schwarz = [0, 0, 0];
  root.setProperty('--green', rgbStr(c));
  root.setProperty('--green-dark', rgbStr(mische(c, schwarz, 0.3)));
  root.setProperty('--green-mid', rgbStr(mische(c, weiss, 0.14)));
  root.setProperty('--green-soft', rgbStr(mische(c, weiss, 0.88)));
  root.setProperty('--green-tint', rgbStr(mische(c, weiss, 0.94)));
  root.setProperty('--green-line', rgbStr(mische(c, weiss, 0.75)));
  root.setProperty('--green-faint', `rgba(${c.join(',')},.10)`);
  root.setProperty('--ring', `0 0 0 3px rgba(${c.join(',')},.25)`);
}

// Seitenleiste: die Arbeit des Jahres oben, Verwaltung und Einstellungen darunter. Einträge mit »admin«
// gehören zum Admin-Bereich (Passwort, falls eingerichtet).
const NAV = [
  { gruppe: 'Abrechnung', items: [
    { tab: 'uebersicht', label: 'Übersicht', ic: 'haus' },
    { tab: 'eingabe', label: 'Zählerstände', ic: 'zaehler' },
    { tab: 'rechnungen', label: 'Rechnungen', ic: 'rechnung' },
    { tab: 'zahlungen', label: 'Zahlungen', ic: 'euro', reiter: [
      { tab: 'zahlungen', label: 'Zahlungen' },
      { admin: 'vorauszahlung', modul: 'vorauszahlung', label: 'Vorauszahlungen' },
      { admin: 'sepa', label: 'Lastschrift', nur: (st) => st.sepaAktiv }] }] },
  { gruppe: 'Verein', items: [
    { tab: 'lageplan', label: 'Lageplan', ic: 'karte' },
    { tab: 'termine', label: 'Termine', ic: 'kalender' },
    { admin: 'paechter', label: 'Pächter', ic: 'personen', tour: 'admin', reiter: [
      { admin: 'paechter', label: 'Alle Pächter' },
      { admin: 'warteliste', modul: 'warteliste', label: 'Warteliste' },
      { admin: 'kuendigungen', label: 'Kündigungen' },
      { admin: 'jubilaeen', modul: 'jubilaeen', label: 'Jubiläen' }] },
    { admin: 'maengel', label: 'Anlage', ic: 'warnung', reiter: [
      { admin: 'maengel', label: 'Mängel & Lauben' },
      { admin: 'inventar', modul: 'inventar', label: 'Schlüssel & Geräte' }] },
    { admin: 'rundschreiben', modul: 'rundschreiben', label: 'Rundschreiben', ic: 'brief' },
    { admin: 'verwaltung', label: 'Vereinsverwaltung', ic: 'akte' }] },
  { gruppe: 'Kasse & Berichte', items: [
    { admin: 'kassenbericht', label: 'Kasse', ic: 'kasse', reiter: [
      { admin: 'kassenbericht', label: 'Kassenbericht' },
      { admin: 'haushalt', modul: 'haushalt', label: 'Haushaltsplan' },
      { admin: 'spenden', modul: 'spenden', label: 'Spenden' }] },
    { admin: 'berichte', label: 'Berichte & Jahresabschluss', ic: 'bericht', reiter: [
      { admin: 'berichte', label: 'Berichte & Meldungen' },
      { admin: 'jahreswechsel', label: 'Jahreswechsel' }] }] },
  { gruppe: 'Einstellungen', items: [
    { admin: 'einstellungen', label: 'Einstellungen', ic: 'regler', reiter: [
      { admin: 'einstellungen', label: 'Preise & Einstellungen' },
      { admin: 'daten', label: 'Import, Export, Sicherung' }] },
    { admin: 'sicherheit', label: 'Sicherheit & Datenschutz', ic: 'sicherung', reiter: [
      { admin: 'sicherheit', label: 'Passwort' },
      { admin: 'datenschutz', label: 'Datenschutz' },
      { admin: 'protokoll', label: 'Änderungsprotokoll' }] }] },
];
// Bereiche: ausgeschaltete Zusatzbereiche (Einstellungen → Bereiche) erscheinen nicht im Menü.
const modulAn = (name) => !S.state || !S.state.module || S.state.module[name] !== false;
const navSichtbar = (x, st) => (!x.nur || x.nur(st)) && (!x.modul || modulAn(x.modul));
const navLabel = (x) => (x.label === 'Mängel & Lauben' && !modulAn('bauten') ? 'Mängel' : x.label);
// Ein Menüpunkt kann mehrere Seiten als Reiter bündeln (x.reiter); der erste Reiter ist die Seite selbst.
const navPasst = (y, tab = S.tab, adminTab = S.adminTab) => (y.admin ? tab === 'admin' && y.admin === adminTab : y.tab === tab);
const navEintrag = (tab, adminTab) => NAV.flatMap((g) => g.items).find((x) => (x.reiter || [x]).some((y) => navPasst(y, tab, adminTab)));

// Wechsel in einen Admin-Bereich: zwischengespeicherte Listen dieses Bereichs neu laden.
function adminTabWaehlen(id) {
  S.tab = 'admin'; S.adminTab = id; S.importPreview = null;
  if (id === 'protokoll') S.protokoll = null;
  if (id === 'jahreswechsel') S.abschluss = null;
  if (id === 'jubilaeen') S.jubilaeen = null;
  if (id === 'datenschutz') S.datenschutz = null;
  if (id === 'warteliste') S.warteliste = null;
  if (id === 'maengel') { S.maengel = null; S.nutzung = null; S.bauten = null; }
  if (id === 'spenden') S.spenden = null;
  if (id === 'rundschreiben') S.rundschreiben = null;
  if (id === 'inventar') S.inventar = null;
  if (id === 'kuendigungen') S.kuend = null;
  if (id === 'verwaltung') S.verw = null;
  if (id === 'haushalt') S.haushalt = null;
  if (id === 'einstellungen') S.pachtgrenze = null;
  if (id === 'kassenbericht') { S.pruefung = null; S.barkasse = null; }
  if (id === 'sepa') { S.sepa = null; S.sepaVorschau = null; }
  if (id === 'vorauszahlung') S.vz = null;
  if (id === 'paechter') S.uebergaben = null;
  if (id === 'berichte') S.meldung = null;
}

async function navGeh(x) {
  const st = S.state;
  if (x.admin) adminTabWaehlen(x.admin); else S.tab = x.tab;
  if (x.tab === 'uebersicht') { S.dashWarteliste = null; S.aufgaben = null; S.vj = null; }
  if ((S.tab === 'admin' || S.tab === 'uebersicht' || S.tab === 'termine') && S.year !== st.currentYear) S.year = st.currentYear;
  S.menueOffen = false;
  await reload();
  window.scrollTo(0, 0);
}

// Reiterleiste unter der Überschrift einer gebündelten Seite (nur, wenn mehr als ein Reiter sichtbar ist).
function reiterKopf() {
  const e = navEintrag(S.tab, S.adminTab);
  const sicht = e && e.reiter ? e.reiter.filter((y) => navSichtbar(y, S.state)) : [];
  if (sicht.length < 2) return null;
  return reiterLeiste(sicht.map((y) => ({ label: navLabel(y), aktiv: navPasst(y), geh: () => navGeh(y) })), navLabel(e));
}

// Einheitliche Reiter für Teile einer Seite (Filter und Ansichten bleiben die runden »subtabs«).
function reiterLeiste(eintraege, name) {
  return h('div', { class: 'reiter', role: 'tablist', 'aria-label': name || null },
    eintraege.map((r) => h('button', { role: 'tab', 'aria-selected': r.aktiv ? 'true' : 'false', class: r.aktiv ? 'active' : '',
      onclick: () => { if (!r.aktiv) r.geh(); } }, r.label)));
}

function seitenleiste(st) {
  const gesperrt = st.hasPassword && !st.loggedIn;
  return h('aside', { class: 'sidebar' + (S.menueOffen ? ' offen' : ''), 'aria-label': 'Bereiche' },
    NAV.map((g) => h('div', { class: 'nav-gruppe' },
      h('div', { class: 'nav-titel' }, g.gruppe),
      g.items.filter((x) => navSichtbar(x, st)).map((x) => {
        const aktiv = (x.reiter || [x]).some((y) => navPasst(y));
        return h('button', { class: 'nav-item' + (aktiv ? ' active' : ''), 'data-tour': x.tour || x.tab || null, title: navLabel(x),
          'aria-current': aktiv ? 'page' : null, onclick: () => navGeh(x) },
        icon(x.ic, 19), h('span', null, navLabel(x)), x.admin && gesperrt ? h('i', { class: 'nav-schloss', title: 'Nur mit Admin-Anmeldung' }, icon('schloss', 13)) : null);
      }))));
}

function render() {
  const st = S.state;
  farbeAnwenden(st.settings.farbe);
  const app = document.getElementById('app');
  app.replaceChildren();

  const istVollAdmin = !st.hasPassword || st.loggedIn;
  if (S.pruefLoginMode && !istVollAdmin) { app.append(viewPruefLogin()); return; }
  if (st.pruefLoggedIn && !istVollAdmin) { app.append(viewPruefShell()); return; }
  if (st.gesperrt) {
    // »Alles nur mit Anmeldung«: ohne Sitzung gibt es nur die Anmeldeseite
    app.append(h('header', { class: 'top' },
      h('div', { class: 'brand' }, st.settings.logo ? h('span', { class: 'logo bild' }, h('img', { src: `/api/logo?v=${encodeURIComponent(st.settings.logo)}`, alt: '' })) : h('span', { class: 'logo' }, icon('blatt', 20)),
        h('div', null, h('b', null, 'Kleingarten-Manager'), h('span', null, st.settings.vereinName)))),
    h('main', { class: 'nur-anmeldung' }, viewLogin()));
    return;
  }

  const yearSel = h('select', {
    onchange: async (e) => { S.year = Number(e.target.value); await reload(); },
    title: 'Abrechnungsjahr',
  }, st.years.map((y) => h('option', { value: y, selected: y === st.year },
    y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  if (S.tab === 'admin' || S.tab === 'uebersicht' || S.tab === 'termine') yearSel.disabled = true;

  if (!S.tourAutoChecked && istVollAdmin) {
    S.tourAutoChecked = true;
    if (!st.tutorialGesehen) setTimeout(geschichteZeigen, 60);
  }

  app.append(
    h('a', { class: 'sprunglink', href: '#inhalt', onclick: (e) => { e.preventDefault(); document.getElementById('inhalt')?.focus(); } }, 'Zum Inhalt springen'),
    h('header', { class: 'top' },
      h('button', { class: 'menue-knopf', title: 'Menü', 'aria-label': 'Menü', onclick: () => { S.menueOffen = !S.menueOffen; render(); } }, icon('menue', 20)),
      h('div', { class: 'brand' }, st.settings.logo ? h('span', { class: 'logo bild' }, h('img', { src: `/api/logo?v=${encodeURIComponent(st.settings.logo)}`, alt: '' })) : h('span', { class: 'logo' }, icon('blatt', 20)), h('div', null, h('b', null, 'Kleingarten-Manager'), h('span', { title: st.settings.vereinName }, st.settings.vereinName))),
      h('div', { class: 'spacer' }),
      globalSearchBox(st),
      h('div', null, h('label', null, 'Jahr'), yearSel),
      h('button', { class: 'quit help', 'data-tour': 'hilfe', onclick: hilfeMenue, title: 'Hilfe: Anleitung, Rundgang und Wörterbuch', 'aria-label': 'Hilfe' }, '?'),
      h('button', { class: 'quit', onclick: quitApp, title: 'Programm beenden' }, 'Beenden'),
    ),
    h('div', { class: 'shell' },
    seitenleiste(st),
    h('main', { id: 'inhalt', tabindex: '-1' },
      st.sperrHinweis ? h('div', { class: 'banner err' }, st.sperrHinweis) : null,
      demoBanner(st),
      sicherungBanner(st),
      openRechnungenBanner(st),
      st.readOnly && S.tab !== 'admin' && S.tab !== 'zahlungen' && S.tab !== 'termine'
        ? h('div', { class: 'banner info' }, `Das Jahr ${st.year} ist abgeschlossen. Du kannst es ansehen und Rechnungen neu ausdrucken, aber nichts mehr ändern.`)
        : null,
      S.tab === 'uebersicht' ? viewUebersicht() : S.tab === 'eingabe' ? viewEingabe() : S.tab === 'lageplan' ? viewLageplan()
        : S.tab === 'rechnungen' ? viewRechnungen() : S.tab === 'zahlungen' ? viewZahlungen() : S.tab === 'termine' ? viewTermine() : viewAdmin(),
      h('div', { class: 'footer' }, `Kleingarten-Manager ${st.version} · Copyright © ${new Date().getFullYear()} ${st.autor || ''} · MIT-Lizenz, ohne Gewähr · Daten liegen in: `, h('span', { class: 'mono' }, st.dataDir), ' · ', zoomKnoepfe()),
    )),
  );
  const tour = tourCard();
  if (tour) app.append(tour);
  tourMark();
  // neue Seite: sanft einblenden und Kennzahlen hochzählen (nicht bei jedem Neuaufbau derselben Seite)
  const ansicht = S.tab === 'admin' ? 'admin/' + S.adminTab : S.tab;
  if (ansicht !== S.letzteAnsicht) {
    S.letzteAnsicht = ansicht;
    seiteMerken(st);
    const main = app.querySelector('main');
    if (main && !bewegungReduziert()) { main.classList.add('ansicht-neu'); zahlenAnimieren(main); }
  }
  const kopf = app.querySelector('header.top');
  if (kopf) document.documentElement.style.setProperty('--kopf', kopf.offsetHeight + 'px');
}

// ------------------------------------------------------------------ Bewegung (dezent, abschaltbar über das System)

const bewegungReduziert = () => window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// Zahlen in Kennzahl-Karten zählen beim Öffnen der Seite kurz hoch (ganze Zahlen und Euro-Beträge).
function zahlenAnimieren(root) {
  const els = root.querySelectorAll('.card.stat .big, .hero-fakt b');
  for (const el of els) {
    const t = el.textContent.trim();
    let ziel, fmt;
    if (/^\d{1,6}$/.test(t)) { ziel = Number(t); fmt = (v) => String(Math.round(v)); }
    else if (/^-?[\d.]+,\d{2} €$/.test(t)) { ziel = parseNum(t.replace(' €', '')); fmt = (v) => eur(Math.round(v * 100) / 100); }
    else continue;
    if (!ziel || Number.isNaN(ziel)) continue;
    const start = performance.now(), dauer = 700;
    const schritt = (jetzt) => {
      if (!el.isConnected) return;
      const x = Math.min(1, (jetzt - start) / dauer), e = 1 - Math.pow(1 - x, 3);
      el.textContent = x < 1 ? fmt(ziel * e) : t;
      if (x < 1) requestAnimationFrame(schritt);
    };
    el.textContent = fmt(0);
    requestAnimationFrame(schritt);
  }
}

// Erfolgsmoment: kleine, ruhige Bestätigung mit gezeichnetem Haken (z. B. wenn alle Rechnungen bezahlt sind).
function erfolgsHinweis(titel, text) {
  const haken = svgEl('svg', { viewBox: '0 0 52 52', class: 'erfolg-haken', 'aria-hidden': 'true' },
    svgEl('circle', { cx: 26, cy: 26, r: 23 }), svgEl('path', { d: 'M15 27l7 7 15-16' }));
  const el = h('div', { class: 'erfolg', role: 'status' }, haken, h('div', null, h('b', null, titel), h('span', null, text)),
    h('button', { class: 'erfolg-zu', 'aria-label': 'Schließen', onclick: () => el.remove() }, '✕'));
  document.body.append(el);
  setTimeout(() => { el.classList.add('weg'); setTimeout(() => el.remove(), 400); }, 7000);
}
