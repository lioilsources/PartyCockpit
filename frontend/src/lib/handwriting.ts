// Handwritten notes (D16): lays text out from a HandWrittenStickers glyph sheet
// (/glyphs/<id>/glyphs.json + one PNG per character) and draws it on a canvas.
// A port of the app's GlyphRenderer (handwritten_stickers/lib/services/
// glyph_renderer.dart) without cursive joins: baseline-aligned glyphs with
// proportional advances, variants that never repeat back to back, accents
// composed from base + accent glyph, a fallback sheet for missing letters, and
// a little wobble in size, rotation and ink. Deterministic per text.

export type SheetInfo = { id: string; name: string; category: string; default?: boolean };

type Metric = { w: number; h: number; baseline: number; advance: number; bearing: number };
type Manifest = {
  version: number;
  cellSize?: { width: number; height: number };
  dpi?: number;
  emHeight?: number;
  baselineRatio?: number;
  glyphs: Record<string, string | string[]>;
  metrics?: Record<string, Metric | Metric[]>;
  compose?: Record<string, [string, string]>;
  fallback?: string;
  colored?: boolean;
};

type Glyph = { img: HTMLImageElement; m?: Metric };

const list = <T>(v: T | T[] | undefined): T[] => (v === undefined ? [] : Array.isArray(v) ? v : [v]);

class Sheet {
  cellH: number;
  baselineRatio: number;
  emH: number;
  hasMetrics: boolean;
  private imgs = new Map<string, Promise<HTMLImageElement | null>>();

  constructor(
    readonly id: string,
    readonly man: Manifest,
  ) {
    const mm = man.cellSize?.height ?? 26.2;
    this.cellH = (mm / 25.4) * (man.dpi ?? 300);
    this.baselineRatio = man.baselineRatio ?? 0.75;
    this.emH = man.emHeight ?? this.cellH * 0.62;
    this.hasMetrics = !!man.metrics && Object.keys(man.metrics).length > 0;
  }

  variants(c: string) {
    return list(this.man.glyphs[c]).length;
  }

  async glyph(c: string, v = 0): Promise<Glyph | null> {
    const file = list(this.man.glyphs[c])[v];
    if (!file) return null;
    const key = `${c}#${v}`;
    let p = this.imgs.get(key);
    if (!p) {
      p = new Promise((res) => {
        const img = new Image();
        img.onload = () => res(img);
        img.onerror = () => res(null);
        img.src = `/glyphs/${this.id}/${encodeURIComponent(file)}`;
      });
      this.imgs.set(key, p);
    }
    const img = await p;
    if (!img) return null;
    const ms = list(this.man.metrics?.[c]);
    return { img, m: ms[v] ?? ms[0] };
  }
}

const sheets = new Map<string, Promise<Sheet | null>>();

export function loadSheet(id: string): Promise<Sheet | null> {
  let p = sheets.get(id);
  if (!p) {
    p = fetch(`/glyphs/${encodeURIComponent(id)}/glyphs.json`)
      .then((r) => (r.ok ? r.json() : null))
      .then((m) => (m ? new Sheet(id, m as Manifest) : null))
      .catch(() => null);
    sheets.set(id, p);
  }
  return p;
}

export async function listSheets(): Promise<SheetInfo[]> {
  try {
    const r = await fetch('/api/glyphs');
    return r.ok ? ((await r.json()) as SheetInfo[]) : [];
  } catch {
    return [];
  }
}

