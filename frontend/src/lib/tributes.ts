// Tributes from PromoClown (D24): one tribute is one white sheet like
// Restyler's photo strip (strip.dart), a 2×2 grid or a 1×4 booth strip.
// Pure helpers shared by the stage, the Studio and the React Cockpit (@pc).

export type TributeFrame = { w: number; h: number; sw: number; sh: number };
export type Tribute = { id: string; credit: string; labels: string[]; frames: TributeFrame[]; v: number };
export type TributeLayout = 'grid' | 'strip';

/** Restyler strip.dart metrics, in its own pixels. */
export const SHEET = { cell: 512, margin: 24, label: 44, footer: 66 } as const;

export const COLS: Record<TributeLayout, { min: number; max: number; def: number }> = {
  grid: { min: 2, max: 4, def: 2 },
  strip: { min: 2, max: 8, def: 4 },
};

export const MAX_TRIBUTES = 40;
export const DEFAULT_TRIBUTES = 24;

export type TributeOptions = { layout: TributeLayout; cols: number; speed: number; labels: boolean; ids: string[] };

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n));

/** Scene params with defaults and bounds (both Cockpits, the Studio and the stage agree). */
export function tributeOptions(params: Record<string, unknown>): TributeOptions {
  const layout: TributeLayout = params.layout === 'strip' ? 'strip' : 'grid';
  const c = COLS[layout];
  const ids = Array.isArray(params.tributes) ? params.tributes.filter((x): x is string => typeof x === 'string' && /^[0-9]{1,9}$/.test(x)) : [];
  return {
    layout,
    cols: clamp(Math.round(Number(params.cols)) || c.def, c.min, c.max),
    speed: clamp(Number(params.speed) || 40, 5, 300),
    labels: params.labels !== false,
    ids: ids.slice(0, MAX_TRIBUTES),
  };
}

/** What a Cockpit button sends: layout and columns within the layout's bounds. */
export function tributeParams(layout: TributeLayout, cols: number): { layout: TributeLayout; cols: number } {
  const c = COLS[layout];
  return { layout, cols: clamp(Math.round(cols) || c.def, c.min, c.max) };
}

/** Cell columns on a sheet: a grid has two unless it has a single frame. */
export const sheetCols = (t: Pick<Tribute, 'frames'>, layout: TributeLayout) => (layout === 'grid' && t.frames.length > 1 ? 2 : 1);

/** Sheet width in Restyler pixels. */
export function sheetWidth(layout: TributeLayout): number {
  const c = layout === 'grid' ? 2 : 1;
  return c * SHEET.cell + (c + 1) * SHEET.margin;
}

/** Sheet height divided by its width. Cells take the first frame's ratio. */
export function sheetAspect(t: Pick<Tribute, 'frames'>, layout: TributeLayout, labels: boolean): number {
  const W = sheetWidth(layout);
  const f = t.frames[0];
  const ratio = f && f.w > 0 ? f.h / f.w : 4 / 3;
  const n = Math.max(1, t.frames.length);
  const c = sheetCols(t, layout);
  const cw = c === 1 ? W - 2 * SHEET.margin : SHEET.cell;
  const rows = Math.ceil(n / c);
  const row = cw * ratio + (labels ? SHEET.label : 0);
  const H = SHEET.margin + rows * row + (rows - 1) * SHEET.margin + SHEET.footer;
  return H / W;
}

/** The credit under every sheet, as the post says it. */
export const creditLine = (credit: string) => `Tribute to ${credit}`;
