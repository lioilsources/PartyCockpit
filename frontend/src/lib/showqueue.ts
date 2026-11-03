/**
 * Show-queue picker (DECISIONS.md D5): play approved items in queue order; new
 * items that were never shown jump ahead; when everything has been shown, a new
 * round starts — in queue order (loop) or shuffled (shuffle).
 */
export class ShowQueue<T extends { id: string }> {
  private played = new Set<string>();
  private round: string[] = [];

  next(items: T[], mode: 'loop' | 'shuffle'): T | null {
    if (items.length === 0) return null;
    const byId = new Map(items.map((i) => [i.id, i]));
    const fresh = items.find((i) => !this.played.has(i.id) && !this.round.includes(i.id));
    if (fresh) {
      this.played.add(fresh.id);
      return fresh;
    }
    this.round = this.round.filter((id) => byId.has(id));
    if (this.round.length === 0) {
      this.round = items.map((i) => i.id);
      if (mode === 'shuffle') shuffle(this.round);
    }
    const id = this.round.shift()!;
    this.played.add(id);
    return byId.get(id) ?? null;
  }
}

export function shuffle<T>(a: T[]): T[] {
  for (let i = a.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}