// deterministic randomness per text (mulberry32 over a string hash)
function rng(text: string) {
  let h = 2166136261;
  for (const ch of text) h = Math.imul(h ^ ch.codePointAt(0)!, 16777619);
  let a = h >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

type Placed = { g: Glyph; x: number; y: number; s: number; rot: number; alpha: number };
// characters no sheet has (emoji): drawn as text, untinted, after the ink
type Extra = { text: string; x: number; y: number; size: number };
const EMOJI = /\p{Extended_Pictographic}/u;
const graphemes = new Intl.Segmenter(undefined, { granularity: 'grapheme' });

const STYLE = { wordSpacing: 0.7, lineHeight: 1.2, baselineWobble: 0.3, sizeVariance: 0.1, rotationVariance: 0.2, opacityVariance: 0.1 };

async function layout(text: string, sheet: Sheet, cell: number, maxWidth: number) {
  const fallback = sheet.man.fallback ? await loadSheet(sheet.man.fallback) : null;
  const rand = rng(text);
  const range = (a: number, b: number) => a + rand() * (b - a);
  const last = new Map<string, number>();
  const pick = (n: number, prev?: number) => {
    if (n <= 1) return 0;
    if (prev === undefined) return Math.floor(rand() * n);
    const v = Math.floor(rand() * (n - 1));
    return v >= prev ? v + 1 : v;
  };
  const from = async (sh: Sheet, c: string) => {
    const v = pick(sh.variants(c), last.get(`${sh.id}:${c}`));
    const g = await sh.glyph(c, v);
    if (g) last.set(`${sh.id}:${c}`, v);
    return g ? { sh, g } : null;
  };
  const resolve = async (c: string) => {
    if (sheet.variants(c) > 0) {
      const own = await from(sheet, c);
      if (own) return { ...own, accent: null as Glyph | null };
    }
    const parts = sheet.man.compose?.[c];
    if (parts) {
      const base = await from(sheet, parts[0]);
      const accent = await sheet.glyph(parts[1]);
      if (base && accent?.m) return { ...base, accent };
    }
    if (fallback && fallback.variants(c) > 0) {
      const fb = await from(fallback, c);
      if (fb) return { ...fb, accent: null };
    }
    return null;
  };

  const out: Placed[] = [];
  const extras: Extra[] = [];
  const pitch = (sheet.hasMetrics ? (cell * sheet.emH) / sheet.cellH : cell) * STYLE.lineHeight;
  const space = cell * 0.5 * STYLE.wordSpacing;
  const baseY = cell * sheet.baselineRatio;
  let x = 0;
  let y = 0;
  let width = 0;

  // wrap by words: measure each word first, then place it
  for (const [wi, word] of text.normalize('NFC').split(/ +/).entries()) {
    // graphemes, so a ZWJ emoji or a letter with a combining mark stays one unit
    const chars = [...graphemes.segment(word)].map((g) => g.segment);
    const resolved = [];
    let wordW = 0;
    for (const c of chars) {
      const r = await resolve(c);
      const s0 = r ? (r.g.m ? cell / r.sh.cellH : cell / r.g.img.naturalHeight) : 0;
      const s = s0 * (1 + range(-0.25, 0.25) * STYLE.sizeVariance);
      const emoji = !r && EMOJI.test(c);
      const adv = r ? (r.g.m ? r.g.m.advance * s : r.g.img.naturalWidth * s) : emoji ? cell * 0.75 : cell * 0.5;
      resolved.push({ c, r, s, adv, emoji });
      wordW += adv;
    }
    if (wi > 0) {
      if (x + space + wordW > maxWidth && x > 0) {
        x = 0;
        y += pitch;
      } else x += space;
    }
    for (const { c, r, s, adv, emoji } of resolved) {
      if (x + adv > maxWidth && x > 0) {
        x = 0;
        y += pitch;
      }
      if (emoji) extras.push({ text: c, x, y: y + baseY, size: cell * 0.6 });
      if (r) {
        const m = r.g.m;
        const dx = m ? x + m.bearing * s : x;
        const dy = (m ? y + baseY - m.baseline * s : y) + range(-2, 2) * STYLE.baselineWobble * (cell / 60);
        const p = { s, rot: range(-12, 12) * STYLE.rotationVariance, alpha: 1 - rand() * STYLE.opacityVariance };
        out.push({ g: r.g, x: dx, y: dy, ...p });
        if (r.accent) {
          const aw = r.accent.img.naturalWidth * s;
          const ah = r.accent.img.naturalHeight * s;
          const bw = r.g.img.naturalWidth * s;
          const em = r.sh.emH * s;
          // ď ť take the caron as an apostrophe beside the letter
          const side = c === 'ď' || c === 'ť';
          out.push({ g: r.accent, x: side ? dx + bw - aw * 0.3 : dx + (bw - aw) / 2, y: side ? dy : dy - em * 0.08 - ah, ...p });
        }
      }
      x += adv + range(-1, 1) * STYLE.baselineWobble;
      width = Math.max(width, x);
    }
  }
  return { glyphs: out, extras, width, height: y + pitch };
}

const cache = new Map<string, Promise<string | null>>();

/**
 * Renders text in the sheet's handwriting as a PNG data URL (transparent),
 * cell px tall letters wrapped at maxWidth px. Ink is tinted to `ink` unless
 * the sheet has coloured letters. null when the sheet can't be loaded.
 */
export function renderHandwriting(text: string, sheetId: string, cell: number, maxWidth: number, ink = '#1d1208'): Promise<string | null> {
  const key = [sheetId, cell, maxWidth, ink, text].join('\u0000');
  let p = cache.get(key);
  if (!p) {
    p = (async () => {
      const sheet = await loadSheet(sheetId);
      if (!sheet) return null;
      // the margin for tilted letters counts into maxWidth, so the image fits its box
      const pad = cell * 0.3;
      const { glyphs, extras, width, height } = await layout(text, sheet, cell, Math.max(cell, maxWidth - 2 * pad));
      if (!glyphs.length) return null;
      const cv = document.createElement('canvas');
      cv.width = Math.ceil(width + pad * 2);
      cv.height = Math.ceil(height + pad * 2);
      const ctx = cv.getContext('2d')!;
      for (const p of glyphs) {
        const w = p.g.img.naturalWidth * p.s;
        const h = p.g.img.naturalHeight * p.s;
        ctx.save();
        ctx.globalAlpha = p.alpha;
        ctx.translate(pad + p.x + w / 2, pad + p.y + h / 2);
        ctx.rotate((p.rot * Math.PI) / 180);
        ctx.drawImage(p.g.img, -w / 2, -h / 2, w, h);
        ctx.restore();
      }
      if (!sheet.man.colored) {
        ctx.globalCompositeOperation = 'source-in';
        ctx.fillStyle = ink;
        ctx.fillRect(0, 0, cv.width, cv.height);
        ctx.globalCompositeOperation = 'source-over';
        // scanned sheets have thin, half-transparent ink: lay it down twice
        ctx.drawImage(cv, 0, 0);
      }
      for (const e of extras) {
        ctx.font = `${e.size}px system-ui, 'Apple Color Emoji', 'Noto Color Emoji', sans-serif`;
        ctx.textBaseline = 'alphabetic';
        ctx.fillText(e.text, pad + e.x, pad + e.y);
      }
      return cv.toDataURL('image/png');
    })();
    p.then((v) => v ?? cache.delete(key));
    cache.set(key, p);
  }
  return p;
}
