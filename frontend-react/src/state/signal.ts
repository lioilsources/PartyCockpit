/**
 * A tiny external store for useSyncExternalStore. The owner mutates whatever
 * it wraps (a Viewer, a Mic) and calls emit(); the snapshot is recomputed once
 * per emit, so getSnapshot is stable between changes as React requires.
 */
export class Signal<T> {
  private listeners = new Set<() => void>();
  private snap: T;

  constructor(private read: () => T) {
    this.snap = read();
  }

  subscribe = (fn: () => void): (() => void) => {
    this.listeners.add(fn);
    return () => {
      this.listeners.delete(fn);
    };
  };

  getSnapshot = (): T => this.snap;

  emit = (): void => {
    this.snap = this.read();
    for (const fn of this.listeners) fn();
  };
}
