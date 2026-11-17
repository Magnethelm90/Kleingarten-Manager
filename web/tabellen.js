'use strict';
// Tabellen: Spalten per Klick sortieren (Sortierung bleibt über Seitenwechsel erhalten) und auf dem Handy als
// Karten darstellen (data-label an jeder Zelle). Arbeitet nachträglich auf dem fertigen DOM, damit keine der
// vielen Tabellen im Programm einzeln angefasst werden muss. Tabellen mit Eingabefeldern, verbundenen Zellen
// oder Summenzeilen bleiben unverändert in ihrer Reihenfolge.
(() => {
  const SPEICHER = 'km-sortierung';
  let merke = {};
  try { merke = JSON.parse(localStorage.getItem(SPEICHER) || '{}') || {}; } catch (e) { merke = {}; }
  const sichern = () => { try { localStorage.setItem(SPEICHER, JSON.stringify(merke)); } catch (e) { /* ohne Speicher geht es auch */ } };

  const text = (z) => (z ? z.textContent.replace(/\s+/g, ' ').trim() : '');

  // Zahl (deutsches Format), Datum (TT.MM.JJJJ oder JJJJ-MM-TT) oder Text
  function wert(t) {
    let m = /^(\d{1,2})\.(\d{1,2})\.(\d{4})/.exec(t);
    if (m) return { z: Number(m[3]) * 10000 + Number(m[2]) * 100 + Number(m[1]) };
    m = /^(\d{4})-(\d{2})-(\d{2})/.exec(t);
    if (m) return { z: Number(m[1]) * 10000 + Number(m[2]) * 100 + Number(m[3]) };
    if (/^[−-]?\s*\d[\d.]*(,\d+)?(\s*[€%]|\s*m²|\s*kWh|\s*m³|\s*h)?$/.test(t)) {
      const n = Number(t.replace(/[^\d,−-]/g, '').replace('−', '-').replace(',', '.'));
      if (!Number.isNaN(n)) return { z: n };
    }
    return { t: t.toLowerCase() };
  }
  function vergleiche(a, b) {
    if (a.z !== undefined && b.z !== undefined) return a.z - b.z;
    if (a.z !== undefined) return -1; // Zahlen vor Text, leere Zellen ans Ende
    if (b.z !== undefined) return 1;
    const x = a.t ?? '', y = b.t ?? '';
    if (!x && y) return 1;
    if (x && !y) return -1;
    return x.localeCompare(y, 'de', { numeric: true });
  }

  function geeignet(tab) {
    if (tab.classList.contains('nosort') || tab.closest('dialog.geschichte, .assistent')) return null;
    const kopf = tab.tHead && tab.tHead.rows.length === 1 ? [...tab.tHead.rows[0].cells] : null;
    if (!kopf || kopf.length < 2) return null;
    const rumpf = tab.tBodies[0];
    if (!rumpf || tab.tBodies.length !== 1) return null;
    const zeilen = [...rumpf.rows];
    if (zeilen.some((r) => r.cells.length !== kopf.length || [...r.cells].some((z) => z.colSpan > 1 || z.rowSpan > 1))) return null;
    return { kopf, rumpf, zeilen };
  }

  function verarbeite(tab) {
    const g = geeignet(tab);
    if (!g) return;
    const { kopf, rumpf } = g;
    // Karten-Ansicht auf dem Handy: jede Zelle kennt ihre Spalte
    tab.classList.add('karten');
    const namen = kopf.map(text);
    for (const r of g.zeilen) [...r.cells].forEach((z, i) => { if (!z.dataset.label && namen[i]) z.dataset.label = namen[i]; });
    // Sortierbar nur bei mehr als einer Zeile und ohne Eingabefelder im Rumpf
    if (g.zeilen.length < 2 || rumpf.querySelector('input:not([type=checkbox]):not([type=hidden]), textarea, select')) return;
    const summe = (r) => /sum|total|gesamt/i.test(r.className);
    const schluessel = namen.join('|');
    g.zeilen.forEach((r, i) => { if (r.dataset.ur === undefined) r.dataset.ur = String(i); });
    const anwenden = (sp, rich) => {
      const feste = g.zeilen.filter(summe);
      const zeilen = g.zeilen.filter((r) => !summe(r));
      if (sp == null) zeilen.sort((a, b) => Number(a.dataset.ur) - Number(b.dataset.ur));
      else {
        const w = new Map(zeilen.map((r) => [r, wert(text(r.cells[sp]))]));
        zeilen.sort((a, b) => rich * vergleiche(w.get(a), w.get(b)) || Number(a.dataset.ur) - Number(b.dataset.ur));
      }
      rumpf.append(...zeilen, ...feste);
      // Knopf zum Zurücksetzen nur, solange sortiert ist
      if (sp == null) { if (tab._reset) { tab._reset.remove(); tab._reset = null; } }
      else if (!tab._reset || !tab._reset.isConnected) {
        const b = document.createElement('button');
        b.type = 'button'; b.className = 'btn small sort-reset'; b.textContent = 'Sortierung zurücksetzen';
        b.addEventListener('click', () => { delete merke[schluessel]; sichern(); anwenden(null, 1); });
        tab.before(b);
        tab._reset = b;
      }
      kopf.forEach((th, i) => {
        if (th.dataset.sortierbar) th.setAttribute('aria-sort', i === sp ? (rich > 0 ? 'ascending' : 'descending') : 'none');
      });
    };
    kopf.forEach((th, i) => {
      // Spalten ohne Beschriftung oder mit Bedienelementen (Auswahlhaken) werden nicht sortiert
      if (!namen[i] || th.querySelector('input, button, select')) return;
      th.dataset.sortierbar = '1';
      th.tabIndex = 0;
      th.title = 'Klicken zum Sortieren';
      const klick = () => {
        const alt = merke[schluessel];
        let neu = { sp: i, rich: 1 };
        if (alt && alt.sp === i) neu = alt.rich === 1 ? { sp: i, rich: -1 } : null;
        if (neu) merke[schluessel] = neu; else delete merke[schluessel];
        sichern();
        anwenden(neu ? neu.sp : null, neu ? neu.rich : 1);
      };
      const taste = (e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); klick(); } };
      // Bei neu gefülltem Rumpf (Filter, Neuladen) die alten Zuhörer ersetzen, sonst sortiert ein Klick doppelt.
      if (th._km) { th.removeEventListener('click', th._km[0]); th.removeEventListener('keydown', th._km[1]); }
      th._km = [klick, taste];
      th.addEventListener('click', klick);
      th.addEventListener('keydown', taste);
    });
    const m = merke[schluessel];
    if (m && m.sp < kopf.length) anwenden(m.sp, m.rich); else kopf.forEach((th) => th.dataset.sortierbar && th.setAttribute('aria-sort', 'none'));
  }

  let geplant = false;
  const lauf = () => {
    geplant = false;
    beobachter.disconnect();
    document.querySelectorAll('#app main table, dialog table').forEach((t) => {
      // Rumpf neu gefüllt (Filter, Neuladen) → neu verarbeiten
      const rumpf = t.tBodies[0];
      if (!rumpf) return;
      const neuesDa = [...rumpf.rows].some((r) => r.dataset.ur === undefined);
      if (t.dataset.fertig && !neuesDa) return;
      try { verarbeite(t); t.dataset.fertig = '1'; } catch (e) { /* Tabelle bleibt, wie sie ist */ }
    });
    beobachter.observe(document.body, { childList: true, subtree: true });
  };
  const beobachter = new MutationObserver(() => { if (!geplant) { geplant = true; requestAnimationFrame(lauf); } });
  beobachter.observe(document.body, { childList: true, subtree: true });
})();
