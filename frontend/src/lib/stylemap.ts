// StyleMap packs (FineTuneGallery /stylemaps/, built by Ol1nLLM tools/stylemap),
// proxied by our backend. A pack is one subject in many styles; `tour` orders
// the images with the smallest visual steps, so playing it looks like a morph.

/** source: party = built here from a FineTuneGallery session (D23), finetune = Ol1nLLM pack. */
export type PackInfo = { id: string; title: string; n: number; aspect: number; atlas: string; source?: 'party' | 'finetune' };
export type Frame = { src: string; label: string };
export type Pack = { id: string; title: string; aspect: number; atlas: string; frames: Frame[] };

type MapJson = {
  id: string;
  title: string;
  aspect?: number;
  atlas?: string;
  thumbs?: string;
  images: { i: number; label: string; style?: string; tour?: number; hilbert: number }[];
};

const packs = new Map<string, Promise<Pack>>();

export async function listPacks(): Promise<PackInfo[]> {
  const r = await fetch('/stylemaps/index.json');
  if (!r.ok) throw new Error(`StyleMap: ${r.status}`);
  return ((await r.json()).packs ?? []) as PackInfo[];
}

export function loadPack(id: string): Promise<Pack> {
  let p = packs.get(id);
  if (!p) {
    p = (async () => {
      const r = await fetch(`/stylemaps/${encodeURIComponent(id)}/map.json`);
      if (!r.ok) throw new Error(`StyleMap ${id}: ${r.status}`);
      const m = (await r.json()) as MapJson;
      const thumbs = m.thumbs ?? 't/{i}.webp';
      const hasTour = m.images.some((im) => im.tour !== undefined);
      const order = [...m.images].sort((a, b) => (hasTour ? (a.tour ?? 1e9) - (b.tour ?? 1e9) : a.hilbert - b.hilbert));
      return {
        id: m.id,
        title: m.title,
        aspect: m.aspect ?? 1,
        atlas: `/stylemaps/${id}/${m.atlas ?? 'atlas.webp'}`,
        frames: order.map((im) => ({ src: `/stylemaps/${id}/${thumbs.replace('{i}', String(im.i))}`, label: im.style ?? im.label })),
      };
    })();
    p.catch(() => packs.delete(id));
    packs.set(id, p);
  }
  return p;
}

/** Warms the browser cache for frames [from, from+n). */
export function preload(frames: Frame[], from: number, n: number) {
  for (let k = 0; k < n && frames.length; k++) {
    const img = new Image();
    img.decoding = 'async';
    img.src = frames[(from + k) % frames.length].src;
  }
}
