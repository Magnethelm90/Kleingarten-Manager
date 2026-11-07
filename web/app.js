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
const ILLU = { personen: 'laube', karte: 'laube', kuendigung: 'laube', schluessel: 'laube', kalender: 'sonne', stern: 'sonne',
  rechnung: 'kanne', euro: 'kanne', kasse: 'kanne', werkzeug: 'kanne', zaehler: 'kanne', warnung: 'beet', brief: 'baum', akte: 'baum', warteliste: 'baum' };
function leer(ic, titel, text, knopf, onclick) {
  const bild = ILLU[ic] ? h('img', { class: 'leer-bild', src: `/bilder/${ILLU[ic]}.webp`, alt: '', onerror: (e) => e.target.replaceWith(h('span', { class: 'leer-ico' }, icon(ic, 30))) }) : null;
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
  s = String(s ?? '').trim().replace(/[€\s ]/g, '');
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

function toast(msg, kind) {
  const box = document.getElementById('toasts');
  const t = h('div', { class: 'toast ' + (kind || '') }, msg);
  box.append(t);
  setTimeout(() => t.remove(), kind === 'err' ? 7000 : 3500);
}

function handleErr(e) {
  if (e.status === 401) {
    S.state.loggedIn = false;
    render();
  }
  toast(e.message, 'err');
}

function modal(title, body, buttons) {
  return new Promise((resolve) => {
    const dlg = h('dialog');
    const close = (val) => { dlg.close(); dlg.remove(); resolve(val); };
    const foot = h('div', { class: 'dfoot' });
    for (const b of buttons) {
      foot.append(h('button', {
        class: 'btn ' + (b.cls || ''), type: 'button',
        onclick: async () => {
          if (b.action && (await b.action()) === false) return;
          close(b.value);
        },
      }, b.label));
    }
    dlg.append(h('div', { class: 'dhead' }, title), h('div', { class: 'dbody' }, body), foot);
    dlg.addEventListener('cancel', (e) => { e.preventDefault(); close(undefined); });
    document.body.append(dlg);
    dlg.showModal();
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
    { tab: 'zahlungen', label: 'Zahlungen', ic: 'euro' }] },
  { gruppe: 'Verein', items: [
    { tab: 'lageplan', label: 'Lageplan', ic: 'karte' },
    { tab: 'termine', label: 'Termine', ic: 'kalender' },
    { admin: 'paechter', label: 'Pächter', ic: 'personen', tour: 'admin' },
    { admin: 'kuendigungen', label: 'Kündigungen', ic: 'kuendigung' },
    { admin: 'warteliste', label: 'Warteliste', ic: 'warteliste' },
    { admin: 'maengel', label: 'Mängel', ic: 'warnung' },
    { admin: 'rundschreiben', label: 'Rundschreiben', ic: 'brief' },
    { admin: 'inventar', label: 'Schlüssel & Geräte', ic: 'schluessel' },
    { admin: 'verwaltung', label: 'Beschlüsse, Verträge, Ämter', ic: 'akte' }] },
  { gruppe: 'Kasse & Berichte', items: [
    { admin: 'kassenbericht', label: 'Kassenbericht', ic: 'kasse' },
    { admin: 'haushalt', label: 'Haushaltsplan', ic: 'waage' },
    { admin: 'spenden', label: 'Spenden', ic: 'stern' },
    { admin: 'vorauszahlung', label: 'Vorauszahlungen', ic: 'kalender' },
    { admin: 'sepa', label: 'Lastschrift', ic: 'euro', nur: (st) => st.sepaAktiv },
    { admin: 'berichte', label: 'Berichte & Meldungen', ic: 'bericht' },
    { admin: 'jahreswechsel', label: 'Jahreswechsel', ic: 'fertig' }] },
  { gruppe: 'Einstellungen', items: [
    { admin: 'einstellungen', label: 'Preise & Einstellungen', ic: 'regler' },
    { admin: 'daten', label: 'Import, Export, Sicherung', ic: 'daten' },
    { admin: 'jubilaeen', label: 'Jubiläen', ic: 'stern' },
    { admin: 'protokoll', label: 'Änderungsprotokoll', ic: 'uhr' },
    { admin: 'datenschutz', label: 'Datenschutz', ic: 'sicherung' },
    { admin: 'sicherheit', label: 'Passwort', ic: 'schluessel' }] },
];
const navEintrag = (tab, adminTab) => NAV.flatMap((g) => g.items).find((x) => (x.admin ? tab === 'admin' && x.admin === adminTab : x.tab === tab));

// Wechsel in einen Admin-Bereich: zwischengespeicherte Listen dieses Bereichs neu laden.
function adminTabWaehlen(id) {
  S.tab = 'admin'; S.adminTab = id; S.importPreview = null;
  if (id === 'protokoll') S.protokoll = null;
  if (id === 'jubilaeen') S.jubilaeen = null;
  if (id === 'datenschutz') S.datenschutz = null;
  if (id === 'warteliste') S.warteliste = null;
  if (id === 'maengel') { S.maengel = null; S.nutzung = null; }
  if (id === 'spenden') S.spenden = null;
  if (id === 'rundschreiben') S.rundschreiben = null;
  if (id === 'inventar') S.inventar = null;
  if (id === 'kuendigungen') S.kuend = null;
  if (id === 'verwaltung') S.verw = null;
  if (id === 'haushalt') S.haushalt = null;
  if (id === 'einstellungen') S.pachtgrenze = null;
  if (id === 'kassenbericht') S.pruefung = null;
  if (id === 'sepa') { S.sepa = null; S.sepaVorschau = null; }
  if (id === 'vorauszahlung') S.vz = null;
  if (id === 'paechter') S.uebergaben = null;
  if (id === 'berichte') S.meldung = null;
}

function seitenleiste(st) {
  const gesperrt = st.hasPassword && !st.loggedIn;
  const geh = async (x) => {
    if (x.admin) adminTabWaehlen(x.admin); else S.tab = x.tab;
    if (x.tab === 'uebersicht') { S.dashWarteliste = null; S.aufgaben = null; }
    if ((S.tab === 'admin' || S.tab === 'uebersicht' || S.tab === 'termine') && S.year !== st.currentYear) S.year = st.currentYear;
    S.menueOffen = false;
    await reload();
    window.scrollTo(0, 0);
  };
  return h('aside', { class: 'sidebar' + (S.menueOffen ? ' offen' : ''), 'aria-label': 'Bereiche' },
    NAV.map((g) => h('div', { class: 'nav-gruppe' },
      h('div', { class: 'nav-titel' }, g.gruppe),
      g.items.filter((x) => !x.nur || x.nur(st)).map((x) => {
        const aktiv = x.admin ? S.tab === 'admin' && S.adminTab === x.admin : S.tab === x.tab;
        return h('button', { class: 'nav-item' + (aktiv ? ' active' : ''), 'data-tour': x.tour || x.tab || null, title: x.label,
          'aria-current': aktiv ? 'page' : null, onclick: () => geh(x) },
        icon(x.ic, 19), h('span', null, x.label), x.admin && gesperrt ? h('i', { class: 'nav-schloss', title: 'Nur mit Admin-Anmeldung' }, icon('schloss', 13)) : null);
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
    h('header', { class: 'top' },
      h('button', { class: 'menue-knopf', title: 'Menü', 'aria-label': 'Menü', onclick: () => { S.menueOffen = !S.menueOffen; render(); } }, icon('menue', 20)),
      h('div', { class: 'brand' }, st.settings.logo ? h('span', { class: 'logo bild' }, h('img', { src: `/api/logo?v=${encodeURIComponent(st.settings.logo)}`, alt: '' })) : h('span', { class: 'logo' }, icon('blatt', 20)), h('div', null, h('b', null, 'Kleingarten-Manager'), h('span', { title: st.settings.vereinName }, st.settings.vereinName))),
      h('div', { class: 'spacer' }),
      globalSearchBox(st),
      h('div', null, h('label', null, 'Jahr'), yearSel),
      h('button', { class: 'quit help', 'data-tour': 'hilfe', onclick: () => geschichteZeigen(), title: 'Hilfe: kurze Einführung, was wohin gehört', 'aria-label': 'Hilfe' }, '?'),
      h('button', { class: 'quit', onclick: quitApp, title: 'Programm beenden' }, 'Beenden'),
    ),
    h('div', { class: 'shell' },
    seitenleiste(st),
    h('main', null,
      st.sperrHinweis ? h('div', { class: 'banner err' }, '🔒 ', st.sperrHinweis) : null,
      demoBanner(st),
      sicherungBanner(st),
      openRechnungenBanner(st),
      st.readOnly && S.tab !== 'admin' && S.tab !== 'zahlungen' && S.tab !== 'termine'
        ? h('div', { class: 'banner info' }, `Das Jahr ${st.year} ist abgeschlossen. Du kannst es ansehen und Rechnungen neu ausdrucken, aber nichts mehr ändern.`)
        : null,
      S.tab === 'uebersicht' ? viewUebersicht() : S.tab === 'eingabe' ? viewEingabe() : S.tab === 'lageplan' ? viewLageplan()
        : S.tab === 'rechnungen' ? viewRechnungen() : S.tab === 'zahlungen' ? viewZahlungen() : S.tab === 'termine' ? viewTermine() : viewAdmin(),
      h('div', { class: 'footer' }, `Kleingarten-Manager ${st.version} · Copyright © ${new Date().getFullYear()} ${st.autor || ''} · Daten liegen in: `, h('span', { class: 'mono' }, st.dataDir), ' · ', zoomKnoepfe()),
    )),
  );
  const tour = tourCard();
  if (tour) app.append(tour);
  tourMark();
  // neue Seite: sanft einblenden und Kennzahlen hochzählen (nicht bei jedem Neuaufbau derselben Seite)
  const ansicht = S.tab === 'admin' ? 'admin/' + S.adminTab : S.tab;
  if (ansicht !== S.letzteAnsicht) {
    S.letzteAnsicht = ansicht;
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

// ------------------------------------------------------------------ Einführungsrundgang

// Jeder Schritt wechselt selbst zur passenden Stelle im Programm und markiert den
// zugehörigen Reiter, die Seite dahinter bleibt bedienbar.
const TOUR = [
  { titel: 'Willkommen im Kleingarten-Manager',
    text: 'In 5 kurzen Schritten siehst du, was wohin gehört. Die Leiste links führt dich von oben nach unten durch ein Abrechnungsjahr, und ich zeige dir jede Stelle direkt im Programm. Alles wird sofort gespeichert, und vor heiklen Schritten legt das Programm automatisch eine Sicherung an.',
    ziel: 'uebersicht', gehe: () => { S.tab = 'uebersicht'; } },
  { titel: '1. Einmalig einrichten',
    text: 'Unter »Pächter« legst du zuerst die Pächter an (einzeln oder per Excel/CSV-Import) und trägst bei »Preise & Einstellungen« Vereinsname, Bankverbindung, Preise und Rechnungsdatum ein. Das bleibt in allen Folgejahren erhalten.',
    ziel: 'admin', gehe: () => { adminTabWaehlen('paechter'); } },
  { titel: '2. Zählerstände eintragen',
    text: 'Das ist die Arbeit jedes Jahr: Nummer eintippen, Enter drücken, neue Zählerstände und Arbeitsstunden eingeben. Name, Größe und Vorjahresstände sind schon da, gespeichert wird automatisch.',
    ziel: 'eingabe', gehe: () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; } },
  { titel: '3. Rechnungen ausstellen',
    text: 'Hier prüfst du jede Rechnung und stellst sie aus. Sie wird als PDF gespeichert und archiviert, spätere Änderungen verändern sie nicht mehr. »Alle offenen ausstellen« erledigt alle auf einmal.',
    ziel: 'rechnungen', gehe: () => { S.tab = 'rechnungen'; S.invTab = 'pruefen'; } },
  { titel: '4. Zahlungen verfolgen',
    text: 'Setze den Haken, sobald eine Rechnung bezahlt ist (Teilzahlungen gehen über »Details«). Für überfällige Rechnungen erzeugst du hier die Mahnung, es wird nie automatisch gemahnt.',
    ziel: 'zahlungen', gehe: () => { S.tab = 'zahlungen'; } },
  { titel: '5. Jahresabschluss & Sicherheit',
    text: 'Am Jahresende schließt »Jahreswechsel« das Jahr ab, mit Checkliste. Deine Daten werden automatisch gesichert; unter »Passwort« schützt du die Verwaltung. Das Suchfeld oben findet jeden Pächter, und diese Einführung erreichst du jederzeit über den runden »?«-Knopf oben rechts.',
    ziel: 'hilfe', gehe: () => { adminTabWaehlen('jahreswechsel'); } },
];

// Willkommen: fünf Bilder mit kurzem Text, danach auf Wunsch der Rundgang durchs Programm.
const GESCHICHTE = [
  { bild: 'garten', titel: 'Willkommen im Kleingarten-Manager', text: 'Das Vereinsbüro für euren Kleingartenverein: Abrechnung, Pächter, Kasse und alles drumherum. Alles liegt auf diesem Rechner – ohne Cloud, ohne Anmeldung.' },
  { bild: 'beet', titel: 'Zählerstände in Minuten', text: 'Gartennummer eintippen, Wasser und Strom eingeben, fertig. Vorjahresstände kennt das Programm schon, gerechnet wird automatisch – auch Pacht, Beiträge und Stunden.' },
  { bild: 'kanne', titel: 'Rechnungen, die bezahlt werden', text: 'Jede Rechnung als PDF mit QR-Code für die Banking-App, per Post oder E-Mail. Zahlungen hakst du ab oder liest sie aus dem Kontoauszug ein, Mahnungen gibt es auf Knopfdruck.' },
  { bild: 'laube', titel: 'Der ganze Verein an einem Ort', text: 'Lageplan, Warteliste, Kündigungen, Termine, Rundschreiben, Beschlüsse und Verträge. Was gerade ansteht, sagt dir die Übersicht von selbst.' },
  { bild: 'sonne', titel: 'Bereit?', text: 'Der kurze Rundgang zeigt dir in fünf Schritten, wo was ist. Oder du schaust dich selbst um – ausprobieren geht gefahrlos mit den Beispieldaten.' },
];

function geschichteZeigen() {
  if (document.querySelector('.geschichte')) return;
  S.tour = null; render();
  let i = 0;
  const dlg = h('dialog', { class: 'geschichte', 'aria-label': 'Willkommen' });
  const buehne = h('div', { class: 'gs-buehne' });
  const punkte = h('div', { class: 'gs-punkte' });
  const zurueck = h('button', { class: 'btn' }, 'Zurück');
  const weiter = h('button', { class: 'btn primary' });
  const selbst = h('button', { class: 'btn' }, 'Selbst umschauen');
  const ende = (rundgang) => { dlg.close(); dlg.remove(); if (rundgang) tourGo(1); else tourEnd(); };
  const zeigen = (neu, richtung) => {
    i = neu;
    const g = GESCHICHTE[i];
    const folie = h('div', { class: 'gs-folie' + (richtung ? ' rein-' + richtung : '') },
      h('div', { class: 'gs-bild' }, h('img', { src: `/bilder/${g.bild}.webp`, alt: '' })),
      h('div', { class: 'gs-text' }, h('span', { class: 'gs-schritt' }, `${i + 1} von ${GESCHICHTE.length}`), h('h2', null, g.titel), h('p', null, g.text)));
    const alt = buehne.firstChild;
    if (alt && richtung && !bewegungReduziert()) { alt.classList.add('raus-' + richtung); setTimeout(() => alt.remove(), 320); } else if (alt) alt.remove();
    buehne.append(folie);
    punkte.replaceChildren(...GESCHICHTE.map((_, k) => h('button', { class: k === i ? 'an' : '', 'aria-label': `Bild ${k + 1}`, onclick: () => k !== i && zeigen(k, k > i ? 'rechts' : 'links') })));
    zurueck.style.visibility = i ? 'visible' : 'hidden';
    const letzte = i === GESCHICHTE.length - 1;
    weiter.textContent = letzte ? 'Rundgang starten' : 'Weiter';
    selbst.style.display = letzte ? '' : 'none';
  };
  zurueck.addEventListener('click', () => i > 0 && zeigen(i - 1, 'links'));
  weiter.addEventListener('click', () => (i === GESCHICHTE.length - 1 ? ende(true) : zeigen(i + 1, 'rechts')));
  selbst.addEventListener('click', () => ende(false));
  dlg.addEventListener('cancel', (e) => { e.preventDefault(); ende(false); });
  dlg.addEventListener('keydown', (e) => {
    if (e.key === 'ArrowRight' && i < GESCHICHTE.length - 1) zeigen(i + 1, 'rechts');
    if (e.key === 'ArrowLeft' && i > 0) zeigen(i - 1, 'links');
  });
  dlg.append(h('button', { class: 'gs-zu', 'aria-label': 'Überspringen', title: 'Überspringen (Esc)', onclick: () => ende(false) }, 'Überspringen ✕'),
    buehne, h('div', { class: 'gs-fuss' }, zurueck, punkte, h('div', { class: 'gs-knoepfe' }, selbst, weiter)));
  document.body.append(dlg);
  zeigen(0, null);
  dlg.showModal();
  weiter.focus();
}

async function tourGo(i) {
  S.tour = { i };
  const step = TOUR[i];
  if (step.gehe) step.gehe();
  if ((S.tab === 'admin' || S.tab === 'uebersicht') && S.year !== S.state.currentYear) S.year = S.state.currentYear;
  await reload();
}

function tourEnd() {
  S.tour = null;
  if (S.state && !S.state.tutorialGesehen) {
    S.state.tutorialGesehen = true;
    api('POST', '/api/tutorial').catch(() => { /* beim nächsten Start erneut anbieten */ });
  }
  render();
}

function tourCard() {
  if (!S.tour) return null;
  const i = S.tour.i, step = TOUR[i], last = i === TOUR.length - 1;
  return h('div', { class: 'tour', role: 'dialog', 'aria-label': 'Einführung' },
    h('button', { class: 'skip', onclick: tourEnd, title: 'Einführung beenden (Esc)' }, last ? 'Schließen ✕' : 'Überspringen ✕'),
    h('div', { class: 'tstep' }, i === 0 ? 'Einführung' : `Schritt ${i} von ${TOUR.length - 1}`),
    h('h3', null, step.titel),
    h('p', null, step.text),
    h('div', { class: 'tfoot' },
      h('div', { class: 'dots' }, TOUR.map((_, k) => h('i', { class: k === i ? 'on' : '' }))),
      i > 0 ? h('button', { class: 'btn small', onclick: () => tourGo(i - 1) }, 'Zurück') : null,
      h('button', { class: 'btn small primary', onclick: () => (last ? tourEnd() : tourGo(i + 1)) }, last ? 'Fertig' : 'Weiter')));
}

function tourMark() {
  document.querySelectorAll('.tour-hl').forEach((el) => el.classList.remove('tour-hl'));
  if (!S.tour) return;
  const el = document.querySelector(`[data-tour="${TOUR[S.tour.i].ziel}"]`);
  if (el) el.classList.add('tour-hl');
}

document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && S.tour && !document.querySelector('dialog[open]')) tourEnd();
});

// Globale Suche im Kopfbereich: springt von jeder Seite direkt zur
// Schnellansicht eines Pächters (Nummer, Gartennummer, Name oder Straße).
function globalSearchBox(st) {
  const list = st.paechter || [];
  const find = (q) => {
    q = q.trim().toLowerCase();
    if (!q) return [];
    return list.filter((p) => [p.mitgliedsnr, p.gartennr, p.name, p.strasse].some((x) => (x || '').toLowerCase().includes(q)))
      .sort(cmpNr).slice(0, 8);
  };
  const results = h('div', { class: 'globalsearch-results' });
  const goTo = (p) => {
    S.tab = 'eingabe';
    S.eingabeMode = 'schnell';
    S.quickId = p.id;
    S.globalSearchQ = '';
    render();
  };
  const draw = () => {
    results.replaceChildren();
    const m = find(S.globalSearchQ);
    if (!S.globalSearchQ.trim()) { results.classList.remove('open'); return; }
    results.classList.add('open');
    if (!m.length) { results.append(h('div', { class: 'gs-empty' }, 'Nichts gefunden.')); return; }
    for (const p of m)
      results.append(h('button', { class: 'gs-item', type: 'button', onclick: () => goTo(p) },
        h('b', null, p.gartennr || p.mitgliedsnr), ' ', p.name,
        h('span', { class: 'hint' }, p.mitgliedsnr)));
  };
  const input = h('input', {
    type: 'search', placeholder: '🔍 Suchen', title: 'Pächter suchen (Name, Mitglieds- oder Gartennummer)', class: 'gs-input', autocomplete: 'off',
    value: S.globalSearchQ,
    oninput: (e) => { S.globalSearchQ = e.target.value; draw(); },
    onkeydown: (e) => {
      if (e.key === 'Escape') { S.globalSearchQ = ''; input.value = ''; draw(); input.blur(); }
      if (e.key !== 'Enter') return;
      const m = find(S.globalSearchQ);
      if (m.length === 1) goTo(m[0]);
    },
    onblur: () => setTimeout(() => { results.classList.remove('open'); }, 150),
    onfocus: () => draw(),
  });
  draw();
  return h('div', { class: 'globalsearch' }, input, results);
}

// ---- Schriftgröße (Anzeige), nur in diesem Programm; wird im Browser-Speicher gemerkt
const ZOOM_STUFEN = [0.9, 1, 1.15, 1.3, 1.5];
function zoomLaden() {
  try { const v = Number(localStorage.getItem('kgm-zoom')); return ZOOM_STUFEN.includes(v) ? v : 1; } catch (e) { return 1; }
}
function zoomSetzen(v) {
  document.documentElement.style.zoom = v === 1 ? '' : String(v);
  try { localStorage.setItem('kgm-zoom', String(v)); } catch (e) { /* ohne Speicher nur für diese Sitzung */ }
}
function zoomKnoepfe() {
  const i = ZOOM_STUFEN.indexOf(zoomLaden());
  const knopf = (label, titel, ziel, aus) => h('button', { class: 'zoombtn', title: titel, disabled: aus, onclick: () => { zoomSetzen(ziel); render(); } }, label);
  return h('span', { class: 'zoom' }, 'Anzeige: ',
    knopf('A−', 'Schrift kleiner', ZOOM_STUFEN[Math.max(0, i - 1)], i <= 0), ' ',
    knopf('A', 'Normale Größe', 1, i === 1), ' ',
    knopf('A+', 'Schrift größer', ZOOM_STUFEN[Math.min(ZOOM_STUFEN.length - 1, i + 1)], i >= ZOOM_STUFEN.length - 1), ' ',
    h('button', { class: 'zoombtn' + (lesbarLaden() ? ' an' : ''), 'aria-pressed': String(lesbarLaden()),
      title: 'Kräftigere Schrift, stärkere Kontraste und größere Knöpfe', onclick: () => { lesbarSetzen(!lesbarLaden()); render(); } },
    lesbarLaden() ? '✓ Gut lesbar' : 'Gut lesbar'));
}
zoomSetzen(zoomLaden());

// »Gut lesbar«: kräftigere Schrift, mehr Kontrast, größere Bedienelemente (je Rechner gemerkt).
function lesbarLaden() {
  try { return localStorage.getItem('kgm-lesbar') === '1'; } catch (e) { return document.documentElement.classList.contains('gutlesbar'); }
}
function lesbarSetzen(an) {
  document.documentElement.classList.toggle('gutlesbar', an);
  try { localStorage.setItem('kgm-lesbar', an ? '1' : '0'); } catch (e) { /* nur für diese Sitzung */ }
}
lesbarSetzen(lesbarLaden());

// Erinnerung an offene Rechnungen des laufenden Jahres, direkt nach dem Öffnen sichtbar.
// Bleibt bis zum nächsten Programmstart ausgeblendet, sobald sie einmal weggeklickt wurde.
// Warnt, falls die jüngste Sicherung beim Programmstart nicht lesbar war
// (z. B. Festplattenfehler, abgebrochener Schreibvorgang). Sehr selten, aber
// sicherheitsrelevant genug, um auf jedem Tab zu erscheinen.
function sicherungBanner(st) {
  if (S.sicherungDismissed || !st.sicherungFehler) return null;
  return h('div', { class: 'banner err' },
    `⚠ ${st.sicherungFehler}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { S.sicherungDismissed = true; render(); } }, 'Ausblenden'));
}

// Offene Posten des laufenden Jahres plus aller Vorjahre (diese liefert der Server getrennt), damit nichts
// nach dem Jahreswechsel aus dem Blick gerät.
function offenePosten(st) {
  const jetzt = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt >= 0).filter(payIsOpen);
  const vor = (st.offeneVorjahre || []).filter(payIsOpen);
  return { jetzt, vor, alle: [...vor, ...jetzt] };
}

// Hinweis in der Zahlungsübersicht: in anderen Jahren sind noch Rechnungen offen (Sprung dorthin)
function anderenJahrenOffen(st) {
  const proJahr = {};
  for (const e of (st.offeneVorjahre || []).filter(payIsOpen)) proJahr[e.jahr] = (proJahr[e.jahr] || 0) + 1;
  const jahre = Object.keys(proJahr).map(Number).sort();
  if (!jahre.length) return null;
  return h('div', { class: 'banner warn' }, 'In anderen Jahren sind noch Rechnungen offen: ',
    jahre.map((j) => [h('button', { class: 'btn small', onclick: async () => { S.year = j; await reload(); } }, `${j} (${proJahr[j]})`), ' ']));
}

function openRechnungenBanner(st) {
  if (S.remindDismissed || S.tab === 'zahlungen' || S.tab === 'uebersicht' || st.year !== st.currentYear) return null;
  const { jetzt, vor, alle } = offenePosten(st);
  if (!alle.length) return null;
  const today = todayIso();
  const overdue = alle.filter((e) => e.faellig && e.faellig < today);
  const vorTxt = vor.length ? ` (davon ${vor.length} aus früheren Jahren, ältestes ${vor[0].jahr})` : '';
  return h('div', { class: 'banner warn' },
    `${alle.length} Rechnung${alle.length === 1 ? '' : 'en'} noch offen${vorTxt}${overdue.length ? `, davon ${overdue.length} überfällig` : ''}.`, ' ',
    h('button', { class: 'btn small', onclick: async () => {
      S.tab = 'zahlungen';
      S.year = jetzt.length ? st.currentYear : vor[0].jahr;
      await reload();
    } }, 'Zu den Zahlungen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.remindDismissed = true; render(); } }, 'Ausblenden'));
}

async function quitApp() {
  if (!(await confirmBox('Soll das Programm beendet werden? Alle Eingaben sind bereits gespeichert.', 'Beenden'))) return;
  try { await api('POST', '/api/quit'); } catch (e) { /* Programm ist weg */ }
  document.getElementById('app').replaceChildren(
    h('div', { class: 'center card' }, h('h2', null, 'Programm beendet'),
      h('p', null, 'Das Fenster schließt sich gleich von selbst. Zum erneuten Starten den Kleingarten-Manager wieder öffnen.')));
}

// ------------------------------------------------------------------ Tab: Übersicht

async function loadDashKasse(year) {
  if (S.dashKasseLoading) return;
  S.dashKasseLoading = true;
  try { S.dashKasse = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { S.dashKasse = false; }
  S.dashKasseLoading = false;
  render();
}

async function loadDashJubilaeen() {
  if (S.dashJubilaeenLoading) return;
  S.dashJubilaeenLoading = true;
  try { S.dashJubilaeen = await api('GET', '/api/garten-historie'); } catch (e) { S.dashJubilaeen = false; }
  S.dashJubilaeenLoading = false;
  render();
}

// Hinweis auf der Übersicht, welche aktiven Mitglieder in diesem Kalenderjahr
// ein rundes Jubiläum (5, 10, 15, ...) haben – aus der Gartenhistorie.
function jubilaeumBanner(st, istAdmin) {
  if (S.jubilaeumDismissed || !istAdmin) return null;
  if (S.dashJubilaeen == null) { if (!S.dashJubilaeenLoading) loadDashJubilaeen(); return null; }
  if (!S.dashJubilaeen) return null;
  const seitByPaechter = {};
  for (const e of S.dashJubilaeen) {
    if (!seitByPaechter[e.paechterId] || e.seit < seitByPaechter[e.paechterId]) seitByPaechter[e.paechterId] = e.seit;
  }
  const jahr = new Date().getFullYear();
  const treffer = [];
  for (const p of st.paechter) {
    const seit = seitByPaechter[p.id];
    if (!seit || p.ausgeschiedenAm) continue;
    const jahre = jahr - Number(seit.slice(0, 4));
    if (jahre > 0 && jahre % 5 === 0) treffer.push(`${p.name} (${jahre} Jahre)`);
  }
  if (!treffer.length) return null;
  return h('div', { class: 'banner info' },
    `🎉 ${treffer.length} Mitglied(er) haben ${jahr} ein rundes Jubiläum: ${treffer.join(', ')}.`, ' ',
    h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('jubilaeen'); render(); } }, 'Zu den Jubiläen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.jubilaeumDismissed = true; render(); } }, 'Ausblenden'));
}

// Beispieldaten: in einen leeren Datenbestand laden und mit einem Klick wieder entfernen.
async function beispieldatenLaden() {
  try {
    await api('POST', '/api/admin/beispieldaten');
    toast('Beispieldaten geladen. Schau dich in Ruhe um, du kannst nichts kaputt machen.', 'ok');
    S.dashKasse = null; S.dashJubilaeen = null;
    await reload();
  } catch (e) { handleErr(e); }
}

async function beispieldatenEntfernen() {
  const ok = await confirmBox('Alle Beispieldaten entfernen und mit einem leeren Programm beginnen? Auch alles, was du in der Zwischenzeit selbst eingegeben oder ausgestellt hast, wird entfernt. Ein Admin-Passwort und der zweite Sicherungsordner bleiben erhalten.', 'Beispieldaten entfernen', true);
  if (!ok) return;
  try {
    await api('DELETE', '/api/admin/beispieldaten');
    toast('Beispieldaten entfernt. Jetzt kannst du deinen Verein eintragen.', 'ok');
    S.dashKasse = null; S.dashJubilaeen = null; S.tab = 'uebersicht';
    await reload();
  } catch (e) { handleErr(e); }
}

function demoBanner(st) {
  if (!st.demo) return null;
  const istAdmin = !st.hasPassword || st.loggedIn;
  return h('div', { class: 'banner info' },
    '🧪 Du siehst Beispieldaten: ein erfundener Verein mit erfundenen Namen und Zahlen zum Ausprobieren.',
    istAdmin ? [' ', h('button', { class: 'btn small', onclick: beispieldatenEntfernen }, 'Beispieldaten entfernen und eigene Daten anlegen')] : null);
}

function beispieldatenKarte(st, istAdmin) {
  if (!st.leer || !istAdmin) return null;
  return h('div', { class: 'card', style: 'margin-bottom:14px' },
    h('h3', { style: 'margin-top:0' }, 'Erst einmal ausprobieren?'),
    h('p', { class: 'hint' }, 'Lade einen erfundenen Verein mit 12 Pächtern, Zählerständen und ein paar Kassenbuchungen. So siehst du sofort, wie Rechnungen, Zahlungen und der Kassenbericht aussehen. Die Beispieldaten lassen sich danach mit einem Klick wieder vollständig entfernen.'),
    h('div', { class: 'actions' },
      h('button', { class: 'btn primary', onclick: beispieldatenLaden }, 'Mit Beispieldaten ausprobieren'),
      h('button', { class: 'btn', onclick: () => { adminTabWaehlen('paechter'); render(); } }, 'Gleich eigene Pächter anlegen')));
}

// Erinnerung an die Datensicherung außer Haus: ohne zweiten Sicherungsordner (USB-Stick, Netzlaufwerk)
// liegen alle Sicherungen auf derselben Festplatte wie die Daten.
async function sicherungJetzt() {
  try {
    const z = await api('POST', '/api/admin/sicherung-jetzt');
    if (!z.eingerichtet) toast('Sicherung angelegt (nur auf diesem Rechner, kein zweiter Ordner eingerichtet).', 'ok');
    else if (z.fehler) toast(`${z.fehler}. Ist der USB-Stick eingesteckt bzw. das Netzlaufwerk verbunden?`, 'err');
    else toast('Sicherung angelegt und in den zweiten Sicherungsordner kopiert.', 'ok');
    await reload();
  } catch (e) { handleErr(e); }
}

function zweiteSicherungBanner(st, istAdmin) {
  const z = st.zweiteSicherung || {};
  if (S.zweiteDismissed || !istAdmin || !st.paechter.length) return null;
  const knoepfe = (mitJetzt) => [' ',
    mitJetzt ? h('button', { class: 'btn small', onclick: sicherungJetzt }, 'Jetzt sichern') : null, mitJetzt ? ' ' : null,
    h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('daten'); render(); } }, 'Zu den Einstellungen'), ' ',
    h('button', { class: 'btn small', onclick: () => { S.zweiteDismissed = true; render(); } }, 'Ausblenden')];
  if (!z.eingerichtet) {
    return h('div', { class: 'banner info' },
      '💾 Alle Sicherungen liegen bisher nur auf diesem Rechner. Geht die Festplatte kaputt, sind auch sie weg. Richte einen zweiten Sicherungsordner ein, zum Beispiel auf einem USB-Stick.', ...knoepfe(false));
  }
  if (z.fehler) return h('div', { class: 'banner warn' }, `💾 ${z.fehler}. Ist der USB-Stick eingesteckt?`, ...knoepfe(true));
  const tage = z.zuletzt ? Math.floor((Date.parse(todayIso()) - Date.parse(z.zuletzt)) / 86400000) : null;
  if (tage === null || tage > 30) {
    return h('div', { class: 'banner warn' },
      `💾 ${tage === null ? 'Im zweiten Sicherungsordner liegt noch keine Kopie.' : `Die letzte Kopie im zweiten Sicherungsordner ist vom ${deDate(z.zuletzt)} (vor ${tage} Tagen).`} Einfach USB-Stick einstecken und »Jetzt sichern« wählen.`, ...knoepfe(true));
  }
  return null;
}

function viewUebersicht() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const op = offenePosten(st);
  const open = op.alle;
  const today = todayIso();
  const overdue = open.filter((e) => e.faellig && e.faellig < today);
  const unvollstaendig = st.paechter.filter((p) => !(st.results[p.id] && st.results[p.id].vollstaendig));

  const card = (t, big, small, cls, ic) => h('div', { class: 'card stat ' + (cls || '') }, ic ? h('div', { class: 'statico' }, icon(ic, 22)) : null,
    h('div', null, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small)));
  const cards = [
    card('Offene Rechnungen', String(open.length), open.length ? `${eur(open.reduce((a, e) => a + payOpen(e), 0))} · ${overdue.length} überfällig${op.vor.length ? ` · ${op.vor.length} aus Vorjahren` : ''}` : 'alles bezahlt', open.length ? 'warn' : 'ok', 'rechnung'),
    card('Unvollständige Pächter', String(unvollstaendig.length), `von ${st.paechter.length} in ${st.currentYear}`, unvollstaendig.length ? 'warn' : 'ok', 'personen'),
    card('Letzte Sicherung', st.letzteSicherung ? deDate(st.letzteSicherung) : 'noch keine',
      st.sicherungFehler ? 'beschädigt!' : 'automatisch bei jeder Änderung',
      st.sicherungFehler ? 'err' : st.letzteSicherung ? 'ok' : 'warn', 'sicherung'),
  ];
  if (istAdmin) {
    if (S.dashKasse === null) { if (!S.dashKasseLoading) loadDashKasse(st.currentYear); }
    else if (S.dashKasse !== false) {
      cards.unshift(card('Kassenbestand', eur(S.dashKasse.kassenbestand), `Stand ${st.currentYear}`, S.dashKasse.kassenbestand >= 0 ? 'ok' : 'err', 'kasse'));
    }
  }

  return h('div', null,
    heroKarte(st),
    beispieldatenKarte(st, istAdmin),
    jubilaeumBanner(st, istAdmin),
    st.demo ? null : zweiteSicherungBanner(st, istAdmin),
    cockpitKarte(st, istAdmin),
    st.paechter.length ? jahresFortschritt(st, istAdmin) : null,
    h('div', { class: 'zwei-spalten' }, naechsteTermine(st), besichtigungenKarte(st, istAdmin)),
    h('div', { class: 'cards' }, cards),
    h('h3', { class: 'sect' }, 'Schnellzugriff'),
    h('div', { class: 'quicklinks' },
      quicklink('zaehler', 'Zu den Zählerständen', 'Nummer eintippen, neue Stände eingeben', () => { S.tab = 'eingabe'; render(); }, true),
      quicklink('rechnung', 'Zu den Rechnungen', 'prüfen, ausstellen, drucken oder per E-Mail', () => { S.tab = 'rechnungen'; render(); }),
      quicklink('euro', 'Zu den Zahlungen', 'bezahlt abhaken, Mahnungen', () => { S.tab = 'zahlungen'; render(); }),
      istAdmin ? quicklink('kasse', 'Zum Kassenbericht', 'Kassenbestand, Ausgaben, Belege', () => { adminTabWaehlen('kassenbericht'); render(); }) : null));
}

// Wo steht das Abrechnungsjahr? Vier Stationen mit Balken und dem nächsten sinnvollen Schritt.
// Begrüßung oben auf der Übersicht: Tageszeit, Verein, Datum, ein paar Eckdaten und die Gartenillustration.
function heroKarte(st) {
  const jetzt = new Date();
  const std = jetzt.getHours();
  const gruss = std < 11 ? 'Guten Morgen' : std < 18 ? 'Guten Tag' : 'Guten Abend';
  const datum = jetzt.toLocaleDateString('de-DE', { weekday: 'long', day: 'numeric', month: 'long', year: 'numeric' });
  const gaerten = new Set(st.paechter.filter((p) => !p.ausgeschiedenAm && (p.gartennr || '').trim()).map((p) => p.gartennr.trim())).size;
  const frei = Object.keys(st.lageplan || {}).filter((nr) => !st.paechter.some((p) => !p.ausgeschiedenAm && (p.gartennr || '').trim() === nr)).length;
  const heute = todayIso();
  const naechster = (st.termine || []).find((t) => t.datum >= heute);
  const tageBis = naechster ? Math.round((isoDatum(naechster.datum) - isoDatum(heute)) / 86400000) : null;
  const fakt = (zahl, text) => h('div', { class: 'hero-fakt' }, h('b', null, zahl), h('span', null, text));
  const bild = h('img', { class: 'hero-bild', src: '/bilder/garten.webp', alt: '', onerror: (e) => e.target.remove() });
  return h('section', { class: 'hero' },
    h('div', { class: 'hero-text' },
      h('span', { class: 'hero-gruss' }, `${gruss} · ${datum}`),
      h('h2', null, st.settings.vereinName),
      h('p', null, `Abrechnungsjahr ${st.currentYear}. Hier siehst du auf einen Blick, wo der Verein steht und was als Nächstes ansteht.`),
      h('div', { class: 'hero-fakten' },
        fakt(String(gaerten), gaerten === 1 ? 'Garten verpachtet' : 'Gärten verpachtet'),
        frei ? fakt(String(frei), frei === 1 ? 'Garten frei' : 'Gärten frei') : null,
        st.wartelisteAnzahl ? fakt(String(st.wartelisteAnzahl), 'auf der Warteliste') : null,
        naechster ? fakt(tageBis === 0 ? 'heute' : tageBis === 1 ? 'morgen' : `in ${tageBis} Tagen`, naechster.titel) : null)),
    bild);
}

function jahresFortschritt(st, istAdmin) {
  const n = st.paechter.length;
  const vollst = st.paechter.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const ausgestellt = st.paechter.filter((p) => (st.issued || {})[p.id]).length;
  const forderungen = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt > 0);
  const bezahlt = forderungen.filter((e) => !payIsOpen(e)).length;
  const go = (tab, extra) => () => { S.tab = tab; Object.assign(S, extra || {}); render(); };
  const stationen = [
    { titel: 'Zählerstände', wert: vollst, von: n, text: `${vollst} von ${n} vollständig`, los: 'Zählerstände eintragen', go: go('eingabe') },
    { titel: 'Rechnungen', wert: ausgestellt, von: vollst || n, text: `${ausgestellt} von ${vollst} ausgestellt`, los: 'Rechnungen ausstellen', go: go('rechnungen', { invTab: 'pruefen' }) },
    // nur Guthaben ausgestellt: es gibt nichts einzusammeln, die Station gilt als erledigt
    forderungen.length || !ausgestellt
      ? { titel: 'Zahlungen', wert: bezahlt, von: forderungen.length, text: forderungen.length ? `${bezahlt} von ${forderungen.length} bezahlt` : 'noch keine Rechnung', los: 'Zahlungen abhaken', go: go('zahlungen') }
      : { titel: 'Zahlungen', wert: 1, von: 1, text: 'nichts offen', los: 'Zahlungen ansehen', go: go('zahlungen') },
    { titel: 'Jahresabschluss', wert: 0, von: 1, text: `${st.currentYear} abschließen`, los: 'Zum Jahreswechsel', go: go('admin', { adminTab: 'jahreswechsel' }), nurAdmin: true },
  ];
  // die erste Station, die noch nicht fertig ist, ist der nächste Schritt
  let naechste = stationen.findIndex((x) => x.von === 0 || x.wert < x.von);
  if (naechste < 0) naechste = stationen.length - 1;
  if (stationen[naechste].nurAdmin && !istAdmin) naechste = -1;
  const fertig = (x) => x.von > 0 && x.wert >= x.von && !x.nurAdmin;
  return h('div', { class: 'card fortschritt' },
    h('div', { class: 'fs-kopf' },
      h('div', null, h('b', null, `Abrechnungsjahr ${st.currentYear}`), h('span', { class: 'hint' }, ' – wo stehen wir?')),
      naechste >= 0 ? h('button', { class: 'btn primary small', onclick: stationen[naechste].go }, `Weiter: ${stationen[naechste].los} →`) : null),
    h('ol', { class: 'fs-stationen' }, stationen.map((x, i) => {
      const anteil = x.von > 0 ? Math.min(1, x.wert / x.von) : 0;
      const cls = fertig(x) ? 'fertig' : i === naechste ? 'aktiv' : '';
      return h('li', { class: cls, onclick: x.nurAdmin && !istAdmin ? null : x.go, title: x.los },
        h('span', { class: 'fs-punkt' }, fertig(x) ? '✓' : String(i + 1)),
        h('span', { class: 'fs-text' }, h('b', null, x.titel), h('span', null, x.text)),
        h('span', { class: 'fs-balken' }, h('i', { style: `width:${Math.round(anteil * 100)}%` })));
    })));
}

function quicklink(ic, titel, text, onclick, primary) {
  return h('button', { class: 'quicklink' + (primary ? ' primary' : ''), onclick, title: text },
    h('span', { class: 'qlico' }, icon(ic, 24)), h('span', { class: 'qltext' }, h('b', null, titel), h('span', null, text)));
}

// ------------------------------------------------------------------ Tab: Zählerstände

function statusBadge(res) {
  if (res.vollstaendig) return h('span', { class: 'badge ok' }, '✓ OK');
  return h('span', { class: 'badge ' + (res.status === 'Angaben fehlen' ? 'warn' : 'err'), title: res.status }, res.status === 'Angaben fehlen' ? 'unvollständig' : res.status);
}

function updateCalc(tr, res) {
  tr.querySelector('.c-wv').textContent = res.wasserVerbrauch == null ? '' : nfFlex.format(res.wasserVerbrauch);
  tr.querySelector('.c-sv').textContent = res.energieVerbrauch == null ? '' : nfFlex.format(res.energieVerbrauch);
  const g = tr.querySelector('.gesamt');
  g.className = 'gesamt' + (res.vollstaendig && res.gesamt < 0 ? ' guthaben' : '');
  g.title = res.vollstaendig && res.gesamt < 0 ? 'Guthaben zu Gunsten des Pächters' : '';
  g.textContent = res.vollstaendig ? eur(res.gesamt) : '–';
  const inf = ((S.state && S.state.issued) || {})[tr.dataset.id];
  const hints = res.hinweise || ((S.state && S.state.hinweise) || {})[tr.dataset.id];
  tr.querySelector('.status').replaceChildren(...[statusBadge(res),
    inf ? h('span', { class: 'badge ' + (inf.geaendert ? 'warn' : 'neu'), title: inf.geaendert ? 'Rechnung wurde ausgestellt, danach wurden Werte geändert' : 'Rechnung ist ausgestellt' }, inf.geaendert ? '⚠ nach Ausstellung geändert' : 'ausgestellt') : null,
    hints && hints.length ? h('span', { class: 'badge warn', title: hints.join('\n') }, '⚠ Verbrauch prüfen') : null].filter(Boolean));
}

// Nach einer Änderung neu vom Programm holen, ob eine ausgestellte Rechnung davon betroffen ist.
async function refreshIssued(tr) {
  try {
    const st = await api('GET', `/api/state?year=${S.year}`);
    S.state.issued = st.issued; S.state.archive = st.archive;
    updateCalc(tr, S.state.results[tr.dataset.id]);
  } catch (e) { /* nicht kritisch */ }
}

let savedTimer;
function showSaved() {
  const el = document.querySelector('.saveind');
  if (!el) return;
  el.classList.add('show');
  clearTimeout(savedTimer);
  savedTimer = setTimeout(() => el.classList.remove('show'), 1800);
}

function hasDetails(a) {
  return !!(a && (a.versicherung || a.grundsteuer || a.auslagen || (a.hinweis && a.hinweis.trim())));
}

async function saveRow(tr, p) {
  const f = (k) => tr.querySelector(`[data-f="${k}"]`);
  const keys = ['wasserVJ', 'wasserAkt', 'stromVJ', 'stromAkt', 'stunden', 'abschlag'];
  const vals = {};
  let bad = false;
  for (const k of keys) {
    const v = parseNum(f(k).value);
    const wrong = Number.isNaN(v) || (v !== null && v < 0);
    f(k).classList.toggle('err', wrong);
    bad = bad || wrong;
    vals[k] = v;
  }
  if (bad) { toast('Bitte nur Zahlen ab 0 eingeben (Komma ist erlaubt).', 'err'); return; }
  try {
    const wz = f('wz').value.trim(), sz = f('sz').value.trim();
    if (wz !== p.wasserzaehlerNr || sz !== p.stromzaehlerNr) {
      const np = await api('PUT', `/api/zaehler/${p.id}`, { wasserzaehlerNr: wz, stromzaehlerNr: sz });
      Object.assign(p, np);
    }
    const cur = S.state.ablesungen[p.id] || {};
    const body = { ...cur, wasserVJ: vals.wasserVJ, wasserAkt: vals.wasserAkt, stromVJ: vals.stromVJ, stromAkt: vals.stromAkt,
      stunden: vals.stunden, abschlag: vals.abschlag ?? 0 };
    const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body);
    S.state.ablesungen[p.id] = body;
    S.state.results[p.id] = res;
    S.state.hinweise = S.state.hinweise || {};
    if (res.hinweise && res.hinweise.length) S.state.hinweise[p.id] = res.hinweise; else delete S.state.hinweise[p.id];
    updateCalc(tr, res);
    showSaved();
    if ((S.state.issued || {})[p.id]) refreshIssued(tr);
  } catch (e) { handleErr(e); }
}

const WECHSEL_WASSER = { wKey: 'wasserWechsel', field: 'wz', label: 'Wasser', unit: 'm³' };
const WECHSEL_STROM = { wKey: 'stromWechsel', field: 'sz', label: 'Strom', unit: 'kWh' };

async function saveWechsel(tr, p, art, wechsel) {
  const cur = S.state.ablesungen[p.id] || {};
  const body = { ...cur, [art.wKey]: wechsel };
  const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body);
  S.state.ablesungen[p.id] = body;
  S.state.results[p.id] = res;
  S.state.hinweise = S.state.hinweise || {};
  if (res.hinweise && res.hinweise.length) S.state.hinweise[p.id] = res.hinweise; else delete S.state.hinweise[p.id];
  updateCalc(tr, res);
  if (wechsel && wechsel.neueNr) {
    p[art.field === 'wz' ? 'wasserzaehlerNr' : 'stromzaehlerNr'] = wechsel.neueNr;
    const el = tr.querySelector(`[data-f="${art.field}"]`);
    if (el) el.value = wechsel.neueNr;
  }
  const btn = tr.querySelector(`.wechselbtn-${art.field}`);
  if (btn) btn.classList.toggle('primary', !!wechsel);
  showSaved();
  if ((S.state.issued || {})[p.id]) refreshIssued(tr);
}

function wechselDialog(tr, p, art) {
  const cur = S.state.ablesungen[p.id] || {};
  const w = cur[art.wKey] || {};
  const altEnde = h('input', { type: 'text', inputmode: 'decimal', value: numIn(w.altEnde), autocomplete: 'off' });
  const neueNr = h('input', { type: 'text', maxlength: 40, value: w.neueNr || '', autocomplete: 'off' });
  const neuStart = h('input', { type: 'text', inputmode: 'decimal', value: numIn(w.neuStart != null ? w.neuStart : 0), autocomplete: 'off' });
  const fld = (label, input, unit) => h('div', { class: 'field' }, h('label', null, label),
    unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input);
  const hatWechsel = w.altEnde != null || w.neuStart != null || (w.neueNr && w.neueNr.trim());
  const body = h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('p', { class: 'hint', style: 'margin:0' },
      `Wurde der ${art.label}zähler unterjährig getauscht? Endstand des alten und Anfangsstand des neuen Zählers eintragen – der Verbrauch wird dann aus beiden Zählern zusammengerechnet. Leer lassen, wenn es keinen Wechsel gab.`)),
    fld('Endstand alter Zähler', altEnde, art.unit), fld('Neue Zähler-Nr.', neueNr), fld('Anfangsstand neuer Zähler', neuStart, art.unit));
  return modal(`Zählerwechsel ${art.label} – ${p.name}`, body, [
    { label: 'Abbrechen', value: false },
    hatWechsel ? { label: 'Wechsel entfernen', cls: 'danger', value: 'del', action: async () => {
      try { await saveWechsel(tr, p, art, null); } catch (e) { handleErr(e); return false; }
    } } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const ae = parseNum(altEnde.value), ns = parseNum(neuStart.value);
      const leer = ae === null && ns === null && !neueNr.value.trim();
      if (!leer && (Number.isNaN(ae) || (ae !== null && ae < 0) || Number.isNaN(ns) || (ns !== null && ns < 0))) {
        toast('Bitte gültige Zahlen ab 0 eingeben.', 'err'); return false;
      }
      try { await saveWechsel(tr, p, art, leer ? null : { altEnde: ae, neueNr: neueNr.value.trim(), neuStart: ns }); } catch (e) { handleErr(e); return false; }
    } },
  ].filter(Boolean));
}

async function detailsDialog(tr, p) {
  const cur = S.state.ablesungen[p.id] || {};
  const num = (val, id) => h('input', { type: 'text', inputmode: 'decimal', id, value: numIn(val || null), autocomplete: 'off' });
  const vers = num(cur.versicherung, 'd-vers'), grund = num(cur.grundsteuer, 'd-grund'), ausl = num(cur.auslagen, 'd-ausl');
  const hint = h('textarea', { rows: 3, maxlength: 300, style: 'width:100%', id: 'd-hint' }, cur.hinweis || '');
  const field = (label, input, unit) => h('div', { class: 'field' }, h('label', null, label),
    unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input);
  const body = h('div', { class: 'form' },
    field('Versicherung', vers, '€'), field('Grundsteuer', grund, '€'), field('Sonstige Auslagen', ausl, '€'),
    h('div', { class: 'field wide' }, h('label', null, 'Hinweis / Erläuterung (steht auf der Rechnung)'), hint));
  await modal(`Weitere Angaben – ${p.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const vs = [vers, grund, ausl].map((i) => parseNum(i.value));
      if (vs.some((v) => Number.isNaN(v) || (v !== null && v < 0))) { toast('Bitte nur Beträge ab 0 eingeben.', 'err'); return false; }
      const body2 = { ...cur, versicherung: vs[0] ?? 0, grundsteuer: vs[1] ?? 0, auslagen: vs[2] ?? 0, hinweis: hint.value.trim() };
      try {
        const res = await api('PUT', `/api/ablesung/${p.id}?year=${S.year}`, body2);
        S.state.ablesungen[p.id] = body2;
        S.state.results[p.id] = res;
        updateCalc(tr, res);
        tr.querySelector('.detailbtn').classList.toggle('primary', hasDetails(body2));
        showSaved();
        if ((S.state.issued || {})[p.id]) refreshIssued(tr);
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

function verlaufDialog(p) {
  const hist = ((S.state && S.state.history) || {})[p.id] || [];
  const row = (e) => h('tr', null,
    h('td', null, e.jahr),
    h('td', { class: 'r' }, e.wasser == null ? '–' : nfFlex.format(e.wasser) + ' m³'),
    h('td', { class: 'r' }, e.strom == null ? '–' : nfFlex.format(e.strom) + ' kWh'),
    h('td', { class: 'r' }, e.stunden == null ? '–' : nfFlex.format(e.stunden)),
    h('td', { class: 'r' }, e.gesamt == null ? '–' : eur(e.gesamt)),
    h('td', null, e.ausgestellt ? h('span', { class: 'badge ok' }, 'ausgestellt') : h('span', { class: 'badge neu' }, 'berechnet')));
  const chrono = [...hist].reverse();
  const body = hist.length
    ? h('div', null,
        h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Gesamtbetrag je Jahr:'),
        barChart(chrono.filter((e) => e.gesamt != null).map((e) => ({ label: String(e.jahr), value: e.gesamt })), { valueFmt: (v) => eur(v) }),
        h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Jahr', 'Wasser', 'Strom', 'Stunden', 'Gesamt', ''].map((t) => h('th', null, t)))),
          h('tbody', null, hist.map(row)))))
    : h('p', { class: 'hint' }, 'Für diesen Pächter liegen noch keine Werte aus Vorjahren vor.');
  return modal(`Jahresverlauf – ${p.name}`, body, [{ label: 'Schließen', value: true }]);
}

function eingabeRow(p, ro) {
  const a = S.state.ablesungen[p.id] || {};
  const res = S.state.results[p.id];
  const tr = h('tr', { 'data-id': p.id });
  const save = () => saveRow(tr, p);
  const inp = (key, val, cls) => h('input', { type: 'text', inputmode: 'decimal', class: cls || 'num', autocomplete: 'off',
    'data-f': key, value: numIn(val), disabled: ro, onchange: save });
  const txt = (key, val) => h('input', { type: 'text', class: 'nr', autocomplete: 'off', maxlength: 40, 'data-f': key,
    value: val || '', disabled: ro, onchange: save });
  tr.append(
    h('td', { class: 'sticky s1' }, p.mitgliedsnr),
    h('td', { class: 'name sticky s2', title: p.name }, p.name, schlussBadge(p)),
    h('td', null, txt('wz', p.wasserzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-wz' + (a.wasserWechsel ? ' primary' : ''),
      title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(tr, p, WECHSEL_WASSER) }, '⇄')),
    h('td', null, inp('wasserVJ', a.wasserVJ)),
    h('td', null, inp('wasserAkt', a.wasserAkt)),
    h('td', { class: 'calc c-wv' }),
    h('td', null, txt('sz', p.stromzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-sz' + (a.stromWechsel ? ' primary' : ''),
      title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(tr, p, WECHSEL_STROM) }, '⇄')),
    h('td', null, inp('stromVJ', a.stromVJ)),
    h('td', null, inp('stromAkt', a.stromAkt)),
    h('td', { class: 'calc c-sv' }),
    h('td', null, inp('stunden', a.stunden, 'small')),
    h('td', null, inp('abschlag', a.abschlag || null, 'small')),
    h('td', { class: 'gesamt' }),
    h('td', { class: 'status mid' }),
    h('td', null, h('button', { class: 'btn small detailbtn' + (hasDetails(a) ? ' primary' : ''), disabled: ro,
      title: 'Versicherung, Grundsteuer, Auslagen, Hinweis', onclick: () => detailsDialog(tr, p) }, 'Weitere'), ' ',
      h('button', { class: 'btn small', title: 'Verbrauch und Betrag der Vorjahre', onclick: () => verlaufDialog(p) }, 'Verlauf')),
  );
  updateCalc(tr, res);
  // Enter springt in die Zeile darunter (gleiche Spalte)
  tr.addEventListener('keydown', (e) => {
    if (e.key !== 'Enter' || !e.target.dataset || !e.target.dataset.f) return;
    e.preventDefault();
    e.target.dispatchEvent(new Event('change'));
    let next = tr.nextElementSibling;
    while (next && next.hidden) next = next.nextElementSibling;
    const t = next && next.querySelector(`[data-f="${e.target.dataset.f}"]`);
    if (t) { t.focus(); t.select(); }
  });
  return tr;
}

function viewEingabe() {
  const st = S.state;
  return h('div', null,
    h('h2', null, `Zählerstände ${st.year}`),
    h('div', { class: 'toolbar' },
      h('div', { class: 'subtabs', style: 'margin:0' },
        h('button', { class: S.eingabeMode === 'schnell' ? 'active' : '', onclick: () => { S.eingabeMode = 'schnell'; render(); } }, 'Schnelleingabe (Nummer eintippen)'),
        h('button', { class: S.eingabeMode === 'tabelle' ? 'active' : '', onclick: () => { S.eingabeMode = 'tabelle'; render(); } }, 'Tabelle (alle Pächter)')),
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', disabled: !st.paechter.length, title: 'Liste aller Gärten mit Zählernummern und Vorjahresständen zum Ausdrucken, mit leeren Feldern für den Ablese-Rundgang',
        onclick: () => ablesebogenDrucken(st.year) }, 'Ablesebogen drucken'),
      !st.hasPassword || st.loggedIn ? h('button', { class: 'btn', disabled: !st.paechter.length, title: 'Wann müssen die Zähler nach dem Eichgesetz getauscht werden?', onclick: eichungDialog }, 'Eichfristen') : null),
    S.eingabeMode === 'schnell' ? viewSchnell() : viewTabelle());
}

// Eichfristen: Eichjahr je Zähler pflegen, Status sehen, Tauschliste drucken.
async function eichungDialog() {
  let d;
  try { d = await api('GET', '/api/admin/eichung'); } catch (e) { handleErr(e); return; }
  const STATUS = { abgelaufen: ['err', 'abgelaufen'], faellig: ['warn', 'dieses Jahr tauschen'], bald: ['neu', 'nächstes Jahr'], unbekannt: ['', 'Eichjahr fehlt'], ok: ['ok', 'gültig'] };
  const box = h('div');
  const zeichne = () => {
    const fw = h('input', { type: 'number', min: 1, max: 30, value: d.fristen.wasser, style: 'width:70px' });
    const fs = h('input', { type: 'number', min: 1, max: 30, value: d.fristen.strom, style: 'width:70px' });
    const fristen = async () => {
      try { d = await api('PUT', '/api/admin/eichung/fristen', { wasser: Number(fw.value), strom: Number(fs.value) }); zeichne(); } catch (e) { handleErr(e); }
    };
    fw.addEventListener('change', fristen); fs.addEventListener('change', fristen);
    const tauschen = d.zeilen.filter((z) => ['abgelaufen', 'faellig', 'bald'].includes(z.status)).length;
    box.replaceChildren(
      h('p', { class: 'hint' }, 'Nach dem Mess- und Eichgesetz dürfen Zähler nur begrenzt messen: Kaltwasserzähler 6 Jahre, elektronische Stromzähler 8 Jahre (Ferraris-Zähler 16). Die Frist endet mit Ablauf des Jahres. Trag das Eichjahr ein (steht meist auf dem Zähler oder der Plombe) – beim Zählerwechsel setzt das Programm es selbst.'),
      h('div', { class: 'toolbar' }, h('label', null, 'Eichfrist Wasser ', fw, ' Jahre'), h('label', null, 'Strom ', fs, ' Jahre'), h('div', { class: 'spacer' }),
        h('button', { class: 'btn', disabled: !tauschen, onclick: () => pdfDrucken({ url: '/api/admin/eichung/tauschliste', oeffnenUrl: '/api/admin/eichung/tauschliste/oeffnen', titel: 'Tauschliste', meldung: () => 'Die Tauschliste wurde im PDF-Programm geöffnet.' }) },
          `Tauschliste drucken${tauschen ? ` (${tauschen})` : ''}`)),
      h('div', { class: 'tablewrap', style: 'max-height:55vh' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Garten', 'Pächter', 'Zähler', 'Eichjahr', 'Gültig bis', ''].map((t) => h('th', null, t)))),
        h('tbody', null, d.zeilen.map((z) => {
          const inp = h('input', { type: 'number', min: 1950, max: d.jahr + 1, value: z.eichjahr || '', placeholder: '–', style: 'width:80px' });
          inp.addEventListener('change', async () => {
            try { d = await api('PUT', `/api/admin/eichung/${z.paechterId}`, { art: z.art, eichjahr: Number(inp.value || 0) }); zeichne(); }
            catch (e) { handleErr(e); inp.value = z.eichjahr || ''; }
          });
          const [cls, txt] = STATUS[z.status] || ['', z.status];
          return h('tr', null, h('td', null, z.gartennr), h('td', null, z.name), h('td', null, (z.art === 'wasser' ? 'Wasser ' : 'Strom ') + z.nr),
            h('td', null, inp), h('td', null, z.gueltigBis ? `31.12.${z.gueltigBis}` : ''), h('td', null, h('span', { class: 'badge ' + cls }, txt)));
        })))));
  };
  zeichne();
  await breiterDialog('Eichfristen der Zähler', box, [{ label: 'Schließen', cls: 'primary', value: true }]);
  S.aufgaben = null;
}

// Schnelleingabe: Nummer eintippen, Name/Adresse/Größe/Zählernummern kommen aus den Stammdaten.
function viewSchnell() {
  const st = S.state;
  const ro = st.readOnly;
  const list = [...st.paechter].sort(cmpNr);
  const done = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const box = h('div');
  const sugg = h('div', { class: 'sugg' });
  const find = (q) => {
    q = q.trim().toLowerCase();
    if (!q) return [];
    const exact = list.filter((p) => (p.mitgliedsnr || '').toLowerCase() === q);
    if (exact.length) return exact;
    const g = list.filter((p) => (p.gartennr || '').toLowerCase() === q);
    if (g.length) return g;
    return list.filter((p) => [p.mitgliedsnr, p.gartennr, p.name].some((x) => (x || '').toLowerCase().includes(q))).slice(0, 8);
  };
  const choose = (p) => { S.quickId = p.id; S.quickQ = ''; input.value = ''; sugg.replaceChildren(); drawCard(true); };
  const input = h('input', { type: 'search', class: 'bignr', placeholder: 'Nummer oder Name', autocomplete: 'off', style: 'width:280px', id: 'quicknr',
    oninput: () => {
      const m = find(input.value);
      sugg.replaceChildren();
      if (input.value.trim() && !m.length) sugg.append(h('span', { class: 'hint' }, 'Nichts gefunden.'));
      if (m.length > 1 || (m.length === 1 && input.value.trim().toLowerCase() !== (m[0].mitgliedsnr || '').toLowerCase()))
        for (const p of m) sugg.append(h('button', { class: 'btn small', onclick: () => choose(p) }, `${p.mitgliedsnr}  ${p.name}`));
    },
    onkeydown: (e) => {
      if (e.key !== 'Enter') return;
      e.preventDefault();
      const m = find(input.value);
      if (m.length === 1) choose(m[0]);
      else if (m.length > 1) toast('Mehrere Treffer – bitte einen anklicken oder die Nummer genauer eingeben.', 'err');
      else if (input.value.trim()) {
        // die Karte des vorherigen Pächters ausblenden, damit nichts versehentlich beim Falschen landet
        S.quickId = null;
        drawCard();
        toast('Nummer nicht gefunden.', 'err');
      }
    } });

  const drawCard = (focusFirst) => {
    box.replaceChildren();
    const p = list.find((x) => x.id === S.quickId);
    if (!p) {
      box.append(list.length ? h('div', { class: 'card empty' }, 'Tippe oben eine Nummer ein und drücke Enter. Alle Angaben des Pächters erscheinen dann automatisch.')
        : leer('personen', 'Noch keine Pächter angelegt', 'Lege zuerst die Pächter an oder übernimm sie aus einer Excel-Liste. Danach trägst du hier die Zählerstände ein.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }));
      return;
    }
    const a = st.ablesungen[p.id] || {};
    const card = h('div', { class: 'card quick', 'data-id': p.id });
    const save = () => saveRow(card, p);
    const inp = (key, val, cls) => h('input', { type: 'text', inputmode: 'decimal', class: cls || 'num', autocomplete: 'off', 'data-f': key, value: numIn(val), disabled: ro, onchange: save });
    const txt = (key, val) => h('input', { type: 'text', class: 'nr', autocomplete: 'off', maxlength: 40, 'data-f': key, value: val || '', disabled: ro, onchange: save });
    const fld = (label, el, unit) => h('div', { class: 'field' }, h('label', null, label), unit ? h('div', { class: 'inputunit' }, el, h('span', null, unit)) : el);
    const info = (k, v) => v ? h('span', { class: 'chip' }, h('i', null, k + ' '), v) : null;
    card.append(
      h('div', { class: 'qhead' },
        h('div', null, h('h3', null, `${p.mitgliedsnr} – ${p.name}`, schlussBadge(p)), h('div', { class: 'hint' }, [p.strasse, p.plzOrt].filter(Boolean).join(', ')),
          p.notiz ? h('div', { class: 'hint', style: 'margin-top:2px' }, '📝 ', p.notiz) : null),
        h('div', { class: 'chips' }, info('Garten', p.gartennr), info('Größe', p.gartengroesse ? `${nfFlex.format(p.gartengroesse)} m²` : ''),
          info('Umlage', eur(p.umlageAbweichend != null ? p.umlageAbweichend : st.settings.umlageStandard)))),
      h('div', { class: 'qgrid' },
        h('fieldset', null, h('legend', null, 'Wasser'),
          fld('Zähler-Nr.', h('div', null, txt('wz', p.wasserzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-wz' + (a.wasserWechsel ? ' primary' : ''),
            title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(card, p, WECHSEL_WASSER) }, '⇄'))),
          fld('Stand Vorjahr', inp('wasserVJ', a.wasserVJ), 'm³'), fld('Stand aktuell', inp('wasserAkt', a.wasserAkt), 'm³'),
          h('div', { class: 'field' }, h('label', null, 'Verbrauch'), h('div', { class: 'calc c-wv qv' }))),
        h('fieldset', null, h('legend', null, 'Strom'),
          fld('Zähler-Nr.', h('div', null, txt('sz', p.stromzaehlerNr), ro ? null : h('button', { class: 'btn small wechselbtn-sz' + (a.stromWechsel ? ' primary' : ''),
            title: 'Unterjähriger Zählerwechsel', onclick: () => wechselDialog(card, p, WECHSEL_STROM) }, '⇄'))),
          fld('Stand Vorjahr', inp('stromVJ', a.stromVJ), 'kWh'), fld('Stand aktuell', inp('stromAkt', a.stromAkt), 'kWh'),
          h('div', { class: 'field' }, h('label', null, 'Verbrauch'), h('div', { class: 'calc c-sv qv' }))),
        h('fieldset', null, h('legend', null, 'Sonstiges'), fld('Arbeitsstunden', inp('stunden', a.stunden, 'small'), 'h'),
          fld('Abschlag', inp('abschlag', a.abschlag || null, 'small'), '€'),
          h('button', { class: 'btn small detailbtn' + (hasDetails(a) ? ' primary' : ''), disabled: ro, onclick: () => detailsDialog(card, p) }, 'Weitere Angaben'))),
      h('div', { class: 'qfoot' },
        h('div', null, h('span', { class: 'hint' }, 'Rechnungsbetrag: '), h('b', { class: 'gesamt' }), ' ', h('span', { class: 'status' })),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn', onclick: () => verlaufDialog(p) }, 'Verlauf'),
        h('button', { class: 'btn', onclick: () => { S.tab = 'rechnungen'; S.selInvoice = p.id; S.invTab = 'pruefen'; S.invView = 'archiv'; render(); } }, 'Rechnung ansehen'),
        h('button', { class: 'btn primary', onclick: () => { input.focus(); } }, 'Fertig – nächste Nummer')));
    updateCalc(card, st.results[p.id]);
    box.append(card);

    // Enter: nächstes Feld; Pflichtfelder immer, Zählernummer/Vorjahresstände nur wenn noch leer
    const seq = ['wz', 'wasserVJ', 'wasserAkt', 'sz', 'stromVJ', 'stromAkt', 'stunden', 'abschlag'];
    const must = new Set(['wasserAkt', 'stromAkt', 'stunden', 'abschlag']);
    const want = (k) => { const el = card.querySelector(`[data-f="${k}"]`); return el && !el.disabled && (must.has(k) || el.value.trim() === ''); };
    const goto = (from) => {
      for (let i = from; i < seq.length; i++) if (want(seq[i])) { const el = card.querySelector(`[data-f="${seq[i]}"]`); el.focus(); el.select(); return; }
      input.focus();
    };
    card.addEventListener('keydown', (e) => {
      if (e.key !== 'Enter' || !e.target.dataset || !e.target.dataset.f) return;
      e.preventDefault();
      e.target.dispatchEvent(new Event('change'));
      goto(seq.indexOf(e.target.dataset.f) + 1);
    });
    if (focusFirst && !ro) goto(0);
  };
  drawCard(false);

  return h('div', null,
    h('p', { class: 'hint' }, 'Nummer eintippen, Enter drücken: Name, Adresse, Gartengröße, Umlage und Zählernummern kommen automatisch aus den Stammdaten, der Vorjahresstand ist schon eingetragen. Du gibst nur noch die neuen Stände ein. Mit Enter springst du zum nächsten Feld, nach dem letzten wieder zur Nummer. Alles wird sofort gespeichert.'),
    h('div', { class: 'toolbar' }, input, h('span', { class: 'stat' }, `${done} von ${list.length} vollständig`), h('span', { class: 'saveind' }, '✓ gespeichert')),
    sugg, box);
}

function viewTabelle() {
  const st = S.state;
  const ro = st.readOnly;
  const list = [...st.paechter].sort(cmpNr);
  const done = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;

  const tbody = h('tbody');
  const fill = () => {
    tbody.replaceChildren();
    const q = S.filter.trim().toLowerCase();
    const shown = list.filter((p) => (!q || [p.mitgliedsnr, p.name, p.gartennr].some((x) => (x || '').toLowerCase().includes(q))) &&
      (!S.nurUnvollstaendig || !(st.results[p.id] && st.results[p.id].vollstaendig)));
    for (const p of shown) tbody.append(eingabeRow(p, ro));
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 15, class: 'empty' },
      list.length ? 'Keine Treffer.' : leer('personen', 'Noch keine Pächter angelegt', 'Lege zuerst die Pächter an oder übernimm sie aus einer Excel-Liste.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }))));
  };
  fill();

  const search = h('input', { type: 'search', placeholder: 'Suchen (Nummer oder Name)', value: S.filter, style: 'width:240px',
    oninput: (e) => { S.filter = e.target.value; fill(); } });
  const nurOffen = h('label', { class: 'hint' },
    h('input', { type: 'checkbox', checked: S.nurUnvollstaendig, onchange: (e) => { S.nurUnvollstaendig = e.target.checked; fill(); } }),
    ` nur unvollständig${list.length - done ? ` (${list.length - done})` : ''}`);

  const th = (t, cls, extra) => h('th', { class: cls || '', ...(extra || {}) }, t);
  return h('div', null,
    h('p', { class: 'hint' }, 'Hier trägst du pro Pächter die Zählernummern, Zählerstände und Arbeitsstunden ein. Alles wird sofort gespeichert und die Beträge werden automatisch berechnet.'),
    h('div', { class: 'toolbar' }, search, nurOffen,
      h('span', { class: 'stat' }, `${list.length} Pächter · ${done} vollständig`),
      h('span', { class: 'saveind' }, '✓ gespeichert'),
      h('div', { class: 'spacer' }),
      h('a', { class: 'btn', href: `/api/export?year=${st.year}` }, 'Übersicht als Excel'),
    ),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null,
        h('tr', { class: 'group' }, th('', 'sticky s1', { colspan: 2 }), th('Wasser', '', { colspan: 4 }), th('Strom / Energie', '', { colspan: 4 }),
          th('', '', { colspan: 5 })),
        h('tr', { class: 'sub' }, th('Nr.', 'sticky s1'), th('Name', 'sticky s2'),
          th('Zähler-Nr.'), th('Stand Vorjahr'), th('Stand aktuell'), th('Verbrauch m³'),
          th('Zähler-Nr.'), th('Stand Vorjahr'), th('Stand aktuell'), th('Verbrauch kWh'),
          th('Arbeits-stunden'), th('Abschlag €'), th('Gesamt-betrag'), th('Status'), th('')),
      ), tbody)),
  );
}

// ------------------------------------------------------------------ Tab: Rechnungen

const fmtWhen = (iso) => { const d = new Date(iso); return isNaN(d) ? '' : d.toLocaleString('de-DE', { dateStyle: 'medium', timeStyle: 'short' }); };

function issueSummary(r) {
  return h('div', null,
    h('p', null, `${r.created.length} Rechnung(en) ausgestellt und gespeichert. Die PDF-Dateien liegen hier:`), h('p', { class: 'mono' }, r.folder),
    (r.skipped || []).length ? h('div', { class: 'banner warn' }, h('b', null, `${r.skipped.length} nicht ausgestellt:`),
      h('ul', { class: 'plain' }, r.skipped.map((x) => h('li', null, `${x.name}: ${x.reason}`)))) : null);
}

async function issue(ids, replace) {
  const st = S.state;
  const r = await api('POST', `/api/invoices/issue?year=${st.year}`, { ids, replace: !!replace });
  const open = await modal('Fertig', issueSummary(r), [{ label: 'Schließen', value: false }, { label: 'Ordner öffnen', cls: 'primary', value: true }]);
  if (open) await api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year });
  await reload();
}

// ------------------------------------------------------------------ Tab: Lageplan

// Kachelübersicht aller Gärten mit Status-Farbcode, sortiert nach Gartennummer
// (natürliche Sortierung, z. B. 2 vor 10). Ein echter geometrischer Lageplan
// bräuchte Koordinaten, die es nicht gibt – die Kacheln geben trotzdem auf
// einen Blick den Überblick, welcher Garten noch offen, unvollständig oder
// erledigt ist.
// Belegungshistorie eines Gartens: welcher Pächter hatte ihn wann. Bleibt
// auch nach endgültigem Löschen eines Pächters erhalten.
async function gartenHistorieDialog(gartennr) {
  let hist;
  try { hist = await api('GET', `/api/garten-historie?gartennr=${encodeURIComponent(gartennr)}`); } catch (e) { handleErr(e); return; }
  const chrono = [...hist].reverse();
  const row = (e) => h('tr', null, h('td', null, e.mitgliedsnr), h('td', null, e.name),
    h('td', null, deDate(e.seit)), h('td', null, e.bis ? deDate(e.bis) : h('span', { class: 'badge ok' }, 'aktuell')));
  const body = chrono.length
    ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Seit', 'Bis'].map((t) => h('th', null, t)))),
        h('tbody', null, chrono.map(row))))
    : h('p', { class: 'hint' }, 'Für diesen Garten liegt noch keine Historie vor.');
  await modal(`Verlauf Garten ${gartennr}`, body, [{ label: 'Schließen', value: true }]);
}

// Lageplan: ohne gespeicherte Anordnung ein Raster nach Gartennummer, sonst eine Karte, auf der jeder
// Garten an seinem Platz auf dem Gelände liegt. Im Admin-Bereich lassen sich die Kacheln per Maus anordnen.
const LP_W = 120, LP_H = 64, LP_RASTER = 10;

function viewLageplan() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const issuedOf = (p) => (st.issued || {})[p.id];
  // wer seinen Garten schon übergeben hat, steht nicht mehr auf der Karte (der Garten gehört dem Nachfolger)
  const list = st.paechter.filter((p) => !p.ausgeschiedenAm).sort((a, b) =>
    (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));

  const statusOf = (p) => {
    const res = st.results[p.id];
    if (!res || !res.vollstaendig) return 'unvollstaendig';
    const inf = issuedOf(p);
    if (!inf) return 'berechnet';
    const e = (st.archive || []).find((x) => x.id === inf.id);
    if (e && e.gesamt >= 0 && payIsOpen(e)) return 'offen';
    return 'bezahlt';
  };
  const labels = { unvollstaendig: 'Angaben fehlen', berechnet: 'berechnet, noch nicht ausgestellt', offen: 'Rechnung offen', bezahlt: 'bezahlt / kein offener Betrag' };
  const counts = { unvollstaendig: 0, berechnet: 0, offen: 0, bezahlt: 0, frei: 0 };
  const status = new Map(list.map((p) => { const x = statusOf(p); counts[x]++; return [p.id, x]; }));
  const edit = !!S.lageEntwurf && istAdmin && !st.readOnly;
  const kachel = (p, extra) => h('div', { class: 'kachel kachel-' + status.get(p.id) + (edit ? ' ziehbar' : ''), title: `${p.name}${p.notiz ? ' · 📝 ' + p.notiz : ''} – ${labels[status.get(p.id)]}`,
    onclick: edit ? null : () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); }, ...(extra || {}) },
    p.gartennr && !edit ? h('button', { class: 'kachel-hist', title: istAdmin ? `Gartenakte von Garten ${p.gartennr}` : `Verlauf von Garten ${p.gartennr}`,
      onclick: (e) => { e.stopPropagation(); if (istAdmin) gartenakteDialog(p.gartennr.trim()); else gartenHistorieDialog(p.gartennr); } }, icon(istAdmin ? 'akte' : 'uhr', 14)) : null,
    h('div', { class: 'kachel-nr' }, p.gartennr || p.mitgliedsnr), h('div', { class: 'kachel-name' }, p.name));
  const vergeben = istAdmin && !st.readOnly && !edit;
  // freier Garten: steht auf der Karte, hat aber keinen aktiven Pächter (z. B. nach einer Kündigung)
  const kachelFrei = (nr) => h('div', { class: 'kachel kachel-frei' + (edit ? ' ziehbar' : ''),
    title: vergeben ? `Garten ${nr} ist frei – klicken, um ihn an jemanden von der Warteliste zu vergeben` : `Garten ${nr} ist frei`,
    onclick: vergeben ? () => freienGartenVergeben(nr) : null },
  edit ? h('button', { class: 'kachel-hist', title: 'Garten von der Karte nehmen', onpointerdown: (e) => e.stopPropagation(),
    onclick: (e) => { e.stopPropagation(); delete S.lageEntwurf[nr]; render(); } }, '✕') : null,
  h('div', { class: 'kachel-nr' }, nr), h('div', { class: 'kachel-name' }, 'frei'));
  const legend = (cls, label) => h('span', { class: 'hint', style: 'margin-right:14px' },
    h('span', { class: 'kachel-dot kachel-' + cls }), ` ${label} (${counts[cls]})`);

  const gespeichert = st.lageplan || {};
  const pos = edit ? S.lageEntwurf : gespeichert;
  const hatKarte = Object.keys(gespeichert).length > 0;
  const mitNr = list.filter((p) => (p.gartennr || '').trim());
  const ohneNr = list.filter((p) => !(p.gartennr || '').trim());
  const belegt = new Set(mitNr.map((p) => p.gartennr.trim()));
  const freie = Object.keys(pos).filter((nr) => !belegt.has(nr)).sort((a, b) => a.localeCompare(b, 'de', { numeric: true }));
  counts.frei = freie.length;

  let kartenAnsicht;
  const kb = st.lageKarte || {};
  const hatHintergrund = !!kb.bild || (kb.objekte || []).length > 0 || (kb.leitungen || []).length > 0;
  if (!hatKarte && !edit && !hatHintergrund) {
    kartenAnsicht = h('div', { class: 'kachelgrid' }, list.map((p) => kachel(p)));
  } else {
    const K = edit ? S.lageKarteEntwurf : kb;
    const wz = edit ? S.lageWerkzeug : null;
    const platziert = mitNr.filter((p) => pos[p.gartennr.trim()]);
    const offen = mitNr.filter((p) => !pos[p.gartennr.trim()]);
    const ms = K.massstab || 1;
    const bildW = kb.bild ? Math.round(kb.bildBreite * ms) : 0, bildH = kb.bild ? Math.round(kb.bildHoehe * ms) : 0;
    let maxX = bildW - LP_W - 40, maxY = bildH - LP_H - 30;
    for (const q of Object.values(pos)) {
      maxX = Math.max(maxX, q.x); maxY = Math.max(maxY, q.y);
      for (const u of q.umriss || []) { maxX = Math.max(maxX, u.x - LP_W); maxY = Math.max(maxY, u.y - LP_H); }
    }
    for (const o of K.objekte || []) { maxX = Math.max(maxX, o.x - LP_W); maxY = Math.max(maxY, o.y - LP_H); }
    for (const l of K.leitungen || []) for (const u of l.punkte) { maxX = Math.max(maxX, u.x - LP_W); maxY = Math.max(maxY, u.y - LP_H); }
    const breite = Math.max(0, maxX) + LP_W + 40, hoehe = Math.max(0, maxY) + LP_H + (edit ? 160 : 30);
    const flaeche = h('div', { class: 'lagekarte' + (edit ? ' bearbeiten' : '') + (wz ? ' werkzeug' : '') + (kb.bild ? ' mit-bild' : ''), style: `width:${breite}px;height:${hoehe}px` });
    if (kb.bild) {
      flaeche.append(h('img', { class: 'lagekarte-bild', src: `/api/lageplan-bild?v=${encodeURIComponent(kb.bild)}`, alt: '', draggable: 'false',
        style: `width:${bildW}px;height:${bildH}px;opacity:${K.deckkraft || 0.85}` }));
      if (kb.quelle) flaeche.append(h('div', { class: 'lagekarte-quelle' }, kb.quelle));
    }
    const svg = svgEl('svg', { class: 'lage-svg', width: breite, height: hoehe });
    flaeche.append(svg);
    const punkteText = (pts) => pts.map((u) => `${u.x},${u.y}`).join(' ');
    // Leitungen
    (K.leitungen || []).forEach((l, i) => {
      const art = LP_LINIEN[l.art] || LP_LINIEN.wasser;
      const g = svgEl('g', { class: 'lage-linie' + (edit && !wz ? ' klickbar' : '') },
        svgEl('title', null, l.bez ? `${art[0]}: ${l.bez}` : art[0]),
        svgEl('polyline', { points: punkteText(l.punkte), class: 'trefferflaeche' }),
        svgEl('polyline', { points: punkteText(l.punkte), stroke: art[1], 'stroke-dasharray': art[2] || null }));
      if (edit && !wz) g.addEventListener('click', () => leitungDialog(i));
      svg.append(g);
    });
    // Gärten mit Umriss: Fläche plus kleines Schild in der Mitte
    const mitte = (u) => {
      let a = 0, cx = 0, cy = 0;
      for (let i = 0; i < u.length; i++) {
        const j = (i + 1) % u.length, k = u[i].x * u[j].y - u[j].x * u[i].y;
        a += k; cx += (u[i].x + u[j].x) * k; cy += (u[i].y + u[j].y) * k;
      }
      if (Math.abs(a) < 1) return { x: u.reduce((s, q) => s + q.x, 0) / u.length, y: u.reduce((s, q) => s + q.y, 0) / u.length };
      return { x: cx / (3 * a), y: cy / (3 * a) };
    };
    const umrissSetzen = (nr, el, klasse, aktion) => {
      const u = pos[nr].umriss;
      const poly = svgEl('polygon', { points: punkteText(u), class: 'umriss ' + klasse });
      if (!edit && aktion) { poly.classList.add('klickbar'); poly.addEventListener('click', aktion); }
      svg.append(poly);
      const m = mitte(u);
      el.classList.add('kachel-mini');
      el.style.left = Math.round(m.x) + 'px'; el.style.top = Math.round(m.y) + 'px';
      if (edit) umrissZiehbar(el, nr, poly);
      return el;
    };
    const kachelSetzen = (nr, el, klasse, aktion) => {
      if ((pos[nr].umriss || []).length >= 3) return umrissSetzen(nr, el, klasse, aktion);
      el.style.left = pos[nr].x + 'px'; el.style.top = pos[nr].y + 'px';
      if (edit) {
        ziehbarMachen(el, nr, flaeche);
        el.append(h('button', { class: 'kachel-umriss', title: `Umriss von Garten ${nr} auf der Karte nachzeichnen`, onpointerdown: (e) => e.stopPropagation(),
          onclick: (e) => { e.stopPropagation(); S.lageWerkzeug = { art: 'umriss', nr, punkte: [] }; render(); } }, '⬠'));
      }
      return el;
    };
    for (const p of platziert) {
      const nr = p.gartennr.trim();
      flaeche.append(kachelSetzen(nr, kachel(p), 'kachel-' + status.get(p.id), () => { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); }));
    }
    for (const nr of freie) flaeche.append(kachelSetzen(nr, kachelFrei(nr), 'kachel-frei', vergeben ? () => freienGartenVergeben(nr) : null));
    // Einrichtungen
    (K.objekte || []).forEach((o, i) => {
      const art = LP_OBJEKTE[o.art] || LP_OBJEKTE.sonstiges;
      const el = h('div', { class: 'lage-obj' + (edit ? ' ziehbar' : ''), style: `left:${o.x}px;top:${o.y}px;background:${art[2]}`, title: o.bez ? `${art[1]}: ${o.bez}` : art[1] }, art[0],
        o.bez ? h('span', { class: 'lage-obj-bez' }, o.bez) : null);
      if (edit) ziehenOderKlicken(el, {
        bewegen: (dx, dy, start) => { o.x = Math.max(0, Math.round(start.x + dx)); o.y = Math.max(0, Math.round(start.y + dy)); el.style.left = o.x + 'px'; el.style.top = o.y + 'px'; },
        start: () => ({ x: o.x, y: o.y }), ende: () => render(), klick: () => objektDialog(i) });
      flaeche.append(el);
    });
    if (wz) werkzeugAnbinden(flaeche, svg, pos);
    const wrap = h('div', { class: 'lagekarte-wrap', onscroll: (e) => { S.lageScroll = { l: e.target.scrollLeft, t: e.target.scrollTop }; } }, flaeche);
    if (S.lageScroll) { const sc = S.lageScroll; queueMicrotask(() => { wrap.scrollLeft = sc.l; wrap.scrollTop = sc.t; }); }
    kartenAnsicht = h('div', null, wrap,
      offen.length || ohneNr.length ? h('div', { style: 'margin-top:14px' },
        h('h3', { class: 'sect' }, edit ? 'Noch nicht auf der Karte – auf die Fläche ziehen' : 'Noch nicht auf der Karte'),
        h('div', { class: 'kachelgrid' }, offen.map((p) => {
          const el = kachel(p);
          if (edit) ziehbarMachen(el, p.gartennr.trim(), flaeche, true);
          return el;
        }), ohneNr.map((p) => kachel(p, { title: `${p.name}: ohne Gartennummer, kann nicht platziert werden` }))))
        : null);
  }

  if (S.lage3d && !edit && list.length) {
    const daten = {
      gaerten: [...mitNr.filter((p) => pos[p.gartennr.trim()] || !hatKarte).map((p, i) => {
        const nr = p.gartennr.trim(), q = pos[nr] || { x: 20 + (i % 8) * (LP_W + 20), y: 20 + Math.floor(i / 8) * (LP_H + 30) };
        return { nr, name: istAdmin ? p.name : '', status: status.get(p.id), text: labels[status.get(p.id)], x: q.x, y: q.y, umriss: q.umriss || null };
      }), ...freie.map((nr) => ({ nr, frei: true, status: 'frei', text: 'frei', x: pos[nr].x, y: pos[nr].y, umriss: pos[nr].umriss || null }))],
      objekte: kb.objekte || [], leitungen: kb.leitungen || [],
      bild: kb.bild ? { url: `/api/lageplan-bild?v=${encodeURIComponent(kb.bild)}`, w: kb.bildBreite * (kb.massstab || 1), h: kb.bildHoehe * (kb.massstab || 1), deckkraft: kb.deckkraft || 0.85 } : null,
      dachfarbe: parseInt((getComputedStyle(document.documentElement).getPropertyValue('--green').trim() || '#2f6f3e').replace('#', ''), 16) || 0x2f6f3e,
    };
    kartenAnsicht = h('div', null, lageplan3dAnsicht(daten),
      h('p', { class: 'hint', style: 'margin-top:8px' }, 'Ziehen dreht die Anlage, Mausrad zoomt, rechte Maustaste verschiebt. Ein Klick auf einen Garten öffnet ihn.'));
  }

  let leiste = null;
  if (istAdmin && !st.readOnly && list.length) {
    leiste = edit
      ? (S.lageWerkzeug ? werkzeugLeiste() : h('div', null, h('div', { class: 'banner info lage-leiste' },
        h('span', null, '✋ Ziehe die Gärten an ihren Platz. ⬠ auf einer Kachel zeichnet den Umriss der Parzelle nach. Gespeichert wird erst mit »Anordnung speichern«.'),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn small', title: 'Einen Garten ohne Pächter auf die Karte setzen, z. B. nach einer Kündigung', onclick: freienGartenHinzufuegen }, '+ Freier Garten'), ' ',
        h('button', { class: 'btn small', onclick: () => { S.lageEntwurf = null; S.lageKarteEntwurf = null; render(); } }, 'Abbrechen'), ' ',
        hatKarte ? h('button', { class: 'btn small danger', onclick: async () => {
          if (!(await confirmBox('Die Anordnung und alle Umrisse löschen und die Gärten wieder als Raster nach Nummer zeigen? Hintergrund, Leitungen und Einrichtungen bleiben.', 'Zurück zum Raster', true))) return;
          try { await api('PUT', '/api/admin/lageplan', { positionen: {} }); S.lageEntwurf = null; S.lageKarteEntwurf = null; toast('Lageplan zurückgesetzt', 'ok'); await reload(); } catch (e) { handleErr(e); }
        } }, 'Zurück zum Raster') : null, ' ',
        h('button', { class: 'btn small primary', onclick: async () => {
          try {
            await api('PUT', '/api/admin/lageplan', { positionen: S.lageEntwurf, karte: S.lageKarteEntwurf });
            S.lageEntwurf = null; S.lageKarteEntwurf = null; toast('Anordnung gespeichert', 'ok'); await reload();
          } catch (e) { handleErr(e); }
        } }, 'Anordnung speichern')),
      werkzeugKasten()))
      : h('button', { class: 'btn', onclick: () => { S.lage3d = false; S.lageEntwurf = startEntwurf(mitNr, gespeichert); S.lageKarteEntwurf = startKarteEntwurf(); S.lageWerkzeug = null; render(); } },
        h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, icon('karte', 18), hatKarte || hatHintergrund ? 'Karte bearbeiten' : 'Gärten wie auf dem Gelände anordnen'));
  }

  const umschalter = list.length && !edit ? h('div', { class: 'segment', role: 'group', 'aria-label': 'Ansicht' },
    h('button', { class: S.lage3d ? '' : 'an', onclick: () => { S.lage3d = false; render(); } }, 'Plan'),
    h('button', { class: S.lage3d ? 'an' : '', title: 'Die Anlage als 3D-Modell zum Drehen und Zoomen', onclick: () => { S.lage3d = true; render(); } }, '3D')) : null;
  const druck = list.length && !edit ? h('button', { class: 'btn', title: 'Lageplan als Aushang zum Ausdrucken (ohne Abrechnungsstand)', onclick: () => lageplanDrucken(istAdmin) },
    h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, icon('drucker', 18), 'Drucken')) : null;
  const wartende = st.wartelisteAnzahl || 0;
  return h('div', null,
    h('div', { class: 'toolbar' }, h('h2', { style: 'margin:0' }, `Lageplan ${st.year}`), h('div', { class: 'spacer' }), umschalter, druck, edit || S.lage3d ? null : leiste),
    freie.length && !edit ? h('div', { class: 'banner info' },
      h('span', null, `${freie.length === 1 ? `Garten ${freie[0]} ist` : `${freie.length} Gärten sind`} frei. `,
        wartende ? `${wartende} Interessent${wartende === 1 ? '' : 'en'} auf der Warteliste.` : 'Die Warteliste ist leer.'),
      istAdmin ? [' ', h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('warteliste'); render(); } }, 'Zur Warteliste')] : null) : null,
    h('p', { class: 'hint' }, hatKarte || edit ? 'Jeder Garten liegt an seinem Platz auf dem Gelände. Die Farbe zeigt den Abrechnungsstand, ein Klick öffnet die Schnelleingabe dieses Pächters.'
      : 'Kachel pro Garten, sortiert nach Gartennummer. Farbe zeigt den Abrechnungsstatus, Klick auf eine Kachel springt zur Schnelleingabe dieses Pächters.'),
    edit ? leiste : null,
    h('div', { style: 'margin-bottom:12px' }, legend('unvollstaendig', 'Angaben fehlen'), legend('berechnet', 'berechnet'), legend('offen', 'Rechnung offen'), legend('bezahlt', 'bezahlt'),
      freie.length ? legend('frei', 'frei') : null),
    list.length ? kartenAnsicht : leer('personen', 'Noch keine Gärten', 'Sobald Pächter mit Gartennummer angelegt sind, erscheint hier jeder Garten als Kachel mit seinem Abrechnungsstand.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); }));
}

// Einen freien Garten (ohne Pächter) auf die Karte setzen, unter die vorhandenen Gärten.
async function freienGartenHinzufuegen() {
  const nrIn = h('input', { type: 'text', maxlength: 20, placeholder: 'z. B. 14' });
  const nr = await modal('Freien Garten auf die Karte setzen', h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('label', null, 'Gartennummer'), nrIn,
      h('span', { class: 'fhint' }, 'Für einen Garten, der gerade keinen Pächter hat. Danach an seinen Platz ziehen und »Anordnung speichern«.'))),
  [{ label: 'Abbrechen', value: '' }, { label: 'Hinzufügen', cls: 'primary', value: true, action: () => {
    const v = nrIn.value.trim();
    if (!v) { toast('Bitte eine Gartennummer eintragen.', 'err'); return false; }
    if (S.lageEntwurf[v]) { toast(`Garten ${v} ist schon auf der Karte.`, 'err'); return false; }
    return true;
  } }]);
  if (!nr) return;
  const v = nrIn.value.trim();
  const maxY = Math.max(0, ...Object.values(S.lageEntwurf).map((q) => q.y));
  S.lageEntwurf[v] = { x: 20, y: Object.keys(S.lageEntwurf).length ? maxY + LP_H + 30 : 20 };
  render();
}

// Freien Garten an jemanden von der Warteliste vergeben (ohne Warteliste: Pächter von Hand anlegen).
async function freienGartenVergeben(nr) {
  let liste = [];
  try { liste = await api('GET', '/api/admin/warteliste'); } catch (e) { handleErr(e); return; }
  if (!liste.length) {
    const ok = await modal(`Garten ${nr} ist frei`, h('p', null, 'Auf der Warteliste steht niemand. Du kannst den neuen Pächter direkt anlegen.'),
      [{ label: 'Abbrechen', value: false }, { label: 'Pächter anlegen', cls: 'primary', value: true }]);
    if (ok && await paechterDialog(null, { gartennr: nr, mitgliedsnr: naechsteMitgliedsnr() })) await reload();
    return;
  }
  const zahl = (ph) => { const el = h('input', { type: 'text', placeholder: ph || '', autocomplete: 'off' }); el.setAttribute('inputmode', 'decimal'); return el; };
  const f = {
    wer: h('select', null, liste.map((b) => h('option', { value: b.id }, `${b.platz}. ${b.name}${b.wunsch ? ' – ' + b.wunsch : ''}`))),
    seit: h('input', { type: 'date', value: todayIso() }),
    mitgliedsnr: h('input', { type: 'text', value: naechsteMitgliedsnr(), maxlength: 20 }),
    groesse: zahl('m²'),
    wz: h('input', { type: 'text', maxlength: 40 }), sz: h('input', { type: 'text', maxlength: 40 }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  let neuerPaechter = null;
  const ok = await modal(`Garten ${nr} vergeben`, h('div', { class: 'form' },
    fld('An', f.wer, 'Name, Anschrift und Kontakt kommen von der Warteliste; der Eintrag wird danach entfernt.', true),
    fld('Übergeben am', f.seit), fld('Mitgliedsnummer *', f.mitgliedsnr), fld('Gartengröße in m² *', f.groesse),
    fld('Wasserzähler-Nr.', f.wz), fld('Stromzähler-Nr.', f.sz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Garten vergeben', cls: 'primary', value: true, action: async () => {
    const g = parseNum(f.groesse.value);
    if (!g || Number.isNaN(g) || g <= 0) { toast('Bitte die Gartengröße eintragen.', 'err'); return false; }
    try {
      neuerPaechter = await api('POST', `/api/admin/warteliste/${f.wer.value}/vergeben`, { gartennr: nr, seit: f.seit.value, mitgliedsnr: f.mitgliedsnr.value, gartengroesse: g, wasserzaehlerNr: f.wz.value, stromzaehlerNr: f.sz.value });
    } catch (e) { handleErr(e); return false; }
  } }]);
  if (!ok) return;
  S.warteliste = null;
  await reload();
  if (neuerPaechter && await modal(`Garten ${nr} vergeben`, h('p', null, `Garten ${nr} gehört jetzt ${neuerPaechter.name}. Möchtest du gleich die Willkommensmappe drucken?`),
    [{ label: 'Später', value: false }, { label: 'Willkommensmappe', cls: 'primary', value: true }])) await willkommenDrucken(neuerPaechter);
}

async function lageplanDrucken(istAdmin) {
  let namen = false;
  if (istAdmin) {
    const w = await modal('Lageplan drucken', h('div', null,
      h('p', null, 'Für den Aushang ohne Namen, für den Vorstand auf Wunsch mit den Namen der Pächter. Der Abrechnungsstand (bezahlt, offen) wird nie gedruckt.')),
    [{ label: 'Abbrechen', value: '' }, { label: 'Mit Namen (Vorstand)', value: 'namen' }, { label: 'Ohne Namen (Aushang)', cls: 'primary', value: 'aushang' }]);
    if (!w) return;
    namen = w === 'namen';
  }
  const q = namen ? '?namen=1' : '';
  await pdfDrucken({ url: `/api/lageplan-druck${q}`, oeffnenUrl: `/api/lageplan-druck/oeffnen${q}`, titel: 'Lageplan',
    meldung: () => 'Der Lageplan wurde im PDF-Programm geöffnet – dort drucken.' });
}

// Entwurf zum Anordnen: gespeicherte Positionen übernehmen; gibt es noch keine, alle Gärten als Raster vorlegen.
function startEntwurf(mitNr, gespeichert) {
  const e = {};
  for (const [k, v] of Object.entries(gespeichert)) e[k] = { x: v.x, y: v.y, umriss: (v.umriss || []).map((u) => ({ x: u.x, y: u.y })) };
  if (!Object.keys(e).length) {
    mitNr.forEach((p, i) => { e[p.gartennr.trim()] = { x: 20 + (i % 8) * (LP_W + 20), y: 20 + Math.floor(i / 8) * (LP_H + 30) }; });
  }
  return e;
}

// 3D-Ansicht: ein Element, das bei jedem Neuaufbau der Seite wiederverwendet wird (die Szene bleibt erhalten).
function lageplan3dAnsicht(daten) {
  if (!S.lage3dZ) S.lage3dZ = { el: h('div', { class: 'lage3d' }, h('div', { class: 'lage3d-laden' }, '3D-Ansicht wird aufgebaut …')), api: null, schluessel: '' };
  const z = S.lage3dZ, schluessel = JSON.stringify(daten);
  const klick = (nr) => {
    const p = S.state.paechter.find((x) => !x.ausgeschiedenAm && (x.gartennr || '').trim() === nr);
    if (p) { S.tab = 'eingabe'; S.eingabeMode = 'schnell'; S.quickId = p.id; render(); return; }
    if (!S.state.hasPassword || S.state.loggedIn) freienGartenVergeben(nr);
  };
  if (!z.api && !z.laedt) {
    z.laedt = true;
    import('/lageplan3d.js').then((m) => { z.api = m.erstelle(z.el, daten, { klick }); z.schluessel = schluessel; })
      .catch(() => { z.el.textContent = 'Die 3D-Ansicht ließ sich nicht laden.'; })
      .finally(() => { z.laedt = false; });
  } else if (z.api && z.schluessel !== schluessel) { z.api.aktualisiere(daten); z.schluessel = schluessel; }
  else if (z.api) z.api.weiter();
  return z.el;
}

// ---- Karte unter dem Lageplan: Hintergrundbild, Umrisse, Leitungen und Einrichtungen
// Kürzel, Name, Farbe (wie objektArten/linienArten in lagekarte.go)
const LP_OBJEKTE = {
  wasserzaehler: ['W', 'Wasserzähler', '#1d6ec4'], absperr: ['A', 'Absperrhahn', '#0e468c'], schacht: ['S', 'Schacht', '#6e6e6e'],
  strom: ['E', 'Stromverteiler', '#d68c00'], tor: ['T', 'Tor / Eingang', '#7d5028'], vereinsheim: ['V', 'Vereinsheim', '#2f6f3e'],
  kompost: ['K', 'Kompost / Container', '#5f7828'], parkplatz: ['P', 'Parkplatz', '#3c3ca0'], sonstiges: ['•', 'Sonstiges', '#963c78'],
};
const LP_LINIEN = { wasser: ['Wasserleitung', '#1d6ec4', ''], strom: ['Stromleitung', '#d68c00', ''], abwasser: ['Abwasser', '#785028', '9 5'], weg: ['Weg', '#8a8a8a', '9 5'] };

function startKarteEntwurf() {
  const kb = S.state.lageKarte || {};
  return { massstab: kb.massstab || 1, massstabStart: kb.massstab || 1, deckkraft: kb.deckkraft || 0.85, quelle: kb.quelle || '',
    objekte: (kb.objekte || []).map((o) => ({ ...o })), leitungen: (kb.leitungen || []).map((l) => ({ ...l, punkte: l.punkte.map((u) => ({ ...u })) })) };
}

// Ziehen mit der Maus; ein Klick ohne Bewegung ruft klick auf.
function ziehenOderKlicken(el, o) {
  el.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0 || ev.target.closest('button')) return;
    ev.preventDefault(); ev.stopPropagation();
    const sx = ev.clientX, sy = ev.clientY, start = o.start();
    let bewegt = false;
    el.setPointerCapture(ev.pointerId);
    const mv = (e) => {
      const dx = e.clientX - sx, dy = e.clientY - sy;
      if (!bewegt && Math.hypot(dx, dy) < 4) return;
      bewegt = true; el.classList.add('zieht');
      o.bewegen(dx, dy, start);
    };
    const up = (e) => {
      el.removeEventListener('pointermove', mv); el.removeEventListener('pointerup', up); el.removeEventListener('pointercancel', up);
      el.classList.remove('zieht');
      if (bewegt) o.ende(); else if (e.type === 'pointerup') o.klick();
    };
    el.addEventListener('pointermove', mv); el.addEventListener('pointerup', up); el.addEventListener('pointercancel', up);
  });
}

// Garten mit Umriss: das Schild in der Mitte verschiebt die ganze Fläche.
function umrissZiehbar(el, nr, poly) {
  let orig;
  ziehenOderKlicken(el, {
    start: () => { orig = S.lageEntwurf[nr].umriss.map((u) => ({ ...u })); return { x: parseFloat(el.style.left), y: parseFloat(el.style.top) }; },
    bewegen: (dx, dy, p0) => {
      dx = Math.round(Math.max(dx, -Math.min(...orig.map((u) => u.x)))); dy = Math.round(Math.max(dy, -Math.min(...orig.map((u) => u.y))));
      const u = orig.map((q) => ({ x: q.x + dx, y: q.y + dy }));
      S.lageEntwurf[nr] = { x: Math.min(...u.map((q) => q.x)), y: Math.min(...u.map((q) => q.y)), umriss: u };
      poly.setAttribute('points', u.map((q) => `${q.x},${q.y}`).join(' '));
      el.style.left = (p0.x + dx) + 'px'; el.style.top = (p0.y + dy) + 'px';
    },
    ende: () => render(),
    klick: () => umrissMenue(nr),
  });
}

async function umrissMenue(nr) {
  const w = await modal(`Garten ${nr}`, h('p', null, 'Der Garten ist als Fläche eingezeichnet. Zum Verschieben das Schild mit der Nummer ziehen.'),
    [{ label: 'Abbrechen', value: '' }, { label: 'Wieder als Kachel', value: 'kachel' }, { label: 'Umriss neu zeichnen', cls: 'primary', value: 'neu' }]);
  if (w === 'kachel') { const q = S.lageEntwurf[nr]; S.lageEntwurf[nr] = { x: q.x, y: q.y }; render(); }
  if (w === 'neu') { S.lageWerkzeug = { art: 'umriss', nr, punkte: [] }; render(); }
}

// Zeichenwerkzeug auf der Fläche: Klicks setzen Einrichtungen oder Punkte einer Linie bzw. eines Umrisses.
// Ecken vorhandener Umrisse rasten ein, damit benachbarte Parzellen lückenlos aneinanderliegen.
function werkzeugAnbinden(flaeche, svg, pos) {
  const wz = S.lageWerkzeug;
  const linie = wz.art === 'linie' ? LP_LINIEN[wz.typ] : null;
  const vorschau = svgEl(wz.art === 'umriss' ? 'polygon' : 'polyline', { class: 'lage-vorschau', stroke: linie ? linie[1] : null });
  const ecken = svgEl('g');
  svg.append(vorschau, ecken);
  const zeichne = () => {
    vorschau.setAttribute('points', wz.punkte.map((u) => `${u.x},${u.y}`).join(' '));
    ecken.replaceChildren(...wz.punkte.map((u, i) => svgEl('circle', { cx: u.x, cy: u.y, r: i === 0 ? 6 : 4, class: 'lage-vorschau-punkt' })));
  };
  S.lageVorschau = zeichne;
  if (wz.punkte) zeichne();
  const fang = [];
  for (const [nr, q] of Object.entries(pos)) if (wz.art !== 'umriss' || nr !== wz.nr) for (const u of q.umriss || []) fang.push(u);
  flaeche.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0 || ev.target.closest('button')) return;
    ev.preventDefault();
    const f = flaeche.getBoundingClientRect();
    let x = Math.max(0, Math.round(ev.clientX - f.left)), y = Math.max(0, Math.round(ev.clientY - f.top));
    let best = 9;
    for (const u of fang) { const d = Math.hypot(u.x - x, u.y - y); if (d < best) { best = d; x = u.x; y = u.y; } }
    if (wz.art === 'objekt') { S.lageKarteEntwurf.objekte.push({ art: wz.typ, x, y, bez: '' }); render(); return; }
    if (wz.art === 'umriss' && wz.punkte.length >= 3 && Math.hypot(wz.punkte[0].x - x, wz.punkte[0].y - y) < 10) { werkzeugFertig(); return; }
    wz.punkte.push({ x, y });
    zeichne();
  });
  flaeche.addEventListener('dblclick', (ev) => { ev.preventDefault(); werkzeugFertig(); });
}

function werkzeugFertig() {
  const wz = S.lageWerkzeug;
  if (!wz) return;
  if (wz.art === 'objekt') { S.lageWerkzeug = null; render(); return; }
  const pts = wz.punkte.filter((u, i, a) => i === 0 || Math.hypot(u.x - a[i - 1].x, u.y - a[i - 1].y) > 3);
  if (wz.art === 'umriss') {
    if (pts.length < 3) { toast('Ein Umriss braucht mindestens 3 Ecken.', 'err'); return; }
    S.lageEntwurf[wz.nr] = { x: Math.min(...pts.map((u) => u.x)), y: Math.min(...pts.map((u) => u.y)), umriss: pts };
  } else {
    if (pts.length < 2) { toast('Eine Linie braucht mindestens 2 Punkte.', 'err'); return; }
    S.lageKarteEntwurf.leitungen.push({ art: wz.typ, punkte: pts, bez: '' });
  }
  S.lageWerkzeug = null;
  render();
}

function werkzeugLeiste() {
  const wz = S.lageWerkzeug;
  const text = wz.art === 'objekt' ? `Klicke auf die Karte, um »${LP_OBJEKTE[wz.typ][1]}« zu setzen. Mehrere nacheinander gehen auch.`
    : wz.art === 'linie' ? `${LP_LINIEN[wz.typ][0]}: Punkt für Punkt auf die Karte klicken. Mit Doppelklick oder »Fertig« abschließen.`
      : `Umriss von Garten ${wz.nr}: die Ecken der Parzelle nacheinander anklicken. Ecken benachbarter Umrisse rasten ein. Klick auf die erste Ecke, Doppelklick oder »Fertig« schließt den Umriss.`;
  return h('div', { class: 'banner info lage-leiste' }, h('span', null, '✏️ ' + text), h('div', { class: 'spacer' }),
    wz.art !== 'objekt' ? h('button', { class: 'btn small', onclick: () => { wz.punkte.pop(); if (S.lageVorschau) S.lageVorschau(); } }, 'Letzten Punkt zurück') : null, ' ',
    wz.art !== 'objekt' ? h('button', { class: 'btn small', onclick: () => { S.lageWerkzeug = null; render(); } }, 'Abbrechen') : null, ' ',
    h('button', { class: 'btn small primary', onclick: werkzeugFertig }, 'Fertig'));
}

function werkzeugKasten() {
  const kb = S.state.lageKarte || {}, K = S.lageKarteEntwurf;
  const bildEl = () => document.querySelector('.lagekarte-bild');
  const regler = (label, min, max, step, wert, setze, anzeige) => {
    const out = h('span', { class: 'lage-regler-wert' }, anzeige(wert));
    return h('label', { class: 'lage-regler' }, label,
      h('input', { type: 'range', min, max, step, value: wert,
        oninput: (e) => { const v = Number(e.target.value); setze(v); out.textContent = anzeige(v); },
        onchange: () => render() }), out);
  };
  const stufe = Math.log2((K.massstab || 1) / (K.massstabStart || 1));
  return h('div', { class: 'lage-werkzeuge' },
    h('button', { class: 'btn small', onclick: lageHintergrundDialog }, h('span', { style: 'display:inline-flex;align-items:center;gap:6px' }, icon('karte', 16),
      kb.bild ? 'Hintergrund ändern' : 'Plan als Hintergrund')),
    kb.bild ? regler('Bildgröße', -2, 2, 0.05, stufe, (v) => {
      K.massstab = Math.round((K.massstabStart || 1) * Math.pow(2, v) * 10000) / 10000;
      const b = bildEl(); if (b) { b.style.width = Math.round(kb.bildBreite * K.massstab) + 'px'; b.style.height = Math.round(kb.bildHoehe * K.massstab) + 'px'; }
    }, (v) => Math.round(Math.pow(2, v) * 100) + ' %') : null,
    kb.bild ? regler('Deckkraft', 0.1, 1, 0.05, K.deckkraft, (v) => { K.deckkraft = v; const b = bildEl(); if (b) b.style.opacity = v; }, (v) => Math.round(v * 100) + ' %') : null,
    h('select', { class: 'lage-auswahl', onchange: (e) => { if (e.target.value) { S.lageWerkzeug = { art: 'objekt', typ: e.target.value }; render(); } } },
      h('option', { value: '' }, '+ Einrichtung setzen …'), Object.entries(LP_OBJEKTE).map(([k, v]) => h('option', { value: k }, `${v[0]} – ${v[1]}`))),
    h('select', { class: 'lage-auswahl', onchange: (e) => { if (e.target.value) { S.lageWerkzeug = { art: 'linie', typ: e.target.value, punkte: [] }; render(); } } },
      h('option', { value: '' }, '+ Leitung oder Weg zeichnen …'), Object.entries(LP_LINIEN).map(([k, v]) => h('option', { value: k }, v[0]))));
}

async function objektDialog(i) {
  const o = S.lageKarteEntwurf.objekte[i];
  const art = h('select', null, Object.entries(LP_OBJEKTE).map(([k, v]) => h('option', { value: k, selected: k === o.art }, `${v[0]} – ${v[1]}`)));
  const bez = h('input', { type: 'text', maxlength: 60, value: o.bez || '', placeholder: 'z. B. Hauptzähler, Hahn Weg B' });
  const w = await modal('Einrichtung', h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Art'), art),
    h('div', { class: 'field' }, h('label', null, 'Bezeichnung'), bez, h('span', { class: 'fhint' }, 'Steht neben dem Zeichen auf der Karte und im Ausdruck.'))),
  [{ label: 'Entfernen', cls: 'danger', value: 'weg' }, { label: 'Abbrechen', value: '' }, { label: 'Übernehmen', cls: 'primary', value: 'ok' }]);
  if (w === 'weg') S.lageKarteEntwurf.objekte.splice(i, 1);
  if (w === 'ok') { o.art = art.value; o.bez = bez.value.trim(); }
  if (w) render();
}

async function leitungDialog(i) {
  const l = S.lageKarteEntwurf.leitungen[i];
  const art = h('select', null, Object.entries(LP_LINIEN).map(([k, v]) => h('option', { value: k, selected: k === l.art }, v[0])));
  const bez = h('input', { type: 'text', maxlength: 60, value: l.bez || '', placeholder: 'z. B. Hauptleitung, 1 Zoll, 2019 erneuert' });
  const w = await modal('Leitung / Weg', h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Art'), art),
    h('div', { class: 'field' }, h('label', null, 'Bezeichnung'), bez, h('span', { class: 'fhint' }, 'Erscheint beim Darüberfahren mit der Maus.'))),
  [{ label: 'Entfernen', cls: 'danger', value: 'weg' }, { label: 'Abbrechen', value: '' }, { label: 'Übernehmen', cls: 'primary', value: 'ok' }]);
  if (w === 'weg') S.lageKarteEntwurf.leitungen.splice(i, 1);
  if (w === 'ok') { l.art = art.value; l.bez = bez.value.trim(); }
  if (w) render();
}

// Hintergrund: ein eigenes Bild, z. B. der eingescannte Vereinsplan.
async function lageHintergrundDialog() {
  const kb = S.state.lageKarte || {};
  let fertig = null;
  const nachLaden = async (k) => {
    if (S.lageKarteEntwurf) Object.assign(S.lageKarteEntwurf, { massstab: k.massstab, massstabStart: k.massstab, deckkraft: k.deckkraft, quelle: k.quelle || '' });
    await reload();
    if (fertig) fertig();
  };
  const datei = h('input', { type: 'file', accept: '.png,.jpg,.jpeg', style: 'display:none', onchange: async () => {
    if (!datei.files.length) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    try { const k = await api('POST', '/api/admin/lageplan/bild', fd, true); toast('Bild gespeichert', 'ok'); await nachLaden(k); } catch (e) { handleErr(e); }
  } });
  const quelle = h('input', { type: 'text', maxlength: 200, value: (S.lageKarteEntwurf && S.lageKarteEntwurf.quelle) || kb.quelle || '', placeholder: 'z. B. Plan: Vorstand 2024',
    oninput: () => { if (S.lageKarteEntwurf) S.lageKarteEntwurf.quelle = quelle.value; } });
  const body = h('div', { class: 'lage-hg' },
    h('p', { class: 'hint' }, 'Ein Bild der Anlage als Hintergrund, zum Beispiel ein Foto oder Scan des Vereinsplans (PNG oder JPG). Darauf ziehst du die Gärten an ihren Platz und zeichnest Umrisse, Leitungen und Einrichtungen ein. Achte bei fremden Karten auf die Nutzungsrechte.'),
    h('button', { class: 'btn primary', onclick: () => datei.click() }, kb.bild ? 'Anderes Bild wählen …' : 'Bild wählen …'), datei,
    kb.bild ? h('div', { style: 'margin-top:18px' },
      h('h4', null, 'Quellenvermerk'),
      h('div', { class: 'form' }, h('div', { class: 'field wide' }, quelle,
        h('span', { class: 'fhint' }, 'Steht klein auf dem Plan und im Ausdruck (freiwillig). Wird mit »Anordnung speichern« übernommen.'))),
      h('button', { class: 'btn danger', style: 'margin-top:10px', onclick: async () => {
        if (!(await confirmBox('Das Hintergrundbild entfernen? Gärten, Umrisse, Leitungen und Einrichtungen bleiben.', 'Entfernen', true))) return;
        try { const k = await api('DELETE', '/api/admin/lageplan/bild'); toast('Hintergrund entfernt', 'ok'); await nachLaden(k); } catch (e) { handleErr(e); }
      } }, 'Hintergrund entfernen')) : null);
  const p = modal('Hintergrund des Lageplans', body, [{ label: 'Schließen', value: true }]);
  const dlg = document.querySelector('dialog:last-of-type');
  fertig = () => { if (dlg && dlg.open) dlg.dispatchEvent(new Event('cancel')); };
  await p;
}

// Kachel per Maus (oder Finger) verschieben; Position rastet auf 10 px ein. Aus der Liste »noch nicht auf der
// Karte« gezogene Kacheln landen dort auf der Fläche, wo sie losgelassen werden.
function ziehbarMachen(el, nr, flaeche, ausListe) {
  el.addEventListener('pointerdown', (ev) => {
    if (ev.button !== 0) return;
    ev.preventDefault();
    const start = el.getBoundingClientRect();
    const dx = ev.clientX - start.left, dy = ev.clientY - start.top;
    const geist = ausListe ? el.cloneNode(true) : el;
    if (ausListe) { geist.classList.add('schwebt'); geist.style.position = 'fixed'; geist.style.width = LP_W + 'px'; document.body.append(geist); }
    el.setPointerCapture(ev.pointerId);
    el.classList.add('zieht');
    const bewegen = (e) => {
      if (ausListe) { geist.style.left = (e.clientX - dx) + 'px'; geist.style.top = (e.clientY - dy) + 'px'; return; }
      const f = flaeche.getBoundingClientRect();
      const x = Math.max(0, Math.round((e.clientX - f.left - dx) / LP_RASTER) * LP_RASTER);
      const y = Math.max(0, Math.round((e.clientY - f.top - dy) / LP_RASTER) * LP_RASTER);
      el.style.left = x + 'px'; el.style.top = y + 'px';
      S.lageEntwurf[nr] = { x, y };
    };
    const ende = (e) => {
      el.removeEventListener('pointermove', bewegen);
      el.removeEventListener('pointerup', ende);
      el.removeEventListener('pointercancel', ende);
      el.classList.remove('zieht');
      if (ausListe) {
        geist.remove();
        const f = flaeche.getBoundingClientRect();
        if (e.type === 'pointerup' && e.clientX >= f.left && e.clientX <= f.right && e.clientY >= f.top && e.clientY <= f.bottom) {
          S.lageEntwurf[nr] = { x: Math.max(0, Math.round((e.clientX - f.left - dx) / LP_RASTER) * LP_RASTER),
            y: Math.max(0, Math.round((e.clientY - f.top - dy) / LP_RASTER) * LP_RASTER) };
        }
      }
      render();
    };
    el.addEventListener('pointermove', bewegen);
    el.addEventListener('pointerup', ende);
    el.addEventListener('pointercancel', ende);
  });
}

function viewRechnungen() {
  const st = S.state;
  const sub = h('div', { class: 'subtabs' },
    h('button', { class: S.invTab !== 'archiv' ? 'active' : '', onclick: () => { S.invTab = 'pruefen'; render(); } }, 'Rechnungen prüfen & ausstellen'),
    h('button', { class: S.invTab === 'archiv' ? 'active' : '', onclick: () => { S.invTab = 'archiv'; render(); } }, `Archiv (${(st.archive || []).length})`));
  return h('div', null, h('h2', null, `Rechnungen ${st.year}`), sub, S.invTab === 'archiv' ? viewArchiv() : viewAusstellen());
}

async function loadArchivAlle() {
  if (S.archivAlleLoading) return;
  S.archivAlleLoading = true;
  try { S.archivAlle = await api('GET', '/api/archiv-alle'); } catch (e) { handleErr(e); }
  S.archivAlleLoading = false;
  render();
}

// Öffnet die gespeicherte Rechnungs-PDF im PDF-Programm des Rechners (dort gibt es den vollen Druckdialog).
async function imPdfProgrammOeffnen(id) {
  try { await api('POST', `/api/open-invoice/${id}`); toast('Die Rechnung wurde im PDF-Programm geöffnet – dort drucken.', 'ok'); }
  catch (e) { handleErr(e); }
}

// Öffnet das E-Mail-Programm mit Empfänger, Betreff und Text zur Rechnung und zeigt die PDF-Datei im
// Ordner an (zum Anhängen). Verschickt wird nichts automatisch.
async function emailVorbereiten(id) {
  try {
    const r = await api('POST', `/api/invoice-email/${id}`);
    toast(`E-Mail an ${r.email} ist im E-Mail-Programm vorbereitet. Bitte die markierte Datei »${r.datei}« anhängen und abschicken.`, 'ok');
    await reload();
  } catch (e) { handleErr(e); }
}

// Öffnet ein serverseitig erzeugtes PDF zum Drucken: im Programmfenster im PDF-Programm des Rechners
// (voller Druckdialog), sonst als Vorschau bzw. neuer Tab im Browser.
async function pdfDrucken({ url, oeffnenUrl, titel, meldung }) {
  try {
    if (inNativeWindow()) {
      const r = await api('POST', oeffnenUrl);
      toast(meldung(r), 'ok');
      return;
    }
    const r = await fetch(url, { headers: { 'X-GA-Request': '1' } });
    if (!r.ok) {
      const d = await r.json().catch(() => null);
      const err = new Error((d && d.error) || `Fehler ${r.status}`);
      err.status = r.status;
      throw err;
    }
    const blobUrl = URL.createObjectURL(await r.blob());
    openDocument(blobUrl, titel);
    setTimeout(() => URL.revokeObjectURL(blobUrl), 60000);
  } catch (e) { handleErr(e); }
}

// Alle Rechnungen mit Postversand als eine PDF-Datei zum Ausdrucken (nach Mitgliedsnummer sortiert).
const postRechnungenDrucken = (year) => pdfDrucken({
  url: `/api/admin/rechnungen-druck?year=${year}`, oeffnenUrl: `/api/admin/rechnungen-druck/oeffnen?year=${year}`,
  titel: 'Rechnungen Postversand', meldung: (r) => `${r.anzahl} Rechnung(en) im PDF-Programm geöffnet – dort drucken.`,
});

// Ablesebogen für den Rundgang: alle Gärten mit Zählernummern und Vorjahresständen, leere Felder zum Eintragen.
const ablesebogenDrucken = (year) => pdfDrucken({
  url: `/api/ablesebogen?year=${year}`, oeffnenUrl: `/api/ablesebogen/oeffnen?year=${year}`,
  titel: `Ablesebogen ${year}`, meldung: () => 'Der Ablesebogen wurde im PDF-Programm geöffnet – dort drucken.',
});

function archivRow(a, mitJahr) {
  const url = `/api/archive/${a.id}.pdf`;
  return h('tr', { class: a.status === 'ersetzt' ? 'ersetzt' : '' },
    mitJahr ? h('td', { class: 'mid' }, a.jahr) : null,
    h('td', null, a.mitgliedsnr), h('td', null, a.name), h('td', { class: 'mono' }, a.nummer), h('td', { class: 'mid' }, a.version),
    h('td', { class: 'r' }, eur(a.gesamt)), h('td', null, fmtWhen(a.ausgestellt)),
    h('td', null, h('span', { class: 'badge ' + (a.status === 'gueltig' ? 'ok' : 'warn') }, a.status === 'gueltig' ? 'gültig' : 'ersetzt')),
    h('td', null, h('a', { class: 'btn small', href: url, target: '_blank' }, 'Ansehen'), ' ',
      h('a', { class: 'btn small', href: url, download: a.datei ? a.datei.split('/').pop() : 'Rechnung.pdf' }, 'Speichern'),
      inNativeWindow() && a.datei ? [' ', h('button', { class: 'btn small', title: 'Im PDF-Programm öffnen und dort drucken', onclick: () => imPdfProgrammOeffnen(a.id) }, 'Drucken')] : null));
}

function viewArchiv() {
  const st = S.state;
  const q = S.archivSuche.trim().toLowerCase();
  const suchModus = q.length > 0;
  if (suchModus && !S.archivAlle && !S.archivAlleLoading) loadArchivAlle();

  const suchfeld = h('input', { type: 'search', placeholder: 'Suche über alle Jahre (Nummer oder Name)', value: S.archivSuche, style: 'width:300px',
    oninput: (e) => { S.archivSuche = e.target.value; render(); } });

  let body;
  if (suchModus) {
    if (!S.archivAlle) {
      body = h('div', { class: 'card empty' }, 'Wird geladen …');
    } else {
      const treffer = S.archivAlle.filter((a) => [a.mitgliedsnr, a.name, a.nummer].some((x) => (x || '').toLowerCase().includes(q)));
      body = treffer.length
        ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
            h('thead', null, h('tr', null, ['Jahr', 'Nr.', 'Name', 'Rechnungs-Nr.', 'Version', 'Betrag', 'Ausgestellt am', 'Status', ''].map((t) => h('th', null, t)))),
            h('tbody', null, treffer.map((a) => archivRow(a, true)))))
        : h('div', { class: 'card empty' }, 'Keine Treffer.');
    }
  } else {
    const rows = st.archive || [];
    body = rows.length
      ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Nr.', 'Name', 'Rechnungs-Nr.', 'Version', 'Betrag', 'Ausgestellt am', 'Status', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows.map((a) => archivRow(a, false)))))
      : leer('rechnung', 'Noch keine Rechnung ausgestellt', `Sobald du im Bereich »Rechnungen prüfen & ausstellen« Rechnungen ausstellst, liegen sie hier für ${st.year} – mit den Werten von damals, unveränderlich.`, 'Zum Ausstellen', () => { S.invTab = 'pruefen'; render(); });
  }

  return h('div', null,
    h('p', { class: 'hint' }, `Hier liegen alle ausgestellten Rechnungen des Jahres ${st.year}. Sie sind mit den Preisen und Angaben von damals gespeichert und ändern sich nie, auch wenn du später Preise, Namen oder Zählerstände änderst. Wurde eine Rechnung neu ausgestellt, bleibt die alte als »ersetzt« erhalten.`),
    h('div', { class: 'toolbar' }, suchfeld, h('div', { class: 'spacer' }),
      h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr) }, 'Rechnungsordner öffnen')),
    body);
}

function viewAusstellen() {
  const st = S.state;
  const list = [...st.paechter].sort(cmpNr);
  if (!list.some((p) => p.id === S.selInvoice)) S.selInvoice = list.length ? list[0].id : null;
  const right = h('div');
  const issuedOf = (p) => (st.issued || {})[p.id];

  const drawPreview = () => {
    right.replaceChildren();
    const p = list.find((x) => x.id === S.selInvoice);
    if (!p) { right.append(leer('rechnung', 'Noch keine Pächter', 'Rechnungen gibt es, sobald Pächter angelegt und ihre Zählerstände eingetragen sind.', 'Pächter anlegen', () => { adminTabWaehlen('paechter'); render(); })); return; }
    const res = st.results[p.id];
    const inf = issuedOf(p);
    const showLive = !inf || S.invView === 'live';
    const url = showLive ? `/api/invoice/${p.id}.pdf?year=${st.year}` : `/api/archive/${inf.id}.pdf`;
    const canShow = showLive ? res.vollstaendig : true;

    const bar = h('div', { class: 'toolbar' }, h('b', null, `${p.mitgliedsnr} – ${p.name}`), schlussBadge(p), h('div', { class: 'spacer' }));
    if (!inf && res.vollstaendig) {
      bar.append(h('button', { class: 'btn primary', onclick: async () => { try { await issue([p.id], false); } catch (e) { handleErr(e); } } }, 'Rechnung ausstellen & speichern'));
    }
    if (canShow) {
      bar.append(h('a', { class: 'btn', href: url, target: '_blank' }, 'PDF öffnen / drucken'),
        h('a', { class: 'btn', href: url, download: `Rechnung_${p.mitgliedsnr}.pdf` }, 'PDF speichern'));
      if (inNativeWindow() && inf && !showLive) {
        bar.append(h('button', { class: 'btn', title: 'Im PDF-Programm des Rechners öffnen und dort drucken', onclick: () => imPdfProgrammOeffnen(inf.id) }, 'Im PDF-Programm öffnen'));
      }
      if (inf && !showLive) {
        bar.append(h('button', { class: 'btn', disabled: !p.email,
          title: p.email ? `Öffnet das E-Mail-Programm mit einer fertigen Nachricht an ${p.email}; die PDF-Datei wird im Ordner markiert und muss nur noch angehängt werden`
            : 'Für diesen Pächter ist keine E-Mail-Adresse eingetragen (unter »Pächter«)',
          onclick: () => emailVorbereiten(inf.id) }, inf.emailAm ? '✉ E-Mail erneut vorbereiten' : '✉ E-Mail vorbereiten'));
      }
    }
    right.append(bar);

    if (inf) {
      right.append(h('div', { class: 'banner info' },
        `Ausgestellt am ${fmtWhen(inf.ausgestellt)} · Rechnungs-Nr. ${inf.nummer}${inf.version > 1 ? ` (Version ${inf.version})` : ''} · ${eur(inf.gesamt)}. Diese Rechnung ist gespeichert und ändert sich nicht mehr.${inf.emailAm ? ` E-Mail vorbereitet am ${fmtWhen(inf.emailAm)}.` : ''}`));
      if (inf.geaendert) {
        right.append(h('div', { class: 'banner warn' },
          'Seit der Ausstellung wurden Werte geändert (Zählerstände, Preise oder Pächterdaten). Die ausgestellte Rechnung bleibt unverändert. ',
          h('button', { class: 'btn small', onclick: () => { S.invView = showLive ? 'archiv' : 'live'; drawPreview(); } }, showLive ? 'Ausgestellte Rechnung ansehen' : 'Mit aktuellen Werten ansehen'), ' ',
          res.vollstaendig ? h('button', { class: 'btn small', onclick: async () => {
            const ok = await confirmBox(`Für ${p.name} wird eine neue Rechnung mit den aktuellen Werten ausgestellt. Die bisherige bleibt als »ersetzt« im Archiv. Fortfahren?`, 'Neu ausstellen');
            if (ok) { try { S.invView = 'archiv'; await issue([p.id], true); } catch (e) { handleErr(e); } }
          } }, 'Neu ausstellen (ersetzt die alte)') : null));
      }
    }
    if (!canShow) {
      right.append(h('div', { class: 'banner warn' }, `Für diese Rechnung fehlt noch etwas: ${res.status}. Bitte im Tab »Zählerstände« ergänzen.`,
        ' ', h('button', { class: 'btn small', onclick: () => { S.tab = 'eingabe'; S.filter = p.mitgliedsnr; render(); } }, 'Zu den Zählerständen')));
      return;
    }
    right.append(h('div', { class: 'preview' }, h('iframe', { src: `${url}${showLive ? '&t=' + Date.now() : ''}#view=FitH`, title: 'Rechnungsvorschau' })));
  };

  const items = h('div', { class: 'card list' });
  const drawList = () => {
    items.replaceChildren();
    for (const p of list) {
      const ok = st.results[p.id] && st.results[p.id].vollstaendig;
      const inf = issuedOf(p);
      const cls = inf ? (inf.geaendert ? 'chg' : 'iss') : ok ? 'ok' : '';
      const tip = inf ? (inf.geaendert ? 'ausgestellt, danach geändert' : 'ausgestellt') : ok ? 'bereit zum Ausstellen' : 'unvollständig';
      items.append(h('button', { class: 'item' + (p.id === S.selInvoice ? ' active' : ''), onclick: () => { S.selInvoice = p.id; S.invView = 'archiv'; drawList(); drawPreview(); } },
        h('span', { class: 'dot ' + cls, title: tip }),
        h('span', { class: 'nr' }, p.mitgliedsnr), h('span', null, p.name, schlussBadge(p)),
        inf && inf.emailAm ? h('span', { title: `E-Mail vorbereitet am ${fmtWhen(inf.emailAm)}`, style: 'margin-left:auto' }, '✉') : null));
    }
    if (!list.length) items.append(h('div', { class: 'empty' }, 'Keine Pächter.'));
  };
  drawList();
  drawPreview();

  const open = list.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig && !issuedOf(p));
  const nIss = list.filter((p) => issuedOf(p)).length;
  const allBtn = h('button', { class: 'btn primary', disabled: !open.length, onclick: async () => {
    const ok = await confirmBox(`${open.length} Rechnung(en) werden jetzt ausgestellt und als PDF gespeichert. Unvollständige Rechnungen werden übersprungen. Fortfahren?`, 'Ausstellen');
    if (!ok) return;
    allBtn.disabled = true;
    try { await issue([], false); } catch (e) { handleErr(e); }
    allBtn.disabled = false;
  } }, `Alle offenen ausstellen (${open.length})`);

  // gezählt wird wie im PDF: nach der Versandart auf der ausgestellten Rechnung, nicht nach den heutigen Stammdaten
  const nPost = (st.archive || []).filter((e) => e.status === 'gueltig' && String(e.versand || '').toLowerCase() === 'postversand').length;
  const postBtn = h('button', { class: 'btn', disabled: !nPost,
    title: 'Alle ausgestellten Rechnungen mit Versandart Postversand als eine PDF-Datei, nach Mitgliedsnummer sortiert',
    onclick: () => postRechnungenDrucken(st.year) }, `Postversand drucken (${nPost})`);
  // E-Mail-Versand nacheinander: springt zur nächsten ausgestellten Rechnung mit Versandart Emailsendung,
  // deren E-Mail noch nicht vorbereitet wurde
  const mailOffen = list.filter((p) => p.versand === 'Emailsendung' && p.email && issuedOf(p) && !issuedOf(p).emailAm);
  const ohneAdresse = list.filter((p) => p.versand === 'Emailsendung' && !p.email && issuedOf(p)).length;
  const mailBtn = h('button', { class: 'btn', disabled: !mailOffen.length,
    title: `Bereitet die E-Mail für die nächste Rechnung mit Versandart Emailsendung vor${ohneAdresse ? ` (${ohneAdresse} ohne eingetragene E-Mail-Adresse)` : ''}`,
    onclick: async () => { const p = mailOffen[0]; S.selInvoice = p.id; S.invView = 'archiv'; await emailVorbereiten(issuedOf(p).id); } },
  `✉ Nächste E-Mail (${mailOffen.length} offen)`);

  return h('div', null,
    h('p', { class: 'hint' }, 'Wähle links einen Pächter und prüfe die Rechnung. Mit »Ausstellen« wird sie festgeschrieben, als PDF gespeichert und ins Archiv gelegt. Punkte: grau = unvollständig, grün = bereit, blau = ausgestellt, orange = ausgestellt, danach geändert.'),
    h('div', { class: 'toolbar' }, allBtn, h('span', { class: 'hint' }, `${nIss} von ${list.length} ausgestellt`), h('div', { class: 'spacer' }), st.readOnly ? null : mailBtn, postBtn,
      h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'rechnungen', year: st.year }).catch(handleErr) }, 'Rechnungsordner öffnen')),
    h('div', { class: 'split' }, items, right));
}

// ------------------------------------------------------------------ Tab: Zahlungen

const deDate = (iso) => (iso ? iso.split('-').reverse().join('.') : '');

// volle Jahre seit einem Datum (JJJJ-MM-TT), Geburtstags-genau gerechnet
function jahreSeit(iso) {
  const seit = new Date(iso + 'T00:00:00');
  const now = new Date();
  let jahre = now.getFullYear() - seit.getFullYear();
  const schonGehabt = now.getMonth() > seit.getMonth() || (now.getMonth() === seit.getMonth() && now.getDate() >= seit.getDate());
  if (!schonGehabt) jahre--;
  return Math.max(0, jahre);
}
// isoInTagen: heutiges Datum (Ortszeit) plus n Tage als JJJJ-MM-TT.
const isoInTagen = (n) => { const d = new Date(); d.setDate(d.getDate() + n); const p = (x) => String(x).padStart(2, '0'); return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`; };
const todayIso = () => isoInTagen(0);
const payTotal = (e) => Math.abs(e.gesamt);
const payPaid = (e) => (e.bezahltAm ? (e.bezahltBetrag != null ? e.bezahltBetrag : payTotal(e)) : 0);
const payOpen = (e) => Math.round((payTotal(e) - payPaid(e)) * 100) / 100;
const payIsOpen = (e) => payOpen(e) > 0.004;

async function savePayment(e, patch) {
  const body = { bezahltAm: e.bezahltAm || '', bezahltBetrag: e.bezahltBetrag ?? null, notiz: e.notiz || '', ...patch };
  await api('PUT', `/api/payment/${e.id}`, body);
  Object.assign(e, { bezahltAm: body.bezahltAm, bezahltBetrag: body.bezahltAm ? body.bezahltBetrag : null, notiz: body.notiz });
  // volle Zahlung: Betrag nicht separat führen
  if (e.bezahltBetrag != null && Math.round(e.bezahltBetrag * 100) === Math.round(payTotal(e) * 100)) e.bezahltBetrag = null;
  if (e.bezahltAm && payOpen(e) < -0.004) toast(`Hinweis: ${eur(-payOpen(e))} mehr als der Rechnungsbetrag eingetragen.`, 'ok');
  // war das die letzte offene Rechnung des Jahres?
  const st = S.state, forderungen = (st.archive || []).filter((x) => x.status === 'gueltig' && x.gesamt > 0);
  S.erfolgGezeigt = S.erfolgGezeigt || {};
  if (e.bezahltAm && forderungen.length >= 3 && !forderungen.some(payIsOpen) && !S.erfolgGezeigt[st.year]) {
    S.erfolgGezeigt[st.year] = true;
    erfolgsHinweis(`Alle Rechnungen ${st.year} sind bezahlt`, `${forderungen.length} Rechnungen, zusammen ${eur(forderungen.reduce((a, x) => a + payTotal(x), 0))}. Gute Arbeit!`);
  }
}

async function paymentDialog(e) {
  const date = h('input', { type: 'date', value: e.bezahltAm || todayIso() });
  const amt = h('input', { type: 'text', inputmode: 'decimal', value: numIn(e.bezahltAm ? payPaid(e) : payTotal(e)), style: 'width:120px' });
  const note = h('input', { type: 'text', maxlength: 200, value: e.notiz || '', style: 'width:100%' });
  const body = h('div', { class: 'form' },
    h('p', null, `${e.mitgliedsnr} – ${e.name}: Rechnungsbetrag ${eur(payTotal(e))}${e.gesamt < 0 ? ' (Guthaben, wird an den Pächter ausgezahlt)' : ''}`),
    h('div', { class: 'field' }, h('label', null, 'Bezahlt am'), date),
    h('div', { class: 'field' }, h('label', null, 'Bezahlter Betrag (bei Teilzahlung anpassen)'), h('div', { class: 'inputunit' }, amt, h('span', null, '€'))),
    h('div', { class: 'field wide' }, h('label', null, 'Notiz (z. B. bar, Überweisung, Ratenzahlung)'), note));
  return modal('Zahlung vermerken', body, [
    { label: 'Abbrechen', value: false },
    e.bezahltAm ? { label: 'Zahlung zurücknehmen', cls: 'danger', value: 'undo', action: async () => { try { await savePayment(e, { bezahltAm: '', bezahltBetrag: null }); } catch (er) { handleErr(er); return false; } } } : null,
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const v = parseNum(amt.value);
      if (!date.value) { toast('Bitte ein Datum angeben.', 'err'); return false; }
      if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte einen gültigen Betrag angeben.', 'err'); return false; }
      try { await savePayment(e, { bezahltAm: date.value, bezahltBetrag: v, notiz: note.value.trim() }); } catch (er) { handleErr(er); return false; }
    } },
  ].filter(Boolean));
}

async function mahnungErzeugen(ids) {
  if (!ids.length) { toast('Bitte mindestens eine offene Rechnung auswählen.', 'err'); return; }
  try {
    const r = await fetch('/api/admin/mahnung', { method: 'POST', headers: { 'X-GA-Request': '1', 'Content-Type': 'application/json' }, body: JSON.stringify({ ids }) });
    if (!r.ok) { const d = await r.json().catch(() => null); throw new Error((d && d.error) || `Fehler ${r.status}`); }
    const blob = await r.blob();
    const url = URL.createObjectURL(blob);
    if ((r.headers.get('content-type') || '').includes('pdf')) openDocument(url, 'Mahnung');
    else { const a = h('a', { href: url, download: 'Mahnungen.zip' }); document.body.append(a); a.click(); a.remove(); }
    setTimeout(() => URL.revokeObjectURL(url), 30000);
  } catch (e) { handleErr(e); }
}

function viewZahlungen() {
  const st = S.state;
  const all = (st.archive || []).filter((e) => e.status === 'gueltig');
  const today = todayIso();
  const summary = h('div', { class: 'cards' });
  const tbody = h('tbody');
  const filters = h('div', { class: 'subtabs' });
  const mahnBar = h('div', { class: 'toolbar' });
  S.mahnSel = S.mahnSel instanceof Set ? S.mahnSel : new Set();

  const draw = () => {
    const sum = (arr, f) => arr.reduce((a, e) => a + f(e), 0);
    const claims = all.filter((e) => e.gesamt >= 0), credits = all.filter((e) => e.gesamt < 0);
    const openClaims = claims.filter(payIsOpen), overdue = openClaims.filter((e) => e.faellig && e.faellig < today);
    const card = (t, big, small, cls, ic) => h('div', { class: 'card stat ' + (cls || '') }, ic ? h('div', { class: 'statico' }, icon(ic, 22)) : null,
    h('div', null, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small)));
    summary.replaceChildren(...[
      card('Forderungen gesamt', eur(sum(claims, payTotal)), `${claims.length} Rechnungen`),
      card('Bereits bezahlt', eur(sum(claims, payPaid)), `${claims.filter((e) => !payIsOpen(e)).length} vollständig bezahlt`, 'ok'),
      card('Noch offen', eur(sum(openClaims, payOpen)), `${openClaims.length} Rechnungen`, openClaims.length ? 'warn' : 'ok'),
      card('Überfällig', String(overdue.length), overdue.length ? eur(sum(overdue, payOpen)) : 'nichts überfällig', overdue.length ? 'err' : 'ok'),
      credits.length ? card('Guthaben auszuzahlen', eur(sum(credits.filter(payIsOpen), payOpen)), `${credits.filter(payIsOpen).length} von ${credits.length} noch offen`) : null].filter(Boolean));

    mahnBar.replaceChildren(
      h('button', { class: 'btn', disabled: !S.mahnSel.size, onclick: () => mahnungErzeugen([...S.mahnSel]) },
        `Mahnung erzeugen${S.mahnSel.size ? ` (${S.mahnSel.size})` : ''}`),
      h('span', { class: 'hint' }, 'Haken bei den gewünschten offenen Rechnungen setzen, dann »Mahnung erzeugen«. Es wird nie automatisch gemahnt.'));

    const nOpen = all.filter(payIsOpen).length;
    filters.replaceChildren(
      ...[['offen', `Noch offen (${nOpen})`], ['bezahlt', `Bezahlt (${all.length - nOpen})`], ['alle', `Alle (${all.length})`]].map(([k, t]) =>
        h('button', { class: S.payFilter === k ? 'active' : '', onclick: () => { S.payFilter = k; draw(); } }, t)));

    const q = S.payQ.trim().toLowerCase();
    const shown = all.filter((e) => (S.payFilter === 'alle' || (S.payFilter === 'offen') === payIsOpen(e)) &&
      (!q || [e.mitgliedsnr, e.name, e.nummer].some((x) => (x || '').toLowerCase().includes(q))))
      .sort((a, b) => a.mitgliedsnr.localeCompare(b.mitgliedsnr, 'de', { numeric: true }));
    tbody.replaceChildren();
    for (const e of shown) {
      const open = payIsOpen(e);
      const late = open && e.gesamt >= 0 && e.faellig && e.faellig < today;
      const partial = e.bezahltAm && open;
      const chk = h('input', { type: 'checkbox', checked: !!e.bezahltAm && !open, title: 'als bezahlt markieren',
        onchange: async (ev) => {
          try {
            if (ev.target.checked) await savePayment(e, { bezahltAm: todayIso(), bezahltBetrag: null });
            else await savePayment(e, { bezahltAm: '', bezahltBetrag: null });
            draw();
          } catch (er) { ev.target.checked = !ev.target.checked; handleErr(er); }
        } });
      const date = h('input', { type: 'date', value: e.bezahltAm || '', disabled: !e.bezahltAm, title: 'Zahldatum',
        onchange: async (ev) => {
          if (!ev.target.value) { ev.target.value = e.bezahltAm; return; }
          try { await savePayment(e, { bezahltAm: ev.target.value }); draw(); } catch (er) { handleErr(er); }
        } });
      const mahnbar = open && e.gesamt >= 0;
      const mchk = h('input', { type: 'checkbox', checked: S.mahnSel.has(e.id), disabled: !mahnbar, title: mahnbar ? 'für Mahnung auswählen' : 'nicht offen oder ein Guthaben',
        onchange: (ev) => { if (ev.target.checked) S.mahnSel.add(e.id); else S.mahnSel.delete(e.id); draw(); } });
      tbody.append(h('tr', { class: late ? 'late' : '' },
        h('td', null, mchk),
        h('td', null, e.mitgliedsnr), h('td', null, e.name), h('td', { class: 'mono' }, e.nummer),
        h('td', { class: 'r' }, (e.gesamt < 0 ? '−' : '') + eur(payTotal(e))),
        h('td', null, deDate(e.faellig)),
        h('td', null, h('span', { class: 'badge ' + (partial ? 'warn' : !open ? 'ok' : late ? 'err' : 'neu') },
          partial ? `Teilzahlung, offen ${eur(payOpen(e))}` : !open ? (e.gesamt < 0 ? 'ausgezahlt' : 'bezahlt') : late ? 'überfällig' : e.gesamt < 0 ? 'Guthaben offen' : 'offen')),
        h('td', { class: 'paycell' }, chk, date),
        h('td', { class: 'note', title: e.notiz || '' }, e.notiz || ''),
        h('td', null, h('button', { class: 'btn small', onclick: async () => { const r = await paymentDialog(e); if (r !== false && r !== undefined) draw(); } }, 'Details'))));
    }
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: 10, class: 'empty' },
      !all.length ? leer('euro', 'Noch nichts zu kassieren', `Für ${st.year} wurde noch keine Rechnung ausgestellt. Sobald das passiert ist, hakst du hier die Zahlungen ab.`, 'Zu den Rechnungen', () => { S.tab = 'rechnungen'; render(); })
        : S.payFilter === 'offen' && !q ? leer('fertig', 'Alles bezahlt', 'Für dieses Jahr ist nichts mehr offen. 🎉') : 'Keine Treffer.')));
  };
  draw();

  const notIssued = st.paechter.length - Object.keys(st.issued || {}).length;
  return h('div', null,
    h('h2', null, `Zahlungen ${st.year}`),
    h('p', { class: 'hint' }, 'Hier siehst du, welche Rechnungen bezahlt sind und welche noch offen. Haken setzen = bezahlt heute (das Datum kannst du ändern). Über »Details« trägst du Teilzahlungen oder eine Notiz ein. Rechnungen mit Guthaben erscheinen hier auch, damit du siehst, was noch an Pächter auszuzahlen ist.'),
    notIssued > 0 ? h('div', { class: 'banner info' }, `${notIssued} Pächter ${notIssued === 1 ? 'hat' : 'haben'} in ${st.year} noch keine ausgestellte Rechnung und ${notIssued === 1 ? 'taucht' : 'tauchen'} deshalb hier noch nicht auf.`) : null,
    anderenJahrenOffen(st),
    summary,
    h('div', { class: 'toolbar' },
      h('input', { type: 'search', placeholder: 'Suchen (Nummer oder Name)', value: S.payQ, style: 'width:240px', oninput: (e) => { S.payQ = e.target.value; draw(); } }),
      !st.hasPassword || st.loggedIn ? h('button', { class: 'btn primary', title: 'CSV-Datei aus dem Online-Banking: Eingänge werden den Rechnungen zugeordnet', onclick: kontoauszugEinlesen }, 'Kontoauszug einlesen') : null,
      h('div', { class: 'spacer' }),
      h('a', { class: 'btn', href: `/api/export-payments?year=${st.year}&nur=offen` }, 'Offene Posten als Excel'),
      h('a', { class: 'btn', href: `/api/export-payments?year=${st.year}` }, 'Alle Zahlungen als Excel'),
      h('a', { class: 'btn', href: `/api/export-ics?year=${st.year}`, title: 'Fälligkeitstermine der offenen Rechnungen zum Import in den eigenen Kalender' }, 'Fälligkeiten als Kalender (.ics)')),
    filters,
    mahnBar,
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['', 'Nr.', 'Name', 'Rechnungs-Nr.', 'Betrag', 'Fällig am', 'Status', 'Bezahlt / am', 'Notiz', ''].map((t) => h('th', null, t)))),
      tbody)));
}

// ------------------------------------------------------------------ Spenden und Zuwendungsbestätigungen

async function loadSpenden() {
  if (S.spendenLoading) return;
  S.spendenLoading = true;
  try { S.spenden = await api('GET', '/api/admin/spenden'); } catch (e) { handleErr(e); S.spenden = { spenden: [], gemeinnuetzigkeit: {} }; }
  S.spendenLoading = false;
  render();
}

function spendeDialog(x) {
  const st = S.state;
  const isNew = !x || !x.id;
  const cur = { datum: todayIso(), betrag: '', name: '', strasse: '', plzOrt: '', paechterId: '', verzicht: false, notiz: '', ...(x || {}) };
  const gesperrt = !!cur.bestaetigtAm;
  const f = {
    paechter: h('select', { disabled: gesperrt }, [h('option', { value: '' }, '– kein Pächter / andere Person –'),
      ...st.paechter.filter((p) => !p.geloescht).map((p) => h('option', { value: p.id, selected: p.id === cur.paechterId }, `${p.gartennr ? 'Garten ' + p.gartennr + ' · ' : ''}${p.name}`))]),
    datum: h('input', { type: 'date', value: cur.datum, disabled: gesperrt }),
    betrag: h('input', { type: 'text', inputmode: 'decimal', value: cur.betrag === '' ? '' : numIn(cur.betrag), disabled: gesperrt, placeholder: '0,00' }),
    name: h('input', { type: 'text', value: cur.name, maxlength: 100, disabled: gesperrt }),
    strasse: h('input', { type: 'text', value: cur.strasse, maxlength: 100 }),
    plzOrt: h('input', { type: 'text', value: cur.plzOrt, maxlength: 80 }),
    verzicht: h('input', { type: 'checkbox', checked: !!cur.verzicht, disabled: gesperrt }),
    notiz: h('input', { type: 'text', value: cur.notiz || '', maxlength: 200 }),
  };
  f.paechter.addEventListener('change', () => {
    const p = st.paechter.find((q) => q.id === f.paechter.value);
    if (p) { f.name.value = p.name; f.strasse.value = p.strasse || ''; f.plzOrt.value = p.plzOrt || ''; }
  });
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', null,
    gesperrt ? h('div', { class: 'banner info' }, `Für diese Spende wurde am ${deDate(cur.bestaetigtAm)} eine Bestätigung ausgestellt. Betrag, Datum und Spender lassen sich deshalb nicht mehr ändern.`) : null,
    h('div', { class: 'form' },
      fld('Spender ist Pächter', f.paechter, 'übernimmt Name und Anschrift', true),
      fld('Tag der Zuwendung *', f.datum), fld('Betrag (€) *', f.betrag),
      fld('Name *', f.name, null, true), fld('Straße', f.strasse), fld('PLZ und Ort', f.plzOrt),
      h('div', { class: 'field wide' }, h('label', { class: 'check' }, f.verzicht, ' Verzicht auf Erstattung von Aufwendungen (Aufwandsspende)')),
      fld('Notiz', f.notiz, null, true)));
  return modal(isNew ? 'Spende erfassen' : 'Spende bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const betrag = parseNum(f.betrag.value);
      if (betrag == null || Number.isNaN(betrag) || betrag <= 0) { toast('Bitte einen Betrag größer als 0 eintragen.', 'err'); return false; }
      const daten = { ...cur, paechterId: f.paechter.value, datum: f.datum.value, betrag, name: f.name.value, strasse: f.strasse.value, plzOrt: f.plzOrt.value, verzicht: f.verzicht.checked, notiz: f.notiz.value };
      try {
        if (isNew) await api('POST', '/api/admin/spenden', daten); else await api('PUT', `/api/admin/spenden/${cur.id}`, daten);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

async function bestaetigungAusstellen(liste) {
  const schon = liste.filter((x) => x.bestaetigtAm);
  if (schon.length && !(await confirmBox(`Für ${schon.length === 1 ? 'diese Spende' : schon.length + ' dieser Spenden'} wurde am ${deDate(schon[0].bestaetigtAm)} schon eine Bestätigung ausgestellt. Nur erneut ausstellen, wenn die erste nicht verwendet wird (z. B. verloren oder fehlerhaft).`, 'Erneut ausstellen'))) return;
  const ids = liste.map((x) => x.id);
  try { await api('POST', '/api/admin/spenden/bestaetigen', { ids }); } catch (e) { handleErr(e); return; }
  const q = ids.map(encodeURIComponent).join(',');
  await pdfDrucken({ url: `/api/admin/zuwendungsbestaetigung?ids=${q}`, oeffnenUrl: `/api/admin/zuwendungsbestaetigung/oeffnen?ids=${q}`,
    titel: 'Zuwendungsbestätigung', meldung: () => 'Die Bestätigung wurde im PDF-Programm geöffnet – ausdrucken, unterschreiben und ein Doppel aufbewahren.' });
  S.spenden = null; render();
}

function gemeinnuetzigkeitKarte(g, offen) {
  const f = {
    anschrift: h('textarea', { rows: 2, style: 'width:100%', placeholder: 'Straße Hausnummer\nPLZ Ort' }, g.anschrift || ''),
    zweck: h('input', { type: 'text', value: g.zweck || 'der Kleingärtnerei', maxlength: 200 }),
    finanzamt: h('input', { type: 'text', value: g.finanzamt || '', maxlength: 80, placeholder: 'z. B. Musterstadt' }),
    steuernummer: h('input', { type: 'text', value: g.steuernummer || '', maxlength: 30 }),
    bescheid: h('select', null, [['freistellung', 'Freistellungsbescheid (bzw. Anlage zum KSt-Bescheid)'], ['feststellung', 'Feststellungsbescheid nach § 60a AO (neu gegründet)']]
      .map(([v, t]) => h('option', { value: v, selected: (g.bescheid || 'freistellung') === v }, t))),
    datum: h('input', { type: 'date', value: g.bescheidDatum || '' }),
    veranlagung: h('input', { type: 'text', value: g.veranlagung || '', maxlength: 30, placeholder: 'z. B. 2021–2023' }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const speichern = async () => {
    try {
      await api('PUT', '/api/admin/spenden/gemeinnuetzigkeit', { anschrift: f.anschrift.value, zweck: f.zweck.value, finanzamt: f.finanzamt.value, steuernummer: f.steuernummer.value,
        bescheid: f.bescheid.value, bescheidDatum: f.datum.value, veranlagung: f.veranlagung.value });
      toast('Gespeichert', 'ok'); S.spenden = null; render();
    } catch (e) { handleErr(e); }
  };
  return h('details', { class: 'card', open: offen },
    h('summary', null, h('b', null, 'Angaben zur Gemeinnützigkeit'), g.steuernummer ? h('span', { class: 'hint' }, ` · Finanzamt ${g.finanzamt}, StNr. ${g.steuernummer}`) : null),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Steht im Bescheid des Finanzamts. Ohne diese Angaben lässt sich keine Zuwendungsbestätigung ausstellen.'),
    h('div', { class: 'form' },
      fld('Anschrift des Vereins', f.anschrift, null, true),
      fld('Begünstigter Zweck', f.zweck, '»Wir sind wegen Förderung … befreit« – meist »der Kleingärtnerei«', true),
      fld('Finanzamt', f.finanzamt), fld('Steuernummer', f.steuernummer),
      fld('Art des Bescheids', f.bescheid, null, true), fld('Datum des Bescheids', f.datum, 'Freistellung höchstens 5, Feststellung höchstens 3 Jahre alt'),
      fld('Letzter Veranlagungszeitraum', f.veranlagung, 'nur beim Freistellungsbescheid')),
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: speichern }, 'Speichern')));
}

function adminSpenden() {
  if (S.spenden == null) {
    if (!S.spendenLoading) loadSpenden();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const d = S.spenden;
  const jahre = [...new Set([new Date().getFullYear(), ...d.spenden.map((x) => Number(x.datum.slice(0, 4)))])].sort((a, b) => b - a);
  const neuestes = d.spenden.length ? Math.max(...d.spenden.map((x) => Number(x.datum.slice(0, 4)))) : jahre[0];
  const jahr = S.spendenJahr && jahre.includes(S.spendenJahr) ? S.spendenJahr : neuestes;
  const l = d.spenden.filter((x) => x.datum.startsWith(jahr + '-'));
  S.spendenAuswahl = S.spendenAuswahl instanceof Set ? S.spendenAuswahl : new Set();
  for (const id of [...S.spendenAuswahl]) if (!l.some((x) => x.id === id)) S.spendenAuswahl.delete(id);
  const neu = async () => { if (await spendeDialog(null)) { S.spenden = null; render(); } };
  const loeschen = async (x) => {
    if (!(await confirmBox(`Spende von ${x.name} über ${eur(x.betrag)} löschen?`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/spenden/${x.id}`); S.spenden = null; render(); } catch (e) { handleErr(e); }
  };
  const auswahl = l.filter((x) => S.spendenAuswahl.has(x.id));
  const summe = l.reduce((a, x) => a + x.betrag, 0);
  const jahrSel = h('select', { onchange: (e) => { S.spendenJahr = Number(e.target.value); S.spendenAuswahl.clear(); render(); } },
    jahre.map((j) => h('option', { value: j, selected: j === jahr }, String(j))));
  return h('div', null,
    h('p', { class: 'hint' }, 'Geldspenden an den Verein erfassen und die Zuwendungsbestätigung nach dem amtlichen Muster ausstellen – einzeln oder als Sammelbestätigung für ein Jahr. Spenden zählen im Kassenbericht als Einnahme.'),
    d.hinweis ? h('div', { class: 'banner warn' }, d.hinweis) : null,
    gemeinnuetzigkeitKarte(d.gemeinnuetzigkeit || {}, !!d.hinweis),
    h('div', { class: 'toolbar', style: 'margin-top:14px' },
      h('button', { class: 'btn primary', onclick: neu }, '+ Spende erfassen'),
      h('label', null, 'Jahr'), jahrSel,
      h('button', { class: 'btn', disabled: !auswahl.length || !!d.hinweis, title: 'Eine Spende: Einzelbestätigung; mehrere desselben Spenders: Sammelbestätigung', onclick: () => bestaetigungAusstellen(auswahl) },
        auswahl.length > 1 ? `Sammelbestätigung (${auswahl.length})` : 'Bestätigung für Auswahl'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Spende(n) · ${eur(summe)}`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['', 'Datum', 'Spender', 'Betrag', 'Bestätigung', ''].map((x) => h('th', null, x)))),
      h('tbody', null, l.map((x) => h('tr', null,
        h('td', null, h('input', { type: 'checkbox', checked: S.spendenAuswahl.has(x.id), disabled: x.bereinigt, 'aria-label': 'auswählen',
          onchange: (e) => { if (e.target.checked) S.spendenAuswahl.add(x.id); else S.spendenAuswahl.delete(x.id); render(); } })),
        h('td', null, deDate(x.datum)),
        h('td', { class: 'wrap' }, h('b', null, x.name), h('div', { class: 'hint' }, [x.strasse, x.plzOrt].filter(Boolean).join(', ') || 'Anschrift fehlt'),
          x.verzicht ? h('span', { class: 'badge neu' }, 'Aufwandsspende') : null),
        h('td', { class: 'num' }, eur(x.betrag)),
        h('td', null, x.bestaetigtAm ? h('span', { class: 'badge ok' }, `ausgestellt ${deDate(x.bestaetigtAm)}`) : h('span', { class: 'badge warn' }, 'offen')),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', disabled: !!d.hinweis || x.bereinigt, onclick: () => bestaetigungAusstellen([x]) }, 'Bestätigung'), ' ',
          h('button', { class: 'btn small', disabled: x.bereinigt, onclick: async () => { if (await spendeDialog(x)) { S.spenden = null; render(); } } }, 'Bearbeiten'), ' ',
          mehrMenue([{ label: 'Löschen', gefahr: true, aktion: () => loeschen(x) }])))))))
      : h('div', { class: 'card' }, leer('stern', `Noch keine Spenden in ${jahr}`, 'Erfasse Spenden hier oder übernimm sie beim Einlesen des Kontoauszugs. Daraus entsteht die Zuwendungsbestätigung für das Finanzamt.', '+ Spende erfassen', neu)),
    h('p', { class: 'hint', style: 'margin-top:14px' }, 'Gut zu wissen: Mitgliedsbeiträge an Kleingartenvereine sind steuerlich nicht abziehbar, deshalb werden hier nur Spenden bestätigt. Bis 300 € je Spende genügt dem Spender meist der Kontoauszug als Nachweis. Ein Doppel jeder Bestätigung muss der Verein aufbewahren. Im Zweifel bitte mit dem Steuerberater klären.'));
}

// ------------------------------------------------------------------ Rundschreiben

const RS_VORLAGEN = [
  ['Wasser wird angestellt', 'Wasser wird angestellt – bitte Leitungen prüfen', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\nam … stellen wir das Wasser in der Anlage wieder an. Bitte schließt vorher die Entleerungshähne in euren Gärten und prüft die Leitungen auf Frostschäden.\n\nWer beim Anstellen helfen möchte, ist um … Uhr am Haupttor herzlich willkommen.'],
  ['Wasser wird abgestellt', 'Wasser wird abgestellt', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\nam … wird das Wasser in der Anlage abgestellt. Bitte entleert danach eure Leitungen und lasst die Hähne geöffnet, damit über den Winter nichts einfriert.\n\nBitte denkt auch daran, uns bis … euren Zählerstand zu melden.'],
  ['Arbeitseinsatz', 'Einladung zum Arbeitseinsatz', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\nam … ab … Uhr findet unser nächster Arbeitseinsatz statt. Treffpunkt ist das Vereinsheim. Geplant sind: …\n\nBitte bringt, wenn vorhanden, eigenes Werkzeug mit. Die geleisteten Stunden werden auf die Gemeinschaftsarbeit angerechnet.'],
  ['Allgemeine Information', '', 'Liebe Gartenfreundinnen und Gartenfreunde,\n\n'],
];

async function loadRundschreiben() {
  if (S.rsLoading) return;
  S.rsLoading = true;
  try { S.rundschreiben = await api('GET', '/api/admin/rundschreiben'); } catch (e) { handleErr(e); S.rundschreiben = []; }
  S.rsLoading = false;
  render();
}

function adminRundschreiben() {
  if (S.rundschreiben == null) { loadRundschreiben(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const l = S.rundschreiben;
  const neu = async (vorlage) => { if (await rundschreibenDialog(vorlage || null)) { S.rundschreiben = null; render(); } };
  const loeschen = async (x) => {
    if (!(await confirmBox(`Rundschreiben »${x.betreff}« löschen?`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/rundschreiben/${x.id}`); S.rundschreiben = null; render(); } catch (e) { handleErr(e); }
  };
  const briefe = (x, alle) => pdfDrucken({ url: `/api/admin/rundschreiben/${x.id}/briefe${alle ? '?alle=1' : ''}`, oeffnenUrl: `/api/admin/rundschreiben/${x.id}/briefe/oeffnen${alle ? '?alle=1' : ''}`,
    titel: 'Rundschreiben', meldung: () => 'Die Briefe wurden im PDF-Programm geöffnet – dort drucken.' }).then(() => { S.rundschreiben = null; render(); });
  const aushang = (x) => pdfDrucken({ url: `/api/admin/rundschreiben/${x.id}/aushang`, oeffnenUrl: `/api/admin/rundschreiben/${x.id}/aushang/oeffnen`,
    titel: 'Aushang', meldung: () => 'Der Aushang wurde im PDF-Programm geöffnet – dort drucken.' });
  return h('div', null,
    h('p', { class: 'hint' }, 'Ein Brief des Vorstands an alle Pächter oder an eine Auswahl. Wer »Versand per E-Mail« und eine Adresse hat, bekommt ihn über dein E-Mail-Programm (alle Adressen unsichtbar im Bcc), alle anderen als Brief zum Ausdrucken – mit Anschrift für den Fensterumschlag. Dazu gibt es den Text als Aushang für den Schaukasten.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => neu() }, '+ Neues Rundschreiben')),
    l.length ? h('div', { class: 'rs-liste' }, l.map((x) => h('div', { class: 'card rs-karte' },
      h('div', { class: 'rs-kopf' },
        h('div', null, h('b', null, x.betreff), h('div', { class: 'hint' }, `${deDate(x.datum)} · ${x.an === 'auswahl' ? 'an ausgewählte Pächter' : 'an alle Pächter'} (${x.empfaenger}): ${x.perMail} per E-Mail, ${x.perPost} per Post`)),
        x.verschickt ? h('span', { class: 'badge ok' }, `verschickt ${deDate(x.verschickt)}`) : h('span', { class: 'badge warn' }, 'Entwurf')),
      h('p', { class: 'rs-text' }, x.text.length > 260 ? x.text.slice(0, 260) + ' …' : x.text),
      h('div', { class: 'rs-knoepfe' },
        h('button', { class: 'btn small primary', disabled: !x.perMail, onclick: () => rundschreibenMail(x) }, `E-Mail an ${x.perMail}`), ' ',
        h('button', { class: 'btn small', disabled: !x.perPost, onclick: () => briefe(x, false) }, `Briefe drucken (${x.perPost})`), ' ',
        h('button', { class: 'btn small', onclick: () => aushang(x) }, 'Aushang'), ' ',
        h('button', { class: 'btn small', onclick: async () => { if (await rundschreibenDialog(x)) { S.rundschreiben = null; render(); } } }, 'Bearbeiten'), ' ',
        mehrMenue([
          { label: `Briefe an alle drucken (${x.empfaenger})`, aktion: () => briefe(x, true) },
          { label: 'Als Vorlage für ein neues', aktion: () => neu({ betreff: x.betreff, text: x.text, an: x.an, auswahl: x.auswahl }) },
          { label: 'Löschen', gefahr: true, aktion: () => loeschen(x) }])))))
      : h('div', { class: 'card' }, leer('brief', 'Noch keine Rundschreiben', 'Zum Beispiel: Wasser wird angestellt, Einladung zum Arbeitseinsatz, neue Gartenordnung. Vorlagen helfen beim Start.', '+ Neues Rundschreiben', () => neu())));
}

// Dialog zum Schreiben. x: gespeichertes Rundschreiben (mit id) oder Vorlage (ohne id) oder null.
async function rundschreibenDialog(x) {
  const st = S.state;
  const aktiv = st.paechter.filter((p) => !p.ausgeschiedenAm).sort((a, b) => (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));
  const f = {
    datum: h('input', { type: 'date', value: (x && x.id && x.datum) || todayIso() }),
    betreff: h('input', { type: 'text', maxlength: 150, value: (x && x.betreff) || '' }),
    text: h('textarea', { rows: 13, maxlength: 12000, class: 'rs-eingabe' }),
  };
  f.text.value = (x && x.text) || 'Liebe Gartenfreundinnen und Gartenfreunde,\n\n';
  let an = (x && x.an) || 'alle';
  const gewaehlt = new Set((x && x.auswahl) || []);
  const zaehler = h('span', { class: 'hint' });
  const zaehlen = () => { zaehler.textContent = `${gewaehlt.size} ausgewählt`; };
  const kaesten = aktiv.map((p) => {
    const cb = h('input', { type: 'checkbox', checked: gewaehlt.has(p.id), onchange: (e) => { if (e.target.checked) gewaehlt.add(p.id); else gewaehlt.delete(p.id); zaehlen(); } });
    const mail = p.versand === 'Emailsendung' && p.email;
    return { p, el: h('label', { class: 'rs-wahl' }, cb, ` ${p.gartennr ? 'Garten ' + p.gartennr + ' – ' : ''}${p.name} `, h('span', { class: 'hint' }, mail ? '✉ E-Mail' : 'Post')), cb };
  });
  const filter = h('input', { type: 'search', placeholder: 'Name oder Garten suchen', oninput: () => {
    const q = filter.value.trim().toLowerCase();
    for (const k of kaesten) k.el.style.display = !q || `${k.p.name} ${k.p.gartennr}`.toLowerCase().includes(q) ? '' : 'none';
  } });
  const alleSetzen = (wert, nur) => () => { for (const k of kaesten) if (k.el.style.display !== 'none' && (!nur || nur(k.p))) { k.cb.checked = wert; if (wert) gewaehlt.add(k.p.id); else gewaehlt.delete(k.p.id); } zaehlen(); };
  const auswahlBox = h('div', { class: 'rs-auswahl', style: an === 'auswahl' ? '' : 'display:none' },
    h('div', { class: 'toolbar' }, filter, h('button', { class: 'btn small', type: 'button', onclick: alleSetzen(true) }, 'Alle'), h('button', { class: 'btn small', type: 'button', onclick: alleSetzen(false) }, 'Keine'), zaehler),
    h('div', { class: 'rs-wahlliste' }, kaesten.map((k) => k.el)));
  zaehlen();
  const radio = (wert, label) => h('label', { class: 'rs-an' }, h('input', { type: 'radio', name: 'rs-an', checked: an === wert, onchange: () => { an = wert; auswahlBox.style.display = wert === 'auswahl' ? '' : 'none'; } }), ' ', label);
  const vorlagen = h('select', { onchange: () => {
    const v = RS_VORLAGEN[Number(vorlagen.value)];
    if (!v) return;
    if (v[1]) f.betreff.value = v[1];
    f.text.value = v[2];
    vorlagen.value = '';
  } }, h('option', { value: '' }, 'Vorlage einsetzen …'), RS_VORLAGEN.map((v, i) => h('option', { value: i }, v[0])));
  const fld = (label, el, hint) => h('div', { class: 'field wide' }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return breiterDialog(x && x.id ? 'Rundschreiben bearbeiten' : 'Neues Rundschreiben', h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Datum'), f.datum),
    h('div', { class: 'field' }, h('label', null, 'Vorlagen'), vorlagen),
    fld('Betreff *', f.betreff),
    fld('Text *', f.text, 'Anrede und Text schreiben; Gruß und »Der Vorstand« setzt das Programm darunter. Leerzeilen trennen Absätze.'),
    h('div', { class: 'field wide' }, h('label', null, 'Empfänger'), h('div', null, radio('alle', 'alle Pächter'), radio('auswahl', 'nur ausgewählte'))),
    h('div', { class: 'field wide' }, auswahlBox)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    if (an === 'auswahl' && !gewaehlt.size) { toast('Bitte mindestens einen Empfänger auswählen.', 'err'); return false; }
    try {
      await api('POST', '/api/admin/rundschreiben', { id: x && x.id ? x.id : '', datum: f.datum.value, betreff: f.betreff.value, text: f.text.value, an, auswahl: an === 'auswahl' ? [...gewaehlt] : [] });
      toast('Rundschreiben gespeichert', 'ok');
    } catch (e) { handleErr(e); return false; }
  } }]);
}

// E-Mail: öffnet das E-Mail-Programm, bei vielen Empfängern in mehreren Paketen. Passt der Text nicht in den
// Link, liegt er in der Zwischenablage und wird im E-Mail-Programm eingefügt.
async function rundschreibenMail(x) {
  for (let n = 1; ; n++) {
    let r;
    try { r = await api('POST', `/api/admin/rundschreiben/${x.id}/mail?paket=${n}`); } catch (e) { handleErr(e); break; }
    let kopiert = r.mitText;
    if (!r.mitText) { try { await navigator.clipboard.writeText(r.text); kopiert = true; } catch { kopiert = false; } }
    const textFeld = h('textarea', { rows: 8, readonly: true, class: 'rs-eingabe' }); textFeld.value = r.text;
    const weiter = n < r.pakete;
    const ok = await modal(r.pakete > 1 ? `E-Mail ${n} von ${r.pakete} geöffnet` : 'E-Mail geöffnet', h('div', null,
      h('p', null, `Dein E-Mail-Programm hat eine neue Nachricht an ${r.pakete > 1 ? 'einen Teil der' : 'alle'} ${r.empfaenger} Empfänger geöffnet (Adressen im Bcc). Bitte prüfen und absenden.`),
      r.mitText ? null : h('div', null,
        h('p', null, kopiert ? 'Der Text ist zu lang für den direkten Weg und liegt in der Zwischenablage: im E-Mail-Programm in die Nachricht klicken und mit Strg+V einfügen.' : 'Der Text ist zu lang für den direkten Weg. Bitte hier markieren, kopieren und im E-Mail-Programm einfügen:'),
        kopiert ? null : textFeld),
      weiter ? h('p', { class: 'hint' }, 'Viele Empfänger passen nicht in eine Nachricht. Nach dem Absenden geht es mit der nächsten weiter.') : null),
    weiter ? [{ label: 'Abbrechen', value: false }, { label: 'Nächste E-Mail öffnen', cls: 'primary', value: true }] : [{ label: 'Fertig', cls: 'primary', value: false }]);
    if (!ok || !weiter) break;
  }
  S.rundschreiben = null; render();
}

// ------------------------------------------------------------------ Schlüssel und Geräte

async function loadInventar() {
  if (S.invLoading) return;
  S.invLoading = true;
  try {
    const [k, g] = await Promise.all([api('GET', '/api/admin/schluessel'), api('GET', '/api/admin/geraete')]);
    S.inventar = { schluessel: k.schluessel, arten: k.arten, geraete: g };
  } catch (e) { handleErr(e); S.inventar = { schluessel: [], arten: [], geraete: [] }; }
  S.invLoading = false;
  render();
}

const invNeuLaden = () => { S.inventar = null; render(); };

function adminInventar() {
  if (S.inventar == null) { loadInventar(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const geraete = S.invSeite === 'geraete';
  const sub = h('div', { class: 'subtabs' },
    h('button', { class: geraete ? '' : 'active', onclick: () => { S.invSeite = 'schluessel'; render(); } }, `Schlüssel (${S.inventar.schluessel.length})`),
    h('button', { class: geraete ? 'active' : '', onclick: () => { S.invSeite = 'geraete'; render(); } }, `Geräte (${S.inventar.geraete.length})`));
  return h('div', null, sub, geraete ? inventarGeraete() : inventarSchluessel());
}

function verlaufDialog(titel, verlauf) {
  const l = [...(verlauf || [])].reverse();
  return modal(titel, l.length ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
    h('thead', null, h('tr', null, ['Datum', 'Vorgang', 'Wer', 'Notiz'].map((t) => h('th', null, t)))),
    h('tbody', null, l.map((v) => h('tr', null, h('td', null, deDate(v.datum)), h('td', null, v.aktion), h('td', null, v.name || ''), h('td', { class: 'wrap' }, v.notiz || ''))))))
    : h('p', { class: 'hint' }, 'Noch keine Vorgänge.'), [{ label: 'Schließen', value: true }]);
}

// Wer bekommt den Schlüssel / das Gerät: ein Pächter oder jemand anderes (z. B. Vorstand).
async function empfaengerDialog(titel, url, mitBis) {
  const st = S.state;
  const aktiv = st.paechter.filter((p) => !p.ausgeschiedenAm).sort((a, b) => (a.gartennr || a.mitgliedsnr).localeCompare(b.gartennr || b.mitgliedsnr, 'de', { numeric: true }));
  const wer = h('select', null, h('option', { value: '' }, 'Jemand anderes (Name eintragen) …'), aktiv.map((p) => h('option', { value: p.id }, `${p.gartennr ? 'Garten ' + p.gartennr + ' – ' : ''}${p.name}`)));
  const name = h('input', { type: 'text', maxlength: 80, placeholder: 'z. B. Vorstand (Vorsitz), Hausmeister' });
  const nameFeld = h('div', { class: 'field wide' }, h('label', null, 'Name'), name);
  wer.addEventListener('change', () => { nameFeld.style.display = wer.value ? 'none' : ''; });
  if (aktiv.length) { wer.value = aktiv[0].id; nameFeld.style.display = 'none'; }
  const datum = h('input', { type: 'date', value: todayIso() });
  const bis = h('input', { type: 'date' });
  const notiz = h('input', { type: 'text', maxlength: 200, placeholder: mitBis ? '' : 'z. B. 20 € Pfand bar erhalten' });
  return modal(titel, h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('label', null, 'An'), wer), nameFeld,
    h('div', { class: 'field' }, h('label', null, 'Datum'), datum),
    mitBis ? h('div', { class: 'field' }, h('label', null, 'Zurück bis'), bis) : null,
    h('div', { class: 'field wide' }, h('label', null, 'Notiz'), notiz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('POST', url, { paechterId: wer.value, name: wer.value ? '' : name.value, datum: datum.value, bis: mitBis ? bis.value : '', notiz: notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

async function datumDialog(titel, text, url) {
  const datum = h('input', { type: 'date', value: todayIso() });
  const notiz = h('input', { type: 'text', maxlength: 200 });
  return modal(titel, h('div', { class: 'form' }, text ? h('p', { class: 'field wide' }, text) : null,
    h('div', { class: 'field' }, h('label', null, 'Datum'), datum), h('div', { class: 'field wide' }, h('label', null, 'Notiz'), notiz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('POST', url, { datum: datum.value, notiz: notiz.value }); } catch (e) { handleErr(e); return false; }
  } }]);
}

async function schluesselDialog(k) {
  const arten = S.inventar.arten;
  const f = {
    art: h('select', null, arten.map((a) => h('option', { value: a, selected: k ? k.art === a : a === 'Tor' }, a))),
    nummer: h('input', { type: 'text', maxlength: 30, value: k ? k.nummer : '' }),
    bez: h('input', { type: 'text', maxlength: 100, value: k ? k.bezeichnung || '' : '', placeholder: 'z. B. Nebentor Parkplatz' }),
    pfand: h('input', { type: 'text', inputmode: 'decimal', value: k && k.pfand ? numIn(k.pfand) : '' }),
    notiz: h('input', { type: 'text', maxlength: 500, value: k ? k.notiz || '' : '' }),
  };
  const fld = (label, el, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el);
  return modal(k ? `Schlüssel ${k.art} ${k.nummer}` : 'Neuer Schlüssel', h('div', { class: 'form' },
    fld('Art', f.art), fld('Nummer *', f.nummer), fld('Bezeichnung', f.bez, true), fld('Pfand in €', f.pfand), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const pfand = f.pfand.value.trim() ? parseNum(f.pfand.value) : 0;
    if (Number.isNaN(pfand)) { toast('Bitte das Pfand als Zahl eintragen.', 'err'); return false; }
    try { await api('POST', '/api/admin/schluessel', { id: k ? k.id : '', art: f.art.value, nummer: f.nummer.value, bezeichnung: f.bez.value, pfand, notiz: f.notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function inventarSchluessel() {
  const l = S.inventar.schluessel;
  const aus = l.filter((k) => k.inhaberName);
  const pfand = aus.reduce((a, k) => a + (k.pfand || 0), 0);
  const nachher = async (p) => { if (await p) invNeuLaden(); };
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer hat welchen Schlüssel? Bei Ausgabe und Rückgabe wird der Verlauf festgehalten. Hat ein ausgeschiedener Pächter noch einen Schlüssel, meldet sich das Cockpit auf der Übersicht.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(schluesselDialog(null)) }, '+ Schlüssel'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Schlüssel · ${aus.length} ausgegeben${pfand ? ' · Pfand ' + eur(pfand) : ''}`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Art', 'Nr.', 'Bezeichnung', 'Bei', 'Seit', 'Pfand', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((k) => h('tr', null,
        h('td', null, k.art), h('td', null, h('b', null, k.nummer)), h('td', { class: 'wrap' }, k.bezeichnung || ''),
        h('td', { class: 'wrap' }, k.inhaberName ? [k.inhaberName, k.gartennr ? h('span', { class: 'hint' }, ` (Garten ${k.gartennr})`) : null,
          k.ausgeschieden || k.inhaber === '(gelöschte Person)' ? h('span', { class: 'badge err', style: 'margin-left:6px' }, 'zurückfordern') : null] : h('span', { class: 'hint' }, 'im Schrank')),
        h('td', null, deDate(k.seit)), h('td', { class: 'num' }, k.pfand ? eur(k.pfand) : ''),
        h('td', { class: 'aktionen' },
          k.inhaberName
            ? h('button', { class: 'btn small', onclick: () => nachher(datumDialog(`Rückgabe ${k.art} ${k.nummer}`, `Zurück von ${k.inhaberName}.${k.pfand ? ' Pfand ' + eur(k.pfand) + ' zurückzahlen.' : ''}`, `/api/admin/schluessel/${k.id}/rueckgabe`)) }, 'Rückgabe')
            : h('button', { class: 'btn small', onclick: () => nachher(empfaengerDialog(`${k.art}schlüssel ${k.nummer} ausgeben`, `/api/admin/schluessel/${k.id}/ausgabe`, false)) }, 'Ausgeben'), ' ',
          mehrMenue([{ label: 'Bearbeiten', aktion: () => nachher(schluesselDialog(k)) }, { label: 'Verlauf', aktion: () => verlaufDialog(`Verlauf ${k.art} ${k.nummer}`, k.verlauf) },
            { label: 'Löschen', gefahr: true, aktion: async () => {
              if (!(await confirmBox(`Schlüssel ${k.art} ${k.nummer} löschen (z. B. verloren oder Schloss getauscht)?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/schluessel/${k.id}`); invNeuLaden(); } catch (e) { handleErr(e); }
            } }])))))))
      : h('div', { class: 'card' }, leer('schluessel', 'Noch keine Schlüssel', 'Trage Tor-, Vereinsheim- und Schachtschlüssel ein – dann weißt du jederzeit, wer welchen hat.', '+ Schlüssel', () => nachher(schluesselDialog(null)))));
}

async function geraetDialog(g) {
  const f = {
    name: h('input', { type: 'text', maxlength: 80, value: g ? g.name : '', placeholder: 'z. B. Gartenhäcksler' }),
    inv: h('input', { type: 'text', maxlength: 30, value: g ? g.inventarnr || '' : '' }),
    ort: h('input', { type: 'text', maxlength: 80, value: g ? g.standort || '' : '', placeholder: 'z. B. Geräteschuppen' }),
    seit: h('input', { type: 'date', value: g ? g.angeschafft || '' : '' }),
    wert: h('input', { type: 'text', inputmode: 'decimal', value: g && g.wert ? numIn(g.wert) : '' }),
    monate: h('input', { type: 'text', inputmode: 'numeric', value: g && g.wartungMonate ? String(g.wartungMonate) : '', placeholder: 'z. B. 12' }),
    letzte: h('input', { type: 'date', value: g ? g.letzteWartung || '' : '' }),
    notiz: h('input', { type: 'text', maxlength: 500, value: g ? g.notiz || '' : '' }),
  };
  const fld = (label, el, wide, hint) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(g ? g.name : 'Neues Gerät', h('div', { class: 'form' },
    fld('Name *', f.name, true), fld('Inventar-Nr.', f.inv), fld('Standort', f.ort), fld('Angeschafft am', f.seit), fld('Wert in €', f.wert),
    fld('Wartung alle … Monate', f.monate, false, 'leer = keine regelmäßige Wartung'), fld('Letzte Wartung', f.letzte), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const wert = f.wert.value.trim() ? parseNum(f.wert.value) : 0, monate = f.monate.value.trim() ? Number(f.monate.value) : 0;
    if (Number.isNaN(wert) || !Number.isInteger(monate)) { toast('Bitte Wert und Wartungsabstand als Zahl eintragen.', 'err'); return false; }
    try { await api('POST', '/api/admin/geraete', { id: g ? g.id : '', name: f.name.value, inventarnr: f.inv.value, standort: f.ort.value, angeschafft: f.seit.value, wert, wartungMonate: monate, letzteWartung: f.letzte.value, notiz: f.notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function inventarGeraete() {
  const l = S.inventar.geraete;
  const heute = todayIso();
  const in30 = isoInTagen(30);
  const nachher = async (p) => { if (await p) invNeuLaden(); };
  const wert = l.reduce((a, g) => a + (g.wert || 0), 0);
  return h('div', null,
    h('p', { class: 'hint' }, 'Vereinsgeräte mit Standort, Wert und Wartung. Verleihst du ein Gerät, steht es mit Rückgabedatum hier; ist es überfällig oder steht eine Wartung an, erinnert das Cockpit daran.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(geraetDialog(null)) }, '+ Gerät'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Geräte${wert ? ' · Wert ' + eur(wert) : ''}`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Gerät', 'Standort', 'Status', 'Nächste Wartung', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((g) => {
        const verliehen = !!g.entleiherName;
        const ueberfaellig = verliehen && g.bis && g.bis < heute;
        return h('tr', null,
          h('td', { class: 'wrap' }, h('b', null, g.name), g.inventarnr ? h('div', { class: 'hint' }, g.inventarnr) : null),
          h('td', null, g.standort || ''),
          h('td', { class: 'wrap' }, verliehen ? [`verliehen an ${g.entleiherName}`, h('div', { class: 'hint' }, `seit ${deDate(g.seit)}${g.bis ? ', zurück bis ' + deDate(g.bis) : ''}`),
            ueberfaellig ? h('span', { class: 'badge err' }, 'überfällig') : null] : h('span', { class: 'badge ok' }, 'da')),
          h('td', null, g.wartungFaellig ? [deDate(g.wartungFaellig), g.wartungFaellig <= in30 ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, 'fällig') : null] : h('span', { class: 'hint' }, '–')),
          h('td', { class: 'aktionen' },
            verliehen
              ? h('button', { class: 'btn small', onclick: () => nachher(datumDialog(`${g.name} zurück`, `Zurück von ${g.entleiherName}.`, `/api/admin/geraete/${g.id}/zurueck`)) }, 'Zurück')
              : h('button', { class: 'btn small', onclick: () => nachher(empfaengerDialog(`${g.name} verleihen`, `/api/admin/geraete/${g.id}/verleihen`, true)) }, 'Verleihen'), ' ',
            mehrMenue([
              { label: 'Wartung erledigt', aktion: () => nachher(datumDialog(`Wartung ${g.name}`, '', `/api/admin/geraete/${g.id}/wartung`)) },
              { label: 'Bearbeiten', aktion: () => nachher(geraetDialog(g)) },
              { label: 'Verlauf', aktion: () => verlaufDialog(`Verlauf ${g.name}`, g.verlauf) },
              { label: 'Löschen', gefahr: true, aktion: async () => {
                if (!(await confirmBox(`Gerät »${g.name}« löschen?`, 'Löschen', true))) return;
                try { await api('DELETE', `/api/admin/geraete/${g.id}`); invNeuLaden(); } catch (e) { handleErr(e); }
              } }])));
      }))))
      : h('div', { class: 'card' }, leer('werkzeug', 'Noch keine Geräte', 'Häcksler, Rasenmäher, Leitern, Schubkarren … – mit Standort, Wartung und wer gerade was ausgeliehen hat.', '+ Gerät', () => nachher(geraetDialog(null)))));
}

// ------------------------------------------------------------------ Kündigungen

async function loadKuendigungen() {
  if (S.kuendLoading) return;
  S.kuendLoading = true;
  try { S.kuend = await api('GET', '/api/admin/kuendigungen'); } catch (e) { handleErr(e); S.kuend = { regel: { zum: '11-30', bis: '08-31' }, kuendigungen: [] }; }
  S.kuendLoading = false;
  render();
}

const mtText = (mt) => { const [m, d] = (mt || '').split('-').map(Number); return m && d ? `${d}.${m}.` : mt; };

// frühestes Pachtende nach der Regel (wie KuendigungsRegel.fristgerecht in kuendigung.go)
function fristgerecht(eingang, regel) {
  if (!eingang) return '';
  let y = Number(eingang.slice(0, 4));
  if (eingang > `${y}-${regel.bis}`) y++;
  let ende = `${y}-${regel.zum}`;
  if (ende < eingang) ende = `${y + 1}-${regel.zum}`;
  return ende;
}

async function kuendigungDialog(p, k) {
  let regel = (S.kuend && S.kuend.regel) || null;
  if (!regel) { try { regel = (await api('GET', '/api/admin/kuendigungen')).regel; } catch (e) { handleErr(e); return false; } }
  const f = {
    eingang: h('input', { type: 'date', value: k ? k.eingang : todayIso() }),
    zum: h('input', { type: 'date', value: k ? k.zum : fristgerecht(todayIso(), regel) }),
    von: h('select', null, [['paechter', 'vom Pächter'], ['verein', 'vom Verein'], ['einvernehmlich', 'im Einvernehmen']].map(([v, l]) => h('option', { value: v, selected: k ? k.von === v : v === 'paechter' }, l))),
    grund: h('input', { type: 'text', maxlength: 200, value: k ? k.grund || '' : '', placeholder: 'z. B. Umzug, Alter' }),
    notiz: h('input', { type: 'text', maxlength: 1000, value: k ? k.notiz || '' : '' }),
  };
  const frist = h('span', { class: 'fhint' });
  const pruefe = () => {
    const fg = fristgerecht(f.eingang.value, regel);
    const zuFrueh = f.von.value === 'paechter' && f.zum.value && f.zum.value < fg;
    frist.textContent = fg ? `Fristgerecht frühestens zum ${deDate(fg)} (Regel: Eingang bis ${mtText(regel.bis)}, Ende zum ${mtText(regel.zum)}).${zuFrueh ? ' Das gewählte Pachtende ist früher – geht nur im Einvernehmen.' : ''}` : '';
    frist.classList.toggle('warn-text', !!zuFrueh);
  };
  f.eingang.addEventListener('change', () => { if (!k) f.zum.value = fristgerecht(f.eingang.value, regel); pruefe(); });
  f.zum.addEventListener('change', pruefe); f.von.addEventListener('change', pruefe);
  pruefe();
  const fld = (label, el, wide, hint) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el, hint || null);
  return modal(k ? `Kündigung Garten ${k.gartennr}` : `Kündigung: ${p.name} (Garten ${p.gartennr})`, h('div', { class: 'form' },
    fld('Eingang der Kündigung', f.eingang), fld('Kündigung', f.von), fld('Pachtende', f.zum, true, frist), fld('Grund', f.grund, true), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try {
      await api('POST', '/api/admin/kuendigungen', { id: k ? k.id : '', paechterId: k ? k.paechterId : p.id, eingang: f.eingang.value, zum: f.zum.value, von: f.von.value, grund: f.grund.value, notiz: f.notiz.value });
      toast('Kündigung gespeichert – den Fahrplan findest du unter »Kündigungen«', 'ok'); S.kuend = null;
    } catch (e) { handleErr(e); return false; }
  } }]);
}

function adminKuendigungen() {
  if (S.kuend == null) { loadKuendigungen(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const d = S.kuend;
  const laufend = d.kuendigungen.filter((k) => !k.abgeschlossen), fertig = d.kuendigungen.filter((k) => k.abgeschlossen);
  const neu = async () => {
    const aktiv = S.state.paechter.filter((p) => !p.ausgeschiedenAm && p.gartennr && !laufend.some((k) => k.paechterId === p.id)).sort(cmpNr);
    if (!aktiv.length) { toast('Es gibt keinen Pächter ohne laufende Kündigung.', 'err'); return; }
    const wer = h('select', null, aktiv.map((p) => h('option', { value: p.id }, `Garten ${p.gartennr} – ${p.name}`)));
    if (!(await modal('Wer kündigt?', h('div', { class: 'form' }, h('div', { class: 'field wide' }, h('label', null, 'Pächter'), wer)),
      [{ label: 'Abbrechen', value: false }, { label: 'Weiter', cls: 'primary', value: true }]))) return;
    if (await kuendigungDialog(aktiv.find((p) => p.id === wer.value))) { S.kuend = null; render(); }
  };
  const schritt = async (k, key, an) => {
    try { await api('POST', `/api/admin/kuendigungen/${k.id}/schritt`, { schritt: key, erledigt: an }); S.kuend = null; render(); } catch (e) { handleErr(e); }
  };
  const bestaetigung = (k) => pdfDrucken({ url: `/api/admin/kuendigungen/${k.id}/bestaetigung`, oeffnenUrl: `/api/admin/kuendigungen/${k.id}/bestaetigung/oeffnen`,
    titel: 'Kündigungsbestätigung', meldung: () => 'Die Bestätigung wurde im PDF-Programm geöffnet – dort drucken.' }).then(() => { S.kuend = null; render(); });
  const karte = (k) => h('div', { class: 'card kd-karte' + (k.abgeschlossen ? ' kd-fertig' : '') },
    h('div', { class: 'rs-kopf' },
      h('div', null, h('b', null, `Garten ${k.gartennr} – ${k.name}`),
        h('div', { class: 'hint' }, `Kündigung ${k.von === 'verein' ? 'durch den Verein' : k.von === 'einvernehmlich' ? 'im Einvernehmen' : 'durch den Pächter'} vom ${deDate(k.eingang)} zum ${deDate(k.zum)}${k.grund ? ' · ' + k.grund : ''}`),
        k.zuFrueh ? h('div', { class: 'warn-text' }, `Fristgerecht wäre erst der ${deDate(k.fristgemaess)}.`) : null),
      k.abgeschlossen ? h('span', { class: 'badge ok' }, `abgeschlossen ${deDate(k.abgeschlossen)}`) : h('span', { class: 'badge ' + (k.offen ? 'warn' : 'ok') }, k.offen ? `${k.offen} offen` : 'alles erledigt')),
    k.abgeschlossen ? null : h('ul', { class: 'kd-schritte' }, k.schritte.map((x) => h('li', { class: x.erledigt ? 'erledigt' : '' },
      h('label', { class: 'check' }, h('input', { type: 'checkbox', checked: !!x.erledigt, disabled: x.auto, onchange: (e) => schritt(k, x.key, e.target.checked) }),
        h('span', null, x.titel, x.erledigt ? h('span', { class: 'hint' }, x.auto ? ' – vom Programm erkannt' : ` – ${deDate(x.erledigt)}`) : null)),
      x.erledigt ? null : h('div', { class: 'hint kd-hilfe' }, x.hilfe)))),
    h('div', { class: 'rs-knoepfe' },
      k.abgeschlossen ? null : h('button', { class: 'btn small primary', onclick: () => bestaetigung(k) }, 'Bestätigung drucken'), ' ',
      k.abgeschlossen ? null : h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('warteliste'); render(); } }, 'Warteliste'), ' ',
      k.abgeschlossen || !k.gartennr ? null : h('button', { class: 'btn small', onclick: () => gartenakteDialog(k.gartennr) }, 'Gartenakte / Wertermittlung'), ' ',
      mehrMenue([
        k.abgeschlossen ? null : { label: 'Bearbeiten', aktion: async () => { if (await kuendigungDialog(null, k)) { S.kuend = null; render(); } } },
        { label: k.abgeschlossen ? 'Wieder öffnen' : 'Abschließen', aktion: () => schritt(k, 'abgeschlossen', !k.abgeschlossen) },
        { label: 'Löschen (zurückgezogen)', gefahr: true, aktion: async () => {
          if (!(await confirmBox(`Kündigung für Garten ${k.gartennr} löschen, z. B. weil sie zurückgezogen wurde?`, 'Löschen', true))) return;
          try { await api('DELETE', `/api/admin/kuendigungen/${k.id}`); S.kuend = null; render(); } catch (e) { handleErr(e); }
        } }])));
  const zum = h('input', { type: 'text', value: d.regel.zum, maxlength: 5, style: 'width:5em', placeholder: 'MM-TT' });
  const bis = h('input', { type: 'text', value: d.regel.bis, maxlength: 5, style: 'width:5em', placeholder: 'MM-TT' });
  return h('div', null,
    h('p', { class: 'hint' }, 'Eine Kündigung erfassen, die Frist prüfen und Schritt für Schritt bis zur Übergabe abhaken. Wertermittlung, Schlüsselrückgabe und Pächterwechsel erkennt das Programm selbst; das Cockpit erinnert an alles, was noch offen ist.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: neu }, '+ Kündigung erfassen')),
    laufend.length ? h('div', { class: 'rs-liste' }, laufend.map(karte))
      : h('div', { class: 'card' }, leer('kuendigung', 'Keine laufenden Kündigungen', 'Kündigt jemand, erfasse es hier oder beim Pächter unter »Mehr«. Dann gibt es Bestätigungsbrief und Fahrplan bis zur Übergabe.', '+ Kündigung erfassen', neu)),
    fertig.length ? [h('h3', null, 'Abgeschlossen'), h('div', { class: 'rs-liste' }, fertig.map(karte))] : null,
    h('h3', null, 'Kündigungsregel des Vereins'),
    h('div', { class: 'card' }, h('div', { class: 'toolbar' },
      h('span', null, 'Kündigung muss eingehen bis'), bis, h('span', null, 'zum Pachtende am'), zum, h('span', { class: 'hint' }, '(Monat-Tag, z. B. 08-31 und 11-30)'),
      h('button', { class: 'btn small', onclick: async () => {
        try { await api('PUT', '/api/admin/kuendigungen/regel', { zum: zum.value.trim(), bis: bis.value.trim() }); toast('Regel gespeichert', 'ok'); S.kuend = null; render(); } catch (e) { handleErr(e); }
      } }, 'Speichern')),
    h('p', { class: 'hint', style: 'margin:6px 0 0' }, 'Steht in eurem Pachtvertrag. Das Bundeskleingartengesetz sieht für Kündigungen durch den Verein den 30. November vor.')));
}

// ------------------------------------------------------------------ Pachtvertrag

async function pachtvertragDialog(p) {
  const beginn = h('input', { type: 'date' });
  const text = h('textarea', { rows: 14, class: 'rs-eingabe' });
  try { text.value = (await api('GET', '/api/admin/vertragsbestimmungen')).text; } catch (e) { handleErr(e); return; }
  const q = () => `?paechter=${encodeURIComponent(p.id)}${beginn.value ? '&beginn=' + beginn.value : ''}`;
  const speichereText = async () => { await api('PUT', '/api/admin/vertragsbestimmungen', { text: text.value }); };
  const w = await breiterDialog(`Pachtvertrag Garten ${p.gartennr} – ${p.name}`, h('div', { class: 'form' },
    h('div', { class: 'field' }, h('label', null, 'Pachtbeginn'), beginn, h('span', { class: 'fhint' }, 'leer = aus der Gartenhistorie')),
    h('div', { class: 'field wide' }, h('label', null, 'Bestimmungen des Vereins'), text,
      h('span', { class: 'fhint' }, 'Gilt für alle Verträge. Zeilen mit »§« werden Überschriften. Pächter, Garten, Pacht, Beiträge, Preise und Kündigungsfrist setzt das Programm selbst ein; der Lageplan kommt als Anlage dazu.'))),
  [{ label: 'Abbrechen', value: '' }, { label: 'In die Gartenakte legen', value: 'akte' }, { label: 'Drucken', cls: 'primary', value: 'druck' }]);
  if (!w) return;
  try { await speichereText(); } catch (e) { handleErr(e); return; }
  if (w === 'druck') await pdfDrucken({ url: `/api/admin/pachtvertrag${q()}`, oeffnenUrl: `/api/admin/pachtvertrag/oeffnen${q()}`, titel: 'Pachtvertrag', meldung: () => 'Der Pachtvertrag wurde im PDF-Programm geöffnet – dort zweimal drucken.' });
  if (w === 'akte') { try { await api('POST', `/api/admin/pachtvertrag/ablegen${q()}`); toast(`Pachtvertrag in der Gartenakte von Garten ${p.gartennr} abgelegt`, 'ok'); } catch (e) { handleErr(e); } }
}

// ------------------------------------------------------------------ Beschlüsse und Verträge

async function loadVerwaltung() {
  if (S.verwLoading) return;
  S.verwLoading = true;
  try {
    const [b, v, am] = await Promise.all([api('GET', '/api/admin/beschluesse'), api('GET', '/api/admin/vertraege'), api('GET', '/api/admin/aemter')]);
    S.verw = { beschluesse: b, vertraege: v.vertraege, arten: v.arten, jaehrlich: v.jaehrlich, aemter: am };
  } catch (e) { handleErr(e); S.verw = { beschluesse: [], vertraege: [], arten: [], jaehrlich: 0, aemter: { aemter: [], punkte: [], vorschlaege: [] } }; }
  S.verwLoading = false;
  render();
}

function adminVerwaltung() {
  if (S.verw == null) { loadVerwaltung(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const seite = ['vertraege', 'aemter'].includes(S.verwSeite) ? S.verwSeite : 'beschluesse';
  const knopf = (id, text) => h('button', { class: seite === id ? 'active' : '', onclick: () => { S.verwSeite = id; render(); } }, text);
  return h('div', null, h('div', { class: 'subtabs' },
    knopf('beschluesse', `Beschlussbuch (${S.verw.beschluesse.length})`),
    knopf('vertraege', `Verträge & Versicherungen (${S.verw.vertraege.length})`),
    knopf('aemter', `Ämter (${S.verw.aemter.aemter.filter((x) => !x.ehemalig).length})`)),
  seite === 'vertraege' ? verwaltungVertraege() : seite === 'aemter' ? verwaltungAemter() : verwaltungBeschluesse());
}

// ---- Ämter und Amtswechsel

async function amtDialog(x, wechsel) {
  const d = S.verw.aemter;
  const liste = h('datalist', { id: 'amt-vorschlaege' }, d.vorschlaege.map((v) => h('option', { value: v })));
  const f = {
    amt: h('input', { type: 'text', maxlength: 60, list: 'amt-vorschlaege', value: x && !wechsel ? x.amt : (x ? x.amt : ''), disabled: !!wechsel, placeholder: 'z. B. Kassenwart/in' }),
    name: h('input', { type: 'text', maxlength: 100, value: x && !wechsel ? x.name : '' }),
    seit: h('input', { type: 'date', value: x && !wechsel ? x.seit || '' : wechsel ? todayIso() : '' }),
    bis: h('input', { type: 'date', value: x && !wechsel ? x.bis || '' : '' }),
    notiz: h('input', { type: 'text', maxlength: 500, value: x && !wechsel ? x.notiz || '' : '' }),
  };
  const fld = (label, el, hint) => h('div', { class: 'field' }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  const titel = wechsel ? `Amtswechsel: ${x.amt}` : x ? `${x.amt} bearbeiten` : 'Amt eintragen';
  return modal(titel, h('div', null,
    wechsel ? h('p', { class: 'hint' }, `${x.name} gibt das Amt ab. Die bisherige Amtszeit endet am Tag der Wahl, danach führt eine Checkliste durch die Übergabe.`) : null,
    h('div', { class: 'form' }, liste, fld('Amt *', f.amt), fld(wechsel ? 'Neu gewählt *' : 'Name *', f.name),
      fld(wechsel ? 'Tag der Wahl *' : 'Im Amt seit', f.seit), fld('Amtszeit bis', f.bis, 'leer = unbefristet'), fld('Notiz', f.notiz))),
  [{ label: 'Abbrechen', value: false }, { label: wechsel ? 'Wechsel eintragen' : 'Speichern', cls: 'primary', value: true, action: async () => {
    const body = { id: x && !wechsel ? x.id : '', amt: f.amt.value, name: f.name.value, seit: f.seit.value, bis: f.bis.value, notiz: f.notiz.value };
    try { S.verw.aemter = await api('POST', wechsel ? `/api/admin/aemter/${x.id}/wechsel` : '/api/admin/aemter', body); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function verwaltungAemter() {
  const d = S.verw.aemter;
  const heute = todayIso();
  const aktiv = d.aemter.filter((x) => !x.ehemalig);
  const frueher = d.aemter.filter((x) => x.ehemalig);
  const nachher = async (p) => { if (await p) render(); };
  const bald = (bis) => { if (!bis) return null; const t = Math.round((new Date(bis) - new Date(heute)) / 86400000);
    return t < 0 ? h('span', { class: 'badge err' }, 'abgelaufen') : t <= 120 ? h('span', { class: 'badge warn' }, 'Wahl vorbereiten') : null; };
  const checkliste = (x) => {
    if (!x.checkliste) return null;
    const offen = d.punkte.filter((p) => !x.checkliste[p.key]).length;
    return h('details', { class: 'amt-check', open: offen > 0 }, h('summary', null, offen ? `Übergabe: ${offen} von ${d.punkte.length} Punkten offen` : 'Übergabe abgeschlossen ✓'),
      h('ul', { class: 'haken-liste' }, d.punkte.map((p) => h('li', null, h('label', null,
        h('input', { type: 'checkbox', checked: !!x.checkliste[p.key], onchange: async (e) => {
          try { S.verw.aemter = await api('PUT', `/api/admin/aemter/${x.id}/checkliste`, { key: p.key, erledigt: e.target.checked }); render(); }
          catch (err) { handleErr(err); e.target.checked = !e.target.checked; }
        } }), ' ', p.text, x.checkliste[p.key] ? h('span', { class: 'hint' }, ` · ${deDate(x.checkliste[p.key])}`) : null)))));
  };
  const loeschen = (x) => ({ label: 'Löschen', gefahr: true, aktion: async () => {
    if (!(await confirmBox(`Eintrag »${x.amt}: ${x.name}« löschen?`, 'Löschen', true))) return;
    try { S.verw.aemter = await api('DELETE', `/api/admin/aemter/${x.id}`); render(); } catch (e) { handleErr(e); }
  } });
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer hat welches Amt bis wann? Rund vier Monate vor Ende einer Amtszeit erinnert das Cockpit an die Wahl. Beim Amtswechsel hilft eine Checkliste, nichts zu vergessen – von der Bankvollmacht bis zum Vereinsregister.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(amtDialog(null)) }, '+ Amt')),
    aktiv.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Amt', 'Name', 'Seit', 'Amtszeit bis', ''].map((t) => h('th', null, t)))),
      h('tbody', null, aktiv.map((x) => h('tr', null,
        h('td', { class: 'wrap' }, h('b', null, x.amt), x.notiz ? h('div', { class: 'hint' }, x.notiz) : null),
        h('td', { class: 'wrap' }, x.name, checkliste(x)),
        h('td', null, x.seit ? deDate(x.seit) : ''),
        h('td', null, x.bis ? deDate(x.bis) : h('span', { class: 'hint' }, 'unbefristet'), ' ', bald(x.bis)),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(amtDialog(x, true)) }, 'Amtswechsel'), ' ',
          mehrMenue([{ label: 'Bearbeiten', aktion: () => nachher(amtDialog(x)) }, loeschen(x)])))))))
      : h('div', { class: 'card' }, leer('personen', 'Noch keine Ämter eingetragen', 'Vorsitz, Kasse, Schriftführung, Kassenprüfung … mit Amtszeit – dann denkt das Programm an die nächste Wahl.', '+ Amt', () => nachher(amtDialog(null)))),
    frueher.length ? h('details', { class: 'card', style: 'margin-top:12px' }, h('summary', null, `Frühere Amtsinhaber (${frueher.length})`),
      h('table', { class: 'data' }, h('tbody', null, frueher.map((x) => h('tr', null, h('td', null, x.amt), h('td', null, x.name),
        h('td', null, `${x.seit ? deDate(x.seit) : '?'} – ${x.bis ? deDate(x.bis) : '?'}`), h('td', { class: 'aktionen' }, mehrMenue([loeschen(x)]))))))) : null);
}

async function beschlussDialog(b) {
  const f = {
    datum: h('input', { type: 'date', value: b ? b.datum : todayIso() }),
    gremium: h('select', null, ['Mitgliederversammlung', 'Vorstand'].map((g) => h('option', { value: g, selected: b ? b.gremium === g : false }, g))),
    nummer: h('input', { type: 'text', maxlength: 30, value: b ? b.nummer : '', placeholder: 'leer = automatisch' }),
    titel: h('input', { type: 'text', maxlength: 200, value: b ? b.titel : '', placeholder: 'z. B. Wasserpreis ab 2027: 2,10 € je m³' }),
    text: h('textarea', { rows: 4, maxlength: 5000, class: 'rs-eingabe' }),
    ja: h('input', { type: 'number', min: 0, value: b && b.ja ? b.ja : '' }), nein: h('input', { type: 'number', min: 0, value: b && b.nein ? b.nein : '' }),
    enth: h('input', { type: 'number', min: 0, value: b && b.enthaltung ? b.enthaltung : '' }),
    ergebnis: h('select', null, ['angenommen', 'abgelehnt', 'zur Kenntnis'].map((g) => h('option', { value: g, selected: b ? b.ergebnis === g : false }, g))),
    stich: h('input', { type: 'text', maxlength: 200, value: b ? b.stichworte || '' : '', placeholder: 'z. B. Wasser, Preise' }),
  };
  f.text.value = b ? b.text || '' : '';
  const fld = (label, el, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el);
  return modal(b ? `Beschluss ${b.nummer}` : 'Neuer Beschluss', h('div', { class: 'form' },
    fld('Datum', f.datum), fld('Gremium', f.gremium), fld('Beschluss *', f.titel, true), fld('Wortlaut / Erläuterung', f.text, true),
    fld('Ja', f.ja), fld('Nein', f.nein), fld('Enthaltung', f.enth), fld('Ergebnis', f.ergebnis), fld('Nummer', f.nummer), fld('Stichworte', f.stich)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const n = (el) => (el.value === '' ? 0 : Number(el.value));
    try { await api('POST', '/api/admin/beschluesse', { id: b ? b.id : '', datum: f.datum.value, gremium: f.gremium.value, nummer: f.nummer.value, titel: f.titel.value, text: f.text.value,
      ja: n(f.ja), nein: n(f.nein), enthaltung: n(f.enth), ergebnis: f.ergebnis.value, stichworte: f.stich.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function verwaltungBeschluesse() {
  const alle = S.verw.beschluesse;
  const q = (S.beschlussSuche || '').trim().toLowerCase();
  const l = alle.filter((b) => !q || [b.titel, b.text, b.stichworte, b.nummer, b.datum.slice(0, 4)].some((x) => (x || '').toLowerCase().includes(q)));
  const nachher = async (p) => { if (await p) { S.verw = null; render(); } };
  const suche = h('input', { type: 'search', placeholder: 'Suchen, z. B. Wasser oder 2024', value: S.beschlussSuche || '', style: 'width:260px',
    oninput: (e) => { S.beschlussSuche = e.target.value; clearTimeout(S.beschlussTimer); S.beschlussTimer = setTimeout(() => { render(); const el = document.querySelector('.beschluss-suche'); if (el) { el.focus(); el.setSelectionRange(el.value.length, el.value.length); } }, 250); } });
  suche.classList.add('beschluss-suche');
  return h('div', null,
    h('p', { class: 'hint' }, 'Was Vorstand und Mitgliederversammlung wann beschlossen haben – mit Abstimmungsergebnis und Stichworten zum Wiederfinden. Die Nummer vergibt das Programm fortlaufend je Jahr (z. B. MV 2025/3).'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(beschlussDialog(null)) }, '+ Beschluss'), suche,
      h('div', { class: 'spacer' }),
      alle.length ? h('button', { class: 'btn', onclick: () => pdfDrucken({ url: '/api/admin/beschlussbuch', oeffnenUrl: '/api/admin/beschlussbuch/oeffnen', titel: 'Beschlussbuch', meldung: () => 'Das Beschlussbuch wurde im PDF-Programm geöffnet.' }) }, 'Beschlussbuch drucken') : null),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Nr.', 'Datum', 'Beschluss', 'Ergebnis', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((b) => h('tr', null, h('td', null, b.nummer), h('td', null, deDate(b.datum), h('div', { class: 'hint' }, b.gremium)),
        h('td', { class: 'wrap' }, h('b', null, b.titel), b.text ? h('div', { class: 'hint' }, b.text) : null, b.stichworte ? h('div', { class: 'hint' }, '🏷 ' + b.stichworte) : null),
        h('td', null, h('span', { class: 'badge ' + (b.ergebnis === 'angenommen' ? 'ok' : b.ergebnis === 'abgelehnt' ? 'err' : 'neu') }, b.ergebnis),
          b.ja + b.nein + b.enthaltung ? h('div', { class: 'hint' }, `${b.ja} : ${b.nein} : ${b.enthaltung}`) : null),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(beschlussDialog(b)) }, 'Bearbeiten'), ' ',
          mehrMenue([{ label: 'Löschen', gefahr: true, aktion: async () => {
            if (!(await confirmBox(`Beschluss ${b.nummer} löschen?`, 'Löschen', true))) return;
            try { await api('DELETE', `/api/admin/beschluesse/${b.id}`); S.verw = null; render(); } catch (e) { handleErr(e); }
          } }])))))))
      : h('div', { class: 'card' }, alle.length ? h('p', { class: 'hint' }, 'Keine Treffer.') : leer('akte', 'Noch keine Beschlüsse', 'Trage Beschlüsse nach jeder Versammlung ein – dann ist die Frage »Wann haben wir das beschlossen?« schnell beantwortet.', '+ Beschluss', () => nachher(beschlussDialog(null)))));
}

async function vertragDialog(v) {
  const f = {
    art: h('select', null, S.verw.arten.map((a) => h('option', { value: a, selected: v ? v.art === a : false }, a))),
    bez: h('input', { type: 'text', maxlength: 120, value: v ? v.bezeichnung : '', placeholder: 'z. B. Vereinshaftpflicht' }),
    partner: h('input', { type: 'text', maxlength: 120, value: v ? v.partner || '' : '' }),
    nummer: h('input', { type: 'text', maxlength: 60, value: v ? v.nummer || '' : '' }),
    beginn: h('input', { type: 'date', value: v ? v.beginn || '' : '' }),
    ende: h('input', { type: 'date', value: v ? v.ende || '' : '' }),
    frist: h('input', { type: 'number', min: 0, max: 60, value: v && v.fristMonate ? v.fristMonate : '' }),
    verl: h('input', { type: 'number', min: 0, max: 120, value: v && v.verlaengert ? v.verlaengert : '' }),
    kosten: h('input', { type: 'text', inputmode: 'decimal', value: v && v.kosten ? numIn(v.kosten) : '' }),
    turnus: h('select', null, ['jährlich', 'vierteljährlich', 'monatlich', 'einmalig'].map((t) => h('option', { value: t, selected: v ? v.turnus === t : false }, t))),
    gekuendigt: h('input', { type: 'date', value: v ? v.gekuendigt || '' : '' }),
    notiz: h('input', { type: 'text', maxlength: 1000, value: v ? v.notiz || '' : '' }),
  };
  const fld = (label, el, wide, hint) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), el, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(v ? v.bezeichnung : 'Neuer Vertrag', h('div', { class: 'form' },
    fld('Art', f.art), fld('Bezeichnung *', f.bez), fld('Vertragspartner', f.partner), fld('Vertrags-/Versicherungsnr.', f.nummer),
    fld('Beginn', f.beginn), fld('Läuft bis', f.ende, false, 'Ende der aktuellen Laufzeit, leer = unbefristet'),
    fld('Kündigungsfrist (Monate)', f.frist), fld('Verlängert sich um (Monate)', f.verl, false, '0 = endet von selbst'),
    fld('Kosten in €', f.kosten), fld('Zahlweise', f.turnus), fld('Gekündigt am', f.gekuendigt), fld('Notiz', f.notiz, true)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const kosten = f.kosten.value.trim() ? parseNum(f.kosten.value) : 0;
    if (Number.isNaN(kosten)) { toast('Bitte die Kosten als Zahl eintragen.', 'err'); return false; }
    try { await api('POST', '/api/admin/vertraege', { id: v ? v.id : '', art: f.art.value, bezeichnung: f.bez.value, partner: f.partner.value, nummer: f.nummer.value, beginn: f.beginn.value, ende: f.ende.value,
      fristMonate: Number(f.frist.value || 0), verlaengert: Number(f.verl.value || 0), kosten, turnus: f.turnus.value, gekuendigt: f.gekuendigt.value, notiz: f.notiz.value }); }
    catch (e) { handleErr(e); return false; }
  } }]);
}

function verwaltungVertraege() {
  const l = S.verw.vertraege;
  const heute = todayIso();
  const in60 = isoInTagen(60);
  const nachher = async (p) => { if (await p) { S.verw = null; render(); } };
  return h('div', null,
    h('p', { class: 'hint' }, 'Versicherungen, Versorger, Zwischenpacht und andere Verträge mit Laufzeit und Kündigungsfrist. Rund zwei Monate vor dem letzten Kündigungstag erinnert das Cockpit – Zeit genug, Angebote zu vergleichen.'),
    h('div', { class: 'toolbar' }, h('button', { class: 'btn primary', onclick: () => nachher(vertragDialog(null)) }, '+ Vertrag'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${l.length} Verträge · ${eur(S.verw.jaehrlich)} im Jahr`)),
    l.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Vertrag', 'Partner', 'Läuft bis', 'Kündigen bis', 'Kosten / Jahr', ''].map((t) => h('th', null, t)))),
      h('tbody', null, l.map((v) => h('tr', null,
        h('td', { class: 'wrap' }, h('b', null, v.bezeichnung), h('div', { class: 'hint' }, v.art + (v.nummer ? ' · ' + v.nummer : ''))),
        h('td', { class: 'wrap' }, v.partner || ''),
        h('td', null, v.laeuftBis ? deDate(v.laeuftBis) : h('span', { class: 'hint' }, 'unbefristet'), v.verlaengert && !v.gekuendigt ? h('div', { class: 'hint' }, `verlängert sich um ${v.verlaengert} Mon.`) : null),
        h('td', null, v.gekuendigt ? h('span', { class: 'badge neu' }, `gekündigt ${deDate(v.gekuendigt)}`) : v.kuendigenBis ? [deDate(v.kuendigenBis),
          v.kuendigenBis >= heute && v.kuendigenBis <= in60 ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, 'bald') : null] : '–'),
        h('td', { class: 'num' }, v.jaehrlich ? eur(v.jaehrlich) : ''),
        h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: () => nachher(vertragDialog(v)) }, 'Bearbeiten'), ' ',
          mehrMenue([{ label: 'Löschen', gefahr: true, aktion: async () => {
            if (!(await confirmBox(`Vertrag »${v.bezeichnung}« löschen?`, 'Löschen', true))) return;
            try { await api('DELETE', `/api/admin/vertraege/${v.id}`); S.verw = null; render(); } catch (e) { handleErr(e); }
          } }])))))))
      : h('div', { class: 'card' }, leer('akte', 'Noch keine Verträge', 'Haftpflicht, Strom, Wasser, Zwischenpacht … – mit Laufzeit und Kündigungsfrist, damit kein Kündigungstermin mehr verpasst wird.', '+ Vertrag', () => nachher(vertragDialog(null)))));
}

// ------------------------------------------------------------------ Haushaltsplan

function adminHaushalt() {
  const st = S.state;
  const jahr = S.haushaltJahr || st.currentYear;
  if (!S.haushalt || S.haushalt.jahr !== jahr) {
    if (!S.haushaltLoading) {
      S.haushaltLoading = true;
      api('GET', `/api/admin/haushalt?year=${jahr}`).then((d) => { S.haushalt = d; }).catch((e) => { handleErr(e); S.haushalt = { jahr, einnahmen: [], ausgaben: [] }; })
        .finally(() => { S.haushaltLoading = false; render(); });
    }
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const d = S.haushalt;
  const eingaben = { einnahmen: new Map(), ausgaben: new Map() };
  const notiz = h('textarea', { rows: 3, class: 'rs-eingabe', maxlength: 2000, placeholder: 'z. B. geplante Anschaffungen, Rücklagen' }); notiz.value = d.notiz || '';
  const jahrSel = h('select', { onchange: (e) => { S.haushaltJahr = Number(e.target.value); render(); } },
    [...new Set([...st.years, st.currentYear + 1])].sort((a, b) => b - a).map((y) => h('option', { value: y, selected: y === jahr }, String(y))));
  const tabelle = (art, titel, zeilen) => {
    let soll = 0, ist = 0, abwSumme = 0;
    const rows = zeilen.map((z) => {
      const inp = h('input', { type: 'text', inputmode: 'decimal', value: z.soll != null ? numIn(z.soll) : '', class: 'hh-in', placeholder: '–' });
      eingaben[art].set(z.posten, inp);
      if (z.soll != null) soll += z.soll;
      ist += z.ist;
      const abw = z.soll != null ? z.ist - z.soll : null;
      if (abw != null) abwSumme += abw;
      const anteil = z.soll ? Math.min(150, Math.round(z.ist / z.soll * 100)) : null;
      const schlecht = abw != null && (art === 'ausgaben' ? abw > 0.004 : abw < -0.004);
      return h('tr', null, h('td', null, z.posten), h('td', { class: 'r' }, inp), h('td', { class: 'r' }, eur(z.ist)),
        h('td', { class: 'r' + (schlecht ? ' diffwarn' : '') }, abw == null ? '–' : eur(abw)),
        h('td', null, anteil == null ? null : h('div', { class: 'hh-erreicht' }, h('div', { class: 'hh-balken' }, h('span', { class: schlecht ? 'zu-viel' : '', style: `width:${Math.min(100, anteil)}%` })), h('i', null, (anteil >= 150 ? '≥ ' : '') + anteil + ' %'))));
    });
    return h('div', { class: 'card tablewrap', style: 'margin-bottom:14px' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, [titel, 'Plan', 'Ist', 'Abweichung', 'erreicht'].map((t) => h('th', null, t)))),
      h('tbody', null, rows, h('tr', { class: 'summe' }, h('td', null, h('b', null, 'Summe')), h('td', { class: 'r' }, h('b', null, eur(soll))), h('td', { class: 'r' }, h('b', null, eur(ist))), h('td', { class: 'r', title: 'nur Posten mit Plan' }, eur(abwSumme)), h('td')))));
  };
  const sammeln = () => {
    const out = { einnahmen: {}, ausgaben: {} };
    for (const art of ['einnahmen', 'ausgaben']) for (const [p, inp] of eingaben[art]) {
      if (!inp.value.trim()) continue;
      const v = parseNum(inp.value);
      if (Number.isNaN(v) || v < 0) throw new Error(`Bitte bei »${p}« einen Betrag ab 0 eintragen.`);
      out[art][p] = v;
    }
    return out;
  };
  const speichern = async () => {
    let werte;
    try { werte = sammeln(); } catch (e) { toast(e.message, 'err'); return; }
    try { S.haushalt = await api('PUT', `/api/admin/haushalt?year=${jahr}`, { ...werte, notiz: notiz.value }); toast('Haushaltsplan gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };
  const ausVorjahr = async () => {
    try {
      const vj = await api('GET', `/api/admin/haushalt?year=${jahr - 1}`);
      for (const art of ['einnahmen', 'ausgaben']) for (const z of vj[art]) { const inp = eingaben[art].get(z.posten); if (inp && z.ist) inp.value = numIn(Math.round(z.ist)); }
      toast('Ist-Werte des Vorjahres eingesetzt (gerundet) – anpassen und speichern', 'ok');
    } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('p', { class: 'hint' }, 'Plane Einnahmen und Ausgaben des Jahres. Das Ist kommt laufend aus dem Kassenbericht (Rechnungen, Spenden, sonstige Ausgaben nach Kategorie) – so sieht man jederzeit, wo der Verein steht. Praktisch für Mitgliederversammlung und Kassenprüfer.'),
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), jahrSel,
      d.vorjahr ? h('button', { class: 'btn', onclick: ausVorjahr }, `Plan aus dem Ist ${jahr - 1} vorschlagen`) : null,
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', onclick: () => pdfDrucken({ url: `/api/admin/haushalt/pdf?year=${jahr}`, oeffnenUrl: `/api/admin/haushalt/pdf/oeffnen?year=${jahr}`, titel: 'Haushaltsplan', meldung: () => 'Der Haushaltsplan wurde im PDF-Programm geöffnet.' }) }, 'Als PDF')),
    tabelle('einnahmen', 'Einnahmen', d.einnahmen),
    tabelle('ausgaben', 'Ausgaben', d.ausgaben),
    h('div', { class: 'card' }, h('div', { class: 'form' }, h('div', { class: 'field wide' }, h('label', null, 'Erläuterungen'), notiz)),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: speichern }, 'Plan speichern'))));
}

// ------------------------------------------------------------------ Vorauszahlungen per Überweisung

async function loadVz(jahr) {
  if (S.vzLoading) return;
  S.vzLoading = true;
  try { S.vz = await api('GET', `/api/admin/vorauszahlungen?jahr=${jahr}`); } catch (e) { handleErr(e); S.vz = { jahr, zeilen: [] }; }
  S.vzLoading = false;
  render();
}

function vzZahlungDialog(z, jahr) {
  const f = { datum: h('input', { type: 'date', value: todayIso() }), betrag: h('input', { type: 'text', inputmode: 'decimal', value: z.soll ? numIn(z.soll) : '' }),
    jahr: h('select', null, [jahr - 1, jahr, jahr + 1].map((j) => h('option', { value: j, selected: j === jahr }, String(j)))),
    notiz: h('input', { type: 'text', maxlength: 200, placeholder: 'z. B. bar beim Arbeitseinsatz' }) };
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(`Vorauszahlung: ${z.name}`, h('div', { class: 'form' },
    fld('Eingegangen am', f.datum), fld('Betrag (€)', f.betrag, z.soll ? `Monatsbetrag ${eur(z.soll)} – mehrere Monate auf einmal gehen auch` : null),
    fld('Für das Abrechnungsjahr', f.jahr), fld('Notiz', f.notiz)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    const b = parseNum(f.betrag.value);
    if (!b || Number.isNaN(b) || b <= 0) { toast('Bitte einen Betrag eintragen.', 'err'); return false; }
    try {
      const v = await api('POST', '/api/admin/vorauszahlungen', { paechterId: z.paechterId, datum: f.datum.value, betrag: b, jahr: Number(f.jahr.value), notiz: f.notiz.value });
      toast(v.gebucht ? 'Gespeichert' : `Gespeichert – vorgemerkt, wird beim Jahreswechsel auf ${v.jahr} gebucht`, 'ok');
    } catch (e) { handleErr(e); return false; }
  } }]);
}

async function vzZahlungenDialog(z) {
  await modal(`Zahlungen ${z.name}`, z.zahlungen.length ? h('div', { class: 'tablewrap' }, h('table', null,
    h('thead', null, h('tr', null, ['Datum', 'Betrag', 'Herkunft', ''].map((x) => h('th', null, x)))),
    h('tbody', null, z.zahlungen.map((v) => h('tr', null, h('td', null, deDate(v.datum)), h('td', { class: 'num' }, eur(v.betrag)),
      h('td', { class: 'klein' }, (v.quelle === 'kontoauszug' ? 'Kontoauszug' : 'von Hand') + (v.gebucht ? '' : ' · vorgemerkt') + (v.notiz ? ' · ' + v.notiz : '')),
      h('td', null, h('button', { class: 'btn small danger', onclick: async () => {
        if (!(await confirmBox(`Zahlung vom ${deDate(v.datum)} über ${eur(v.betrag)} löschen? Sie wird auch vom Abschlag der Jahresrechnung abgezogen.`, 'Löschen', true))) return;
        try { await api('DELETE', `/api/admin/vorauszahlungen/${v.id}`); toast('Gelöscht', 'ok'); S.vz = null; document.querySelector('dialog[open] .btn.primary')?.click(); await reload(); } catch (e) { handleErr(e); }
      } }, 'Löschen')))))))
    : h('p', { class: 'hint' }, 'Noch keine Zahlungen in diesem Jahr.'),
  [{ label: 'Schließen', cls: 'primary', value: true }]);
}

function adminVorauszahlung() {
  const jahrHeute = S.state.currentYear; // Abrechnungsjahr: darauf werden die Vorauszahlungen gebucht
  const jahr = S.vzJahr || jahrHeute;
  if (S.vz == null || S.vz.jahr !== jahr) { if (!S.vzLoading) loadVz(jahr); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const d = S.vz;
  const jahrSel = h('select', { onchange: (e) => { S.vzJahr = Number(e.target.value); S.vz = null; render(); } },
    [jahrHeute - 1, jahrHeute, jahrHeute + 1].map((j) => h('option', { value: j, selected: j === jahr }, String(j))));
  const sollSpeichern = async (z, feld) => {
    const b = parseNum(feld.value) ?? 0;
    if (Number.isNaN(b) || b < 0) { toast('Bitte einen gültigen Betrag eintragen.', 'err'); feld.value = z.soll ? numIn(z.soll) : ''; return; }
    if (b === z.soll) return;
    try { await api('PUT', '/api/admin/vorauszahlungen/soll', { paechterId: z.paechterId, betrag: b }); toast('Monatsbetrag gespeichert', 'ok'); S.vz = null; render(); } catch (e) { handleErr(e); }
  };
  const zelle = (z, m) => {
    const wert = z.monate[m];
    const faellig = m < d.faelligeMonate;
    const cls = !z.soll ? 'vz-leer' : wert >= 0.999 ? 'vz-ok' : wert > 0 ? 'vz-teil' : faellig ? 'vz-offen' : 'vz-kommt';
    return h('td', { class: 'vz-zelle' }, h('span', { class: 'vz-punkt ' + cls, title: `${MONATE[m]}: ${!z.soll ? 'kein Monatsbetrag' : wert >= 0.999 ? 'bezahlt' : wert > 0 ? 'teilweise' : faellig ? 'offen' : 'noch nicht fällig'}` }));
  };
  const erinnerung = (z) => pdfDrucken({ url: `/api/admin/vorauszahlungen/erinnerung?paechter=${z.paechterId}&jahr=${jahr}`, oeffnenUrl: `/api/admin/vorauszahlungen/erinnerung/oeffnen?paechter=${z.paechterId}&jahr=${jahr}`,
    titel: `Erinnerung ${z.name}`, meldung: () => 'Die Erinnerung wurde im PDF-Programm geöffnet – dort drucken oder verschicken.' });
  const mitSoll = d.zeilen.filter((z) => z.soll > 0).length;
  const sepaSchalter = h('input', { type: 'checkbox', checked: !!S.state.sepaAktiv });
  sepaSchalter.addEventListener('change', async () => {
    try {
      const sepa = await api('GET', '/api/admin/sepa');
      await api('PUT', '/api/admin/sepa/einstellungen', { ...sepa.einstellungen, aktiv: sepaSchalter.checked });
      await reload();
    } catch (e) { handleErr(e); sepaSchalter.checked = !sepaSchalter.checked; }
  });
  return h('div', null,
    h('p', { class: 'hint' }, 'Monatliche Vorauszahlungen, die Pächter selbst überweisen (z. B. per Dauerauftrag). Monatsbetrag je Pächter eintragen; eingegangene Zahlungen erkennt der Kontoauszug (Name und Betrag), sonst »+ Zahlung«. Alles wird als Abschlag auf die Jahresrechnung gebucht.'),
    d.vorgemerkt ? h('div', { class: 'banner info' }, `${d.vorgemerkt} Zahlung(en) für ${jahr} sind vorgemerkt – das Abrechnungsjahr ist im Programm noch nicht angelegt. Sie werden beim Jahreswechsel automatisch gebucht.`) : null,
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), jahrSel,
      h('button', { class: 'btn', onclick: () => { S.tab = 'zahlungen'; render(); }, title: 'Unter »Zahlungen« → »Kontoauszug einlesen«' }, 'Kontoauszug einlesen →'),
      h('div', { class: 'spacer' }), h('span', { class: 'stat' }, `${mitSoll} mit Monatsbetrag · eingegangen ${eur(d.summe)}${d.rueckstand ? ' · Rückstand ' + eur(d.rueckstand) : ''}`)),
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data vz-tab' },
      h('thead', null, h('tr', null, h('th', null, 'Garten'), h('th', null, 'Pächter'), h('th', null, 'je Monat'), ...MONATE.map((m) => h('th', { class: 'vz-zelle' }, m.slice(0, 3))),
        h('th', null, 'Eingang'), h('th', null, 'Rückstand'), h('th', null, ''))),
      h('tbody', null, d.zeilen.map((z) => {
        const feld = h('input', { type: 'text', inputmode: 'decimal', class: 'std-in', value: z.soll ? numIn(z.soll) : '', placeholder: '–', 'aria-label': `Monatsbetrag ${z.name}` });
        feld.addEventListener('change', () => sollSpeichern(z, feld));
        return h('tr', null, h('td', null, z.gartennr), h('td', null, z.name), h('td', null, feld), ...z.monate.map((_, m) => zelle(z, m)),
          h('td', { class: 'num' }, z.summe ? eur(z.summe) : '–'), h('td', { class: 'num' }, z.rueckstand ? h('b', { class: 'warn-text' }, eur(z.rueckstand)) : '–'),
          h('td', { class: 'aktionen' }, h('button', { class: 'btn small', onclick: async () => { if (await vzZahlungDialog(z, jahr)) { S.vz = null; await reload(); } } }, '+ Zahlung'), ' ',
            mehrMenue([
              { label: `Zahlungen ansehen (${z.zahlungen.length})`, aktion: () => vzZahlungenDialog(z) },
              z.rueckstand ? { label: 'Erinnerung schreiben', aktion: () => erinnerung(z) } : null])));
      })))),
    h('div', { class: 'vz-legende hint' }, h('span', { class: 'vz-punkt vz-ok' }), ' bezahlt ', h('span', { class: 'vz-punkt vz-teil' }), ' teilweise ',
      h('span', { class: 'vz-punkt vz-offen' }), ' offen ', h('span', { class: 'vz-punkt vz-kommt' }), ' noch nicht fällig (ab dem 10. eines Monats fällig)'),
    h('details', { class: 'card', style: 'margin-top:16px' }, h('summary', null, 'Zahlt euer Verein (auch) per Lastschrift?'),
      h('p', { class: 'hint' }, 'Dann lässt sich der Bereich »Lastschrift« einschalten: Mandate, Datei für das Online-Banking (SEPA), Vorabankündigung.'),
      h('label', { class: 'check' }, sepaSchalter, ' Bereich »Lastschrift« anzeigen')));
}

// ------------------------------------------------------------------ SEPA-Lastschrift

async function loadSepa() {
  if (S.sepaLoading) return;
  S.sepaLoading = true;
  try { S.sepa = await api('GET', '/api/admin/sepa'); } catch (e) { handleErr(e); S.sepa = { mandate: [], laeufe: [], einstellungen: {} }; }
  S.sepaLoading = false;
  render();
}
const isoTag = (d) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
const ibanKurz = (iban) => (iban && iban.length > 8 ? `${iban.slice(0, 4)} … ${iban.slice(-4)}` : iban || '');

// Vorschlag fürs Fälligkeitsdatum: Vorauszahlung zum Fälligkeitstag des nächsten möglichen Monats, Rechnungen in einer Woche
function sepaFaelligVorschlag(art, tag) {
  const werktag = (d) => { while (d.getDay() === 0 || d.getDay() === 6) d.setDate(d.getDate() + 1); return d; };
  const frueh = new Date(); frueh.setDate(frueh.getDate() + 3);
  if (art === 'rechnungen') { const d = new Date(); d.setDate(d.getDate() + 7); return isoTag(werktag(d)); }
  let d = werktag(new Date(frueh.getFullYear(), frueh.getMonth(), tag || 1));
  if (d < frueh) d = werktag(new Date(frueh.getFullYear(), frueh.getMonth() + 1, tag || 1));
  return isoTag(d);
}

function mandatDialog(m) {
  const st = S.state;
  const isNew = !m || !m.id;
  const cur = { paechterId: '', kontoinhaber: '', iban: '', bic: '', unterschrieben: todayIso(), monatsbetrag: 0, rechnungen: true, referenz: '', widerrufen: '', ...(m || {}) };
  const genutzt = !!cur.zuletztGenutzt;
  const f = {
    paechter: h('select', { disabled: !isNew }, [h('option', { value: '' }, '– Pächter wählen –'),
      ...st.paechter.filter((p) => !p.geloescht && !p.ausgeschiedenAm).map((p) => h('option', { value: p.id, selected: p.id === cur.paechterId }, `${p.gartennr ? 'Garten ' + p.gartennr + ' · ' : ''}${p.name}`))]),
    inhaber: h('input', { type: 'text', value: cur.kontoinhaber, maxlength: 70, placeholder: 'leer = Name des Pächters' }),
    iban: h('input', { type: 'text', value: cur.iban, maxlength: 42, placeholder: 'DE12 3456 …', autocomplete: 'off' }),
    bic: h('input', { type: 'text', value: cur.bic || '', maxlength: 11, placeholder: 'bei deutschen Konten nicht nötig' }),
    datum: h('input', { type: 'date', value: cur.unterschrieben, disabled: genutzt }),
    betrag: h('input', { type: 'text', inputmode: 'decimal', value: cur.monatsbetrag ? numIn(cur.monatsbetrag) : '', placeholder: '0,00' }),
    rechnungen: h('input', { type: 'checkbox', checked: !!cur.rechnungen }),
    referenz: h('input', { type: 'text', value: cur.referenz, maxlength: 35, disabled: genutzt, placeholder: 'wird automatisch vergeben' }),
    widerrufen: h('input', { type: 'date', value: cur.widerrufen || '' }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Pächter *', f.paechter, null, true), fld('Kontoinhaber', f.inhaber), fld('IBAN *', f.iban), fld('BIC', f.bic),
    fld('Unterschrieben am *', f.datum), fld('Monatliche Vorauszahlung (€)', f.betrag, '0 = keine Vorauszahlung'),
    h('div', { class: 'field wide' }, h('label', { class: 'check' }, f.rechnungen, ' Auch den Restbetrag der Jahresrechnung einziehen')),
    fld('Mandatsreferenz', f.referenz, genutzt ? 'schon genutzt – nicht mehr änderbar' : null),
    isNew ? null : fld('Widerrufen am', f.widerrufen, 'leer = gültig'));
  return modal(isNew ? 'Lastschriftmandat anlegen' : 'Lastschriftmandat bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const betrag = parseNum(f.betrag.value) ?? 0;
      if (Number.isNaN(betrag) || betrag < 0) { toast('Bitte einen gültigen Monatsbetrag eintragen.', 'err'); return false; }
      const daten = { ...cur, paechterId: f.paechter.value, kontoinhaber: f.inhaber.value, iban: f.iban.value, bic: f.bic.value, unterschrieben: f.datum.value,
        monatsbetrag: betrag, rechnungen: f.rechnungen.checked, referenz: f.referenz.value, widerrufen: f.widerrufen.value };
      try {
        if (isNew) await api('POST', '/api/admin/sepa/mandate', daten); else await api('PUT', `/api/admin/sepa/mandate/${cur.id}`, daten);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

const mandatsformularDrucken = (pid) => pdfDrucken({
  url: `/api/admin/sepa/formular?paechter=${encodeURIComponent(pid)}`, oeffnenUrl: `/api/admin/sepa/formular/oeffnen?paechter=${encodeURIComponent(pid)}`,
  titel: 'SEPA-Lastschriftmandat', meldung: () => 'Das Mandat wurde im PDF-Programm geöffnet – drucken und unterschreiben lassen.',
});
const vorabDrucken = (jahr) => pdfDrucken({
  url: `/api/admin/sepa/vorab?jahr=${jahr}`, oeffnenUrl: `/api/admin/sepa/vorab/oeffnen?jahr=${jahr}`,
  titel: `Vorabankündigung ${jahr}`, meldung: () => 'Die Vorabankündigungen wurden im PDF-Programm geöffnet – je Pächter eine Seite.',
});

async function laufDatei(l) {
  if (inNativeWindow()) {
    try { const r = await api('POST', `/api/admin/sepa/lauf/${l.id}/datei`); toast(`${r.datei} liegt im Ordner »Lastschriften« – im Online-Banking als SEPA-Sammellastschrift hochladen.`, 'ok'); } catch (e) { handleErr(e); }
  } else location.href = `/api/admin/sepa/lauf/${l.id}/xml`;
}

async function laufDetails(l) {
  const zeilen = l.posten.map((x) => h('tr', { class: x.zurueck ? 'vorbei' : '' },
    h('td', null, x.name), h('td', { class: 'klein' }, x.referenz), h('td', { class: 'num' }, eur(x.betrag)),
    h('td', null, x.zurueck ? h('span', { class: 'badge warn' }, `zurück ${deDate(x.zurueck)}`) : h('button', { class: 'btn small', onclick: async () => {
      if (!(await confirmBox(`Lastschrift von ${x.name} über ${eur(x.betrag)} ist zurückgegangen? Die Buchung wird zurückgenommen${l.art === 'rechnungen' ? ' (Rechnung wieder offen)' : ' (Vorauszahlung nicht verrechnet)'}.`, 'Rücklastschrift buchen', true))) return;
      try { await api('POST', `/api/admin/sepa/lauf/${l.id}/rueck`, { endToEnd: x.endToEnd }); toast('Rücklastschrift gebucht', 'ok'); S.sepa = null; await reload(); } catch (e) { handleErr(e); }
    } }, 'Rücklastschrift'))));
  await breiterDialog(`Lastschrift zum ${deDate(l.faellig)}`, h('div', { class: 'tablewrap', style: 'max-height:420px;overflow:auto' }, h('table', null,
    h('thead', null, h('tr', null, ['Kontoinhaber', 'Mandat', 'Betrag', ''].map((x) => h('th', null, x)))), h('tbody', null, zeilen))),
  [{ label: 'Schließen', cls: 'primary', value: true }]);
}

function adminSepa() {
  if (S.sepa == null) { if (!S.sepaLoading) loadSepa(); return h('p', { class: 'hint' }, 'Wird geladen …'); }
  const d = S.sepa, e = d.einstellungen || {};
  const heute = new Date();
  const glaeubiger = h('input', { type: 'text', value: e.glaeubigerId || '', maxlength: 35, placeholder: 'DE98ZZZ09999999999' });
  const tagSel = h('select', null, Array.from({ length: 28 }, (_, i) => h('option', { value: i + 1, selected: (e.faelligkeitstag || 1) === i + 1 }, `${i + 1}.`)));
  const einstSpeichern = async () => {
    try { await api('PUT', '/api/admin/sepa/einstellungen', { glaeubigerId: glaeubiger.value, faelligkeitstag: Number(tagSel.value) }); toast('Gespeichert', 'ok'); S.sepa = null; render(); } catch (er) { handleErr(er); }
  };
  const art = h('select', null, [['vorauszahlung', 'Monatliche Vorauszahlung'], ['rechnungen', 'Jahresrechnungen (Restbetrag)']].map(([v, t]) => h('option', { value: v, selected: (S.sepaArt || 'vorauszahlung') === v }, t)));
  const faellig = h('input', { type: 'date', value: S.sepaFaellig || sepaFaelligVorschlag(S.sepaArt || 'vorauszahlung', e.faelligkeitstag) });
  art.addEventListener('change', () => { S.sepaArt = art.value; S.sepaFaellig = null; S.sepaVorschau = null; render(); });
  faellig.addEventListener('change', () => { S.sepaFaellig = faellig.value; S.sepaVorschau = null; render(); });
  const vorschau = async () => {
    try { S.sepaVorschau = await api('GET', `/api/admin/sepa/vorschau?art=${art.value}&faellig=${faellig.value}`); render(); } catch (er) { handleErr(er); }
  };
  const erstellen = async () => {
    const v = S.sepaVorschau;
    if (!(await confirmBox(`${v.posten.length} Lastschrift(en) über ${eur(v.summe)} zum ${deDate(v.faellig)} erstellen? ${v.art === 'vorauszahlung' ? 'Die Beträge werden als Vorauszahlung auf die Jahresrechnung gebucht.' : 'Die Rechnungen werden als bezahlt vermerkt.'} Danach die Datei im Online-Banking hochladen.`, 'Erstellen'))) return;
    try {
      const l = await api('POST', '/api/admin/sepa/lauf', { art: v.art, faellig: v.faellig });
      S.sepaVorschau = null; S.sepa = null; await reload();
      await laufDatei(l);
    } catch (er) { handleErr(er); }
  };
  const v = S.sepaVorschau;
  const status = (m) => (m.widerrufen ? h('span', { class: 'badge' }, `widerrufen ${deDate(m.widerrufen)}`) : m.verfallen ? h('span', { class: 'badge err' }, 'verfallen') : h('span', { class: 'badge ok' }, 'gültig'));
  const neuMandat = async () => { if (await mandatDialog(null)) { S.sepa = null; render(); } };
  return h('div', null,
    h('p', { class: 'hint' }, 'Monatliche Vorauszahlungen und die Jahresrechnung per SEPA-Lastschrift einziehen. Das Programm erstellt die Datei für das Online-Banking und bucht gleich mit: Vorauszahlungen werden auf der Jahresrechnung abgezogen, eingezogene Rechnungen als bezahlt vermerkt.'),
    d.hinweis ? h('div', { class: 'banner warn' }, d.hinweis) : null,
    h('details', { class: 'card', open: !!d.hinweis },
      h('summary', null, h('b', null, 'Einstellungen'), e.glaeubigerId ? h('span', { class: 'hint' }, ` · Gläubiger-ID ${e.glaeubigerId} · Vorauszahlung zum ${e.faelligkeitstag || 1}. des Monats`) : null),
      h('div', { class: 'form', style: 'margin-top:10px' },
        h('div', { class: 'field' }, h('label', null, 'Gläubiger-Identifikationsnummer'), glaeubiger, h('span', { class: 'fhint' }, 'beantragt der Verein kostenlos online bei der Deutschen Bundesbank')),
        h('div', { class: 'field' }, h('label', null, 'Vorauszahlung fällig am'), tagSel, h('span', { class: 'fhint' }, 'Tag im Monat')),
        h('div', { class: 'field' }, h('label', null, ' '), h('button', { class: 'btn primary', onclick: einstSpeichern }, 'Speichern')))),

    h('h3', null, 'Lastschrift erstellen'),
    h('div', { class: 'card' },
      h('div', { class: 'toolbar' }, h('label', null, 'Art'), art, h('label', null, 'Fällig am'), faellig,
        h('button', { class: 'btn', disabled: !!d.hinweis, onclick: vorschau }, 'Vorschau')),
      v ? h('div', null,
        v.hinweise.length ? h('div', { class: 'banner info' }, h('b', null, 'Nicht dabei: '), v.hinweise.join(' · ')) : null,
        v.posten.length ? [
          h('div', { class: 'tablewrap', style: 'max-height:300px;overflow:auto' }, h('table', null,
            h('thead', null, h('tr', null, ['Kontoinhaber', 'IBAN', 'Verwendungszweck', 'Betrag'].map((x) => h('th', null, x)))),
            h('tbody', null, v.posten.map((x) => h('tr', null, h('td', null, x.name), h('td', { class: 'klein' }, ibanKurz(x.iban)), h('td', { class: 'klein wrap' }, x.zweck), h('td', { class: 'num' }, eur(x.betrag))))))),
          h('div', { class: 'toolbar', style: 'margin-top:10px' }, h('b', null, `${v.posten.length} Lastschrift(en) · ${eur(v.summe)}`), h('div', { class: 'spacer' }),
            h('button', { class: 'btn primary', onclick: erstellen }, 'Lauf erstellen und Datei speichern'))]
          : h('p', { class: 'hint' }, 'Nichts einzuziehen.')) : h('p', { class: 'hint' }, 'Fälligkeitsdatum wählen und »Vorschau« klicken. Die Bank braucht die Datei rechtzeitig vor dem Termin (meist einen Bankarbeitstag vorher). Pächter müssen die Termine vorher kennen – dafür gibt es die Vorabankündigung.')),

    h('h3', null, `Mandate (${d.mandate.length})`),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: neuMandat }, '+ Mandat erfassen'),
      h('button', { class: 'btn', disabled: !!d.hinweis, title: 'Je Pächter mit Vorauszahlung eine Seite mit allen Abbuchungsterminen', onclick: () => vorabDrucken(heute.getFullYear()) }, `Vorabankündigung ${heute.getFullYear()}`),
      heute.getMonth() >= 9 ? h('button', { class: 'btn', disabled: !!d.hinweis, onclick: () => vorabDrucken(heute.getFullYear() + 1) }, `Vorabankündigung ${heute.getFullYear() + 1}`) : null),
    d.mandate.length ? h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Garten', 'Kontoinhaber', 'Mandatsreferenz', 'IBAN', 'Monatlich', 'Jahresrechnung', 'Status', ''].map((x) => h('th', null, x)))),
      h('tbody', null, d.mandate.map((m) => h('tr', null,
        h('td', null, m.gartennr), h('td', null, m.kontoinhaber, m.name && m.name !== m.kontoinhaber ? h('div', { class: 'hint' }, m.name) : null),
        h('td', { class: 'klein' }, m.referenz, h('div', { class: 'hint' }, `unterschrieben ${deDate(m.unterschrieben)}`)),
        h('td', { class: 'klein' }, ibanKurz(m.iban)), h('td', { class: 'num' }, m.monatsbetrag ? eur(m.monatsbetrag) : '–'),
        h('td', null, m.rechnungen ? 'ja' : 'nein'), h('td', null, status(m)),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', onclick: async () => { if (await mandatDialog(m)) { S.sepa = null; render(); } } }, 'Bearbeiten'), ' ',
          mehrMenue([
            { label: 'Mandatsformular drucken', aktion: () => mandatsformularDrucken(m.paechterId) },
            m.zuletztGenutzt ? null : { label: 'Löschen', gefahr: true, aktion: async () => {
              if (!(await confirmBox(`Mandat ${m.referenz} löschen?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/sepa/mandate/${m.id}`); S.sepa = null; render(); } catch (er) { handleErr(er); }
            } }])))))))
      : h('div', { class: 'card' }, leer('euro', 'Noch keine Mandate', 'Pächter, die per Lastschrift zahlen möchten, unterschreiben ein SEPA-Mandat. Das Formular druckst du nach dem Anlegen über »Mehr« oder vorab für jeden Pächter.', '+ Mandat erfassen', neuMandat)),

    d.laeufe.length ? [h('h3', null, 'Bisherige Lastschriften'),
      h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Fällig', 'Art', 'Posten', 'Summe', ''].map((x) => h('th', null, x)))),
        h('tbody', null, d.laeufe.map((l) => h('tr', null,
          h('td', null, deDate(l.faellig)), h('td', null, l.art === 'vorauszahlung' ? 'Vorauszahlung' : 'Jahresrechnungen'),
          h('td', null, String(l.posten.length), l.zurueckAnzahl ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, `${l.zurueckAnzahl} zurück`) : null),
          h('td', { class: 'num' }, eur(l.summe)),
          h('td', { class: 'aktionen' }, h('button', { class: 'btn small', disabled: l.bereinigt, onclick: () => laufDatei(l) }, 'Datei'), ' ',
            h('button', { class: 'btn small', disabled: l.bereinigt, onclick: () => laufDetails(l) }, 'Einzelposten')))))))] : null,
    h('p', { class: 'hint', style: 'margin-top:14px' }, 'Gut zu wissen: Ein Mandat verfällt, wenn 36 Monate lang nicht damit eingezogen wurde. Zahlungspflichtige können eine Lastschrift innerhalb von acht Wochen zurückgeben – dann hier unter »Einzelposten« die Rücklastschrift buchen. Die Datei hat das Format pain.008 (SEPA-Basislastschrift), das deutsche Banken im Online-Banking annehmen.'));
}

// ------------------------------------------------------------------ Kontoauszug einlesen

const KONTO_STATUS = {
  passt: ['ok', 'Nummer und Betrag passen'], teilbetrag: ['warn', 'Teil- oder Mehrzahlung'], vorschlag: ['neu', 'Vorschlag – bitte prüfen'],
  vorauszahlung: ['ok', 'Vorauszahlung'], spende: ['neu', 'Spende?'], bezahlt: ['', 'schon bezahlt'], doppelt: ['', 'schon übernommen'], offen: ['', 'nicht zugeordnet'],
};

function kontoauszugEinlesen() {
  const datei = h('input', { type: 'file', accept: '.csv,.txt,text/csv', style: 'display:none' });
  datei.addEventListener('change', async () => {
    if (!datei.files.length) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    let v;
    try { v = await api('POST', '/api/admin/kontoauszug', fd, true); } catch (e) { handleErr(e); return; } finally { datei.remove(); }
    if (await kontoauszugVorschau(v)) await reload();
  });
  document.body.append(datei);
  datei.click();
}

function kontoauszugVorschau(v) {
  if (!v.buchungen.length) { toast(`Keine Eingänge in der Datei gefunden${v.ausgaben ? ` (${v.ausgaben} Abbuchungen übersprungen)` : ''}.`, 'err'); return false; }
  const optionen = (sel) => [h('option', { value: '' }, '– nicht zuordnen –'), h('option', { value: 'spende', selected: sel === 'spende' }, '→ als Spende erfassen'),
    ...(v.vorauszahler || []).map((p) => h('option', { value: 'vz:' + p.id, selected: sel === 'vz:' + p.id }, `→ Vorauszahlung: ${p.gartennr ? 'Garten ' + p.gartennr + ' · ' : ''}${p.name} (${eur(p.soll)}/Monat)`)),
    ...v.offene.map((r) => h('option', { value: r.id, selected: r.id === sel }, `${r.nummer} · ${r.name} · offen ${eur(r.offen)}${r.jahr ? ` (${r.jahr})` : ''}`))];
  const zeilen = v.buchungen.map((b) => {
    const fest = b.status === 'doppelt' || b.status === 'bezahlt';
    const wahl = h('select', { disabled: fest, class: 'konto-wahl' }, optionen(b.status === 'spende' ? 'spende' : b.status === 'vorauszahlung' ? 'vz:' + b.paechterId : b.rechnungId));
    const an = h('input', { type: 'checkbox', disabled: fest, checked: b.status === 'passt' || b.status === 'teilbetrag' || b.status === 'vorauszahlung', 'aria-label': 'übernehmen' });
    wahl.addEventListener('change', () => { an.checked = !!wahl.value; zaehlen(); });
    an.addEventListener('change', () => { if (an.checked && !wahl.value) { an.checked = false; toast('Bitte zuerst eine Rechnung auswählen.', 'err'); } zaehlen(); });
    const [cls, text] = KONTO_STATUS[b.status] || ['', b.status];
    return { b, wahl, an, tr: h('tr', { class: fest ? 'vorbei' : '' },
      h('td', null, an), h('td', null, deDate(b.datum)),
      h('td', { class: 'wrap klein' }, h('b', null, b.name || '–'), h('div', { class: 'hint' }, b.zweck)),
      h('td', { class: 'num' }, eur(b.betrag)),
      h('td', null, wahl, h('div', null, h('span', { class: 'badge ' + cls }, text), b.hinweis ? h('span', { class: 'hint', style: 'margin-left:6px' }, b.hinweis) : null))) };
  });
  const zaehlerEl = h('b', null);
  const zaehlen = () => {
    const l = zeilen.filter((z) => z.an.checked);
    zaehlerEl.textContent = `${l.length} Zahlung(en) · ${eur(l.reduce((a, z) => a + z.b.betrag, 0))}`;
  };
  zaehlen();
  const n = (st) => v.buchungen.filter((b) => b.status === st).length;
  const body = h('div', null,
    h('p', { class: 'hint' }, `${v.buchungen.length} Eingänge gelesen${v.ausgaben ? `, ${v.ausgaben} Abbuchungen übersprungen` : ''}: ${n('passt')} passen genau, ${n('teilbetrag')} Teilzahlungen, ${n('vorschlag')} Vorschläge über den Namen, ${n('vorauszahlung')} Vorauszahlungen, ${n('spende')} mögliche Spenden, ${n('offen')} nicht zugeordnet${n('doppelt') ? `, ${n('doppelt')} schon früher übernommen` : ''}. Übernommen wird nur, was angehakt ist. Die Datei selbst wird nicht gespeichert.`),
    h('div', { class: 'toolbar' }, h('div', { class: 'spacer' }), zaehlerEl),
    h('div', { class: 'tablewrap', style: 'max-height:420px;overflow:auto' }, h('table', { class: 'konto-tab' },
      h('thead', null, h('tr', null, ['', 'Datum', 'Von / Verwendungszweck', 'Betrag', 'Rechnung'].map((x) => h('th', null, x)))),
      h('tbody', null, zeilen.map((z) => z.tr)))));
  return breiterDialog('Kontoauszug einlesen', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Angehakte übernehmen', cls: 'primary', value: true, action: async () => {
      const zahlungen = zeilen.filter((z) => z.an.checked && z.wahl.value).map((z) => {
        const basis = { datum: z.b.datum, betrag: z.b.betrag, fingerprint: z.b.fingerprint };
        if (z.wahl.value === 'spende') return { ...basis, spende: true, name: z.b.name };
        if (z.wahl.value.startsWith('vz:')) return { ...basis, vorauszahlung: z.wahl.value.slice(3) };
        return { ...basis, rechnungId: z.wahl.value };
      });
      if (!zahlungen.length) { toast('Es ist nichts angehakt.', 'err'); return false; }
      try {
        const r = await api('POST', '/api/admin/kontoauszug/buchen', { zahlungen });
        toast(`${r.gebucht} Zahlung(en) über ${eur(r.summe)} übernommen${r.vorauszahlungen ? ` – davon ${r.vorauszahlungen} Vorauszahlung(en)` : ''}${r.spenden ? ` – davon ${r.spenden} als Spende; Anschrift unter »Spenden« ergänzen` : ''}`, 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

// ------------------------------------------------------------------ Tab: Admin

function viewAdmin() {
  const st = S.state;
  if (st.hasPassword && !st.loggedIn) return viewLogin();
  const t = S.adminTab;
  const body = t === 'paechter' ? adminPaechter() : t === 'warteliste' ? adminWarteliste() : t === 'maengel' ? adminMaengel() : t === 'spenden' ? adminSpenden() : t === 'sepa' ? adminSepa() : t === 'vorauszahlung' ? adminVorauszahlung() : t === 'rundschreiben' ? adminRundschreiben() : t === 'inventar' ? adminInventar() : t === 'kuendigungen' ? adminKuendigungen() : t === 'verwaltung' ? adminVerwaltung() : t === 'haushalt' ? adminHaushalt() : t === 'einstellungen' ? adminSettings()
    : t === 'jahreswechsel' ? adminYear() : t === 'kassenbericht' ? adminKassenbericht(false) : t === 'berichte' ? adminBerichte()
      : t === 'jubilaeen' ? adminJubilaeen() : t === 'daten' ? adminData() : t === 'protokoll' ? adminProtokoll() : t === 'datenschutz' ? adminDatenschutz() : adminSecurity();
  const eintrag = navEintrag('admin', t);
  return h('div', null,
    h('h2', null, eintrag ? eintrag.label : 'Verwaltung'),
    !st.hasPassword ? h('div', { class: 'banner warn' }, 'Für die Verwaltung ist noch kein Passwort gesetzt – jeder an diesem Rechner kann Pächter und Preise ändern. ',
      h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('sicherheit'); render(); } }, 'Passwort festlegen')) : null,
    body);
}

function viewLogin() {
  const st = S.state;
  const pw = h('input', { type: 'password', autocomplete: 'current-password', style: 'width:100%', autofocus: true });
  const go = async () => {
    try { await api('POST', '/api/admin/login', { password: pw.value }); await reload(); } catch (e) { handleErr(e); pw.select(); }
  };
  pw.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => pw.focus(), 50);
  return h('div', { class: 'center card login' }, h('img', { class: 'login-bild', src: '/bilder/garten.webp', alt: '', onerror: (e) => e.target.remove() }), h('h2', null, 'Anmeldung zur Verwaltung'),
    h('p', { class: 'hint' }, 'Dieser Bereich ist durch ein Passwort geschützt.'), pw,
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: go }, 'Anmelden')),
    st.hasPruefPassword ? h('p', { style: 'margin-top:18px' },
      h('button', { class: 'btn', onclick: () => { S.pruefLoginMode = true; render(); } }, 'Als Kassenprüfer anmelden')) : null);
}

function viewPruefLogin() {
  const pw = h('input', { type: 'password', autocomplete: 'current-password', style: 'width:100%', autofocus: true });
  const go = async () => {
    try { await api('POST', '/api/pruef/login', { password: pw.value }); S.pruefLoginMode = false; await reload(); } catch (e) { handleErr(e); pw.select(); }
  };
  pw.addEventListener('keydown', (e) => { if (e.key === 'Enter') go(); });
  setTimeout(() => pw.focus(), 50);
  return h('div', { class: 'center card' }, h('h2', null, 'Kassenprüfer-Anmeldung'),
    h('p', { class: 'hint' }, 'Lesezugriff auf den Kassenbericht, ohne die übrigen Admin-Rechte.'), pw,
    h('div', { class: 'actions' },
      h('button', { class: 'btn primary', onclick: go }, 'Anmelden'),
      h('button', { class: 'btn', onclick: () => { S.pruefLoginMode = false; render(); } }, 'Zurück')));
}

// Eigenständige, schmale Oberfläche für den Kassenprüfer-Zugang: nur Lesezugriff
// auf den Kassenbericht (inkl. Geprüft-Haken), kein Zugriff auf die übrigen Tabs.
function viewPruefShell() {
  const st = S.state;
  if (S.kasseYear == null) S.kasseYear = st.year;
  const app = h('div');
  const yearSel = h('select', { onchange: async (e) => { S.year = Number(e.target.value); S.kasse = null; S.kasseYear = S.year; await reload(); } },
    st.years.map((y) => h('option', { value: y, selected: y === st.year }, y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  app.append(
    h('header', { class: 'top' },
      h('div', { class: 'brand' }, h('b', null, 'Kleingarten-Manager'), h('span', null, st.settings.vereinName + ' · Kassenprüfer-Zugang')),
      h('div', { class: 'spacer' }),
      h('div', null, h('label', null, 'Jahr'), yearSel),
      h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/pruef/logout'); } catch (e) { /* egal */ } await reload(); } }, 'Abmelden')),
    h('main', null, h('h2', null, 'Kassenbericht'), adminKassenbericht(true)));
  return app;
}

// ---- Pächter verwalten

function paechterDialog(p, vorlage) {
  const isNew = !p;
  const cur = p || { mitgliedsnr: '', gartennr: '', anrede: 'Herr', name: '', strasse: '', plzOrt: S.state.settings.ort, versand: 'Emailsendung',
    email: '', telefon: '', gartengroesse: 0, umlageAbweichend: null, wasserzaehlerNr: '', stromzaehlerNr: '', notiz: '', ...(vorlage || {}) };
  const t = (val, ph) => h('input', { type: 'text', value: val || '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const f = {
    mitgliedsnr: t(cur.mitgliedsnr, 'z. B. 35-95'), gartennr: t(cur.gartennr, 'z. B. 35'),
    anrede: h('select', { value: cur.anrede }, ['Herr', 'Frau', 'Familie', 'Herr und Frau', ''].map((a) => h('option', { value: a }, a || '(keine)'))),
    name: t(cur.name), strasse: t(cur.strasse), plzOrt: t(cur.plzOrt),
    versand: h('select', { value: cur.versand }, ['Emailsendung', 'Postversand'].map((a) => h('option', { value: a }, a))),
    email: h('input', { type: 'email', value: cur.email || '', placeholder: 'name@beispiel.de', maxlength: 100, autocomplete: 'off' }),
    telefon: h('input', { type: 'tel', value: cur.telefon || '', maxlength: 40, autocomplete: 'off' }),
    gartengroesse: t(numIn(cur.gartengroesse || null)),
    umlage: t(numIn(cur.umlageAbweichend), `leer = Standard (${eur(S.state.settings.umlageStandard)})`),
    wz: t(cur.wasserzaehlerNr), sz: t(cur.stromzaehlerNr),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%' }, cur.notiz || ''),
  };
  f.gartengroesse.setAttribute('inputmode', 'decimal');
  f.umlage.setAttribute('inputmode', 'decimal');
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Mitgliedsnummer *', f.mitgliedsnr), fld('Gartennummer', f.gartennr), fld('Anrede', f.anrede),
    fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('Versandart', f.versand, 'Steht oben rechts auf der Rechnung'),
    fld('E-Mail-Adresse', f.email, 'Empfänger beim Vorbereiten der Rechnungs-E-Mail'), fld('Telefon', f.telefon),
    fld('Gartengröße in m² *', f.gartengroesse), fld('Umlage abweichend in €', f.umlage, 'Nur ausfüllen, wenn dieser Pächter nicht die Standard-Umlage zahlt'),
    fld('Wasserzähler-Nr.', f.wz), fld('Stromzähler-Nr.', f.sz),
    fld('Notiz (nur intern, steht nicht auf der Rechnung – bitte keine Gesundheitsdaten oder sonstige sensible Angaben)', f.notiz, null, true));
  return modal(isNew ? 'Pächter anlegen' : `Pächter bearbeiten – ${cur.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const gg = parseNum(f.gartengroesse.value), um = parseNum(f.umlage.value);
      if (Number.isNaN(gg) || Number.isNaN(um) || (gg !== null && gg < 0) || (um !== null && um < 0)) { toast('Gartengröße und Umlage müssen Zahlen ab 0 sein.', 'err'); return false; }
      const data = { mitgliedsnr: f.mitgliedsnr.value, gartennr: f.gartennr.value, anrede: f.anrede.value, name: f.name.value,
        strasse: f.strasse.value, plzOrt: f.plzOrt.value, versand: f.versand.value, email: f.email.value.trim(), telefon: f.telefon.value, gartengroesse: gg ?? 0, umlageAbweichend: um,
        wasserzaehlerNr: f.wz.value, stromzaehlerNr: f.sz.value, notiz: f.notiz.value.trim() };
      if (data.versand === 'Emailsendung' && !data.email && !(await confirmBox('Versandart ist »Emailsendung«, aber es ist keine E-Mail-Adresse eingetragen. Trotzdem speichern?', 'Trotzdem speichern'))) return false;
      const gnr = data.gartennr.trim().toLowerCase();
      const gleich = gnr ? S.state.paechter.find((q) => q.id !== cur.id && (q.gartennr || '').trim().toLowerCase() === gnr) : null;
      if (gleich && !(await confirmBox(`Die Gartennummer ${data.gartennr.trim()} ist schon bei ${gleich.name} (${gleich.mitgliedsnr}) eingetragen. Trotzdem speichern?`, 'Trotzdem speichern'))) return false;
      try {
        if (isNew) await api('POST', '/api/admin/paechter', data);
        else await api('PUT', `/api/admin/paechter/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

function adminPaechter() {
  const st = S.state;
  const list = [...st.paechter].sort(cmpNr);
  // Spalte »Umlage« nur, wenn jemand eine abweichende Umlage zahlt (spart Platz im Programmfenster)
  const mitUmlage = list.some((p) => p.umlageAbweichend != null);
  const tbody = h('tbody');
  const fill = () => {
    tbody.replaceChildren();
    const q = S.filter.trim().toLowerCase();
    const shown = list.filter((p) => !q || [p.mitgliedsnr, p.name, p.gartennr, p.strasse, p.email, p.telefon].some((x) => (x || '').toLowerCase().includes(q)));
    for (const p of shown) {
      tbody.append(h('tr', null,
        h('td', null, p.mitgliedsnr), h('td', null, p.gartennr),
        h('td', { class: 'name', title: p.name }, p.name, p.notiz ? h('span', { title: p.notiz, style: 'margin-left:4px;cursor:help' }, '📝') : null, schlussBadge(p)),
        h('td', { class: 'wrap' }, [p.strasse, p.plzOrt].filter(Boolean).join(', ')),
        h('td', null, [p.telefon, p.email].filter(Boolean).map((x, i) => [i ? h('br') : null, x])),
        h('td', { class: 'num' }, nfFlex.format(p.gartengroesse) + ' m²'),
        mitUmlage ? h('td', { class: 'num' }, p.umlageAbweichend == null ? 'Standard' : eur(p.umlageAbweichend)) : null,
        h('td', { class: 'zaehler-zelle' }, p.wasserzaehlerNr ? h('div', null, h('span', { class: 'hint' }, 'W '), p.wasserzaehlerNr) : null,
          p.stromzaehlerNr ? h('div', null, h('span', { class: 'hint' }, 'S '), p.stromzaehlerNr) : null),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small', onclick: async () => { if (await paechterDialog(p)) await reload(); } }, 'Bearbeiten'), ' ',
          p.gartennr ? [h('button', { class: 'btn small', title: `Gartenakte von Garten ${p.gartennr}: Dokumente, Wertermittlungen, Übergaben, Verlauf`, onclick: () => gartenakteDialog(p.gartennr.trim()) }, 'Akte'), ' '] : null,
          mehrMenue([
            p.gartennr && !p.ausgeschiedenAm ? { label: 'Pächterwechsel …', title: 'Garten an einen neuen Pächter übergeben (Schlussrechnung, Übergabeprotokoll, neuer Pächter)', aktion: () => paechterwechselDialog(p) } : null,
            p.gartennr && !p.ausgeschiedenAm ? { label: 'Kündigung erfassen …', title: 'Kündigung mit Frist und Fahrplan bis zur Übergabe', aktion: async () => { if (await kuendigungDialog(p)) { adminTabWaehlen('kuendigungen'); render(); } } } : null,
            p.gartennr && p.gartengroesse > 0 ? { label: 'Pachtvertrag …', title: 'Unterpachtvertrag zum Unterschreiben, auf Wunsch in die Gartenakte', aktion: () => pachtvertragDialog(p) } : null,
            p.gartennr && !p.ausgeschiedenAm ? { label: 'Willkommensmappe', title: 'PDF für neue Pächter: Garten, Zähler, Termine, Lageplan, Ansprechpartner', aktion: () => willkommenDrucken(p) } : null,
            { label: 'Auskunft (DSGVO)', title: 'Alle gespeicherten Daten dieser Person als Datei herunterladen (Art. 15 DSGVO)', aktion: () => { location.href = `/api/admin/paechter/${p.id}/auskunft`; } },
            { label: 'In den Papierkorb', gefahr: true, aktion: async () => {
              const ok = await confirmBox(`Pächter „${p.name}“ (${p.mitgliedsnr}) in den Papierkorb legen? Er verschwindet aus allen Ansichten, Zählerstände und Rechnungen bleiben aber erhalten und lassen sich im Papierkorb wiederherstellen.`, 'In den Papierkorb', true);
              if (!ok) return;
              try { await api('DELETE', `/api/admin/paechter/${p.id}`); toast('In den Papierkorb gelegt', 'ok'); S.papierkorb = null; await reload(); } catch (e) { handleErr(e); }
            } }]))));
    }
    if (!shown.length) tbody.append(h('tr', null, h('td', { colspan: mitUmlage ? 9 : 8, class: 'empty' }, list.length ? 'Keine Treffer.'
      : leer('personen', 'Noch keine Pächter', 'Lege den ersten mit »+ Pächter anlegen« an oder übernimm alle auf einmal aus einer Excel- oder CSV-Liste.', 'Liste importieren', () => { adminTabWaehlen('daten'); render(); }))));
  };
  fill();
  return h('div', null,
    h('p', { class: 'hint' }, 'Hier pflegst du die Stammdaten der Pächter. Sie ändern sich selten und müssen nur einmal angelegt werden.'),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: async () => { if (await paechterDialog(null)) await reload(); } }, '+ Pächter anlegen'),
      h('input', { type: 'search', placeholder: 'Suchen', value: S.filter, style: 'width:220px', oninput: (e) => { S.filter = e.target.value; fill(); } }),
      h('span', { class: 'stat' }, `${list.length} Pächter`),
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', title: 'Liste aller Pächter mit Anschrift als PDF zum Ausdrucken', onclick: paechterlisteDrucken }, 'Pächterliste drucken')),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Garten', 'Name', 'Anschrift', 'Kontakt', 'Größe', mitUmlage ? 'Umlage' : null, 'Zähler', ''].filter((x) => x !== null).map((x) => h('th', null, x)))), tbody)),
    uebergabenCard(),
    papierkorbCard());
}

// Kleines Aufklappmenü »Mehr« für seltener gebrauchte Aktionen einer Tabellenzeile.
function mehrMenue(eintraege) {
  const liste = h('div', { class: 'mehr-liste', role: 'menu' });
  const knopf = h('button', { class: 'btn small mehr-knopf', 'aria-haspopup': 'menu', 'aria-expanded': 'false', title: 'Weitere Aktionen' }, 'Mehr ▾');
  const box = h('div', { class: 'mehr' }, knopf, liste);
  const platzieren = () => {
    // fest am Fenster ausrichten (die Tabelle scrollt und würde das Menü sonst abschneiden);
    // nach oben aufklappen, wenn unten kein Platz ist
    const r = knopf.getBoundingClientRect(), hoehe = liste.offsetHeight, breite = liste.offsetWidth;
    liste.style.left = Math.max(8, r.right - breite) + 'px';
    liste.style.top = (r.bottom + 4 + hoehe > window.innerHeight ? r.top - 4 - hoehe : r.bottom + 4) + 'px';
  };
  const zu = () => {
    box.classList.remove('offen'); knopf.setAttribute('aria-expanded', 'false');
    document.removeEventListener('click', aussen, true); window.removeEventListener('scroll', platzieren, true);
  };
  const aussen = (e) => { if (!box.contains(e.target)) zu(); };
  knopf.addEventListener('click', () => {
    if (box.classList.contains('offen')) { zu(); return; }
    document.querySelectorAll('.mehr.offen').forEach((m) => m.classList.remove('offen'));
    box.classList.add('offen'); knopf.setAttribute('aria-expanded', 'true');
    platzieren();
    document.addEventListener('click', aussen, true);
    window.addEventListener('scroll', platzieren, true);
  });
  for (const x of eintraege.filter(Boolean)) {
    liste.append(h('button', { class: 'mehr-eintrag' + (x.gefahr ? ' gefahr' : ''), role: 'menuitem', title: x.title || null,
      onclick: () => { zu(); x.aktion(); } }, x.label));
  }
  return box;
}

// Pächterliste (Garten, Name, Anschrift, Größe, Versandart, Zähler) zum Ausdrucken.
const paechterlisteDrucken = () => pdfDrucken({
  url: '/api/admin/paechterliste', oeffnenUrl: '/api/admin/paechterliste/oeffnen',
  titel: 'Pächterliste', meldung: () => 'Die Pächterliste wurde im PDF-Programm geöffnet – dort drucken.',
});

function auskunftLink(p) {
  return h('a', { class: 'btn small', href: `/api/admin/paechter/${p.id}/auskunft`,
    title: 'Alle gespeicherten Daten dieser Person als Datei herunterladen (Auskunft nach Art. 15 DSGVO)' }, 'Auskunft');
}

// ---- Datenschutz

async function loadDatenschutz() {
  if (S.datenschutzLoading) return;
  S.datenschutzLoading = true;
  try { S.datenschutz = await api('GET', '/api/admin/datenschutz'); } catch (e) { handleErr(e); S.datenschutz = false; }
  S.datenschutzLoading = false;
  render();
}

function adminDatenschutz() {
  const st = S.state;
  if (S.datenschutz == null) { if (!S.datenschutzLoading) loadDatenschutz(); }
  const d = S.datenschutz || null;
  const list = [...st.paechter].sort(cmpNr);
  const sel = h('select', { style: 'min-width:260px' }, list.map((p) => h('option', { value: p.id }, `${p.mitgliedsnr}  ${p.name}`)));
  const bereinigen = async () => {
    const ok = await confirmBox(`Bei ${d.faelligRechnungen} Rechnung(en) und ${d.faelligJahre} Jahresunterlage(n) ist die Aufbewahrungsfrist (${d.aufbewahrungJahre} Jahre) abgelaufen. Name und Anschrift werden dort entfernt, die PDF-Dateien gelöscht, auch in den Sicherungen. Die Beträge bleiben für die Statistik. Das kann nicht rückgängig gemacht werden.`, 'Jetzt bereinigen', true);
    if (!ok) return;
    try {
      const r = await api('POST', '/api/admin/datenschutz/bereinigen');
      toast(`Bereinigt: ${r.rechnungen} Rechnung(en), ${r.jahre} Jahresunterlage(n), ${r.dateien} PDF-Datei(en), ${r.sicherungen} Sicherung(en)`, 'ok');
      S.datenschutz = null; S.archivAlle = null; await reload();
    } catch (e) { handleErr(e); }
  };
  const li = (...kids) => h('li', null, ...kids);
  return h('div', null,
    h('p', { class: 'hint' }, 'Hilfen für den Umgang mit personenbezogenen Daten nach der DSGVO. Für die Einhaltung ist der Verein verantwortlich; das Programm unterstützt dabei, ersetzt aber keine Rechtsberatung.'),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Was wird wo gespeichert?'),
      h('ul', { style: 'margin:0;padding-left:20px;line-height:1.6' },
        li(h('b', null, 'Alles liegt nur auf diesem Rechner'), ' (Datenordner: ', h('span', { class: 'mono' }, st.dataDir), '). Es gibt keine Cloud und keine Übertragung ins Internet.'),
        li('Pächter: Name, Anschrift, Mitgliedsnummer, Garten, Zählernummern, interne Notiz; dazu Zählerstände, Arbeitsstunden, Rechnungen, Zahlungen, Übergabeprotokolle.'),
        li('Warteliste: Name, Anschrift und Kontakt der Interessenten. Wer kein Interesse mehr hat, wird gelöscht (auch aus den Sicherungen).'),
        li('Kopien davon stecken in ausgestellten Rechnungen (Archiv und PDF-Dateien), abgeschlossenen Jahren, der Garten-Historie, dem Änderungsprotokoll und den automatischen Sicherungen.'),
        li('Einzige Verbindung nach außen: der Knopf »Nach Updates suchen« (nur auf Klick, überträgt dabei die IP-Adresse an GitHub).'))),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Auskunft und Datenkopie (Art. 15 und 20 DSGVO)'),
      h('p', { class: 'hint' }, 'Wer wissen möchte, was über sie oder ihn gespeichert ist, bekommt hier alle Daten als Datei (maschinenlesbar, JSON). Dieselbe Datei gibt es auch je Person in der Pächter-Liste (»Auskunft«) und im Papierkorb – am besten erstellst du sie, bevor du jemanden löschst.'),
      list.length
        ? h('div', { class: 'toolbar' }, sel, h('a', { class: 'btn primary', href: '#', onclick: (e) => { e.preventDefault(); location.href = `/api/admin/paechter/${sel.value}/auskunft`; } }, 'Auskunft herunterladen'))
        : h('p', { class: 'hint' }, 'Noch keine Pächter angelegt.')),

    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('h3', { style: 'margin-top:0' }, 'Löschen und Aufbewahrungsfristen (Art. 17 DSGVO)'),
      h('p', { class: 'hint' }, 'Ein Pächter, der nicht mehr dabei ist, kommt zuerst in den Papierkorb. Dort kannst du ihn »endgültig löschen«: Dabei werden Stammdaten, Notizen, offene Zählerstände und sein Name im Änderungsprotokoll, in der Garten-Historie und in allen Sicherungen entfernt. Rechnungen und Jahresunterlagen müssen aus steuerlichen Gründen aber noch eine Zeit lang bleiben. Diese Frist beträgt hier ' + (d ? d.aufbewahrungJahre : 10) + ' Jahre ab Ende des Ausstellungsjahres (die längere der üblichen Fristen; ob bei euch 8 oder 10 Jahre genügen, klärt ihr mit Steuerberatung oder Kassenprüfern). Danach entfernst du hier den Personenbezug.'),
      !d ? h('p', { class: 'hint' }, d === false ? 'Konnte nicht geladen werden.' : 'Wird geladen …') : h('div', null,
        h('ul', { style: 'margin:0 0 10px;padding-left:20px;line-height:1.6' },
          li(`Im Papierkorb wartend: ${d.papierkorb} Pächter`),
          d.wartelisteNachfragen ? li(h('b', null, `Warteliste: ${d.wartelisteNachfragen} Eintrag/Einträge seit über 2 Jahren nicht bestätigt – nachfragen oder löschen (unter »Warteliste«).`)) : null,
          li(d.faelligRechnungen + d.faelligJahre > 0
            ? h('b', null, `Aufbewahrungsfrist abgelaufen: ${d.faelligRechnungen} Rechnung(en), ${d.faelligJahre} Jahresunterlage(n) (ausgestellt bis ${d.bereinigtBisJahr})`)
            : 'Keine Unterlagen mit abgelaufener Aufbewahrungsfrist.'),
          d.naechsteFrist ? li(`Die nächsten Rechnungen werden ${d.naechsteFrist} freigegeben.`) : null),
        h('button', { class: 'btn danger', disabled: d.faelligRechnungen + d.faelligJahre === 0, onclick: bereinigen }, 'Abgelaufene Unterlagen bereinigen'))),

    h('div', { class: 'card' },
      h('h3', { style: 'margin-top:0' }, 'Was der Verein selbst erledigen muss'),
      h('ul', { style: 'margin:0;padding-left:20px;line-height:1.6' },
        li('Verzeichnis der Verarbeitungstätigkeiten führen und die Mitglieder über die Verarbeitung informieren (Vorlagen: Datei ', h('span', { class: 'mono' }, 'DATENSCHUTZ.md'), ' im Projekt).'),
        li('Admin-Passwort setzen (Reiter »Passwort«) und die Festplatte verschlüsseln (z. B. BitLocker, FileVault); die Datendatei selbst ist nicht verschlüsselt.'),
        li('Sicherungen (auch den zweiten Sicherungsordner, USB-Stick) sicher aufbewahren; sie enthalten dieselben Daten.'),
        li('Nur nötige Angaben erfassen, keine Gesundheitsdaten oder sonstigen sensiblen Angaben in Notizfeldern.'))));
}

async function loadPapierkorb() {
  if (S.papierkorbLoading) return;
  S.papierkorbLoading = true;
  try { S.papierkorb = await api('GET', '/api/admin/paechter-papierkorb'); } catch (e) { handleErr(e); }
  S.papierkorbLoading = false;
  render();
}

function papierkorbCard() {
  if (!S.papierkorbOffen) {
    return h('div', { style: 'margin-top:14px' },
      h('button', { class: 'btn', onclick: () => { S.papierkorbOffen = true; S.papierkorb = null; render(); } }, '🗑 Papierkorb'));
  }
  if (!S.papierkorb) {
    if (!S.papierkorbLoading) loadPapierkorb();
    return h('div', { class: 'card', style: 'margin-top:14px' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const rows = S.papierkorb.map((p) => h('tr', null,
    h('td', null, p.mitgliedsnr), h('td', null, p.name), h('td', null, deDate(p.geloeschtAm)),
    h('td', null,
      h('button', { class: 'btn small primary', onclick: async () => {
        try { await api('POST', `/api/admin/paechter/${p.id}/wiederherstellen`); toast('Wiederhergestellt', 'ok'); S.papierkorb = null; await reload(); }
        catch (e) { handleErr(e); }
      } }, 'Wiederherstellen'), ' ',
      auskunftLink(p), ' ',
      h('button', { class: 'btn small danger', onclick: async () => {
        const ok = await confirmBox(`Pächter „${p.name}“ (${p.mitgliedsnr}) endgültig löschen? Das kann nicht rückgängig gemacht werden. Stammdaten, Notizen, offene Zählerstände sowie der Name im Änderungsprotokoll, in der Garten-Historie und in allen Sicherungen werden entfernt. Bereits ausgestellte Rechnungen müssen wegen der gesetzlichen Aufbewahrungspflicht noch 10 Jahre (ab Ende des Ausstellungsjahres) bleiben und verlieren ihren Namen erst danach unter Admin → Datenschutz.`, 'Endgültig löschen', true);
        if (!ok) return;
        try { await api('DELETE', `/api/admin/paechter/${p.id}/endgueltig`); toast('Endgültig gelöscht', 'ok'); S.papierkorb = null; render(); await loadPapierkorb(); }
        catch (e) { handleErr(e); }
      } }, 'Endgültig löschen'))));
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'toolbar' }, h('b', null, 'Papierkorb'), h('div', { class: 'spacer' }),
      h('button', { class: 'btn small', onclick: () => { S.papierkorbOffen = false; render(); } }, 'Schließen')),
    S.papierkorb.length
      ? h('div', { class: 'tablewrap', style: 'margin-top:10px' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Gelöscht am', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows)))
      : h('p', { class: 'hint', style: 'margin:10px 0 0' }, 'Papierkorb ist leer.'));
}

// ---- Einstellungen

const SETTINGS_FORM = [
  ['Verein und Rechnung', [
    ['vereinName', 'Vereinsname', 'text', '', true], ['ort', 'PLZ und Ort', 'text'], ['absenderzeile', 'Absenderzeile (kleine Zeile über der Anschrift)', 'text', '', true],
    ['rechnungsdatum', 'Rechnungsdatum', 'date'], ['zahlungsziel', 'Zahlbar bis', 'date'],
    ['rechnungsnrPraefix', 'Rechnungsnummer-Vorsatz', 'text', 'Rechnungsnr. = Vorsatz + Mitgliedsnr., z. B. 100-35-95'],
    ['einspruchTage', 'Einspruchsfrist', 'num', 'Tage']]],
  ['Bankverbindung', [['bankName', 'Bank', 'text'], ['iban', 'IBAN', 'text'], ['bic', 'BIC', 'text']]],
  ['Wasser', [['wasserGrundpreis', 'Grundpreis pro Jahr', 'num', '€'], ['wasserPreis', 'Preis je m³', 'num', '€']]],
  ['Strom / Energie', [['energieGrundpreis', 'Grundpreis pro Jahr', 'num', '€'], ['energiePreis', 'Preis je kWh', 'num', '€']]],
  ['Arbeitsstunden', [
    ['pflichtstunden', 'Pflichtstunden', 'num', 'Stunden'], ['stundenObergrenze', 'Vergütung bis zu dieser Stundenzahl', 'num', 'Stunden'],
    ['verguetungJeStd', 'Vergütung je Stunde über der Pflicht', 'num', '€'], ['nachzahlungJeStd', 'Nachzahlung je fehlende Stunde', 'num', '€']]],
  ['Pacht und Beiträge (für das Folgejahr)', [
    ['pachtJeQm', 'Pacht je m²', 'num', '€'], ['vereinsflaecheQm', 'Anteil Vereinsfläche', 'num', 'm²'], ['freieGaertenQm', 'Pachtanteil freie Gärten', 'num', 'm²'],
    ['vereinsbeitrag', 'Vereinsbeitrag', 'num', '€'], ['territorialverband', 'Territorialverband', 'num', '€'], ['umlageStandard', 'Umlage (Standard für alle)', 'num', '€']]],
];

function adminSettings() {
  const st = S.state;
  const inputs = {};
  const form = h('div', { class: 'form' });
  for (const [title, fields] of SETTINGS_FORM) {
    form.append(h('div', { class: 'section-title' }, title));
    for (const [key, label, type, hint, wide] of fields) {
      const val = st.settings[key];
      const input = type === 'date' ? h('input', { type: 'date', value: val })
        : type === 'num' ? h('input', { type: 'text', inputmode: 'decimal', value: numIn(val), autocomplete: 'off' })
          : h('input', { type: 'text', value: val, maxlength: 250, autocomplete: 'off' });
      inputs[key] = { input, type };
      const unit = type === 'num' ? hint : null;
      form.append(h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label),
        unit ? h('div', { class: 'inputunit' }, input, h('span', null, unit)) : input,
        type !== 'num' && hint ? h('span', { class: 'fhint' }, hint) : null));
    }
  }
  const save = async () => {
    // alle übrigen Einstellungen (zweiter Sicherungsordner, Logo, Farbe) unverändert mitschicken, sonst würden sie geleert
    const data = { ...st.settings, jahr: st.settings.jahr };
    for (const [key, { input, type }] of Object.entries(inputs)) {
      if (type === 'num') {
        const v = parseNum(input.value);
        if (v === null || Number.isNaN(v) || v < 0) { input.classList.add('err'); toast('Bitte in allen Zahlenfeldern eine Zahl ab 0 eintragen.', 'err'); return; }
        input.classList.remove('err');
        data[key] = v;
      } else data[key] = input.value;
    }
    data.einspruchTage = Math.round(data.einspruchTage);
    try { await api('PUT', '/api/admin/settings', data); toast('Einstellungen gespeichert', 'ok'); await reload(); } catch (e) { handleErr(e); }
  };
  return h('div', null,
    aussehenKarte(st),
    h('p', { class: 'hint' }, `Diese Werte gelten für das laufende Abrechnungsjahr ${st.settings.jahr} und für alle Rechnungen. Nach einer Änderung rechnen alle Beträge automatisch neu.`),
    h('div', { class: 'card' }, form, h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: save }, 'Speichern'))),
    pachtgrenzeKarte());
}

// Pachtobergrenze nach § 5 BKleingG: höchstens das Vierfache der ortsüblichen Pacht im Erwerbsobstbau.
function pachtgrenzeKarte() {
  if (S.pachtgrenze == null) {
    if (!S.pachtgrenzeLaedt) {
      S.pachtgrenzeLaedt = true;
      api('GET', '/api/admin/pachtgrenze').then((d) => { S.pachtgrenze = d; }).catch(() => { S.pachtgrenze = false; }).finally(() => { S.pachtgrenzeLaedt = false; render(); });
    }
    return null;
  }
  const d = S.pachtgrenze;
  if (!d) return null;
  const wert = h('input', { type: 'text', inputmode: 'decimal', value: d.erwerbsobstbauJeQm ? numIn(d.erwerbsobstbauJeQm) : '', placeholder: 'z. B. 0,03', autocomplete: 'off' });
  const quelle = h('input', { type: 'text', maxlength: 120, value: d.quelle || '', placeholder: 'z. B. Gutachterausschuss 2025' });
  const speichern = async () => {
    const v = wert.value.trim() === '' ? 0 : parseNum(wert.value);
    if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte eine Zahl ab 0 eintragen.', 'err'); return; }
    try { S.pachtgrenze = await api('PUT', '/api/admin/pachtgrenze', { erwerbsobstbauJeQm: v, quelle: quelle.value }); S.aufgaben = null; toast('Gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };
  const fmt4 = (x) => new Intl.NumberFormat('de-DE', { maximumFractionDigits: 4 }).format(x);
  let ergebnis = h('p', { class: 'hint' }, 'Trag die ortsübliche Pacht ein – das Programm rechnet die Höchstgrenze aus und vergleicht sie mit eurem Pachtsatz.');
  if (d.hoechst != null) {
    const pct = Math.min(100, d.auslastung);
    ergebnis = h('div', { class: 'pg-ergebnis' },
      h('div', { class: 'pg-zahlen' }, h('span', null, 'Euer Pachtsatz ', h('b', null, fmt4(d.pachtJeQm) + ' €/m²')), h('span', null, 'Höchstpacht ', h('b', null, fmt4(d.hoechst) + ' €/m²'))),
      h('div', { class: 'balken' + (d.ueber ? ' err' : '') }, h('div', { style: `width:${pct}%` })),
      h('p', { class: d.ueber ? 'warn-text' : 'hint' }, d.ueber
        ? `Der Pachtsatz liegt ${nfFlex.format(Math.round((d.auslastung - 100) * 10) / 10)} % über der gesetzlichen Höchstgrenze. Bitte mit dem Verband klären.`
        : `Ihr nutzt ${nfFlex.format(d.auslastung)} % der zulässigen Pacht – bis zur Grenze sind noch ${fmt4(Math.round((d.hoechst - d.pachtJeQm) * 10000) / 10000)} €/m² Luft.`));
  }
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'section-title', style: 'margin-top:0' }, 'Pachtobergrenze (§ 5 Bundeskleingartengesetz)'),
    h('p', { class: 'hint' }, 'Die Pacht darf höchstens das Vierfache der ortsüblichen Pacht im erwerbsmäßigen Obst- und Gemüseanbau betragen. Den Wert nennt der Gutachterausschuss der Gemeinde oder der Verband. Gemeinschaftsflächen dürfen anteilig mit umgelegt werden.'),
    h('div', { class: 'form' },
      h('div', { class: 'field' }, h('label', null, 'Ortsübliche Pacht Erwerbsobstbau'), h('div', { class: 'inputunit' }, wert, h('span', null, '€/m²'))),
      h('div', { class: 'field' }, h('label', null, 'Quelle'), quelle)),
    h('div', { class: 'actions' }, h('button', { class: 'btn', onclick: speichern }, 'Übernehmen')),
    ergebnis);
}

// ---- Aussehen: Vereinsfarbe und Logo (Programm und Rechnung)

const FARBVORSCHLAEGE = [['#2f6f3e', 'Gartengrün'], ['#1e5aa8', 'Blau'], ['#0f6e6e', 'Petrol'], ['#8c2f39', 'Weinrot'], ['#7a4b1f', 'Erdbraun'], ['#3d4752', 'Anthrazit']];

function aussehenKarte(st) {
  const aktuell = st.settings.farbe || '#2f6f3e';
  const speichereFarbe = async (hex) => {
    try { await api('PUT', '/api/admin/settings', { ...st.settings, farbe: hex === '#2f6f3e' ? '' : hex }); toast('Vereinsfarbe gespeichert', 'ok'); await reload(); } catch (e) { handleErr(e); }
  };
  const waehler = h('input', { type: 'color', value: aktuell, title: 'Eigene Farbe wählen',
    oninput: (e) => farbeAnwenden(e.target.value), onchange: (e) => speichereFarbe(e.target.value) });
  const datei = h('input', { type: 'file', accept: '.png,.jpg,.jpeg', style: 'display:none', onchange: async () => {
    if (!datei.files.length) return;
    const fd = new FormData(); fd.append('file', datei.files[0]);
    try { await api('POST', '/api/admin/logo', fd, true); toast('Logo gespeichert – es erscheint auf allen neuen Rechnungen', 'ok'); await reload(); } catch (e) { handleErr(e); }
  } });
  return h('div', { class: 'card', style: 'margin-bottom:16px' },
    h('h3', null, 'Aussehen: Vereinsfarbe und Logo'),
    h('p', { class: 'hint' }, 'Farbe und Logo erscheinen im Programm und auf allen neu erzeugten Rechnungen und Mahnungen. Bereits ausgestellte Rechnungen bleiben, wie sie sind. Zu helle Farben werden automatisch so abgedunkelt, dass alles gut lesbar bleibt.'),
    h('div', { class: 'aussehen' },
      h('div', null, h('div', { class: 'form' }, h('label', null, 'Vereinsfarbe')),
        h('div', { class: 'farbwahl', style: 'margin-top:6px' },
          FARBVORSCHLAEGE.map(([hex, name]) => h('button', { class: 'swatch' + (hex === aktuell ? ' an' : ''), title: name, 'aria-label': name,
            style: `background:${hex}`, onclick: () => speichereFarbe(hex) })),
          waehler, h('span', { class: 'hint' }, 'eigene Farbe'))),
      h('div', null, h('div', { class: 'form' }, h('label', null, 'Vereinslogo')),
        h('div', { class: 'toolbar', style: 'margin-top:6px' },
          h('div', { class: 'logovorschau' }, st.settings.logo ? h('img', { src: `/api/logo?v=${encodeURIComponent(st.settings.logo)}`, alt: 'Vereinslogo' }) : h('span', { class: 'hint' }, 'kein Logo')),
          h('div', null, datei,
            h('button', { class: 'btn', onclick: () => datei.click() }, st.settings.logo ? 'Anderes Logo wählen' : 'Logo hochladen'), ' ',
            st.settings.logo ? h('button', { class: 'btn danger', onclick: async () => {
              try { await api('DELETE', '/api/admin/logo'); toast('Logo entfernt', 'ok'); await reload(); } catch (e) { handleErr(e); }
            } }, 'Entfernen') : null,
            h('div', { class: 'hint', style: 'margin-top:6px' }, 'PNG oder JPG, am besten mit hellem oder durchsichtigem Hintergrund.'))))));
}

// ---- Jahreswechsel

async function loadAbschlussCheck(year) {
  if (S.abschlussLoading) return;
  S.abschlussLoading = true;
  try { S.abschlussCheck = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { S.abschlussCheck = false; }
  S.abschlussLoading = false;
  render();
}

// Geführter Überblick vor dem Jahreswechsel: zeigt, was noch offen ist. Ist
// nur ein Hinweis, blockiert den Jahreswechsel nicht (wie die übrigen
// Plausibilitätshinweise im Programm auch).
function abschlussChecklist(st) {
  if (S.abschlussCheck == null || (S.abschlussCheck && S.abschlussCheck.jahr !== st.currentYear)) {
    if (!S.abschlussLoading) loadAbschlussCheck(st.currentYear);
  }
  const total = st.paechter.length;
  const done = st.paechter.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  const issued = Object.keys(st.issued || {}).length;
  const offenePosten = (st.archive || []).filter((e) => e.status === 'gueltig' && e.gesamt >= 0 && payIsOpen(e)).length;
  const k = S.abschlussCheck || null;
  const ungeprueft = k ? k.ausgaben.filter((x) => !x.geprueft).length : null;

  const item = (ok, label) => h('li', { class: ok ? 'ok' : 'warn' }, ok ? '✓ ' : '⚠ ', label);
  const items = [
    item(done === total, `Zählerstände vollständig: ${done} von ${total} Pächtern`),
    item(issued >= done, `Rechnungen ausgestellt: ${issued} von ${done} vollständigen Pächtern`),
    item(offenePosten === 0, offenePosten === 0 ? 'Keine offenen Zahlungen mehr' : `${offenePosten} Rechnung(en) noch offen`),
  ];
  if (k) {
    items.push(item(ungeprueft === 0, ungeprueft === 0 ? 'Alle sonstigen Ausgaben sind geprüft' : `${ungeprueft} sonstige Ausgabe(n) noch ohne Geprüft-Haken`));
    items.push(item(k.kassenbestand >= 0, k.kassenbestand >= 0 ? `Kassenbestand positiv (${eur(k.kassenbestand)})` : `Kassenbestand negativ (${eur(k.kassenbestand)})`));
    const wk = k.ausgaben.filter((x) => x.wiederkehrend).length;
    if (wk > 0) items.push(h('li', null, '🔁 ', `${wk} wiederkehrende Ausgabe(n) werden automatisch als Vorschlag fürs neue Jahr übernommen`));
  }
  return h('div', { class: 'card narrow', style: 'max-width:760px;margin-bottom:16px' },
    h('h3', { style: 'margin-top:0' }, 'Vor dem Abschluss prüfen'),
    h('ul', { class: 'checklist' }, items),
    h('p', { class: 'hint', style: 'margin-bottom:0' }, 'Nur ein Hinweis – der Jahreswechsel lässt sich trotzdem durchführen, offene Punkte bleiben danach weiter bearbeitbar.'));
}

function adminYear() {
  const st = S.state;
  const total = st.paechter.length;
  const done = st.paechter.filter((p) => st.results[p.id] && st.results[p.id].vollstaendig).length;
  return h('div', null,
    abschlussChecklist(st),
    h('div', { class: 'card narrow', style: 'max-width:760px' },
      h('h3', { style: 'margin-top:0' }, `Abrechnungsjahr ${st.currentYear} abschließen`),
      h('p', null, 'Am Ende der Abrechnung schließt du das Jahr ab. Dabei passiert Folgendes:'),
      h('ul', { class: 'plain' },
        h('li', null, `Das Jahr ${st.currentYear} wird eingefroren. Preise und Pächterdaten bleiben so gespeichert, dass du alte Rechnungen jederzeit unverändert neu drucken kannst.`),
        h('li', null, `Es startet das Jahr ${st.currentYear + 1}: Die aktuellen Zählerstände werden zu den Vorjahresständen. Stunden, Abschlagszahlungen und weitere Angaben sind wieder leer.`),
        h('li', null, 'Rechnungsdatum und Zahlungsziel rücken um ein Jahr weiter. Prüfe sie danach unter »Preise & Einstellungen«.'),
        h('li', null, 'Vorher wird automatisch eine Sicherung angelegt.')),
      done < total ? h('div', { class: 'banner warn', style: 'margin-top:12px' }, `Achtung: Bei ${total - done} von ${total} Pächtern fehlen noch Angaben.`) : null,
      h('p', { class: 'hint', style: 'margin-top:12px' }, 'Tipp: Vor dem Abschluss den ',
        h('button', { class: 'btn small', onclick: () => jahresberichtDrucken(st.currentYear) }, `Jahresbericht ${st.currentYear}`),
        ' für die Mitgliederversammlung drucken (später geht das auch noch im Kassenbericht).'),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: async () => {
        const offen = [];
        if (done < total) offen.push(`bei ${total - done} Pächter(n) fehlen Angaben`);
        const nIssued = Object.keys(st.issued || {}).length;
        if (nIssued < done) offen.push(`${done - nIssued} Rechnung(en) sind noch nicht ausgestellt`);
        const nOffen = offenePosten(st).jetzt.length;
        if (nOffen) offen.push(`${nOffen} Rechnung(en) sind noch nicht bezahlt`);
        const warnung = offen.length ? ` Achtung: ${offen.join(', ')}.` : '';
        if (!(await confirmBox(`Jahr ${st.currentYear} jetzt abschließen und ${st.currentYear + 1} beginnen?${warnung} Das kann nicht rückgängig gemacht werden (die Sicherung vorher bleibt aber erhalten).`, 'Jahr abschließen', true))) return;
        try { await api('POST', '/api/admin/jahreswechsel'); toast('Neues Jahr gestartet', 'ok'); S.year = null; S.dashKasse = null; S.kasse = null; S.kasseVerlauf = null; S.abschlussCheck = null; await reload(); } catch (e) { handleErr(e); }
      } }, `Jahr ${st.currentYear} abschließen`))));
}

// ---- Kassenbericht

async function loadKasse(year) {
  if (S.kasseLoading) return;
  S.kasseLoading = true;
  try { S.kasse = await api('GET', `/api/admin/kassenbericht?year=${year}`); } catch (e) { handleErr(e); }
  S.kasseLoading = false;
  render();
}

async function refreshKasse() { S.kasse = null; S.kasseVerlauf = null; await loadKasse(S.kasseYear); }

async function loadKasseVerlauf() {
  if (S.kasseVerlaufLoading) return;
  S.kasseVerlaufLoading = true;
  try { S.kasseVerlauf = await api('GET', '/api/admin/kassenbericht-verlauf'); } catch (e) { handleErr(e); }
  S.kasseVerlaufLoading = false;
  render();
}

const AUSGABEN_KATEGORIEN = ['Instandhaltung', 'Anschaffung', 'Verwaltung', 'Versicherung & Gebühren', 'Sonstiges'];

function ausgabeDialog(a) {
  const isNew = !a;
  const datum = h('input', { type: 'date', value: (a && a.datum) || todayIso() });
  const besch = h('input', { type: 'text', maxlength: 120, value: (a && a.beschreibung) || '', autocomplete: 'off' });
  const kategorie = h('input', { type: 'text', list: 'kategorien-liste', maxlength: 40, value: (a && a.kategorie) || '', placeholder: AUSGABEN_KATEGORIEN[AUSGABEN_KATEGORIEN.length - 1], autocomplete: 'off' });
  const katList = h('datalist', { id: 'kategorien-liste' }, AUSGABEN_KATEGORIEN.map((x) => h('option', { value: x })));
  const betrag = h('input', { type: 'text', inputmode: 'decimal', value: a ? numIn(a.betrag) : '', autocomplete: 'off' });
  const wiederkehrend = h('input', { type: 'checkbox', checked: !!(a && a.wiederkehrend) });
  const fileInput = h('input', { type: 'file', accept: '.jpg,.jpeg,.png,.webp,.pdf', multiple: true });
  const fld = (label, input) => h('div', { class: 'field' }, h('label', null, label), input);

  // Vorhandene Belege: Liste mit eigenem Entfernen-Knopf je Beleg, aktualisiert
  // sich selbst ohne den Dialog zu schließen.
  const belegeBox = h('div', { style: 'display:flex;flex-wrap:wrap;gap:8px;margin:4px 0' });
  let belegeAktuell = (a && a.belege) || [];
  const zeichneBelege = () => {
    belegeBox.replaceChildren();
    if (!belegeAktuell.length) { belegeBox.append(h('span', { class: 'hint' }, 'Noch keine Belege hinterlegt.')); return; }
    belegeAktuell.forEach((_, i) => {
      belegeBox.append(h('span', { class: 'chip' },
        h('a', { href: `/api/admin/beleg/${a.id}?year=${S.kasseYear}&idx=${i}`, target: '_blank' }, `Beleg ${i + 1}`), ' ',
        h('button', { class: 'btn small danger', type: 'button', title: 'Beleg entfernen', onclick: async () => {
          const ok = await confirmBox(`Beleg ${i + 1} wirklich entfernen?`, 'Entfernen', true);
          if (!ok) return;
          try {
            await api('DELETE', `/api/admin/ausgaben/${a.id}/beleg?year=${S.kasseYear}&idx=${i}`);
            belegeAktuell = belegeAktuell.filter((_, j) => j !== i);
            zeichneBelege();
            // Tabelle hinter dem Dialog sofort aktualisieren, damit sie nicht
            // veraltet bleibt, falls danach "Abbrechen" statt "Speichern" kommt
            // (das Entfernen ist bereits endgültig passiert, nicht Teil des
            // Speichern-Schritts).
            await refreshKasse();
          } catch (e) { handleErr(e); }
        } }, '×')));
    });
  };
  if (!isNew) zeichneBelege();

  const body = h('div', { class: 'form' },
    fld('Datum', datum), fld('Beschreibung (z. B. Rasenmäher, Kontoführungsgebühren)', besch),
    fld('Kategorie', h('div', null, kategorie, katList)),
    h('div', { class: 'field' }, h('label', null, 'Betrag'), h('div', { class: 'inputunit' }, betrag, h('span', null, '€'))),
    h('div', { class: 'field wide', style: 'border:none' },
      h('label', { style: 'display:flex;align-items:center;gap:6px;font-weight:normal' }, wiederkehrend, 'Jährlich wiederkehrend'),
      h('span', { class: 'fhint' }, 'Wird beim Jahreswechsel automatisch als Vorschlag fürs neue Jahr angelegt (gleiche Beschreibung, Kategorie und Betrag, ungeprüft, ohne Beleg) – z. B. für Kontoführungsgebühren. Lässt sich dort weiterhin anpassen oder löschen.')),
    h('div', { class: 'field wide' }, h('label', null, 'Belege (Fotos oder PDF, z. B. Vorder- und Rückseite, optional)'),
      isNew ? null : belegeBox,
      fileInput,
      isNew ? h('span', { class: 'fhint' }, 'Werden nach dem Anlegen hochgeladen.') : null));
  return modal(isNew ? 'Ausgabe erfassen' : `Ausgabe bearbeiten – ${a.beschreibung}`, body, [
    { label: 'Abbrechen', value: false },
    isNew ? null : { label: 'Löschen', cls: 'danger', value: 'del', action: async () => {
      const ok = await confirmBox(`Ausgabe „${a.beschreibung}“ (${eur(a.betrag)}) wirklich löschen?`, 'Löschen', true);
      if (!ok) return false;
      try { await api('DELETE', `/api/admin/ausgaben/${a.id}?year=${S.kasseYear}`); } catch (e) { handleErr(e); return false; }
    } },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const b = parseNum(betrag.value);
      if (!datum.value) { toast('Bitte ein Datum angeben.', 'err'); return false; }
      if (!besch.value.trim()) { toast('Bitte eine Beschreibung eintragen.', 'err'); return false; }
      if (b === null || Number.isNaN(b) || b <= 0) { toast('Bitte einen Betrag größer 0 eintragen.', 'err'); return false; }
      const data = { datum: datum.value, beschreibung: besch.value.trim(), kategorie: kategorie.value.trim(), betrag: b, wiederkehrend: wiederkehrend.checked };
      try {
        const saved = isNew ? await api('POST', `/api/admin/ausgaben?year=${S.kasseYear}`, data) : await api('PUT', `/api/admin/ausgaben/${a.id}?year=${S.kasseYear}`, data);
        for (const file of fileInput.files) {
          const fd = new FormData();
          fd.append('file', file);
          await api('POST', `/api/admin/ausgaben/${saved.id}/beleg?year=${S.kasseYear}`, fd, true);
        }
      } catch (e) { handleErr(e); return false; }
    } },
  ].filter(Boolean));
}

// Hinweis, wenn sich der Kassenbestand des Vorjahres nach dem Jahreswechsel noch geändert hat (z. B. eine
// Vorjahresrechnung wurde erst jetzt bezahlt): sonst fehlt dieses Geld im Kassenbestand des laufenden Jahres.
function anfangsbestandHinweis(k, pruefMode) {
  const vj = k.vorjahrEndbestand;
  if (vj == null || Math.abs(vj - k.anfangsbestand) < 0.005) return null;
  const text = `Der Kassenbestand Ende ${k.jahr - 1} beträgt inzwischen ${eur(vj)}, der Anfangsbestand ${k.jahr} ist ${eur(k.anfangsbestand)} (Unterschied ${eur(vj - k.anfangsbestand)}). Das passiert zum Beispiel, wenn eine Vorjahresrechnung erst nach dem Jahreswechsel bezahlt oder eine Ausgabe nachgetragen wurde.`;
  if (pruefMode || vj < 0) return h('div', { class: 'banner warn' }, text);
  return h('div', { class: 'banner warn' }, text, ' ',
    h('button', { class: 'btn small', onclick: async () => {
      try { S.kasse = await api('PUT', `/api/admin/anfangsbestand?year=${k.jahr}`, { betrag: vj }); S.kasseVerlauf = null; S.dashKasse = null; toast(`Anfangsbestand auf ${eur(vj)} gesetzt`, 'ok'); render(); } catch (e) { handleErr(e); }
    } }, `Anfangsbestand auf ${eur(vj)} setzen`));
}

// ---- Kassenprüfung: Status-Karte im Kassenbericht und geführter Dialog

function pruefungKarte() {
  const jahr = S.kasseYear;
  if (!S.pruefung || S.pruefung.jahr !== jahr) {
    if (!S.pruefungLaedt) {
      S.pruefungLaedt = true;
      api('GET', `/api/admin/kassenpruefung?year=${jahr}`).then((d) => { S.pruefung = d; }).catch(() => { S.pruefung = { jahr, fehler: true }; })
        .finally(() => { S.pruefungLaedt = false; render(); });
    }
    return null;
  }
  const d = S.pruefung;
  if (d.fehler) return null;
  const erledigt = d.punkte.filter((p) => d.pruefung.punkte[p.key]).length;
  const status = d.pruefung.ergebnis === 'ohne' ? ['ok', 'ohne Beanstandungen abgeschlossen'] : d.pruefung.ergebnis === 'mit' ? ['warn', 'mit Beanstandungen abgeschlossen']
    : erledigt ? ['neu', `${erledigt} von ${d.punkte.length} Punkten geprüft`] : ['', 'noch nicht begonnen'];
  return h('div', { class: 'card pruef-karte' },
    h('div', null, h('b', null, `Kassenprüfung ${jahr}`), ' ', h('span', { class: 'badge ' + status[0] }, status[1]),
      h('div', { class: 'hint' }, 'Geführte Prüfung: Kontostand abgleichen, Belege und Stichprobe prüfen – am Ende steht der Bericht für die Mitgliederversammlung.')),
    h('div', { class: 'spacer' }),
    h('button', { class: 'btn primary', onclick: pruefungDialog }, erledigt || d.pruefung.ergebnis ? 'Prüfung fortsetzen' : 'Prüfung beginnen'),
    h('button', { class: 'btn', onclick: () => pdfDrucken({ url: `/api/admin/kassenpruefung/bericht?year=${jahr}`, oeffnenUrl: `/api/admin/kassenpruefung/bericht/oeffnen?year=${jahr}`, titel: 'Prüfbericht', meldung: () => 'Der Prüfbericht wurde im PDF-Programm geöffnet.' }) }, 'Prüfbericht'));
}

async function pruefungDialog() {
  let d = S.pruefung;
  const p = d.pruefung;
  const f = {
    datum: h('input', { type: 'date', value: p.datum || todayIso() }),
    pruefer: h('input', { type: 'text', maxlength: 200, value: p.pruefer || '', placeholder: 'z. B. Uwe Brandt, Karin Vogel' }),
    konto: h('input', { type: 'text', inputmode: 'decimal', value: p.kontostand != null ? numIn(p.kontostand) : '', placeholder: 'laut Kontoauszug' }),
    bar: h('input', { type: 'text', inputmode: 'decimal', value: p.barkasse != null ? numIn(p.barkasse) : '', placeholder: 'falls vorhanden' }),
    fest: h('textarea', { rows: 3, maxlength: 3000, class: 'rs-eingabe', placeholder: 'z. B. Beleg für die Reparatur vom 12.05. fehlt, wird nachgereicht.' }),
    ergebnis: h('select', null, [['', 'Prüfung noch offen'], ['ohne', 'Ohne Beanstandungen'], ['mit', 'Mit Beanstandungen']].map(([v, t]) => h('option', { value: v, selected: (p.ergebnis || '') === v }, t))),
    entl: h('input', { type: 'checkbox', checked: !!p.entlastung }),
  };
  f.fest.value = p.feststellungen || '';
  const haken = Object.fromEntries(d.punkte.map((x) => [x.key, h('input', { type: 'checkbox', checked: !!p.punkte[x.key] })]));
  const punkte = h('ul', { class: 'haken-liste pruef-punkte' });
  const stich = h('div');
  const zeichne = () => {
    punkte.replaceChildren(...d.punkte.map((x) => h('li', null, h('label', null, haken[x.key], ' ', h('b', null, x.text)),
      h('div', { class: 'hint' + (x.warnung ? ' warn-text' : '') }, x.hinweis))));
    stich.replaceChildren(
      h('div', { class: 'toolbar' }, h('b', null, 'Stichprobe'), h('div', { class: 'spacer' }),
        h('button', { class: 'btn small', disabled: !d.ausgaben, onclick: async () => {
          try { d = await api('POST', `/api/admin/kassenpruefung/stichprobe?year=${d.jahr}`); S.pruefung = d; zeichne(); } catch (e) { handleErr(e); }
        } }, d.stichprobe.length ? 'Neu ziehen' : 'Stichprobe ziehen')),
      d.stichprobe.length ? h('table', { class: 'data' }, h('tbody', null, d.stichprobe.map((x) => h('tr', null, h('td', null, deDate(x.datum)), h('td', null, x.beschreibung), h('td', { class: 'r' }, eur(x.betrag)),
        h('td', null, x.belege && x.belege.length ? h('a', { href: `/api/admin/beleg/${x.id}?year=${d.jahr}&idx=0`, target: '_blank' }, 'Beleg ansehen') : h('span', { class: 'badge warn' }, 'kein Beleg')),
        h('td', null, h('label', null, h('input', { type: 'checkbox', checked: x.geprueft, onchange: async (e) => {
          try { await api('PUT', `/api/admin/ausgaben/${x.id}/geprueft?year=${d.jahr}`, { geprueft: e.target.checked }); d = await api('GET', `/api/admin/kassenpruefung?year=${d.jahr}`); S.pruefung = d; S.kasse = null; zeichne(); }
          catch (err) { handleErr(err); }
        } }), ' geprüft')))))) : h('p', { class: 'hint' }, 'Das Programm wählt die drei größten Ausgaben und zufällig weitere aus (etwa jede zehnte, mindestens fünf). Diese vergleicht ihr mit den Belegen.'));
  };
  zeichne();
  const fld = (label, el) => h('div', { class: 'field' }, h('label', null, label), el);
  const speichern = async () => {
    const zahl = (el) => (el.value.trim() === '' ? null : parseNum(el.value));
    const konto = zahl(f.konto); const bar = zahl(f.bar);
    if (Number.isNaN(konto) || Number.isNaN(bar)) { toast('Bitte Kontostand und Barkasse als Zahl eintragen.', 'err'); return false; }
    try {
      S.pruefung = await api('PUT', `/api/admin/kassenpruefung?year=${d.jahr}`, { datum: f.datum.value, pruefer: f.pruefer.value, kontostand: konto, barkasse: bar,
        punkte: Object.fromEntries(Object.entries(haken).map(([k, el]) => [k, el.checked])), feststellungen: f.fest.value, ergebnis: f.ergebnis.value, entlastung: f.entl.checked });
    } catch (e) { handleErr(e); return false; }
    return true;
  };
  const ok = await breiterDialog(`Kassenprüfung ${d.jahr}`, h('div', null,
    h('div', { class: 'form' }, fld('Tag der Prüfung', f.datum), fld('Kassenprüfer', f.pruefer), fld('Kontostand 31.12. in €', f.konto), fld('Barkasse in €', f.bar)),
    h('p', { class: 'hint' }, `Kassenbestand laut Programm: ${eur(d.kassenbestand)}. Nach dem Speichern zeigt das Programm die Differenz.`),
    h('h4', null, 'Prüfpunkte'), punkte, stich,
    h('h4', null, 'Ergebnis'),
    h('div', { class: 'form' }, h('div', { class: 'field wide' }, h('label', null, 'Feststellungen'), f.fest), fld('Ergebnis', f.ergebnis),
      h('div', { class: 'field' }, h('label', null, ' '), h('label', null, f.entl, ' Entlastung des Vorstands empfehlen')))),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: speichern }]);
  if (ok) { S.aufgaben = null; toast('Kassenprüfung gespeichert', 'ok'); }
  render();
}

function adminKassenbericht(pruefMode) {
  const st = S.state;
  if (S.kasseYear == null) S.kasseYear = st.currentYear;
  const yearSel = h('select', { onchange: (e) => { S.kasseYear = Number(e.target.value); S.kasse = null; render(); } },
    st.years.map((y) => h('option', { value: y, selected: y === S.kasseYear }, y === st.currentYear ? `${y} (aktuell)` : `${y} (abgeschlossen)`)));
  if (!S.kasse || S.kasse.jahr !== S.kasseYear) {
    if (!S.kasseLoading) loadKasse(S.kasseYear);
    return h('div', null, h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), yearSel), h('p', { class: 'hint' }, 'Wird geladen …'));
  }
  const k = S.kasse;
  if (!S.kasseVerlauf && !S.kasseVerlaufLoading) loadKasseVerlauf();
  const vorjahr = (S.kasseVerlauf || []).find((j) => j.jahr === S.kasseYear - 1);
  const vjDelta = (cur, prev) => {
    if (prev == null) return '';
    const diff = cur - prev;
    if (Math.abs(diff) < 0.005) return ` · = Vorjahr (${S.kasseYear - 1})`;
    const pct = prev !== 0 ? Math.round((diff / Math.abs(prev)) * 1000) / 10 : null;
    const pctTxt = pct == null ? '' : ` (${pct > 0 ? '+' : ''}${nfFlex.format(pct)} %)`;
    return ` · ${diff > 0 ? '▲' : '▼'} ${eur(Math.abs(diff))}${pctTxt} ggü. ${S.kasseYear - 1}`;
  };
  const card = (t, big, small, cls, ic) => h('div', { class: 'card stat ' + (cls || '') }, ic ? h('div', { class: 'statico' }, icon(ic, 22)) : null,
    h('div', null, h('div', { class: 'hint' }, t), h('div', { class: 'big' }, big), h('div', { class: 'hint' }, small)));
  const cards = h('div', { class: 'cards' },
    card('Kassenbestand', eur(k.kassenbestand), `Anfangsbestand ${eur(k.anfangsbestand)} + Zahlungen${k.spenden ? ' + Spenden' : ''} − Ausgaben${vorjahr ? vjDelta(k.kassenbestand, vorjahr.kassenbestand) : ''}`, k.kassenbestand >= 0 ? 'ok' : 'err'),
    card('Von Pächtern eingegangen', `${eur(k.einnahmenBezahlt)}`, `tatsächlich gezahlt, nach heutigem Stand${vorjahr ? vjDelta(k.einnahmenBezahlt, vorjahr.einnahmenBezahlt) : ''}`, 'ok'),
    k.guthabenAusgezahlt ? card('An Pächter ausgezahlt', eur(k.guthabenAusgezahlt), 'Guthaben') : null,
    k.vorauszahlungen ? card('Vorauszahlungen', eur(k.vorauszahlungen), 'monatliche Raten der Pächter', 'ok') : null,
    k.spenden ? card('Spenden', eur(k.spenden), `an den Verein${vorjahr ? vjDelta(k.spenden, vorjahr.spenden || 0) : ''}`, 'ok') : null,
    card('Sonstige Ausgaben', eur(k.ausgabenSumme), `${k.ausgaben.length} Buchung(en)${vorjahr ? vjDelta(k.ausgabenSumme, vorjahr.ausgabenSumme) : ''}`, k.ausgabenSumme ? 'warn' : 'ok'));

  const anfInput = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.anfangsbestand), style: 'width:140px', autocomplete: 'off' });
  const saveAnfang = async () => {
    const v = parseNum(anfInput.value);
    if (v === null || Number.isNaN(v) || v < 0) { toast('Bitte eine Zahl ab 0 eingeben.', 'err'); return; }
    try { S.kasse = await api('PUT', `/api/admin/anfangsbestand?year=${S.kasseYear}`, { betrag: v }); S.kasseVerlauf = null; toast('Gespeichert', 'ok'); render(); } catch (e) { handleErr(e); }
  };

  const ohneBeleg = k.ausgaben.filter((x) => !(x.belege && x.belege.length)).length;
  const belegFilter = h('input', { type: 'checkbox', checked: S.kasseNurOhneBeleg,
    onchange: (e) => { S.kasseNurOhneBeleg = e.target.checked; render(); } });
  const gezeigt = S.kasseNurOhneBeleg ? k.ausgaben.filter((x) => !(x.belege && x.belege.length)) : k.ausgaben;
  const toggleGeprueft = async (x, checked) => {
    try { await api('PUT', `/api/admin/ausgaben/${x.id}/geprueft?year=${S.kasseYear}`, { geprueft: checked }); await refreshKasse(); }
    catch (e) { handleErr(e); }
  };
  const tbody = h('tbody');
  for (const x of gezeigt) {
    tbody.append(h('tr', null, h('td', null, deDate(x.datum)),
      h('td', null, x.beschreibung, x.wiederkehrend ? h('span', { title: 'Jährlich wiederkehrend', style: 'margin-left:4px;cursor:help' }, '🔁') : null),
      h('td', null, x.kategorie),
      h('td', { class: 'r' }, eur(x.betrag)),
      h('td', null, x.belege && x.belege.length ? h('a', { href: `/api/admin/beleg/${x.id}?year=${S.kasseYear}&idx=0`, target: '_blank', title: `${x.belege.length} Beleg(e) ansehen` }, `📎${x.belege.length > 1 ? ' ' + x.belege.length : ''}`) : null),
      h('td', { title: x.geprueft ? `Geprüft am ${deDate(x.geprueftAm)}` : 'Von der Kassenprüfung abhaken' },
        h('input', { type: 'checkbox', checked: x.geprueft, onchange: (e) => toggleGeprueft(x, e.target.checked) })),
      h('td', null, pruefMode ? null : h('button', { class: 'btn small', onclick: async () => { const r = await ausgabeDialog(x); if (r !== false && r !== undefined) await refreshKasse(); } }, 'Bearbeiten'))));
  }
  if (!gezeigt.length) tbody.append(h('tr', null, h('td', { colspan: 7, class: 'empty' },
    k.ausgaben.length ? 'Alle Ausgaben haben einen Beleg.' : leer('kasse', 'Noch keine Ausgaben', 'Hier stehen Vereinskosten neben der Pächterabrechnung, z. B. Kontoführung oder Reparaturen – jede mit Beleg-Foto, wenn du möchtest.'))));

  const katTable = (k.ausgabenKategorie || []).length > 1
    ? h('div', { class: 'tablewrap', style: 'max-width:360px;margin-top:10px' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Kategorie', 'Summe'].map((t) => h('th', null, t)))),
        h('tbody', null, k.ausgabenKategorie.map((kat) => h('tr', null, h('td', null, kat.kategorie), h('td', { class: 'r' }, eur(kat.summe)))))))
    : null;

  const vW3 = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.wasserM3), autocomplete: 'off', disabled: !!pruefMode });
  const vWE = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.wasserEUR), autocomplete: 'off', disabled: !!pruefMode });
  const vSK = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.stromKWh), autocomplete: 'off', disabled: !!pruefMode });
  const vSE = h('input', { type: 'text', inputmode: 'decimal', value: numIn(k.versorger.stromEUR), autocomplete: 'off', disabled: !!pruefMode });
  const diffRow = (label, total, vInput, unit) => {
    const v = parseNum(vInput.value);
    const diff = v == null || Number.isNaN(v) ? null : Math.round((v - total) * 100) / 100;
    return h('tr', null, h('td', null, label), h('td', { class: 'r' }, unit === '€' ? eur(total) : nfFlex.format(total) + ' ' + unit),
      h('td', null, vInput), h('td', { class: 'r' + (diff != null && Math.abs(diff) > 0.004 ? ' diffwarn' : '') },
        diff == null ? '–' : (unit === '€' ? eur(diff) : nfFlex.format(diff) + ' ' + unit)));
  };
  const versorgerTable = h('table', { class: 'data' },
    h('thead', null, h('tr', null, ['', 'Pächter gesamt', 'Versorger / Hauptzähler', 'Differenz'].map((t) => h('th', null, t)))),
    h('tbody', null,
      diffRow('Wasser', k.wasserVerbrauch, vW3, 'm³'), diffRow('Wasser (€)', k.summen.kostenWasser, vWE, '€'),
      diffRow('Strom', k.stromVerbrauch, vSK, 'kWh'), diffRow('Strom (€)', k.summen.kostenEnergie, vSE, '€')));
  // Wasser: liegt der Hauptzähler deutlich über der Summe der Pächter, kann eine Leitung undicht sein
  const wasserHaupt = parseNum(vW3.value);
  const wasserLoch = wasserHaupt != null && !Number.isNaN(wasserHaupt) && wasserHaupt > 0 ? wasserHaupt - k.wasserVerbrauch : 0;
  const verlustHinweis = wasserLoch > 0 && wasserLoch / wasserHaupt > 0.15
    ? h('div', { class: 'banner warn', style: 'margin-top:10px' },
      `Wasser: Der Hauptzähler zeigt ${nfFlex.format(Math.round(wasserLoch * 100) / 100)} m³ (${Math.round(wasserLoch / wasserHaupt * 100)} %) mehr, als die Pächter zusammen verbraucht haben. Das kann auf eine undichte Leitung oder einen falsch abgelesenen Zähler hindeuten.`)
    : null;
  const saveVersorger = async () => {
    const vals = [vW3, vWE, vSK, vSE].map((i) => parseNum(i.value));
    if (vals.some((v) => Number.isNaN(v) || (v !== null && v < 0))) { toast('Bitte nur Zahlen ab 0 eingeben.', 'err'); return; }
    try {
      S.kasse = await api('PUT', `/api/admin/versorger?year=${S.kasseYear}`, { wasserM3: vals[0], wasserEUR: vals[1], stromKWh: vals[2], stromEUR: vals[3] });
      S.schwund = null;
      toast('Gespeichert', 'ok');
      render();
    } catch (e) { handleErr(e); }
  };

  return h('div', null,
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), yearSel),
    h('p', { class: 'hint' }, `Übersicht für ${S.kasseYear}: ${k.paechter} Pächter (${k.ausgestellt} aus ausgestellten Rechnungen, ${k.berechnet} berechnet, aber noch nicht ausgestellt${k.unvollstaendig.length ? `, ${k.unvollstaendig.length} unvollständig` : ''}). Die Beträge »sonstige Ausgaben« sind Vereinskosten neben der Pächterabrechnung, z. B. Kontoführungsgebühren, Anwaltskosten oder Anschaffungen.`),
    k.unvollstaendig.length ? h('div', { class: 'banner warn' }, `Nicht enthalten (Angaben fehlen): ${k.unvollstaendig.join(', ')}`) : null,
    anfangsbestandHinweis(k, pruefMode),
    h('div', { class: 'card', style: 'margin-bottom:14px' },
      h('div', { class: 'toolbar' },
        h('label', null, 'Anfangsbestand der Kasse'),
        pruefMode ? h('b', null, eur(k.anfangsbestand)) : h('div', { class: 'inputunit' }, anfInput, h('span', null, '€')),
        pruefMode ? null : h('button', { class: 'btn small primary', onclick: saveAnfang }, 'Speichern'),
        pruefMode ? null : h('span', { class: 'hint' }, 'wird beim Jahreswechsel automatisch aus dem Kassenbestand des Vorjahres übernommen, ist hier aber jederzeit änderbar'))),
    cards,
    pruefungKarte(),
    h('h3', null, 'Sonstige Ausgaben der Vereinskasse'),
    h('div', { class: 'card' },
      h('div', { class: 'toolbar' },
        pruefMode ? null : h('button', { class: 'btn primary', onclick: async () => { const r = await ausgabeDialog(null); if (r !== false && r !== undefined) await refreshKasse(); } }, '+ Ausgabe erfassen'),
        h('div', { class: 'spacer' }),
        h('label', { class: 'hint' }, belegFilter, ` nur ohne Beleg${ohneBeleg ? ` (${ohneBeleg})` : ''}`)),
      h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Datum', 'Beschreibung', 'Kategorie', 'Betrag', 'Beleg', 'Geprüft', ''].map((t) => h('th', null, t)))), tbody)),
      katTable),
    h('h3', null, 'Vergleich mit dem Versorger'),
    h('div', { class: 'card' },
      h('p', { class: 'hint' }, 'Trage hier die Werte der Hauptzähler bzw. der Versorgerrechnung ein, um sie mit der Summe der Pächterabrechnung zu vergleichen. Eine größere Abweichung kann auf einen Zählerfehler, Schwund oder eine falsche Ablesung hindeuten.'),
      h('div', { class: 'tablewrap' }, versorgerTable), verlustHinweis,
      pruefMode ? null : h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: saveVersorger }, 'Werte speichern'))),
    h('h3', null, 'Schwund über die Jahre'),
    schwundKarte(),
    h('div', { class: 'actions', style: 'margin-top:16px;margin-bottom:22px' },
      h('button', { class: 'btn primary', title: 'Zusammenfassung des Jahres für die Mitgliederversammlung, ohne Namen', onclick: () => jahresberichtDrucken(S.kasseYear) }, `Jahresbericht ${S.kasseYear} (PDF)`),
      h('a', { class: 'btn', href: `/api/admin/export-kassenbericht?year=${S.kasseYear}` }, `Kassenbericht ${S.kasseYear} als Excel`)),
    h('h3', null, 'Verlauf über die Jahre'),
    kassenberichtVerlaufCard());
}

// Wasserschwund: Hauptzähler gegen die Summe der Gartenzähler, Jahr für Jahr, und Gärten mit auffälligem Verbrauch.
function schwundKarte() {
  if (!S.schwund || S.schwund.jahr !== S.kasseYear) {
    if (!S.schwundLoading) {
      S.schwundLoading = true;
      api('GET', `/api/admin/schwund?year=${S.kasseYear}`).then((d) => { S.schwund = d; }).catch((e) => { handleErr(e); S.schwund = { jahr: S.kasseYear, jahre: [], auffaellig: [] }; })
        .finally(() => { S.schwundLoading = false; render(); });
    }
    return h('div', { class: 'card' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const d = S.schwund;
  const mitWert = d.jahre.filter((j) => j.wasser.prozent != null || j.strom.prozent != null);
  const zahl = (x, einheit) => (x == null ? '–' : nfFlex.format(x) + (einheit ? ' ' + einheit : ''));
  const prozent = (w, warn) => (w.prozent == null ? '–' : h('span', { class: 'badge ' + (w.prozent >= warn ? 'err' : w.prozent < 0 ? 'warn' : 'ok') }, nfFlex.format(w.prozent) + ' %'));
  const zeile = (j) => h('tr', null, h('td', null, j.jahr, j.unvollstaendig ? h('div', { class: 'hint' }, `${j.unvollstaendig} Gärten fehlen`) : null),
    h('td', { class: 'r' }, zahl(j.wasser.haupt, 'm³')), h('td', { class: 'r' }, zahl(j.wasser.gaerten, 'm³')), h('td', { class: 'r' }, zahl(j.wasser.menge, 'm³')),
    h('td', { class: 'r' }, prozent(j.wasser, d.warnAb)), h('td', { class: 'r' }, j.wasser.kosten == null ? '–' : eur(j.wasser.kosten)),
    h('td', { class: 'r' }, zahl(j.strom.menge, 'kWh')), h('td', { class: 'r' }, j.strom.prozent == null ? '–' : nfFlex.format(j.strom.prozent) + ' %'));
  const chart = mitWert.filter((j) => j.wasser.prozent != null).map((j) => ({ label: String(j.jahr), value: Math.max(0, j.wasser.prozent) }));
  return h('div', { class: 'card' },
    h('p', { class: 'hint' }, `Schwund ist Wasser (oder Strom), das der Hauptzähler zählt, das aber in keinem Garten ankommt – durch undichte Leitungen, stehende oder falsch abgelesene Zähler. Bezahlt wird es vom Verein. Ab ${nfFlex.format(d.warnAb)} % oder bei einem deutlichen Anstieg meldet sich das Cockpit. Grundlage sind die Hauptzählerwerte oben, je Jahr eingetragen.`),
    mitWert.length ? [
      chart.length > 1 ? [h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Wasserschwund in Prozent:'), barChart(chart, { valueFmt: (v) => nfFlex.format(v) + ' %' })] : null,
      h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
        h('thead', null, h('tr', null, ['Jahr', 'Hauptzähler', 'Gärten', 'Schwund', 'Anteil', 'Kosten', 'Strom-Schwund', 'Anteil'].map((t) => h('th', null, t)))),
        h('tbody', null, [...mitWert].reverse().map(zeile))))]
      : h('p', { class: 'hint', style: 'margin:0' }, 'Noch keine Hauptzählerwerte eingetragen. Trage oben den Wert des Hauptzählers bzw. der Versorgerrechnung ein – dann erscheint hier der Schwund, auch im Vergleich der Jahre.'),
    h('h4', { style: 'margin:16px 0 6px' }, `Auffällige Gärten ${d.jahr}`),
    d.auffaellig.length ? h('ul', { class: 'schwund-liste' }, d.auffaellig.map((x) => h('li', null, h('b', null, `Garten ${x.gartennr || '–'} – ${x.name}: `), x.hinweise.join('; '))))
      : h('p', { class: 'hint', style: 'margin:0' }, 'Kein Garten fällt auf (Verbrauch im üblichen Rahmen, kein Zähler steht still).'));
}

function kassenberichtVerlaufCard() {
  if (!S.kasseVerlauf) {
    if (!S.kasseVerlaufLoading) loadKasseVerlauf();
    return h('div', { class: 'card' }, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const v = S.kasseVerlauf;
  if (!v.length) return h('div', { class: 'card empty' }, 'Noch keine Jahre vorhanden.');
  const rows = v.map((j) => h('tr', null,
    h('td', null, j.jahr),
    h('td', { class: 'r' }, eur(j.rechnungssumme)),
    h('td', { class: 'r' }, eur(j.einnahmenBezahlt)),
    h('td', { class: 'r' }, eur(j.ausgabenSumme)),
    h('td', { class: 'r' }, eur(j.kassenbestand))));
  const chartData = [...v].reverse().map((j) => ({ label: String(j.jahr), value: j.kassenbestand }));
  return h('div', { class: 'card' },
    h('p', { class: 'hint' }, 'Summen des Vereins insgesamt (nicht je Pächter), zum Vergleich über die Jahre – z. B. für die Mitgliederversammlung.'),
    h('p', { class: 'hint', style: 'margin:0 0 4px' }, 'Kassenbestand am Jahresende:'),
    barChart(chartData, { valueFmt: (v) => eur(v) }),
    h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Jahr', 'Rechnungen gestellt', 'Zahlungen eingegangen', 'Sonstige Ausgaben', 'Kassenbestand'].map((t) => h('th', null, t)))),
      h('tbody', null, rows))));
}

// ---- Import / Export / Sicherung

async function vereinsakteErstellen() {
  try {
    const r = await api('POST', '/api/admin/vereinsakte');
    toast(`Vereinsakte erstellt: ${r.datei} (${r.dateien} Dateien, ${groesseText(r.groesse)}). Der Ordner wurde geöffnet – Datei auf den USB-Stick kopieren.`, 'ok');
  } catch (e) { handleErr(e); }
}

function adminData() {
  const st = S.state;
  const wrap = h('div');
  const file = h('input', { type: 'file', accept: '.xlsx,.csv,.txt' });
  const previewBox = h('div');

  const drawPreview = () => {
    previewBox.replaceChildren();
    const pv = S.importPreview;
    if (!pv) return;
    const checks = [];
    const tb = h('tbody');
    for (const r of pv.rows) {
      const c = h('input', { type: 'checkbox', checked: true });
      checks.push([c, r]);
      tb.append(h('tr', null, h('td', null, c), h('td', null, h('span', { class: 'badge ' + (r.aktion === 'neu' ? 'neu' : 'warn') }, r.aktion === 'neu' ? 'neu' : 'aktualisieren')),
        h('td', null, r.paechter.mitgliedsnr), h('td', null, r.paechter.name), h('td', null, [r.paechter.strasse, r.paechter.plzOrt].filter(Boolean).join(', ')),
        h('td', { class: 'num' }, r.paechter.gartengroesse ? nfFlex.format(r.paechter.gartengroesse) + ' m²' : ''),
        h('td', null, r.ablesung ? '✓ Zählerstände' : ''), h('td', null, (r.warnungen || []).join('; '))));
    }
    previewBox.append(h('div', null,
      h('h3', null, `Vorschau: ${pv.rows.length} Zeilen gefunden`),
      (pv.warnings || []).length ? h('div', { class: 'banner warn' }, h('ul', { class: 'plain', style: 'margin:0' }, pv.warnings.map((w) => h('li', null, w)))) : null,
      h('p', { class: 'hint' }, 'Erkannte Spalten: ', (pv.rows[0].felder || []).join(', ') || 'nur Mitgliedsnr. und Name',
        '. Bereits vorhandene Pächter (gleiche Mitgliedsnummer) werden aktualisiert, alle anderen neu angelegt. Angaben, die in der Datei fehlen, bleiben unverändert.'),
      h('div', { class: 'tablewrap', style: 'max-height:360px' }, h('table', null,
        h('thead', null, h('tr', null, ['', 'Aktion', 'Mitgl.-Nr.', 'Name', 'Anschrift', 'Größe', 'Zählerstände', 'Hinweis'].map((x) => h('th', null, x)))), tb)),
      h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: async () => {
        const rows = checks.filter(([c]) => c.checked).map(([, r]) => r);
        if (!rows.length) { toast('Keine Zeile ausgewählt.', 'err'); return; }
        try {
          const r = await api('POST', '/api/admin/import/apply', { rows });
          toast(`Import fertig: ${r.neu} neu, ${r.aktualisiert} aktualisiert`, 'ok');
          S.importPreview = null;
          await reload();
        } catch (e) { handleErr(e); }
      } }, 'Ausgewählte Zeilen importieren'), h('button', { class: 'btn', onclick: () => { S.importPreview = null; drawPreview(); file.value = ''; } }, 'Abbrechen'))));
  };
  file.addEventListener('change', async () => {
    if (!file.files.length) return;
    const fd = new FormData();
    fd.append('file', file.files[0]);
    try { S.importPreview = await api('POST', '/api/admin/import/preview', fd, true); drawPreview(); } catch (e) { S.importPreview = null; drawPreview(); handleErr(e); }
  });
  drawPreview();

  wrap.append(
    h('div', { class: 'card', style: 'margin-bottom:16px' },
      h('h3', { style: 'margin-top:0' }, 'Pächter aus Excel oder CSV importieren'),
      h('p', { class: 'hint' }, 'Die Datei braucht eine Kopfzeile mit mindestens »Mitgliedsnr.« und »Name«. Weitere Spalten werden automatisch erkannt: Gartennr., Anrede, Straße, PLZ Ort, Versandart, E-Mail, Telefon, Gartengröße, Umlage abweichend, Wasserzähler-Nr., Stromzähler-Nr., Wasser/Energie Stand Vorjahr und aktuell, Arbeitsstunden, Versicherung, Grundsteuer, Auslagen, Hinweis, Abschlag. Eine passend aufgebaute Excel-Vorlage (Blatt »Mitglieder«) funktioniert direkt. Bei bekannten Mitgliedsnummern werden die Angaben aktualisiert; leere Zellen lassen die gespeicherten Werte unverändert.'),
      file, previewBox),
    h('div', { class: 'card', style: 'margin-bottom:16px' },
      h('h3', { style: 'margin-top:0' }, 'Export'),
      h('p', { class: 'hint' }, `Alle Pächter des Jahres ${st.year} mit Zählerständen und Beträgen als Excel-Datei, zum Beispiel für die Kassenprüfung.`),
      h('a', { class: 'btn', href: `/api/export?year=${st.year}` }, `Jahresübersicht ${st.year} als Excel`)),
    h('div', { class: 'card' },
      h('h3', { style: 'margin-top:0' }, 'Datensicherung'),
      h('p', { class: 'hint' }, 'Das Programm legt bei Änderungen automatisch eine Tagessicherung an (die letzten 60 Tage) und vor Löschen und Import zusätzlich eine eigene. Die Sicherung vor jedem Jahreswechsel bleibt 12 Jahre erhalten, so lässt sich der Stand jedes Abrechnungsjahres über die ganze Aufbewahrungsfrist zurückholen. Du kannst außerdem den gesamten Datenbestand herunterladen. Eine Sicherung spielst du direkt hier unten wieder ein.'),
      h('div', { class: 'actions', style: 'margin-top:8px' },
        h('button', { class: 'btn', title: 'Sofort eine Sicherung anlegen und in den zweiten Sicherungsordner kopieren', onclick: sicherungJetzt }, 'Jetzt sichern'),
        h('a', { class: 'btn', href: '/api/admin/backup' }, 'Gesamten Datenbestand herunterladen'),
        h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'sicherungen' }).catch(handleErr) }, 'Sicherungsordner öffnen'),
        h('button', { class: 'btn', onclick: () => api('POST', '/api/open-folder', { which: 'daten' }).catch(handleErr) }, 'Datenordner öffnen')),
      zweiteSicherungField()),
    h('div', { class: 'card', style: 'margin-top:16px' },
      h('h3', { style: 'margin-top:0' }, 'Amtsübergabe'),
      h('p', { class: 'hint' }, 'Neuer Vorstand oder Kassenwart? Die Vereinsakte enthält den kompletten Datenordner (Daten, Rechnungen, Gartenakten, Belege) und ein PDF »Lies mich zuerst« mit Zahlen, Preisen, Terminen, Gemeinnützigkeit und dem, was gerade ansteht. Auf einen USB-Stick kopieren und persönlich übergeben – das Passwort steht nicht darin. Es wird immer nur die neueste Akte aufbewahrt.'),
      h('button', { class: 'btn primary', onclick: vereinsakteErstellen }, 'Vereinsakte erstellen')),
    sicherungenCard());
  return wrap;
}

async function loadSicherungen() {
  if (S.sicherungenLoading) return;
  S.sicherungenLoading = true;
  try { S.sicherungen = await api('GET', '/api/admin/sicherungen'); } catch (e) { handleErr(e); S.sicherungen = null; S.sicherungenZeigen = false; }
  S.sicherungenLoading = false;
  render();
}

// Sicherung wiederherstellen: Liste der vorhandenen Sicherungen, Auswahl mit deutlicher Rückfrage.
function sicherungenCard() {
  const kopf = h('h3', { style: 'margin-top:0' }, 'Sicherung wiederherstellen');
  if (!S.sicherungenZeigen) {
    return h('div', { class: 'card', style: 'margin-top:14px' }, kopf,
      h('p', { class: 'hint' }, 'Hat sich etwas versehentlich verändert oder ist die Datendatei beschädigt? Hier siehst du alle vorhandenen Sicherungen und kannst eine davon einspielen. Der aktuelle Stand wird vorher selbst gesichert, auch das lässt sich also rückgängig machen.'),
      h('button', { class: 'btn', onclick: () => { S.sicherungenZeigen = true; S.sicherungen = null; render(); } }, 'Sicherungen anzeigen'));
  }
  if (!S.sicherungen) {
    if (!S.sicherungenLoading) loadSicherungen();
    return h('div', { class: 'card', style: 'margin-top:14px' }, kopf, h('p', { class: 'hint', style: 'margin:0' }, 'Wird geladen …'));
  }
  const wiederherstellen = async (x) => {
    const ok = await confirmBox(`Den Stand vom ${fmtWhen(x.zeit)} (${x.art}; ${x.paechter} Pächter, ${x.rechnungen} Rechnungen) wiederherstellen? Alles, was seitdem geändert wurde, verschwindet aus dem Programm, auch Einstellungen und Passwörter gehen auf den damaligen Stand zurück. Der heutige Stand bleibt als Sicherung »vor-Wiederherstellung« erhalten.`, 'Wiederherstellen', true);
    if (!ok) return;
    try {
      await api('POST', '/api/admin/sicherungen/wiederherstellen', { name: x.name });
      toast('Sicherung wiederhergestellt', 'ok');
      location.reload();
    } catch (e) { handleErr(e); }
  };
  const rows = S.sicherungen.map((x) => h('tr', null,
    h('td', null, fmtWhen(x.zeit)), h('td', null, x.art),
    h('td', { class: 'num' }, x.lesbar ? x.paechter : '–'), h('td', { class: 'num' }, x.lesbar ? x.rechnungen : '–'),
    h('td', null, x.lesbar
      ? h('button', { class: 'btn small danger', onclick: () => wiederherstellen(x) }, 'Wiederherstellen')
      : h('span', { class: 'hint' }, x.hinweis || 'nicht lesbar'))));
  return h('div', { class: 'card', style: 'margin-top:14px' },
    h('div', { class: 'toolbar' }, kopf, h('div', { class: 'spacer' }),
      h('button', { class: 'btn small', onclick: () => { S.sicherungenZeigen = false; render(); } }, 'Schließen')),
    S.sicherungen.length
      ? h('div', { class: 'tablewrap' }, h('table', { class: 'data' },
          h('thead', null, h('tr', null, ['Stand vom', 'Art', 'Pächter', 'Rechnungen', ''].map((t) => h('th', null, t)))),
          h('tbody', null, rows)))
      : h('p', { class: 'hint', style: 'margin:0' }, 'Noch keine Sicherungen vorhanden.'),
    S.sicherungen.length >= 40 ? h('p', { class: 'hint', style: 'margin-bottom:0' }, `Es werden die neuesten ${S.sicherungen.length} Sicherungen angezeigt. Ältere liegen im Sicherungsordner.`) : null);
}

function zweiteSicherungField() {
  const st = S.state;
  const inp = h('input', { type: 'text', value: st.settings.zweiteSicherung || '', style: 'width:100%', autocomplete: 'off',
    placeholder: 'z. B. D:\\Sicherung-Kleingarten-Manager oder ein Netzlaufwerk-Pfad' });
  const save = async () => {
    try {
      await api('PUT', '/api/admin/settings', { ...st.settings, zweiteSicherung: inp.value.trim() });
      toast('Gespeichert', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', { style: 'margin-top:16px;border-top:1px solid var(--line,#ddd);padding-top:12px' },
    h('div', { class: 'field wide' }, h('label', null, 'Zweiter Sicherungsordner (optional, z. B. USB-Stick oder Netzlaufwerk)'), inp,
      h('span', { class: 'fhint' }, 'Leer lassen = deaktiviert. Ist der Pfad gerade nicht erreichbar (USB-Stick nicht eingesteckt), läuft das Programm trotzdem normal weiter, es wird nur dort nicht zusätzlich gesichert.')),
    h('div', { class: 'actions' }, h('button', { class: 'btn', onclick: save }, 'Speichern')));
}

// ---- Jubiläen

async function loadJubilaeen() {
  if (S.jubilaeenLoading) return;
  S.jubilaeenLoading = true;
  try { S.jubilaeen = await api('GET', '/api/garten-historie'); } catch (e) { handleErr(e); S.jubilaeen = []; }
  S.jubilaeenLoading = false;
  render();
}

// Zeigt, wie lange ein Pächter schon dabei ist – aus dem frühesten Eintrag der
// Gartenhistorie je Pächter, nützlich z. B. für die Ehrung langjähriger
// Mitglieder bei der Mitgliederversammlung. Steht erst ab Einführung der
// Gartenhistorie zur Verfügung; ältere Pächter ohne Eintrag landen gesondert.
function adminJubilaeen() {
  const st = S.state;
  if (!S.jubilaeen) {
    if (!S.jubilaeenLoading) loadJubilaeen();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const seitByPaechter = {};
  for (const e of S.jubilaeen) {
    if (!seitByPaechter[e.paechterId] || e.seit < seitByPaechter[e.paechterId]) seitByPaechter[e.paechterId] = e.seit;
  }
  const list = st.paechter.filter((p) => !p.ausgeschiedenAm).sort(cmpNr);
  const rows = [];
  const ohneDatum = [];
  for (const p of list) {
    const seit = seitByPaechter[p.id];
    if (!seit) { ohneDatum.push(p); continue; }
    rows.push({ p, seit, jahre: jahreSeit(seit) });
  }
  rows.sort((a, b) => a.seit.localeCompare(b.seit));
  const tbody = h('tbody', null, rows.map((r) => h('tr', null,
    h('td', null, r.p.mitgliedsnr), h('td', null, r.p.name), h('td', null, r.p.gartennr),
    h('td', null, deDate(r.seit)),
    h('td', { class: 'r' }, r.jahre > 0 && r.jahre % 5 === 0
      ? h('b', { title: 'Jubiläum', style: 'color:var(--ok)' }, `🎉 ${r.jahre} Jahre`)
      : `${r.jahre} Jahre`))));
  return h('div', null,
    h('p', { class: 'hint' }, 'Mitglied-seit-Datum aus der Gartenhistorie (frühester Eintrag je Pächter). Nützlich für die Mitgliederversammlung, z. B. um langjährige Mitglieder zu ehren. Steht erst ab Einführung der Gartenhistorie-Funktion zur Verfügung – bei Pächtern von davor fehlt das Datum, sie stehen unten in einer eigenen Liste.'),
    h('div', { class: 'card tablewrap' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, ['Mitgl.-Nr.', 'Name', 'Garten', 'Mitglied seit', 'Dabei seit'].map((t) => h('th', null, t)))),
      rows.length ? tbody : h('tbody', null, h('tr', null, h('td', { colspan: 5, class: 'empty' }, 'Noch keine Daten.'))))),
    ohneDatum.length ? h('div', { class: 'card', style: 'margin-top:12px' },
      h('h3', { style: 'margin-top:0' }, 'Ohne bekanntes Eintrittsdatum'),
      h('p', { class: 'hint' }, ohneDatum.map((p) => `${p.mitgliedsnr} ${p.name}`).join(', '))) : null);
}

// ---- Änderungsprotokoll

async function loadProtokoll() {
  if (S.protokollLoading) return;
  S.protokollLoading = true;
  try { S.protokoll = await api('GET', '/api/admin/audit-log'); } catch (e) { handleErr(e); S.protokoll = []; }
  S.protokollLoading = false;
  render();
}

function adminProtokoll() {
  if (!S.protokoll) {
    if (!S.protokollLoading) loadProtokoll();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const fmtZeit = (iso) => { const d = new Date(iso); return isNaN(d) ? iso : d.toLocaleString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit' }).replace(',', ''); };
  const rows = S.protokoll.map((e) => h('tr', null, h('td', { class: 'mono', style: 'white-space:nowrap' }, fmtZeit(e.zeit)), h('td', null, e.aktion)));
  return h('div', null,
    h('p', { class: 'hint' }, 'Wer wann welchen Pächter oder welche Preise geändert hat. Da es nur ein gemeinsames Admin-Passwort gibt, wird nicht festgehalten, welche Person es war – nur Zeitpunkt und Art der Änderung. Die letzten 1000 Einträge bleiben erhalten.'),
    h('div', { class: 'card tablewrap', style: 'max-height:520px' }, h('table', { class: 'data' },
      h('thead', null, h('tr', null, h('th', null, 'Zeitpunkt'), h('th', null, 'Änderung'))),
      h('tbody', null, rows.length ? rows : h('tr', null, h('td', { colspan: 2, class: 'empty' }, 'Noch keine Einträge.'))))));
}

// ---- Passwort

function adminSecurity() {
  const st = S.state;
  const old = h('input', { type: 'password', autocomplete: 'current-password' });
  const n1 = h('input', { type: 'password', autocomplete: 'new-password' });
  const n2 = h('input', { type: 'password', autocomplete: 'new-password' });
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const save = async () => {
    if (n1.value !== n2.value) { toast('Die beiden neuen Passwörter sind nicht gleich.', 'err'); return; }
    if (!st.hasPassword && !n1.value) { toast('Bitte ein Passwort eingeben.', 'err'); return; }
    try {
      await api('POST', '/api/admin/password', { old: old.value, new: n1.value });
      toast(n1.value ? 'Passwort gespeichert' : 'Passwortschutz entfernt', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('div', { class: 'card narrow' },
      h('h3', { style: 'margin-top:0' }, st.hasPassword ? 'Admin-Passwort ändern oder entfernen' : 'Admin-Passwort festlegen'),
      h('p', { class: 'hint' }, 'Das Passwort schützt Pächter, Preise, Jahreswechsel und Import. Die Zählerstände kann weiterhin jeder eintragen, der das Programm öffnet.'),
      h('div', { class: 'form', style: 'grid-template-columns:1fr' },
        st.hasPassword ? fld('Bisheriges Passwort', old) : null,
        fld('Neues Passwort', n1, st.hasPassword ? 'Leer lassen, um den Passwortschutz zu entfernen' : 'Mindestens 8 Zeichen'),
        fld('Neues Passwort wiederholen', n2)),
      h('div', { class: 'actions' },
        h('button', { class: 'btn primary', onclick: save }, 'Speichern'),
        st.hasPassword ? h('button', { class: 'btn', onclick: async () => { try { await api('POST', '/api/admin/logout'); await reload(); } catch (e) { handleErr(e); } } }, 'Abmelden') : null),
      h('p', { class: 'hint', style: 'margin-top:16px' }, 'Passwort vergessen? Das Programm mit dem Zusatz "--reset-admin" starten (in der Eingabeaufforderung: Kleingarten-Manager.exe --reset-admin). Dann ist der Passwortschutz entfernt und du kannst ein neues Passwort festlegen.')),
    pruefPasswortCard(),
    updateCheckCard());
}

// ---- Kassenprüfer-Zugang verwalten (nur der Admin kann ihn einrichten)

function pruefPasswortCard() {
  const st = S.state;
  const n1 = h('input', { type: 'password', autocomplete: 'new-password' });
  const n2 = h('input', { type: 'password', autocomplete: 'new-password' });
  const fld = (label, input, hint) => h('div', { class: 'field' }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const save = async () => {
    if (n1.value !== n2.value) { toast('Die beiden Passwörter sind nicht gleich.', 'err'); return; }
    try {
      await api('POST', '/api/admin/pruef-password', { new: n1.value });
      toast(n1.value ? 'Kassenprüfer-Zugang gespeichert' : 'Kassenprüfer-Zugang entfernt', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  return h('div', { class: 'card narrow', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Kassenprüfer-Zugang'),
    h('p', { class: 'hint' }, `Eigenes, optionales Passwort für den Kassenprüfer: Lesezugriff auf den Kassenbericht (inkl. Geprüft-Haken für die sonstigen Ausgaben), aber ohne die übrigen Admin-Rechte. ${st.hasPruefPassword ? 'Zugang ist eingerichtet.' : 'Zugang ist nicht eingerichtet.'}`),
    h('div', { class: 'form', style: 'grid-template-columns:1fr' },
      fld('Passwort', n1, st.hasPruefPassword ? 'Leer lassen, um den Zugang zu entfernen' : 'Mindestens 8 Zeichen'),
      fld('Passwort wiederholen', n2)),
    h('div', { class: 'actions' }, h('button', { class: 'btn primary', onclick: save }, 'Speichern')));
}

// ---- Update-Prüfung (nur auf Knopfdruck, nie automatisch)

async function checkUpdate() {
  if (S.updateChecking) return;
  S.updateChecking = true;
  try { S.updateCheck = await api('GET', '/api/admin/check-update'); } catch (e) { S.updateCheck = { hinweis: e.message }; }
  S.updateChecking = false;
  render();
}

function updateCheckCard() {
  const st = S.state;
  const r = S.updateCheck;
  return h('div', { class: 'card narrow', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Nach Updates suchen'),
    h('p', { class: 'hint' }, `Installierte Version: ${st.version}. Die Prüfung fragt nur auf Knopfdruck einmalig die öffentliche GitHub-Release-Seite dieses Projekts ab, es läuft sonst keine Internetverbindung im Hintergrund.`),
    h('div', { class: 'actions' },
      h('button', { class: 'btn', disabled: S.updateChecking, onclick: checkUpdate }, S.updateChecking ? 'Prüfe …' : 'Nach Updates suchen')),
    r ? (r.available
      ? h('div', { class: 'banner info', style: 'margin-top:10px' }, `Version ${r.version} ist verfügbar. `,
          h('a', { href: r.url, target: '_blank', rel: 'noopener noreferrer', onclick: (e) => {
            // Im eigenen Programmfenster über den Systembrowser öffnen statt im Fenster selbst.
            if (typeof window.kgmOpenExternal !== 'function') return;
            e.preventDefault();
            window.kgmOpenExternal(r.url).catch(() => toast('Die Release-Seite konnte nicht geöffnet werden.', 'err'));
          } }, 'Release-Seite öffnen'))
      : r.hinweis
        ? h('p', { class: 'hint', style: 'margin-top:10px' }, r.hinweis)
        : h('p', { class: 'hint', style: 'margin-top:10px' }, 'Du hast die aktuelle Version.')) : null);
}

// ------------------------------------------------------------------ Pächterwechsel

// Hinweis an Pächtern, die ihren Garten schon übergeben haben: sie bleiben bis zum Jahreswechsel für ihre
// Schlussrechnung im laufenden Jahr.
function schlussBadge(p) {
  if (!p.ausgeschiedenAm) return null;
  return h('span', { class: 'badge neu', style: 'margin-left:6px', title: `Garten am ${deDate(p.ausgeschiedenAm)} übergeben. Abgerechnet wird nur der Verbrauch bis zur Übergabe, die Beiträge fürs Folgejahr zahlt der Nachfolger. Kommt beim Jahreswechsel in den Papierkorb.` }, 'Schlussrechnung');
}

// nächste freie Mitgliedsnummer als Vorschlag (größte Zahl + 1)
function naechsteMitgliedsnr() {
  let max = 0;
  for (const p of S.state.paechter) { const n = parseInt(p.mitgliedsnr, 10); if (n > max) max = n; }
  return String(max + 1);
}

async function paechterwechselDialog(alt, bewerber) {
  const st = S.state;
  const ab = st.ablesungen[alt.id] || {};
  if (st.issued && st.issued[alt.id]) {
    toast(`Für ${alt.name} ist die Rechnung ${st.currentYear} schon ausgestellt. Bitte zuerst den Jahreswechsel machen, dann den Pächterwechsel.`, 'err');
    return false;
  }
  let liste = [], wert = null;
  try { liste = await api('GET', '/api/admin/warteliste'); } catch (e) { handleErr(e); }
  try { wert = (await api('GET', `/api/admin/gartenakte?gartennr=${encodeURIComponent((alt.gartennr || '').trim())}`)).wertermittlungen[0] || null; } catch (e) { /* ohne Vorschlag weiter */ }
  const t = (val, ph, typ) => h('input', { type: typ || 'text', value: val ?? '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const zahl = (val, ph) => { const el = t(numIn(val), ph); el.setAttribute('inputmode', 'decimal'); return el; };
  const f = {
    datum: t(todayIso(), '', 'date'),
    wasserVJ: zahl(ab.wasserVJ), stromVJ: zahl(ab.stromVJ),
    wasser: zahl(null, ab.wasserVJ != null ? `Vorjahr: ${numIn(ab.wasserVJ)}` : ''),
    strom: zahl(null, ab.stromVJ != null ? `Vorjahr: ${numIn(ab.stromVJ)}` : ''),
    stunden: zahl(ab.stunden, '0, wenn keine'),
    warte: h('select', null, h('option', { value: '' }, liste.length ? '– neuen Pächter von Hand eintragen –' : '(Warteliste ist leer)'),
      liste.map((b) => h('option', { value: b.id, selected: bewerber && bewerber.id === b.id }, `${b.platz}. ${b.name}${b.wunsch ? ' – ' + b.wunsch : ''}`))),
    mitgliedsnr: t(naechsteMitgliedsnr()),
    anrede: h('select', null, ['Herr', 'Frau', 'Familie', 'Herr und Frau', ''].map((a) => h('option', { value: a }, a || '(keine)'))),
    name: t(''), strasse: t(''), plzOrt: t(st.settings.ort),
    versand: h('select', null, ['Emailsendung', 'Postversand'].map((a) => h('option', { value: a }, a))),
    email: t('', 'name@beispiel.de', 'email'), telefon: t('', '', 'tel'),
    groesse: zahl(alt.gartengroesse),
    abloese: zahl(wert ? wert.summe : null, 'leer = keine'),
    schluessel: t('', 'z. B. 2 Torschlüssel, 1 Laubenschlüssel'),
    bemerkungen: h('textarea', { rows: 3, maxlength: 1500, style: 'width:100%', placeholder: 'Zustand von Laube und Garten, Mängel, Absprachen …' }),
  };
  const uebernehmen = () => {
    const b = liste.find((x) => x.id === f.warte.value);
    if (!b) return;
    f.anrede.value = b.anrede || ''; f.name.value = b.name; f.strasse.value = b.strasse || ''; f.plzOrt.value = b.plzOrt || st.settings.ort;
    f.email.value = b.email || ''; f.telefon.value = b.telefon || ''; f.versand.value = b.email ? 'Emailsendung' : 'Postversand';
  };
  f.warte.addEventListener('change', uebernehmen);
  uebernehmen();
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const sect = (titel) => h('div', { class: 'section-title' }, titel);
  const body = h('div', { class: 'form' },
    h('p', { class: 'hint wide', style: 'grid-column:1/-1;margin:0' },
      `${alt.name} gibt Garten ${alt.gartennr} ab. ${alt.name} bekommt eine Schlussrechnung über den Verbrauch bis zur Übergabe (ohne Pacht und Beiträge fürs Folgejahr) und kommt beim Jahreswechsel in den Papierkorb. Der neue Pächter startet mit den heutigen Zählerständen. Vorher wird automatisch eine Sicherung angelegt.`),
    sect('1. Übergabe und Zählerstände'),
    fld('Tag der Übergabe *', f.datum),
    ab.wasserVJ == null ? fld('Wasser: Vorjahresstand *', f.wasserVJ, 'fehlt noch beim bisherigen Pächter') : null,
    ab.stromVJ == null ? fld('Strom: Vorjahresstand *', f.stromVJ, 'fehlt noch beim bisherigen Pächter') : null,
    fld(`Wasserzähler ${alt.wasserzaehlerNr || ''} – Stand bei Übergabe (m³) *`, f.wasser),
    fld(`Stromzähler ${alt.stromzaehlerNr || ''} – Stand bei Übergabe (kWh) *`, f.strom),
    fld(`Arbeitsstunden von ${alt.name} bis zur Übergabe *`, f.stunden, `Pflicht sind ${nfFlex.format(st.settings.pflichtstunden)} h; fehlende werden berechnet`),
    sect('2. Neuer Pächter'),
    fld('Von der Warteliste', f.warte, liste.length ? 'Übernimmt Name und Kontakt; der Eintrag wird danach von der Warteliste genommen.' : null, true),
    fld('Mitgliedsnummer *', f.mitgliedsnr, 'Neue Nummer; die alte bleibt bis zum Jahreswechsel beim bisherigen Pächter'),
    fld('Anrede', f.anrede), fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('Versandart', f.versand), fld('E-Mail-Adresse', f.email), fld('Telefon', f.telefon),
    fld('Gartengröße in m²', f.groesse),
    sect('3. Vereinbarungen (für das Übergabeprotokoll)'),
    fld('Ablösesumme in €', f.abloese, wert ? `aus der Wertermittlung vom ${deDate(wert.datum)} (${eur(wert.summe)}); zahlt der neue an den bisherigen Pächter` : 'zahlt der neue an den bisherigen Pächter; eine Wertermittlung lässt sich in der Gartenakte anlegen'), fld('Übergebene Schlüssel', f.schluessel),
    fld('Zustand und Bemerkungen', f.bemerkungen, null, true));
  let ergebnis = null;
  const ok = await modal(`Pächterwechsel Garten ${alt.gartennr}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Wechsel durchführen', cls: 'primary', value: true, action: async () => {
      const n = (el) => parseNum(el.value);
      const werte = { wasserVJ: n(f.wasserVJ), stromVJ: n(f.stromVJ), wasserStand: n(f.wasser), stromStand: n(f.strom), stunden: n(f.stunden), abloese: n(f.abloese), groesse: n(f.groesse) };
      if (Object.values(werte).some((x) => Number.isNaN(x) || (x != null && x < 0))) { toast('Zählerstände, Stunden, Größe und Ablöse müssen Zahlen ab 0 sein.', 'err'); return false; }
      const daten = {
        altId: alt.id, datum: f.datum.value, wasserVJ: werte.wasserVJ, stromVJ: werte.stromVJ, wasserStand: werte.wasserStand, stromStand: werte.stromStand,
        stunden: werte.stunden, abloese: werte.abloese, schluessel: f.schluessel.value, bemerkungen: f.bemerkungen.value.trim(), wartelisteId: f.warte.value,
        neu: { mitgliedsnr: f.mitgliedsnr.value, anrede: f.anrede.value, name: f.name.value, strasse: f.strasse.value, plzOrt: f.plzOrt.value,
          versand: f.versand.value, email: f.email.value.trim(), telefon: f.telefon.value, gartengroesse: werte.groesse ?? 0 },
      };
      try { ergebnis = await api('POST', '/api/admin/paechterwechsel', daten); } catch (e) { handleErr(e); return false; }
    } },
  ]);
  if (!ok || !ergebnis) return false;
  S.uebergaben = null; S.papierkorb = null;
  await reload();
  const u = ergebnis.uebergabe;
  const weiter = await modal('Pächterwechsel erledigt', h('div', null,
    h('p', null, `Garten ${u.gartennr} gehört ab ${deDate(u.datum)} ${u.neuName}.`),
    h('ul', null,
      h('li', null, 'Das Übergabeprotokoll zum Unterschreiben kannst du jetzt drucken (auch später in der Gartenakte oder unter »Pächter« → Gartenübergaben).'),
      h('li', null, `Die Schlussrechnung für ${u.altName} kannst du gleich ausstellen oder später zusammen mit den anderen Rechnungen.`))),
  [{ label: 'Schließen', value: '' }, { label: 'Willkommensmappe', value: 'willkommen' }, { label: 'Schlussrechnung ausstellen', value: 'rechnung' }, { label: 'Protokoll drucken', cls: 'primary', value: 'protokoll' }]);
  if (weiter === 'protokoll') await protokollDrucken(u);
  if (weiter === 'willkommen') await willkommenDrucken({ id: u.neuId, name: u.neuName });
  if (weiter === 'rechnung') { try { await issue([u.altId]); } catch (e) { handleErr(e); } }
  return true;
}

const protokollDrucken = (u) => pdfDrucken({
  url: `/api/admin/uebergabe/${u.id}/protokoll`, oeffnenUrl: `/api/admin/uebergabe/${u.id}/protokoll/oeffnen`,
  titel: `Übergabeprotokoll Garten ${u.gartennr}`, meldung: () => 'Das Übergabeprotokoll wurde im PDF-Programm geöffnet – dort drucken.',
});

async function loadUebergaben() {
  if (S.uebergabenLoading) return;
  S.uebergabenLoading = true;
  try { S.uebergaben = await api('GET', '/api/admin/uebergaben'); } catch (e) { handleErr(e); S.uebergaben = []; }
  S.uebergabenLoading = false;
  render();
}

function uebergabenCard() {
  if (S.uebergaben == null) { if (!S.uebergabenLoading) loadUebergaben(); return null; }
  if (!S.uebergaben.length) return null;
  return h('div', { class: 'card', style: 'margin-top:16px' },
    h('h3', { style: 'margin-top:0' }, 'Gartenübergaben'),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Datum', 'Garten', 'Bisher', 'Neu', 'Ablöse', ''].map((x) => h('th', null, x)))),
      h('tbody', null, S.uebergaben.map((u) => h('tr', null,
        h('td', null, deDate(u.datum)), h('td', null, u.gartennr), h('td', null, u.altName), h('td', null, u.neuName),
        h('td', { class: 'num' }, u.abloese ? eur(u.abloese) : '–'),
        h('td', null, h('button', { class: 'btn small', onclick: () => protokollDrucken(u) }, 'Protokoll drucken'))))))));
}

// ------------------------------------------------------------------ Admin: Warteliste

async function loadWarteliste() {
  if (S.wartelisteLoading) return;
  S.wartelisteLoading = true;
  try { S.warteliste = await api('GET', '/api/admin/warteliste'); } catch (e) { handleErr(e); S.warteliste = []; }
  S.wartelisteLoading = false;
  render();
}

function bewerberDialog(b) {
  const isNew = !b;
  const cur = b || { seit: todayIso(), anrede: '', name: '', strasse: '', plzOrt: S.state.settings.ort, email: '', telefon: '', wunsch: '', notiz: '', bestaetigtAm: '' };
  const t = (val, ph, typ) => h('input', { type: typ || 'text', value: val || '', placeholder: ph || '', maxlength: 100, autocomplete: 'off' });
  const f = {
    seit: t(cur.seit, '', 'date'), bestaetigt: t(cur.bestaetigtAm, '', 'date'),
    anrede: h('select', { value: cur.anrede }, ['Herr', 'Frau', 'Familie', 'Herr und Frau', ''].map((a) => h('option', { value: a, selected: a === cur.anrede }, a || '(keine)'))),
    name: t(cur.name), strasse: t(cur.strasse), plzOrt: t(cur.plzOrt),
    email: t(cur.email, 'name@beispiel.de', 'email'), telefon: t(cur.telefon, '', 'tel'),
    wunsch: t(cur.wunsch, 'z. B. klein, mit Laube'),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%' }, cur.notiz || ''),
    bAm: t(cur.besichtigungAm, '', 'date'), bUm: t(cur.besichtigungUm, '', 'time'), bGarten: t(cur.besichtigungGarten, 'z. B. 12'),
    bErg: h('select', null, [['', 'noch offen'], ['besichtigt', 'besichtigt'], ['möchte den Garten', 'möchte den Garten'], ['kein Interesse', 'kein Interesse'], ['nicht erschienen', 'nicht erschienen']]
      .map(([v, l]) => h('option', { value: v, selected: v === (cur.besichtigungErgebnis || '') }, l))),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Angemeldet am *', f.seit, 'bestimmt den Platz auf der Liste'), fld('Interesse zuletzt bestätigt am', f.bestaetigt),
    fld('Anrede', f.anrede), fld('Name *', f.name), fld('Straße und Hausnummer', f.strasse), fld('PLZ und Ort', f.plzOrt),
    fld('E-Mail-Adresse', f.email), fld('Telefon', f.telefon), fld('Wunsch', f.wunsch, null, true),
    fld('Notiz (nur intern – bitte keine sensiblen Angaben)', f.notiz, null, true),
    h('div', { class: 'section-title' }, 'Besichtigung eines freien Gartens'),
    fld('Besichtigung am', f.bAm), fld('Uhrzeit', f.bUm), fld('Garten', f.bGarten), fld('Ergebnis', f.bErg));
  return modal(isNew ? 'Auf die Warteliste setzen' : `Warteliste – ${cur.name}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const data = { seit: f.seit.value, bestaetigtAm: f.bestaetigt.value, anrede: f.anrede.value, name: f.name.value, strasse: f.strasse.value, plzOrt: f.plzOrt.value,
        email: f.email.value.trim(), telefon: f.telefon.value, wunsch: f.wunsch.value, notiz: f.notiz.value.trim(),
        besichtigungAm: f.bAm.value, besichtigungUm: f.bUm.value, besichtigungGarten: f.bGarten.value, besichtigungErgebnis: f.bErg.value };
      try {
        if (isNew) await api('POST', '/api/admin/warteliste', data);
        else await api('PUT', `/api/admin/warteliste/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

async function gartenVergeben(b) {
  const st = S.state;
  const gaerten = [...st.paechter].filter((p) => !p.ausgeschiedenAm && p.gartennr).sort((x, y) => x.gartennr.localeCompare(y.gartennr, 'de', { numeric: true }));
  if (!gaerten.length) { toast('Es gibt keinen Garten mit Gartennummer, der übergeben werden könnte.', 'err'); return; }
  const sel = h('select', { style: 'min-width:280px' }, gaerten.map((p) => h('option', { value: p.id }, `Garten ${p.gartennr} – bisher ${p.name}`)));
  const ok = await modal(`Garten an ${b.name} vergeben`, h('div', null,
    h('p', null, 'Welcher Garten wird frei?'), sel,
    h('p', { class: 'hint' }, 'Im nächsten Schritt trägst du Übergabetag und Zählerstände ein. Die Angaben von der Warteliste sind dort schon ausgefüllt.')),
  [{ label: 'Abbrechen', value: false }, { label: 'Weiter', cls: 'primary', value: true }]);
  if (!ok) return;
  const alt = gaerten.find((p) => p.id === sel.value);
  if (await paechterwechselDialog(alt, b)) { S.warteliste = null; render(); }
}

function adminWarteliste() {
  if (S.warteliste == null) {
    if (!S.wartelisteLoading) loadWarteliste();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const l = S.warteliste;
  const nachfragen = l.filter((b) => b.nachfragen).length;
  const neu = async () => { if (await bewerberDialog(null)) { S.warteliste = null; render(); } };
  const bestaetigen = async (b) => {
    try { await api('PUT', `/api/admin/warteliste/${b.id}`, { ...b, bestaetigtAm: todayIso() }); toast('Interesse als bestätigt vermerkt', 'ok'); S.warteliste = null; render(); } catch (e) { handleErr(e); }
  };
  const loeschen = async (b) => {
    if (!(await confirmBox(`${b.name} von der Warteliste löschen? Die Angaben werden auch aus den Sicherungen entfernt. Das kann nicht rückgängig gemacht werden.`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/warteliste/${b.id}`); toast('Gelöscht', 'ok'); S.warteliste = null; render(); } catch (e) { handleErr(e); }
  };
  return h('div', null,
    h('p', { class: 'hint' }, 'Interessenten für einen Garten, in der Reihenfolge ihrer Anmeldung. Wird ein Garten frei, klickst du beim nächsten auf »Garten vergeben«: Der Pächterwechsel übernimmt Name und Kontakt und nimmt den Eintrag von der Liste.'),
    nachfragen ? h('div', { class: 'banner warn' }, `${nachfragen} Eintrag/Einträge seit über 2 Jahren nicht bestätigt. Bitte nachfragen, ob noch Interesse besteht, und sonst löschen – nicht mehr benötigte Daten sollen nicht unbegrenzt liegen bleiben.`) : null,
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: neu }, '+ Auf die Warteliste setzen'),
      h('span', { class: 'stat' }, `${l.length} Interessent(en)`)),
    l.length ? h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Platz', 'Name und Wunsch', 'Kontakt', 'Angemeldet', 'Besichtigung', ''].map((x) => h('th', null, x)))),
      h('tbody', null, l.map((b) => h('tr', null,
        h('td', { class: 'num' }, `${b.platz}.`),
        h('td', { class: 'wrap' }, h('b', null, [b.anrede, b.name].filter(Boolean).join(' ')), b.notiz ? h('span', { title: b.notiz, style: 'margin-left:4px;cursor:help' }, '📝') : null,
          b.wunsch ? h('div', { class: 'hint' }, b.wunsch) : null),
        h('td', { class: 'wrap klein' }, [b.telefon, b.email, [b.strasse, b.plzOrt].filter(Boolean).join(', ')].filter(Boolean).map((x, i) => [i ? h('br') : null, x])),
        h('td', { class: 'klein' }, deDate(b.seit), h('div', { class: 'hint' }, b.bestaetigtAm ? `bestätigt ${deDate(b.bestaetigtAm)}` : 'nicht bestätigt'),
          b.nachfragen ? h('span', { class: 'badge warn' }, 'nachfragen') : null),
        h('td', { class: 'klein' }, b.besichtigungAm ? [`${deDate(b.besichtigungAm)}${b.besichtigungUm ? ' ' + b.besichtigungUm : ''}`,
          h('div', { class: 'hint' }, b.besichtigungGarten ? 'Garten ' + b.besichtigungGarten : ''),
          h('span', { class: 'badge ' + (b.besichtigungErgebnis === 'möchte den Garten' ? 'ok' : b.besichtigungErgebnis ? 'neu' : 'warn') }, b.besichtigungErgebnis || (b.besichtigungAm >= todayIso() ? 'geplant' : 'Ergebnis eintragen'))] : '–'),
        h('td', { class: 'aktionen' },
          h('button', { class: 'btn small primary', onclick: () => gartenVergeben(b) }, 'Garten vergeben'), ' ',
          h('button', { class: 'btn small', onclick: async () => { if (await bewerberDialog(b)) { S.warteliste = null; S.dashWarteliste = null; render(); } } }, 'Bearbeiten'), ' ',
          mehrMenue([
            { label: 'Interesse heute bestätigt', aktion: () => bestaetigen(b) },
            { label: 'Von der Liste löschen', gefahr: true, aktion: () => loeschen(b) }])))))))
      : leer('personen', 'Niemand auf der Warteliste', 'Wer sich für einen Garten interessiert, kommt hier auf die Liste. Wird ein Garten frei, siehst du sofort, wer als Nächstes dran ist.', '+ Auf die Warteliste setzen', neu));
}

// ------------------------------------------------------------------ Tab: Mängel (Gartenbegehung)

const MANGEL_ARTEN = ['Pflege und Unkraut', 'Hecke und Bäume', 'Laube und Bauten', 'Abfall und Lagerung', 'Wege und Zäune', 'Wasser und Strom', 'Sonstiges'];

async function loadMaengel() {
  if (S.maengelLoading) return;
  S.maengelLoading = true;
  try { S.maengel = await api('GET', '/api/admin/maengel'); } catch (e) { handleErr(e); S.maengel = []; }
  S.maengelLoading = false;
  render();
}

const begehungslisteDrucken = () => pdfDrucken({
  url: '/api/admin/begehungsliste', oeffnenUrl: '/api/admin/begehungsliste/oeffnen',
  titel: 'Begehungsliste', meldung: () => 'Die Begehungsliste wurde im PDF-Programm geöffnet – dort drucken und zur Begehung mitnehmen.',
});
const maengelschreibenDrucken = (nr) => pdfDrucken({
  url: `/api/admin/maengelschreiben?gartennr=${encodeURIComponent(nr)}`,
  oeffnenUrl: `/api/admin/maengelschreiben/oeffnen?gartennr=${encodeURIComponent(nr)}`,
  titel: `Aufforderung Garten ${nr}`, meldung: () => 'Das Schreiben wurde im PDF-Programm geöffnet – dort drucken oder als PDF verschicken.',
});

function mangelDialog(m, gartennr) {
  const isNew = !m || !m.id;
  const cur = { gartennr: gartennr || '', datum: todayIso(), art: 'Pflege und Unkraut', beschreibung: '', frist: '', erledigtAm: '', ...(m || {}) };
  const f = {
    gartennr: h('input', { type: 'text', value: cur.gartennr, maxlength: 20, style: 'width:6em', placeholder: 'z. B. 12' }),
    datum: h('input', { type: 'date', value: cur.datum }),
    art: h('select', null, MANGEL_ARTEN.map((a) => h('option', { value: a, selected: a === cur.art }, a))),
    beschreibung: h('textarea', { rows: 3, maxlength: 300, style: 'width:100%', placeholder: 'Was ist zu beseitigen? z. B. Hecke zur Wegseite höher als 1,25 m, bitte zurückschneiden' }, cur.beschreibung),
    frist: h('input', { type: 'date', value: cur.frist || (isNew ? isoInTagen(14) : '') }),
    erledigt: h('input', { type: 'date', value: cur.erledigtAm }),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Garten *', f.gartennr), fld('Begehung am *', f.datum), fld('Art', f.art),
    fld('Beschreibung *', f.beschreibung, null, true),
    fld('Frist zur Beseitigung', f.frist), fld('Erledigt am', f.erledigt, 'leer = noch offen'));
  return modal(isNew ? 'Mangel eintragen' : 'Mangel bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const data = { gartennr: f.gartennr.value, datum: f.datum.value, art: f.art.value, beschreibung: f.beschreibung.value, frist: f.frist.value, erledigtAm: f.erledigt.value };
      try {
        if (isNew) await api('POST', '/api/admin/maengel', data); else await api('PUT', `/api/admin/maengel/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

async function loadNutzung() {
  if (S.nutzungLoading) return;
  S.nutzungLoading = true;
  try { S.nutzung = await api('GET', '/api/admin/nutzung'); } catch (e) { handleErr(e); S.nutzung = []; }
  S.nutzungLoading = false;
  render();
}

function nutzungDialog(nr, letzte) {
  const zahl = (v) => h('input', { type: 'text', inputmode: 'decimal', value: v == null ? '' : String(v), style: 'width:5em' });
  const f = { datum: h('input', { type: 'date', value: todayIso() }), anbau: zahl(letzte ? letzte.anbau : ''), erholung: zahl(letzte ? letzte.erholung : ''),
    bebauung: zahl(letzte ? letzte.bebauung : ''), notiz: h('input', { type: 'text', maxlength: 200, placeholder: 'z. B. Rasen statt Beete hinter der Laube' }) };
  const summe = h('b', null);
  const rechnen = () => {
    const w = [f.anbau, f.erholung, f.bebauung].map((x) => parseNum(x.value) || 0);
    const s = w.reduce((a, b) => a + b, 0);
    summe.textContent = `Summe ${nfFlex.format(s)} %${w[0] < 100 / 3 ? ' · Anbau unter einem Drittel' : ''}`;
    summe.className = w[0] < 100 / 3 || Math.abs(s - 100) > 10 ? 'warn-text' : '';
  };
  [f.anbau, f.erholung, f.bebauung].forEach((x) => x.addEventListener('input', rechnen));
  rechnen();
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(`Nutzung Garten ${nr} schätzen`, h('div', null,
    h('p', { class: 'hint' }, 'Grob geschätzte Anteile der Gartenfläche in Prozent. In der Regel soll mindestens ein Drittel dem Anbau von Obst und Gemüse dienen.'),
    h('div', { class: 'form' }, fld('Begehung am', f.datum, null, true),
      fld('Anbau %', f.anbau, 'Obst, Gemüse, Kräuter, Beeren'), fld('Erholung %', f.erholung, 'Rasen, Zierpflanzen, Sitzplatz'), fld('Bebauung %', f.bebauung, 'Laube, Wege, befestigte Flächen'),
      fld('Notiz', f.notiz, null, true)), h('p', null, summe)),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try {
      await api('POST', '/api/admin/nutzung', { gartennr: nr, datum: f.datum.value, anbau: parseNum(f.anbau.value) ?? 0, erholung: parseNum(f.erholung.value) ?? 0,
        bebauung: parseNum(f.bebauung.value) ?? 0, notiz: f.notiz.value });
      toast('Gespeichert', 'ok');
    } catch (e) { handleErr(e); return false; }
  } }]);
}

function nutzungKarte() {
  if (S.nutzung == null) { if (!S.nutzungLoading) loadNutzung(); return null; }
  const l = S.nutzung;
  const geschaetzt = l.filter((z) => z.letzte), unter = l.filter((z) => z.unterDrittel);
  const balken = (n) => h('div', { class: 'nutzung-balken', title: `Anbau ${n.anbau} % · Erholung ${n.erholung} % · Bebauung ${n.bebauung} %` },
    h('span', { class: 'n-anbau', style: `width:${n.anbau}%` }), h('span', { class: 'n-erholung', style: `width:${n.erholung}%` }), h('span', { class: 'n-bebauung', style: `width:${n.bebauung}%` }),
    h('i', { class: 'n-drittel' }));
  const schaetzen = async (z) => { if (await nutzungDialog(z.gartennr, z.letzte)) { S.nutzung = null; render(); } };
  return h('details', { class: 'card', style: 'margin-top:12px', open: unter.length > 0 },
    h('summary', null, h('b', null, 'Kleingärtnerische Nutzung (Drittelregelung)'),
      h('span', { class: 'hint' }, ` · ${geschaetzt.length} von ${l.length} Gärten geschätzt`), unter.length ? h('span', { class: 'badge warn', style: 'margin-left:8px' }, `${unter.length} unter einem Drittel Anbau`) : null),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Bei der Begehung grob schätzen, wie sich die Fläche aufteilt. Die Linie im Balken markiert ein Drittel. Das ist eine Gesprächsgrundlage, keine Vermessung – maßgeblich sind Satzung, Gartenordnung und die Vorgaben eures Verbands.'),
    h('div', { class: 'nutzung-legende hint' }, h('span', { class: 'n-anbau' }), ' Anbau ', h('span', { class: 'n-erholung' }), ' Erholung ', h('span', { class: 'n-bebauung' }), ' Bebauung'),
    h('div', { class: 'tablewrap' }, h('table', null,
      h('thead', null, h('tr', null, ['Garten', 'Pächter', 'Aufteilung', 'Anbau', 'Geschätzt', ''].map((x) => h('th', null, x)))),
      h('tbody', null, l.map((z) => h('tr', null, h('td', null, z.gartennr), h('td', null, z.name),
        h('td', { style: 'min-width:180px' }, z.letzte ? balken(z.letzte) : h('span', { class: 'hint' }, 'noch nicht geschätzt')),
        h('td', { class: 'num' }, z.letzte ? [`${z.letzte.anbau} %`, z.unterDrittel ? h('span', { class: 'badge warn', style: 'margin-left:6px' }, 'unter ⅓') : null] : '–'),
        h('td', { class: 'klein' }, z.letzte ? deDate(z.letzte.datum) : ''),
        h('td', null, h('button', { class: 'btn small', onclick: () => schaetzen(z) }, 'Schätzen'))))))));
}

// Begehung durchführen: Garten für Garten Mängel ankreuzen und die Nutzung schätzen, gespeichert wird beim Weitergehen.
async function begehungDurchfuehren() {
  let nutzung;
  try { nutzung = await api('GET', '/api/admin/nutzung'); } catch (e) { handleErr(e); return; }
  if (!nutzung.length) { toast('Es gibt noch keine Gärten mit Pächtern.', 'err'); return; }
  const offen = (S.maengel || []).filter((m) => !m.erledigtAm);
  let i = 0, gespeichert = 0;
  const datum = todayIso();
  const frist = isoInTagen(14);
  const box = h('div');
  const zeichne = () => {
    const z = nutzung[i];
    const alt = offen.filter((m) => m.gartennr === z.gartennr);
    const arten = MANGEL_ARTEN.map((a) => ({ art: a, cb: h('input', { type: 'checkbox' }), text: h('input', { type: 'text', maxlength: 300, placeholder: 'Was ist zu tun?', style: 'display:none;width:100%' }) }));
    arten.forEach((a) => a.cb.addEventListener('change', () => { a.text.style.display = a.cb.checked ? '' : 'none'; if (a.cb.checked) a.text.focus(); }));
    const zahl = (v) => h('input', { type: 'text', inputmode: 'decimal', value: v == null ? '' : String(v), style: 'width:5em' });
    const l = z.letzte;
    const nf = { anbau: zahl(l ? l.anbau : ''), erholung: zahl(l ? l.erholung : ''), bebauung: zahl(l ? l.bebauung : '') };
    const speichern = async () => {
      try {
        for (const a of arten.filter((x) => x.cb.checked)) {
          await api('POST', '/api/admin/maengel', { gartennr: z.gartennr, datum, art: a.art, beschreibung: a.text.value.trim() || a.art, frist, erledigtAm: '' });
          gespeichert++;
        }
        const w = Object.fromEntries(Object.entries(nf).map(([k, el]) => [k, el.value.trim() === '' ? null : parseNum(el.value)]));
        if (Object.values(w).some((v) => Number.isNaN(v))) { toast('Bitte die Nutzung in Prozent als Zahl eintragen.', 'err'); return false; }
        const geaendert = !l ? Object.values(w).some((v) => v != null) : ['anbau', 'erholung', 'bebauung'].some((k) => (w[k] ?? 0) !== l[k]);
        if (geaendert) await api('POST', '/api/admin/nutzung', { gartennr: z.gartennr, datum, anbau: w.anbau ?? 0, erholung: w.erholung ?? 0, bebauung: w.bebauung ?? 0, notiz: '' });
      } catch (e) { handleErr(e); return false; }
      return true;
    };
    const gehe = async (schritt, sichern) => {
      if (sichern && !(await speichern())) return;
      i = Math.min(nutzung.length - 1, Math.max(0, i + schritt));
      zeichne();
    };
    const letzter = i === nutzung.length - 1;
    box.replaceChildren(
      h('div', { class: 'begehung-kopf' }, h('span', { class: 'big' }, `Garten ${z.gartennr}`), h('span', null, z.name), h('span', { class: 'hint' }, `${i + 1} von ${nutzung.length}`)),
      alt.length ? h('div', { class: 'banner warn', style: 'margin-top:8px' }, 'Noch offen: ', alt.map((m) => `${m.art} – ${m.beschreibung}${m.frist ? ` (Frist ${deDate(m.frist)})` : ''}`).join(' · ')) : null,
      h('h4', null, 'Neue Mängel'),
      h('div', { class: 'begehung-maengel' }, arten.map((a) => h('div', null, h('label', null, a.cb, ' ', a.art), a.text))),
      h('h4', null, 'Nutzung in % (Drittelregelung)'),
      h('div', { class: 'toolbar' }, h('label', null, 'Anbau ', nf.anbau), h('label', null, 'Erholung ', nf.erholung), h('label', null, 'Bebauung ', nf.bebauung),
        l ? h('span', { class: 'hint' }, `zuletzt ${deDate(l.datum)}`) : null),
      h('div', { class: 'toolbar', style: 'margin-top:14px' },
        h('button', { class: 'btn', disabled: i === 0, onclick: () => gehe(-1, false) }, '← Zurück'),
        h('div', { class: 'spacer' }),
        h('button', { class: 'btn primary', onclick: async () => { if (letzter) { if (await speichern()) { box.replaceChildren(h('p', null, h('b', null, 'Begehung abgeschlossen.'), ` ${gespeichert} Mängel eingetragen.`)); } } else gehe(1, true); } },
          letzter ? 'Speichern und abschließen' : 'Speichern und weiter →')),
      h('p', { class: 'hint' }, `Neue Mängel bekommen die Frist ${deDate(frist)}. Ohne Häkchen und ohne Änderung wird nichts gespeichert.`));
  };
  zeichne();
  await breiterDialog(`Begehung am ${deDate(datum)}`, box, [{ label: 'Beenden', value: true }]);
  if (gespeichert) toast(`${gespeichert} Mängel eingetragen`, 'ok');
  S.maengel = null; S.nutzung = null; S.aufgaben = null;
  render();
}

function adminMaengel() {
  if (S.maengel == null) {
    if (!S.maengelLoading) loadMaengel();
    return h('p', { class: 'hint' }, 'Wird geladen …');
  }
  const heute = todayIso();
  const alle = S.maengel;
  const offen = alle.filter((m) => !m.erledigtAm);
  const erledigt = alle.filter((m) => m.erledigtAm);
  const neu = async (nr) => { if (await mangelDialog(null, nr)) { S.maengel = null; render(); } };
  const bearbeiten = async (m) => { if (await mangelDialog(m)) { S.maengel = null; render(); } };
  const speichern = async (m, aenderung) => {
    try { await api('PUT', `/api/admin/maengel/${m.id}`, { gartennr: m.gartennr, datum: m.datum, art: m.art, beschreibung: m.beschreibung, frist: m.frist || '', erledigtAm: m.erledigtAm || '', ...aenderung }); S.maengel = null; render(); } catch (e) { handleErr(e); }
  };
  const loeschen = async (m) => {
    if (!(await confirmBox('Diesen Mangel löschen?', 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/maengel/${m.id}`); S.maengel = null; render(); } catch (e) { handleErr(e); }
  };
  const zeile = (m) => h('li', { class: 'mangel' + (m.erledigtAm ? ' erledigt' : '') },
    h('div', { class: 'mangel-text' }, h('b', null, m.art), h('div', null, m.beschreibung),
      h('div', { class: 'hint' }, `Begehung ${deDate(m.datum)}`, m.frist && !m.erledigtAm ? ` · Frist ${deDate(m.frist)}` : '', m.erledigtAm ? ` · erledigt ${deDate(m.erledigtAm)}` : '')),
    !m.erledigtAm && m.frist && m.frist < heute ? h('span', { class: 'badge warn' }, 'Frist abgelaufen') : null,
    h('div', { class: 'termin-aktionen' },
      m.erledigtAm ? h('button', { class: 'btn small', onclick: () => speichern(m, { erledigtAm: '' }) }, 'Wieder öffnen')
        : h('button', { class: 'btn small primary', onclick: () => speichern(m, { erledigtAm: heute }) }, 'Erledigt'),
      h('button', { class: 'btn small', onclick: () => bearbeiten(m) }, 'Bearbeiten'),
      mehrMenue([{ label: 'Löschen', gefahr: true, aktion: () => loeschen(m) }])));
  const gruppen = new Map();
  for (const m of offen) { if (!gruppen.has(m.gartennr)) gruppen.set(m.gartennr, []); gruppen.get(m.gartennr).push(m); }
  const garten = [...gruppen.entries()].sort((a, b) => a[0].localeCompare(b[0], 'de', { numeric: true }));
  return h('div', null,
    h('p', { class: 'hint' }, 'Beim Rundgang gefundene Mängel je Garten mit Frist. Mit der Begehungsliste gehst du durch die Anlage, hier trägst du die Mängel ein und schreibst dem Pächter eine Aufforderung. Nur der Vorstand sieht diese Liste.'),
    h('div', { class: 'toolbar' },
      h('button', { class: 'btn primary', onclick: () => neu('') }, '+ Mangel eintragen'),
      h('button', { class: 'btn', onclick: begehungslisteDrucken }, 'Begehungsliste drucken'),
      h('button', { class: 'btn', onclick: begehungDurchfuehren, title: 'Garten für Garten durchgehen und Mängel und Nutzung gleich eintragen, z. B. mit dem Laptop beim Rundgang' }, 'Begehung durchführen'),
      h('div', { class: 'spacer' }),
      h('span', { class: 'stat' }, `${offen.length} offen · ${erledigt.length} erledigt`)),
    garten.length ? garten.map(([nr, l]) => h('div', { class: 'card mangel-garten' },
      h('div', { class: 'fs-kopf' }, h('b', null, `Garten ${nr}`, l[0].name ? ` · ${l[0].name}` : ''),
        h('span', { class: 'toolbar-gruppe' },
          h('button', { class: 'btn small', onclick: () => neu(nr) }, '+ Mangel'),
          h('button', { class: 'btn small primary', onclick: () => maengelschreibenDrucken(nr) }, 'Aufforderung schreiben'))),
      h('ul', { class: 'mangel-liste' }, l.map(zeile))))
      : h('div', { class: 'card' }, leer('fertig', 'Keine offenen Mängel', 'Bei der nächsten Begehung gefundene Mängel trägst du hier ein. Die Begehungsliste zum Ausdrucken liegt oben bereit.', '+ Mangel eintragen', () => neu(''))),
    nutzungKarte(),
    erledigt.length ? h('details', { class: 'card', style: 'margin-top:12px' }, h('summary', null, `Erledigt (${erledigt.length})`),
      h('ul', { class: 'mangel-liste' }, erledigt.map((m) => h('li', { class: 'mangel erledigt' }, h('div', { class: 'mangel-text' },
        h('b', null, `Garten ${m.gartennr} · ${m.art}`), h('div', null, m.beschreibung), h('div', { class: 'hint' }, `erledigt ${deDate(m.erledigtAm)}`)),
      h('div', { class: 'termin-aktionen' }, h('button', { class: 'btn small', onclick: () => speichern(m, { erledigtAm: '' }) }, 'Wieder öffnen'),
        mehrMenue([{ label: 'Löschen', gefahr: true, aktion: () => loeschen(m) }])))))) : null);
}

// ------------------------------------------------------------------ Tab: Termine (Jahresplaner)

const TERMIN_ARTEN = ['Arbeitseinsatz', 'Versammlung', 'Ablesung', 'Fest', 'Vorstand', 'Sonstiges'];
const MONATE = ['Januar', 'Februar', 'März', 'April', 'Mai', 'Juni', 'Juli', 'August', 'September', 'Oktober', 'November', 'Dezember'];
const WOCHENTAGE = ['So', 'Mo', 'Di', 'Mi', 'Do', 'Fr', 'Sa'];
const isoDatum = (iso) => { const [y, m, d] = iso.split('-').map(Number); return new Date(y, m - 1, d); };
const terminZeit = (t) => (t.von ? `${t.von}${t.bis ? '–' + t.bis : ''} Uhr` : 'ganztägig');

function terminDialog(t) {
  const isNew = !t || !t.id;
  const cur = { datum: todayIso(), von: '', bis: '', titel: '', art: 'Arbeitseinsatz', ort: '', notiz: '', tops: '', ...(t || {}) };
  const f = {
    datum: h('input', { type: 'date', value: cur.datum }),
    von: h('input', { type: 'time', value: cur.von }), bis: h('input', { type: 'time', value: cur.bis }),
    titel: h('input', { type: 'text', value: cur.titel, maxlength: 100, placeholder: 'z. B. Frühjahrsarbeitseinsatz' }),
    art: h('select', null, TERMIN_ARTEN.map((a) => h('option', { value: a, selected: a === cur.art }, a))),
    ort: h('input', { type: 'text', value: cur.ort, maxlength: 100, placeholder: 'z. B. Vereinsheim' }),
    notiz: h('textarea', { rows: 2, maxlength: 300, style: 'width:100%', placeholder: 'z. B. Bitte Arbeitshandschuhe mitbringen' }, cur.notiz || ''),
    tops: h('textarea', { rows: 5, maxlength: 3000, style: 'width:100%', placeholder: 'Ein Punkt pro Zeile, z. B.\nBegrüßung\nKassenbericht\nVerschiedenes' }, cur.tops || ''),
  };
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  const body = h('div', { class: 'form' },
    fld('Titel *', f.titel, null, true), fld('Datum *', f.datum), fld('Art', f.art),
    fld('Beginn', f.von, 'leer = ganztägig'), fld('Ende', f.bis), fld('Ort', f.ort),
    fld('Hinweis (für alle sichtbar – keine Namen oder persönlichen Angaben)', f.notiz, null, true),
    fld('Tagesordnung für die Einladung (optional)', f.tops, 'Leer lassen: bei Versammlungen werden die üblichen Punkte eingesetzt. Für alle sichtbar – keine Namen.', true));
  return modal(isNew ? 'Termin eintragen' : 'Termin bearbeiten', body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const data = { datum: f.datum.value, von: f.von.value, bis: f.bis.value, titel: f.titel.value, art: f.art.value, ort: f.ort.value, notiz: f.notiz.value.trim(), tops: f.tops.value };
      try {
        if (isNew) await api('POST', '/api/admin/termine', data);
        else await api('PUT', `/api/admin/termine/${cur.id}`, data);
        toast('Gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } },
  ]);
}

// Jahresbericht für die Mitgliederversammlung: Kennzahlen, Kasse, Ausgaben, Verlauf, Termine – ohne Namen.
const jahresberichtDrucken = (jahr) => pdfDrucken({
  url: `/api/admin/jahresbericht?year=${jahr}`, oeffnenUrl: `/api/admin/jahresbericht/oeffnen?year=${jahr}`,
  titel: `Jahresbericht ${jahr}`, meldung: () => 'Der Jahresbericht wurde im PDF-Programm geöffnet – dort drucken.',
});

const jahresplanDrucken = (jahr) => pdfDrucken({
  url: `/api/termine-druck?jahr=${jahr}`, oeffnenUrl: `/api/termine-druck/oeffnen?jahr=${jahr}`,
  titel: `Vereinstermine ${jahr}`, meldung: () => 'Der Jahresplan wurde im PDF-Programm geöffnet – dort drucken oder aushängen.',
});

const einladungDrucken = (t) => pdfDrucken({
  url: `/api/admin/termine/${t.id}/einladung`, oeffnenUrl: `/api/admin/termine/${t.id}/einladung/oeffnen`,
  titel: `Einladung ${t.titel}`, meldung: () => 'Die Einladung wurde im PDF-Programm geöffnet – dort drucken oder per E-Mail verschicken.',
});
const anwesenheitDrucken = (t) => pdfDrucken({
  url: `/api/admin/termine/${t.id}/anwesenheit`, oeffnenUrl: `/api/admin/termine/${t.id}/anwesenheit/oeffnen`,
  titel: `Anwesenheitsliste ${t.titel}`, meldung: () => 'Die Anwesenheitsliste wurde im PDF-Programm geöffnet – dort drucken.',
});

const stundenlisteDrucken = (t) => pdfDrucken({
  url: `/api/admin/termine/${t.id}/stundenliste`, oeffnenUrl: `/api/admin/termine/${t.id}/stundenliste/oeffnen`,
  titel: `Stundenliste ${t.titel}`, meldung: () => 'Die Stundenliste wurde im PDF-Programm geöffnet – dort drucken und zum Einsatz mitnehmen.',
});

// zeitspanneStunden versteht »9-11:30«, »09:00–12:00« oder »9 bis 12« und liefert die Stunden (sonst null).
function zeitspanneStunden(text) {
  const m = String(text).trim().match(/^(\d{1,2})(?:[:.](\d{2}))?\s*(?:-|–|bis)\s*(\d{1,2})(?:[:.](\d{2}))?$/i);
  if (!m) return null;
  const von = Number(m[1]) * 60 + Number(m[2] || 0), bis = Number(m[3]) * 60 + Number(m[4] || 0);
  if (Number(m[1]) > 23 || Number(m[3]) > 24 || Number(m[2] || 0) > 59 || Number(m[4] || 0) > 59 || bis <= von) return null;
  return Math.round((bis - von) / 60 * 100) / 100;
}
// Stunden: Zahl (»2,5«) oder Zeitspanne (»9-11:30«)
const stundenWert = (text) => { const z = zeitspanneStunden(text); return z != null ? z : parseNum(text); };

// Stunden eines Arbeitseinsatzes abhaken: wer da war, bekommt die Dauer des Termins vorgeschlagen.
async function stundenDialog(t) {
  let v;
  try { v = await api('GET', `/api/admin/termine/${t.id}/stunden`); } catch (e) { handleErr(e); return false; }
  const zeilen = v.zeilen.map((z) => {
    const std = h('input', { type: 'text', inputmode: 'decimal', class: 'std-in', value: z.stunden != null ? nfFlex.format(z.stunden) : '', placeholder: '–', 'aria-label': `Stunden ${z.name}`, title: 'Stunden (z. B. 2,5) oder Zeit von–bis (z. B. 9-11:30)' });
    const dabei = h('input', { type: 'checkbox', checked: z.stunden != null, 'aria-label': `${z.name} war dabei` });
    dabei.addEventListener('change', () => {
      if (dabei.checked && !std.value) std.value = v.dauer ? nfFlex.format(v.dauer) : '';
      if (!dabei.checked) std.value = '';
      summe();
    });
    std.addEventListener('input', () => { dabei.checked = std.value.trim() !== ''; summe(); });
    // »9-11:30« beim Verlassen des Feldes in Stunden umrechnen, damit man sieht, was gebucht wird
    std.addEventListener('blur', () => { const z = zeitspanneStunden(std.value); if (z != null) { std.value = nfFlex.format(z); summe(); } });
    const bisher = (z.gesamt || 0) - (z.stunden || 0);
    return { z, std, dabei, tr: h('tr', null, h('td', null, dabei), h('td', null, z.gartennr), h('td', null, z.name), h('td', null, std),
      h('td', { class: 'num klein' }, `${nfFlex.format(bisher)} h`, v.pflicht && bisher >= v.pflicht ? h('span', { class: 'badge ok', style: 'margin-left:6px' }, 'erfüllt') : null)) };
  });
  const summeEl = h('b', null);
  const summe = () => {
    let n = 0, s = 0;
    for (const x of zeilen) { const w = stundenWert(x.std.value); if (x.dabei.checked && w > 0) { n++; s += w; } }
    summeEl.textContent = `${n} Teilnehmer · ${nfFlex.format(Math.round(s * 100) / 100)} Stunden`;
  };
  summe();
  const jahrSel = h('select', null, v.jahre.map((j) => h('option', { value: j, selected: j === v.jahr }, String(j))));
  const alle = () => { for (const x of zeilen) { if (!x.dabei.checked) { x.dabei.checked = true; if (!x.std.value) x.std.value = v.dauer ? nfFlex.format(v.dauer) : ''; } } summe(); };
  const body = h('div', null,
    h('p', { class: 'hint' }, `${deDate(t.datum)} · ${terminZeit(t)}. Hake ab, wer dabei war – die Dauer des Termins${v.dauer ? ` (${nfFlex.format(v.dauer)} h)` : ''} wird vorgeschlagen. Wer früher gegangen oder später gekommen ist, bekommt seine eigenen Stunden: als Zahl (2,5) oder als Uhrzeit von–bis (9-11:30). Die Stunden werden auf die Gemeinschaftsstunden gebucht; spätere Korrekturen zählen nicht doppelt.`),
    h('div', { class: 'toolbar' }, h('label', null, 'Buchen auf Abrechnungsjahr'), jahrSel, h('button', { class: 'btn small', onclick: alle }, 'Alle waren dabei'), h('div', { class: 'spacer' }), summeEl),
    h('div', { class: 'tablewrap', style: 'max-height:380px;overflow:auto' }, h('table', { class: 'std-tab' },
      h('thead', null, h('tr', null, ['Dabei', 'Garten', 'Name', 'Stunden', 'Bisher im Jahr'].map((x) => h('th', null, x)))),
      h('tbody', null, zeilen.map((x) => x.tr)))));
  return breiterDialog(`Stunden: ${t.titel}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Stunden buchen', cls: 'primary', value: true, action: async () => {
      const eintraege = [];
      for (const x of zeilen) {
        if (!x.dabei.checked) continue;
        const w = stundenWert(x.std.value);
        if (w == null || Number.isNaN(w) || w < 0 || w > 24) { toast(`Bitte für ${x.z.name} Stunden (z. B. 2,5) oder die Zeit (z. B. 9-11:30) eintragen.`, 'err'); x.std.focus(); return false; }
        eintraege.push({ paechterId: x.z.paechterId, stunden: w });
      }
      try {
        const r = await api('PUT', `/api/admin/termine/${t.id}/stunden`, { jahr: Number(jahrSel.value), eintraege });
        toast(`${r.teilnehmer} Teilnehmer, ${nfFlex.format(r.stunden)} Stunden gebucht`, 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

function viewTermine() {
  const st = S.state;
  const istAdmin = !st.hasPassword || st.loggedIn;
  const heute = todayIso();
  const jahr = S.termineJahr || new Date().getFullYear();
  const jahre = new Set([new Date().getFullYear(), new Date().getFullYear() + 1, ...st.termine.map((t) => Number(t.datum.slice(0, 4)))]);
  const imJahr = st.termine.filter((t) => t.datum.startsWith(jahr + '-'));
  const vorjahr = st.termine.filter((t) => t.datum.startsWith((jahr - 1) + '-')).length;
  const jahrSel = h('select', { onchange: (e) => { S.termineJahr = Number(e.target.value); render(); } },
    [...jahre].sort().map((j) => h('option', { value: j, selected: j === jahr }, String(j))));
  const neu = async (vorlage) => { if (await terminDialog(vorlage)) await reload(); };
  const uebernehmen = async () => {
    try {
      const r = await api('POST', '/api/admin/termine-uebernehmen', { von: jahr - 1, nach: jahr });
      toast(r.uebernommen ? `${r.uebernommen} Termin(e) übernommen – bitte die Daten kurz prüfen.` : 'Alle Termine des Vorjahres sind schon eingetragen.', 'ok');
      await reload();
    } catch (e) { handleErr(e); }
  };
  const loeschen = async (t) => {
    if (!(await confirmBox(`Termin „${t.titel}“ am ${deDate(t.datum)} löschen?`, 'Löschen', true))) return;
    try { await api('DELETE', `/api/admin/termine/${t.id}`); await reload(); } catch (e) { handleErr(e); }
  };
  const gruppen = [];
  for (const t of imJahr) {
    const m = Number(t.datum.slice(5, 7)) - 1;
    if (!gruppen.length || gruppen[gruppen.length - 1].m !== m) gruppen.push({ m, termine: [] });
    gruppen[gruppen.length - 1].termine.push(t);
  }
  const naechster = imJahr.find((t) => t.datum >= heute);
  return h('div', null,
    h('h2', null, `Vereinstermine ${jahr}`),
    h('p', { class: 'hint' }, 'Der Jahresplaner des Vereins: Arbeitseinsätze, Versammlungen, Ablesung, Feste. Als Aushang drucken oder als Kalenderdatei ins Handy übernehmen.'),
    h('div', { class: 'toolbar' },
      h('label', null, 'Jahr'), jahrSel,
      istAdmin ? h('button', { class: 'btn primary', onclick: () => neu({ datum: jahr === new Date().getFullYear() ? heute : `${jahr}-01-01` }) }, '+ Termin eintragen') : null,
      istAdmin && vorjahr ? h('button', { class: 'btn', title: `Alle Termine aus ${jahr - 1} auf denselben Wochentag in ${jahr} übertragen`, onclick: uebernehmen }, `Termine aus ${jahr - 1} übernehmen`) : null,
      h('div', { class: 'spacer' }),
      h('button', { class: 'btn', disabled: !imJahr.length, onclick: () => jahresplanDrucken(jahr) }, 'Als Aushang drucken'),
      h('a', { class: 'btn', href: '/api/termine.ics', download: 'Vereinstermine.ics', title: 'Kalenderdatei für Outlook, Google-Kalender oder das Handy' }, 'In den Kalender')),
    imJahr.length ? h('div', { class: 'termine' }, gruppen.map((g) => h('div', { class: 'card termin-monat' },
      h('h3', null, MONATE[g.m]),
      h('ul', { class: 'termin-liste' }, g.termine.map((t) => {
        const d = isoDatum(t.datum);
        const vorbei = t.datum < heute;
        return h('li', { class: 'termin' + (vorbei ? ' vorbei' : '') + (t === naechster ? ' naechster' : '') },
          h('div', { class: 'termin-tag' }, h('b', null, String(d.getDate())), h('span', null, WOCHENTAGE[d.getDay()])),
          h('div', { class: 'termin-text' },
            h('b', null, t.titel), t === naechster ? h('span', { class: 'badge ok' }, 'als Nächstes') : null,
            h('div', { class: 'hint' }, [terminZeit(t), t.ort].filter(Boolean).join(' · ')),
            t.notiz ? h('div', { class: 'hint' }, t.notiz) : null),
          h('span', { class: 'chip termin-art' }, t.art),
          istAdmin ? h('div', { class: 'termin-aktionen' },
            t.art === 'Versammlung' || t.art === 'Vorstand' ? h('button', { class: 'btn small', title: 'Einladung mit Tagesordnung als PDF', onclick: () => einladungDrucken(t) }, 'Einladung') : null,
            t.art === 'Versammlung' || t.art === 'Vorstand' ? h('button', { class: 'btn small', title: 'Liste aller Pächter zum Unterschreiben', onclick: () => anwesenheitDrucken(t) }, 'Anwesenheit') : null,
            t.art === 'Arbeitseinsatz' ? h('button', { class: 'btn small', title: 'Liste zum Eintragen von Beginn und Ende', onclick: () => stundenlisteDrucken(t) }, 'Stundenliste') : null,
            t.art === 'Arbeitseinsatz' ? h('button', { class: 'btn small primary', title: 'Abhaken, wer dabei war, und die Stunden buchen', onclick: async () => { if (await stundenDialog(t)) await reload(); } }, 'Stunden eintragen') : null,
            h('button', { class: 'btn small', onclick: () => neu(t) }, 'Bearbeiten'),
            h('button', { class: 'btn small danger', onclick: () => loeschen(t) }, 'Löschen')) : null);
      })))))
      : h('div', { class: 'card' }, leer('kalender', `Noch keine Termine für ${jahr}`,
        vorjahr && istAdmin ? `Aus ${jahr - 1} sind ${vorjahr} Termine eingetragen. Mit einem Klick übernimmst du sie auf dieselben Wochentage.` : 'Trag Arbeitseinsätze, Versammlungen und Feste ein. Daraus wird ein Aushang zum Ausdrucken und eine Kalenderdatei.',
        istAdmin ? (vorjahr ? `Termine aus ${jahr - 1} übernehmen` : '+ Termin eintragen') : null,
        vorjahr ? uebernehmen : () => neu({ datum: `${jahr}-01-01` }))));
}

// Übersicht: die nächsten Termine der kommenden acht Wochen
function naechsteTermine(st) {
  const heute = todayIso();
  const bis = new Date(); bis.setDate(bis.getDate() + 56);
  const p = (n) => String(n).padStart(2, '0');
  const grenze = `${bis.getFullYear()}-${p(bis.getMonth() + 1)}-${p(bis.getDate())}`;
  const l = (st.termine || []).filter((t) => t.datum >= heute && t.datum <= grenze).slice(0, 3);
  if (!l.length) return null;
  return h('div', { class: 'card naechste-termine' },
    h('div', { class: 'fs-kopf' }, h('b', null, 'Nächste Termine'),
      h('button', { class: 'btn small', onclick: () => { S.tab = 'termine'; render(); } }, 'Alle Termine →')),
    h('ul', { class: 'termin-liste' }, l.map((t) => {
      const d = isoDatum(t.datum);
      return h('li', { class: 'termin' },
        h('div', { class: 'termin-tag' }, h('b', null, String(d.getDate())), h('span', null, `${WOCHENTAGE[d.getDay()]} · ${MONATE[d.getMonth()].slice(0, 3)}`)),
        h('div', { class: 'termin-text' }, h('b', null, t.titel), h('div', { class: 'hint' }, [terminZeit(t), t.ort].filter(Boolean).join(' · '))),
        h('span', { class: 'chip termin-art' }, t.art));
    })));
}

// ------------------------------------------------------------------ Gartenakte

const DOKUMENT_ARTEN = ['Bauantrag', 'Genehmigung', 'Foto', 'Pachtvertrag', 'Schriftverkehr', 'Protokoll', 'Sonstiges'];
const PERSONEN_ARTEN = new Set(['Pachtvertrag', 'Schriftverkehr']);
const groesseText = (n) => (n >= 1 << 20 ? `${nfFlex.format(Math.round(n / 104857.6) / 10)} MB` : `${Math.max(1, Math.round(n / 1024))} KB`);

// breiter Dialog (für Akte und Wertermittlung)
function breiterDialog(titel, body, knoepfe) {
  const p = modal(titel, body, knoepfe);
  const dlg = document.querySelector('dialog:last-of-type');
  if (dlg) dlg.classList.add('breit');
  return p;
}

async function gartenakteDialog(nr) {
  let ak;
  try { ak = await api('GET', `/api/admin/gartenakte?gartennr=${encodeURIComponent(nr)}`); } catch (e) { handleErr(e); return; }
  const inhalt = h('div', { class: 'akte' });
  const neuLaden = async () => {
    try { ak = await api('GET', `/api/admin/gartenakte?gartennr=${encodeURIComponent(nr)}`); } catch (e) { handleErr(e); return; }
    zeichnen();
  };
  const abschnitt = (ic, titel, knopf, ...kids) => h('section', { class: 'akte-abschnitt' },
    h('div', { class: 'akte-kopf' }, h('span', { class: 'akte-ico' }, icon(ic, 18)), h('b', null, titel), h('div', { class: 'spacer' }), knopf), ...kids);
  const zeichnen = () => {
    const p = ak.paechter;
    const docs = ak.dokumente.map((d) => {
      const bild = /\.(jpe?g|png|webp)$/i.test(d.datei);
      const url = `/api/admin/dokumente/${d.id}`;
      return h('div', { class: 'dok' },
        bild ? h('img', { class: 'dok-bild', src: url, alt: '', loading: 'lazy' }) : h('div', { class: 'dok-bild pdf' }, 'PDF'),
        h('div', { class: 'dok-text' }, h('b', { title: d.titel }, d.titel),
          h('span', { class: 'hint' }, `${d.art} · ${deDate(d.datum)} · ${groesseText(d.groesse)}`),
          d.paechterId ? h('span', { class: 'badge neu', title: 'Gehört zum Pächter und wird mit ihm endgültig gelöscht' }, 'zur Person') : null),
        h('div', { class: 'dok-aktionen' },
          h('a', { class: 'btn small', href: url, target: '_blank', rel: 'noopener' }, 'Öffnen'),
          h('button', { class: 'btn small danger', onclick: async () => {
            if (!(await confirmBox(`Dokument „${d.titel}“ löschen?`, 'Löschen', true))) return;
            try { await api('DELETE', url); await neuLaden(); } catch (e) { handleErr(e); }
          } }, 'Löschen')));
    });
    inhalt.replaceChildren(
      h('div', { class: 'akte-steckbrief' },
        h('div', { class: 'akte-nr' }, h('span', null, 'Garten'), h('b', null, nr)),
        h('div', null,
          p ? h('div', null, h('b', null, p.name), h('span', { class: 'hint' }, ` · Mitgl.-Nr. ${p.mitgliedsnr} · ${nfFlex.format(p.gartengroesse)} m²`)) : h('div', null, h('b', null, 'Zurzeit frei'), h('span', { class: 'hint' }, ' · kein Pächter')),
          p ? h('div', { class: 'hint' }, [p.strasse, p.plzOrt, p.telefon, p.email].filter(Boolean).join(' · ')) : null,
          h('div', { class: 'hint' }, `Zähler: Wasser ${p && p.wasserzaehlerNr || '–'}, Strom ${p && p.stromzaehlerNr || '–'}`))),
      abschnitt('akte', `Dokumente (${ak.dokumente.length})`, h('button', { class: 'btn small primary', onclick: () => dokumentHochladen(nr, !!p).then((ok) => ok && neuLaden()) }, '+ Dokument'),
        docs.length ? h('div', { class: 'dok-liste' }, docs) : h('p', { class: 'hint' }, 'Noch keine Dokumente. Lege Bauantrag, Genehmigungen, Fotos der Laube oder den Pachtvertrag hier ab – alles bleibt nur auf diesem Rechner.')),
      abschnitt('waage', `Wertermittlungen (${ak.wertermittlungen.length})`, h('button', { class: 'btn small primary', onclick: () => wertermittlungDialog(nr, null).then((ok) => ok && neuLaden()) }, '+ Wertermittlung'),
        ak.wertermittlungen.length ? h('table', { class: 'akte-tab' }, h('tbody', null, ak.wertermittlungen.map((x) => h('tr', null,
          h('td', null, deDate(x.datum)), h('td', null, x.anlass), h('td', { class: 'hint' }, x.wertermittler || ''), h('td', { class: 'num' }, h('b', null, eur(x.summe))),
          h('td', { class: 'r' },
            h('button', { class: 'btn small', onclick: () => wertermittlungDialog(nr, x).then((ok) => ok && neuLaden()) }, 'Bearbeiten'), ' ',
            h('button', { class: 'btn small', onclick: () => wertermittlungDrucken(x) }, 'Drucken'), ' ',
            h('button', { class: 'btn small danger', onclick: async () => {
              if (!(await confirmBox(`Wertermittlung vom ${deDate(x.datum)} löschen?`, 'Löschen', true))) return;
              try { await api('DELETE', `/api/admin/wertermittlungen/${x.id}`); await neuLaden(); } catch (e) { handleErr(e); }
            } }, 'Löschen'))))))
          : h('p', { class: 'hint' }, 'Vor einem Pächterwechsel den Wert von Laube, Anpflanzungen und Einrichtungen festhalten. Die Summe wird beim Wechsel als Ablösesumme vorgeschlagen.')),
      abschnitt('personen', 'Übergaben und Pächter', null,
        ak.uebergaben.length ? h('table', { class: 'akte-tab' }, h('tbody', null, ak.uebergaben.map((u) => h('tr', null,
          h('td', null, deDate(u.datum)), h('td', null, `${u.altName} → ${u.neuName}`), h('td', { class: 'num' }, u.abloese ? `Ablöse ${eur(u.abloese)}` : ''),
          h('td', { class: 'r' }, h('button', { class: 'btn small', onclick: () => protokollDrucken(u) }, 'Protokoll')))))) : null,
        ak.historie.length ? h('ul', { class: 'akte-verlauf' }, ak.historie.map((e) => h('li', null,
          h('b', null, e.name), ` (${e.mitgliedsnr || '–'}) · seit ${deDate(e.seit)}${e.bis ? ' bis ' + deDate(e.bis) : ' – heute'}`)))
          : h('p', { class: 'hint' }, 'Noch kein Eintrag im Gartenverlauf.')));
  };
  zeichnen();
  await breiterDialog(`Gartenakte – Garten ${nr}`, inhalt, [
    { label: 'Exposé', value: false, action: () => { exposeDrucken(nr); return false; } },
    p_wechselKnopf(ak, nr),
    { label: 'Schließen', cls: 'primary', value: true }].filter(Boolean));
  await reload();
}

// Knopf »Pächterwechsel« in der Akte, wenn der Garten einen Pächter hat
const willkommenDrucken = (p) => pdfDrucken({
  url: `/api/admin/paechter/${p.id}/willkommen`, oeffnenUrl: `/api/admin/paechter/${p.id}/willkommen/oeffnen`,
  titel: `Willkommensmappe ${p.name}`, meldung: () => 'Die Willkommensmappe wurde im PDF-Programm geöffnet – dort drucken oder per E-Mail schicken.',
});
const exposeDrucken = (nr) => pdfDrucken({
  url: `/api/admin/expose?gartennr=${encodeURIComponent(nr)}`, oeffnenUrl: `/api/admin/expose/oeffnen?gartennr=${encodeURIComponent(nr)}`,
  titel: `Garten ${nr}`, meldung: () => 'Das Exposé wurde im PDF-Programm geöffnet – für Interessenten drucken oder verschicken.',
});

// Texte für Willkommensmappe und Exposé: Ansprechpartner und Hinweise
async function vereinsinfoDialog() {
  let v;
  try { v = await api('GET', '/api/admin/vereinsinfo'); } catch (e) { handleErr(e); return; }
  const ap = h('textarea', { rows: 4, style: 'width:100%', placeholder: 'z. B.\nVorsitz: Name, Telefon\nKasse: Name, E-Mail\nSprechstunde: samstags 10–12 Uhr' }, v.ansprechpartner || '');
  const hw = h('textarea', { rows: 6, style: 'width:100%', placeholder: 'Ein Hinweis pro Zeile, z. B.\nMittagsruhe 13–15 Uhr\nWasser wird im April angestellt' }, v.hinweise || '');
  await modal('Texte für Willkommensmappe und Exposé', h('div', { class: 'form' },
    h('div', { class: 'field wide' }, h('label', null, 'Ansprechpartner'), ap, h('span', { class: 'fhint' }, 'Erscheint in Willkommensmappe und Exposé – nur angeben, wer damit einverstanden ist.')),
    h('div', { class: 'field wide' }, h('label', null, 'Bitte beachten (Willkommensmappe)'), hw, h('span', { class: 'fhint' }, 'Kurz das Wichtigste aus der Gartenordnung, ein Punkt pro Zeile.'))),
  [{ label: 'Abbrechen', value: false }, { label: 'Speichern', cls: 'primary', value: true, action: async () => {
    try { await api('PUT', '/api/admin/vereinsinfo', { ansprechpartner: ap.value, hinweise: hw.value }); toast('Gespeichert', 'ok'); } catch (e) { handleErr(e); return false; }
  } }]);
}

function p_wechselKnopf(ak, nr) {
  const p = ak.paechter && S.state.paechter.find((x) => x.id === ak.paechter.id && !x.ausgeschiedenAm);
  if (!p) return null;
  return { label: 'Pächterwechsel …', value: false, action: async () => { setTimeout(() => paechterwechselDialog(p), 0); return true; } };
}

async function dokumentHochladen(nr, hatPaechter) {
  const datei = h('input', { type: 'file', accept: '.pdf,.jpg,.jpeg,.png,.webp' });
  const titel = h('input', { type: 'text', maxlength: 120, placeholder: 'z. B. Baugenehmigung Laube' });
  const art = h('select', null, DOKUMENT_ARTEN.map((a) => h('option', { value: a }, a)));
  const datum = h('input', { type: 'date', value: todayIso() });
  const zurPerson = h('input', { type: 'checkbox' });
  const sync = () => { zurPerson.checked = hatPaechter && PERSONEN_ARTEN.has(art.value); };
  art.addEventListener('change', sync);
  datei.addEventListener('change', () => {
    const f = datei.files[0];
    if (f && !titel.value) titel.value = f.name.replace(/\.[^.]+$/, '');
    if (f && /\.(jpe?g|png|webp)$/i.test(f.name) && art.value === 'Bauantrag') { art.value = 'Foto'; sync(); }
  });
  sync();
  const fld = (label, input, hint, wide) => h('div', { class: 'field' + (wide ? ' wide' : '') }, h('label', null, label), input, hint ? h('span', { class: 'fhint' }, hint) : null);
  return modal(`Dokument zu Garten ${nr}`, h('div', { class: 'form' },
    fld('Datei (PDF oder Foto, höchstens 12 MB) *', datei, null, true),
    fld('Bezeichnung', titel), fld('Art', art), fld('Datum des Dokuments', datum),
    hatPaechter ? h('div', { class: 'field wide' }, h('label', { class: 'check' }, zurPerson, ' Gehört zum jetzigen Pächter (z. B. Pachtvertrag, Schriftverkehr)'),
      h('span', { class: 'fhint' }, 'Solche Dokumente werden gelöscht, wenn der Pächter endgültig gelöscht wird. Fotos und Genehmigungen bleiben beim Garten.')) : null),
  [{ label: 'Abbrechen', value: false }, { label: 'Hochladen', cls: 'primary', value: true, action: async () => {
    if (!datei.files[0]) { toast('Bitte eine Datei auswählen.', 'err'); return false; }
    const fd = new FormData();
    fd.append('file', datei.files[0]); fd.append('gartennr', nr); fd.append('titel', titel.value); fd.append('art', art.value);
    fd.append('datum', datum.value); fd.append('zurPerson', zurPerson.checked ? '1' : '');
    try { await api('POST', '/api/admin/dokumente', fd, true); toast('Dokument gespeichert', 'ok'); } catch (e) { handleErr(e); return false; }
  } }]);
}

// ------------------------------------------------------------------ Wertermittlung

const WERT_VORLAGEN = [
  ['Baulichkeiten', 'Gartenlaube', 'm²', 0], ['Baulichkeiten', 'Gewächshaus', 'Stück', 0], ['Baulichkeiten', 'Geräteschuppen', 'Stück', 0],
  ['Baulichkeiten', 'Terrasse / Wege (befestigt)', 'm²', 0], ['Anpflanzungen', 'Obstbaum', 'Stück', 0], ['Anpflanzungen', 'Beerenstrauch', 'Stück', 0],
  ['Anpflanzungen', 'Hecke', 'm', 0], ['Anpflanzungen', 'Stauden und Beete', 'm²', 0], ['Einrichtungen', 'Wasserleitung im Garten', 'pauschal', 0],
  ['Einrichtungen', 'Elektroinstallation', 'pauschal', 0], ['Einrichtungen', 'Regentonne / Zisterne', 'Stück', 0], ['Einrichtungen', 'Komposter', 'Stück', 0],
  ['Abzug', 'Beseitigung von Mängeln', 'pauschal', 0], ['Abzug', 'Rückbau nicht zulässiger Bauten', 'pauschal', 0],
];
const WERT_KATEGORIEN = ['Baulichkeiten', 'Anpflanzungen', 'Einrichtungen', 'Abzug'];

async function wertermittlungDialog(nr, x) {
  const cur = x ? JSON.parse(JSON.stringify(x)) : { gartennr: nr, datum: todayIso(), anlass: 'Pächterwechsel', wertermittler: '', bemerkung: '', positionen: [] };
  const datum = h('input', { type: 'date', value: cur.datum });
  const anlass = h('select', null, ['Pächterwechsel', 'Kündigung', 'Gartenteilung', 'Sonstiges'].map((a) => h('option', { value: a, selected: a === cur.anlass }, a)));
  const wer = h('input', { type: 'text', value: cur.wertermittler || '', maxlength: 100, placeholder: 'Name der Wertermittlerin / des Wertermittlers' });
  const bem = h('textarea', { rows: 2, maxlength: 1500, style: 'width:100%' }, cur.bemerkung || '');
  const tbody = h('tbody');
  const summeEl = h('b', { class: 'wert-summe' });
  const zahl = (v) => { const el = h('input', { type: 'text', value: v ? numIn(v) : '', class: 'num-in' }); el.setAttribute('inputmode', 'decimal'); return el; };
  const zeilen = [];
  const rechnen = () => {
    let summe = 0;
    for (const z of zeilen) {
      const m = parseNum(z.menge.value) ?? 1, e = parseNum(z.wert.value) ?? 0, a = parseNum(z.abschlag.value) ?? 0;
      let w = Number.isNaN(m) || Number.isNaN(e) || Number.isNaN(a) ? 0 : Math.round(m * e * (1 - Math.min(100, a) / 100) * 100) / 100;
      if (z.kat.value === 'Abzug') w = -w;
      z.ergebnis.textContent = eur(w);
      z.tr.classList.toggle('abzug', z.kat.value === 'Abzug');
      summe += w;
    }
    summeEl.textContent = eur(Math.max(0, summe));
  };
  const zeile = (p) => {
    const z = {
      kat: h('select', null, WERT_KATEGORIEN.map((k) => h('option', { value: k, selected: k === p.kategorie }, k))),
      bez: h('input', { type: 'text', value: p.bezeichnung || '', maxlength: 120, placeholder: 'Bezeichnung' }),
      menge: zahl(p.menge || ''), einheit: h('input', { type: 'text', value: p.einheit || '', maxlength: 15, class: 'einheit-in', placeholder: 'm²' }),
      wert: zahl(p.einzelwert), abschlag: zahl(p.abschlag), ergebnis: h('span', { class: 'num' }),
    };
    z.tr = h('tr', null, h('td', null, z.kat), h('td', null, z.bez), h('td', null, z.menge), h('td', null, z.einheit), h('td', null, z.wert), h('td', null, z.abschlag),
      h('td', { class: 'num' }, z.ergebnis),
      h('td', null, h('button', { class: 'btn small', title: 'Zeile entfernen', onclick: () => { zeilen.splice(zeilen.indexOf(z), 1); z.tr.remove(); rechnen(); } }, '✕')));
    z.tr.addEventListener('input', rechnen); z.tr.addEventListener('change', rechnen);
    zeilen.push(z); tbody.append(z.tr); rechnen();
    return z;
  };
  for (const p of cur.positionen) zeile(p);
  if (!cur.positionen.length) zeile({ kategorie: 'Baulichkeiten', bezeichnung: 'Gartenlaube', einheit: 'm²' });
  const vorlagen = h('div', { class: 'chips' }, h('span', { class: 'hint' }, 'Schnell hinzufügen:'),
    WERT_VORLAGEN.map(([k, b, e]) => h('button', { class: 'chip chip-knopf', type: 'button', onclick: () => zeile({ kategorie: k, bezeichnung: b, einheit: e }).bez.focus() }, b)));
  const body = h('div', null,
    h('div', { class: 'form', style: 'margin-bottom:14px' },
      h('div', { class: 'field' }, h('label', null, 'Datum *'), datum), h('div', { class: 'field' }, h('label', null, 'Anlass'), anlass),
      h('div', { class: 'field' }, h('label', null, 'Wertermittlung durch'), wer)),
    h('div', { class: 'tablewrap' }, h('table', { class: 'wert-tab' },
      h('thead', null, h('tr', null, ['Art', 'Position', 'Menge', 'Einheit', 'Einzelwert €', 'Abschlag %', 'Wert', ''].map((t) => h('th', null, t)))), tbody)),
    h('div', { class: 'toolbar', style: 'margin-top:8px' }, h('button', { class: 'btn small', onclick: () => zeile({ kategorie: 'Einrichtungen' }).bez.focus() }, '+ Zeile'), h('div', { class: 'spacer' }),
      h('span', null, 'Ermittelter Wert: '), summeEl),
    vorlagen,
    h('div', { class: 'field', style: 'margin-top:12px' }, h('label', null, 'Bemerkung'), bem),
    h('p', { class: 'hint', style: 'margin-top:10px' }, 'Wert = Menge × Einzelwert, vermindert um den Abschlag für Alter und Zustand; Abzüge werden abgezogen. Maßgeblich sind die Richtlinien eures Landesverbands – hier wird das Ergebnis festgehalten und als Ablösesumme beim Pächterwechsel vorgeschlagen.'));
  return breiterDialog(x ? `Wertermittlung Garten ${nr} bearbeiten` : `Wertermittlung Garten ${nr}`, body, [
    { label: 'Abbrechen', value: false },
    { label: 'Speichern', cls: 'primary', value: true, action: async () => {
      const positionen = zeilen.map((z) => ({ kategorie: z.kat.value, bezeichnung: z.bez.value, menge: parseNum(z.menge.value) ?? 0, einheit: z.einheit.value,
        einzelwert: parseNum(z.wert.value) ?? 0, abschlag: parseNum(z.abschlag.value) ?? 0 }));
      if (positionen.some((p) => [p.menge, p.einzelwert, p.abschlag].some((v) => Number.isNaN(v) || v < 0))) { toast('Menge, Einzelwert und Abschlag müssen Zahlen ab 0 sein.', 'err'); return false; }
      const daten = { gartennr: nr, datum: datum.value, anlass: anlass.value, wertermittler: wer.value, bemerkung: bem.value.trim(), positionen };
      try {
        if (x) await api('PUT', `/api/admin/wertermittlungen/${x.id}`, daten); else await api('POST', '/api/admin/wertermittlungen', daten);
        toast('Wertermittlung gespeichert', 'ok');
      } catch (e) { handleErr(e); return false; }
    } }]);
}

const wertermittlungDrucken = (x) => pdfDrucken({
  url: `/api/admin/wertermittlungen/${x.id}/pdf`, oeffnenUrl: `/api/admin/wertermittlungen/${x.id}/pdf/oeffnen`,
  titel: `Wertermittlung Garten ${x.gartennr}`, meldung: () => 'Die Wertermittlung wurde im PDF-Programm geöffnet – dort drucken.',
});

// ------------------------------------------------------------------ Zählerstand-Erinnerung

async function zaehlerErinnerungDialog(jahr) {
  let d;
  try { d = await api('GET', `/api/admin/zaehler-fehlen?year=${jahr}`); } catch (e) { handleErr(e); return; }
  const in14 = new Date(); in14.setDate(in14.getDate() + 14);
  const frist = h('input', { type: 'date', value: `${in14.getFullYear()}-${String(in14.getMonth() + 1).padStart(2, '0')}-${String(in14.getDate()).padStart(2, '0')}` });
  const mitMail = d.fehlend.filter((f) => f.email).length;
  const body = h('div', null,
    d.fehlend.length
      ? h('div', null,
        h('p', null, `Bei ${d.fehlend.length} von ${d.gesamt} Gärten mit Zählern fehlt noch ein aktueller Stand.`),
        h('div', { class: 'tablewrap', style: 'max-height:260px;overflow:auto' }, h('table', null,
          h('thead', null, h('tr', null, ['Garten', 'Name', 'Es fehlt', 'E-Mail'].map((t) => h('th', null, t)))),
          h('tbody', null, d.fehlend.map((f) => h('tr', null, h('td', null, f.gartennr), h('td', null, f.name), h('td', null, f.fehlt.join(' + ')),
            h('td', { class: 'klein' }, f.email || '–')))))),
        h('div', { class: 'field', style: 'margin-top:12px;max-width:16em' }, h('label', null, 'Bitte melden bis'), frist),
        h('p', { class: 'hint', style: 'margin-top:8px' }, `Der Aushang nennt nur die Gartennummern, keine Namen. Die E-Mail geht als Blindkopie (Bcc) an ${mitMail} Pächter mit hinterlegter Adresse – niemand sieht die Adressen der anderen.`))
      : leer('fertig', 'Alles gemeldet', `Für ${jahr} liegen von allen ${d.gesamt} Gärten mit Zählern aktuelle Stände vor.`));
  const knoepfe = [{ label: 'Schließen', value: false }];
  if (d.fehlend.length) {
    knoepfe.push({ label: 'Aushang drucken', value: true, action: async () => {
      pdfDrucken({ url: `/api/admin/zaehler-aushang?year=${jahr}&frist=${encodeURIComponent(frist.value)}`, oeffnenUrl: `/api/admin/zaehler-aushang/oeffnen?year=${jahr}&frist=${encodeURIComponent(frist.value)}`,
        titel: 'Zählerstände fehlen', meldung: () => 'Der Aushang wurde im PDF-Programm geöffnet – dort drucken.' });
      return false;
    } });
    if (mitMail) {
      knoepfe.push({ label: 'E-Mail vorbereiten', cls: 'primary', value: true, action: async () => {
        try {
          const r = await api('POST', `/api/admin/zaehler-mail?year=${jahr}&frist=${encodeURIComponent(frist.value)}`);
          toast(r.ohneMail ? `E-Mail an ${r.empfaenger} vorbereitet – ${r.ohneMail} ohne Adresse oder zu viele für einen Link.` : `E-Mail an ${r.empfaenger} vorbereitet.`, 'ok');
        } catch (e) { handleErr(e); }
        return false;
      } });
    }
  }
  return breiterDialog(`Fehlende Zählerstände ${jahr}`, body, knoepfe);
}

// ------------------------------------------------------------------ Berichte & Meldungen

async function loadMeldung(jahr) {
  if (S.meldungLoading) return;
  S.meldungLoading = true;
  try { S.meldung = await api('GET', `/api/admin/jahresmeldung?year=${jahr}`); } catch (e) { handleErr(e); S.meldung = false; }
  S.meldungLoading = false;
  render();
}

function adminBerichte() {
  const st = S.state;
  const jahr = S.berichtJahr || st.currentYear;
  if (S.meldung == null || (S.meldung && S.meldung.jahr !== jahr)) { if (!S.meldungLoading) loadMeldung(jahr); }
  const m = S.meldung && S.meldung.jahr === jahr ? S.meldung : null;
  const jahrSel = h('select', { onchange: (e) => { S.berichtJahr = Number(e.target.value); S.meldung = null; render(); } },
    st.years.map((y) => h('option', { value: y, selected: y === jahr }, String(y))));
  const kachel = (ic, titel, text, knopf) => h('div', { class: 'bericht-kachel' },
    h('span', { class: 'qlico' }, icon(ic, 24)), h('div', null, h('b', null, titel), h('p', { class: 'hint' }, text)), knopf);
  const fmt = (z) => (z.einheit === '€' ? eur(z.wert) : `${nfFlex.format(z.wert)}${z.einheit ? ' ' + z.einheit : ''}`);
  const gruppen = [];
  for (const z of (m ? m.zeilen : [])) {
    if (!gruppen.length || gruppen[gruppen.length - 1].g !== z.gruppe) gruppen.push({ g: z.gruppe, z: [] });
    gruppen[gruppen.length - 1].z.push(z);
  }
  return h('div', null,
    h('p', { class: 'hint' }, 'Alles, was der Vorstand im Jahr vorlegen oder melden muss, an einer Stelle.'),
    h('div', { class: 'toolbar' }, h('label', null, 'Jahr'), jahrSel),
    h('div', { class: 'bericht-raster' },
      kachel('bericht', `Jahresbericht ${jahr}`, 'Für die Mitgliederversammlung: Kennzahlen mit Vorjahresvergleich, Kasse, Ausgaben, Verlauf, Termine – ohne Namen.',
        h('button', { class: 'btn primary', onclick: () => jahresberichtDrucken(jahr) }, 'Als PDF')),
      kachel('waage', `Jahresmeldung ${jahr}`, 'Zahlen für den Landes- oder Regionalverband: Parzellen, Flächen, Wechsel, Warteliste, Stunden, Abgaben.',
        h('a', { class: 'btn primary', href: `/api/admin/jahresmeldung?year=${jahr}&format=xlsx` }, 'Als Excel')),
      kachel('personen', 'Pächterliste', 'Alle Pächter mit Anschrift, Kontakt und Zählern zum Ausdrucken (vertraulich).',
        h('button', { class: 'btn', onclick: paechterlisteDrucken }, 'Drucken')),
      kachel('karte', 'Lageplan', 'Als Aushang ohne Namen oder für den Vorstand mit Namen.',
        h('button', { class: 'btn', onclick: () => lageplanDrucken(true) }, 'Drucken')),
      kachel('zaehler', `Fehlende Zählerstände ${st.currentYear}`, 'Wer hat noch keinen Stand gemeldet? Mit Aushang ohne Namen und E-Mail-Erinnerung.',
        h('button', { class: 'btn', onclick: () => zaehlerErinnerungDialog(st.currentYear) }, 'Prüfen')),
      kachel('personen', 'Willkommensmappe & Exposé', 'Texte für die Mappe neuer Pächter und den Steckbrief freier Gärten: Ansprechpartner und das Wichtigste aus der Gartenordnung.',
        h('button', { class: 'btn', onclick: vereinsinfoDialog }, 'Texte bearbeiten')),
      kachel('daten', 'Amtsübergabe', 'Vereinsakte für den neuen Vorstand: alle Daten und Dateien plus »Lies mich zuerst« mit Zahlen, Regeln und offenen Punkten.',
        h('button', { class: 'btn', onclick: vereinsakteErstellen }, 'Erstellen')),
      kachel('warnung', 'Begehungsliste', 'Vordruck für den Rundgang durch die Anlage, mit den noch offenen Mängeln je Garten.',
        h('button', { class: 'btn', onclick: begehungslisteDrucken }, 'Drucken')),
      kachel('kalender', `Vereinstermine ${new Date().getFullYear()}`, 'Jahresplaner als Aushang für den Schaukasten.',
        h('button', { class: 'btn', onclick: () => jahresplanDrucken(new Date().getFullYear()) }, 'Drucken')),
      kachel('zaehler', `Ablesebogen ${st.currentYear}`, 'Alle Gärten mit Zählern und Vorjahresständen für den Rundgang.',
        h('button', { class: 'btn', onclick: () => ablesebogenDrucken(st.currentYear) }, 'Drucken'))),
    h('h3', null, `Jahresmeldung ${jahr} – Übersicht`),
    !m ? h('p', { class: 'hint' }, S.meldung === false ? 'Konnte nicht geladen werden.' : 'Wird geladen …')
      : h('div', { class: 'meldung-raster' }, gruppen.map((g) => h('div', { class: 'card' },
        h('h4', null, g.g),
        h('table', { class: 'meldung' }, h('tbody', null, g.z.map((z) => h('tr', null, h('td', null, z.angabe), h('td', { class: 'num' }, h('b', null, fmt(z)))))))))),
    m ? h('p', { class: 'hint', style: 'margin-top:12px' }, 'Neu vergebene Gärten zählen nur Vergaben über Pächterwechsel oder Warteliste, nicht das erstmalige Eintragen langjähriger Pächter. Das Formular eures Verbands kann andere Angaben verlangen – die Excel-Datei lässt sich frei ergänzen.') : null);
}

// ------------------------------------------------------------------ Übersicht: Was steht an?

async function loadAufgaben() {
  if (S.aufgabenLoading) return;
  S.aufgabenLoading = true;
  try { S.aufgaben = await api('GET', '/api/admin/aufgaben'); } catch (e) { S.aufgaben = []; }
  S.aufgabenLoading = false;
  render();
}

const AUFGABE_ICON = { err: 'warnung', warn: 'uhr', info: 'bericht' };

function cockpitKarte(st, istAdmin) {
  if (!istAdmin || st.leer) return null;
  if (S.aufgaben == null) { if (!S.aufgabenLoading) loadAufgaben(); return null; }
  const geh = (ziel) => {
    const [art, wohin, unter] = ziel.split(':');
    if (art === 'admin') adminTabWaehlen(wohin); else S.tab = wohin;
    if (wohin === 'inventar') S.invSeite = unter || 'schluessel';
    if (wohin === 'verwaltung') S.verwSeite = unter || 'beschluesse';
    if (wohin === 'kassenbericht') S.kasseNurOhneBeleg = unter === 'ohnebeleg';
    render();
    if (unter === 'eichung') eichungDialog();
  };
  if (!S.aufgaben.length) {
    return h('div', { class: 'card cockpit cockpit-leer' }, h('span', { class: 'cockpit-ico ok' }, icon('fertig', 20)),
      h('div', null, h('b', null, 'Gerade steht nichts an'), h('div', { class: 'hint' }, 'Termine, Fristen, Zahlungen und Spenden sind im grünen Bereich.')));
  }
  const alle = S.aufgabenAlle || S.aufgaben.length <= 5;
  const l = alle ? S.aufgaben : S.aufgaben.slice(0, 5);
  return h('div', { class: 'card cockpit' },
    h('div', { class: 'fs-kopf' }, h('b', null, 'Was steht an?'), h('span', { class: 'hint' }, mehrzahlText(S.aufgaben.length, 'Punkt', 'Punkte'))),
    h('ul', { class: 'cockpit-liste' }, l.map((a) => h('li', { class: 'cockpit-' + a.stufe },
      h('span', { class: 'cockpit-ico ' + a.stufe }, icon(AUFGABE_ICON[a.stufe] || 'fertig', 18)),
      h('div', { class: 'cockpit-text' }, h('b', null, a.titel), a.text ? h('div', { class: 'hint' }, a.text) : null),
      h('button', { class: 'btn small', onclick: () => geh(a.ziel) }, a.knopf + ' →')))),
    alle ? null : h('button', { class: 'btn small', style: 'margin-top:6px', onclick: () => { S.aufgabenAlle = true; render(); } }, `Alle ${S.aufgaben.length} anzeigen`));
}
const mehrzahlText = (n, eins, viele) => `${n} ${n === 1 ? eins : viele}`;

// ------------------------------------------------------------------ Übersicht: Besichtigungen

async function loadDashWarteliste() {
  if (S.dashWartelisteLoading) return;
  S.dashWartelisteLoading = true;
  try { S.dashWarteliste = await api('GET', '/api/admin/warteliste'); } catch (e) { S.dashWarteliste = []; }
  S.dashWartelisteLoading = false;
  render();
}

function besichtigungenKarte(st, istAdmin) {
  if (!istAdmin || !st.wartelisteAnzahl) return null;
  if (S.dashWarteliste == null) { if (!S.dashWartelisteLoading) loadDashWarteliste(); return null; }
  const heute = todayIso();
  const l = S.dashWarteliste.filter((b) => b.besichtigungAm && b.besichtigungAm >= heute && !b.besichtigungErgebnis)
    .sort((a, b) => (a.besichtigungAm + (a.besichtigungUm || '')).localeCompare(b.besichtigungAm + (b.besichtigungUm || ''))).slice(0, 4);
  if (!l.length) return null;
  return h('div', { class: 'card naechste-termine' },
    h('div', { class: 'fs-kopf' }, h('b', null, 'Besichtigungen'),
      h('button', { class: 'btn small', onclick: () => { adminTabWaehlen('warteliste'); render(); } }, 'Zur Warteliste →')),
    h('ul', { class: 'termin-liste' }, l.map((b) => {
      const d = isoDatum(b.besichtigungAm);
      return h('li', { class: 'termin' },
        h('div', { class: 'termin-tag' }, h('b', null, String(d.getDate())), h('span', null, `${WOCHENTAGE[d.getDay()]} · ${MONATE[d.getMonth()].slice(0, 3)}`)),
        h('div', { class: 'termin-text' }, h('b', null, b.name),
          h('div', { class: 'hint' }, [b.besichtigungUm ? b.besichtigungUm + ' Uhr' : '', b.besichtigungGarten ? 'Garten ' + b.besichtigungGarten : '', b.telefon].filter(Boolean).join(' · '))),
        h('span', { class: 'chip termin-art' }, 'Besichtigung'));
    })));
}

// ------------------------------------------------------------------ Start

(async function start() {
  document.body.append(h('div', { id: 'toasts' }));
  try { await load(); render(); } catch (e) {
    document.getElementById('app').replaceChildren(h('div', { class: 'center card' }, h('h2', null, 'Verbindung fehlgeschlagen'), h('p', null, e.message)));
  }
})();
