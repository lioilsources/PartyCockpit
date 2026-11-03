import type { Tempo } from './types';

export type Beat = { n: number; bpm: number; energy: number; source: string };
type Listener = (b: Beat) => void;

/**
 * BeatClock: one beat source for every scene (DECISIONS.md D4).
 * 1. tempo from the server (operator tap or DJfy nowPlaying.bpm) → metronome
 * 2. otherwise the stage's own audio input (BlackHole / VB-Cable / mic) → onsets
 * 3. otherwise a slow idle pulse (no beat at all), so scenes still advance
 */
export class BeatClock {
  private listeners = new Set<Listener>();
  private tempo: Tempo = { bpm: 0, source: 'none', anchor: 0 };
  private n = 0;
  private lastBeat = 0;
  private raf = 0;
  energy = 0;
  private audio: AudioBeat | null = null;

  constructor() {
    const loop = () => {
      this.tick();
      this.raf = requestAnimationFrame(loop);
    };
    this.raf = requestAnimationFrame(loop);
  }

  on(fn: Listener) {
    this.listeners.add(fn);
    return () => this.listeners.delete(fn);
  }

  setTempo(t: Tempo) {
    this.tempo = t;
  }

  get source() {
    if (this.tempo.bpm > 0) return this.tempo.source;
    if (this.audio?.active) return 'audio';
    return 'idle';
  }

  get bpm() {
    if (this.tempo.bpm > 0) return this.tempo.bpm;
    if (this.audio?.active) return this.audio.bpm;
    return 0;
  }

  async enableAudio() {
    if (this.audio) return;
    this.audio = new AudioBeat(() => {
      if (this.tempo.bpm <= 0) this.emit();
    });
    await this.audio.start();
  }

  private emit() {
    this.n++;
    const b = { n: this.n, bpm: this.bpm, energy: this.energy, source: this.source };
    for (const l of this.listeners) l(b);
  }

  private tick() {
    const now = Date.now();
    this.energy = this.audio?.energy ?? 0;
    if (this.tempo.bpm > 0) {
      const period = 60000 / this.tempo.bpm;
      const anchor = this.tempo.anchor || now;
      const idx = Math.floor((now - anchor) / period);
      const beatAt = anchor + idx * period;
      if (beatAt > this.lastBeat && now - beatAt < period / 2) {
        this.lastBeat = beatAt;
        this.emit();
      }
    } else if (!this.audio?.active) {
      // idle pulse: 60 bpm, so "every 8 beats" ≈ 8 s
      if (now - this.lastBeat >= 1000) {
        this.lastBeat = now;
        this.emit();
      }
    }
  }

  destroy() {
    cancelAnimationFrame(this.raf);
    this.audio?.stop();
  }
}

/** Onset detection on the low band of the audio input (spectral flux + adaptive threshold). */
class AudioBeat {
  active = false;
  energy = 0;
  bpm = 0;
  private ctx: AudioContext | null = null;
  private stream: MediaStream | null = null;
  private timer = 0;
  private history: number[] = [];
  private onsets: number[] = [];
  private lastOnset = 0;
  private prev: Float32Array | null = null;

  constructor(private onBeat: () => void) {}

  async start() {
    this.stream = await navigator.mediaDevices.getUserMedia({
      audio: { echoCancellation: false, noiseSuppression: false, autoGainControl: false },
    });
    this.ctx = new AudioContext();
    const src = this.ctx.createMediaStreamSource(this.stream);
    const an = this.ctx.createAnalyser();
    an.fftSize = 1024;
    an.smoothingTimeConstant = 0.2;
    src.connect(an);
    const bins = new Float32Array(an.frequencyBinCount);
    const lowBins = Math.max(4, Math.round((150 / (this.ctx.sampleRate / 2)) * bins.length));
    this.active = true;
    this.timer = window.setInterval(() => {
      an.getFloatFrequencyData(bins);
      let flux = 0;
      let e = 0;
      for (let i = 0; i < lowBins; i++) {
        const v = Math.pow(10, bins[i] / 20);
        e += v;
        if (this.prev) flux += Math.max(0, v - this.prev[i]);
      }
      this.prev = this.prev ?? new Float32Array(lowBins);
      for (let i = 0; i < lowBins; i++) this.prev[i] = Math.pow(10, bins[i] / 20);
      this.energy = Math.min(1, this.energy * 0.9 + (e / lowBins) * 4);
      this.history.push(flux);
      if (this.history.length > 43) this.history.shift(); // ~1 s at 23 ms
      const mean = this.history.reduce((a, b) => a + b, 0) / this.history.length;
      const sd = Math.sqrt(this.history.reduce((a, b) => a + (b - mean) ** 2, 0) / this.history.length);
      const now = performance.now();
      if (flux > mean + 1.5 * sd && flux > 0.002 && now - this.lastOnset > 280) {
        this.lastOnset = now;
        this.onsets.push(now);
        if (this.onsets.length > 16) this.onsets.shift();
        if (this.onsets.length > 4) {
          const gaps = this.onsets.slice(1).map((t, i) => t - this.onsets[i]).sort((a, b) => a - b);
          const med = gaps[Math.floor(gaps.length / 2)];
          let bpm = 60000 / med;
          while (bpm < 80) bpm *= 2;
          while (bpm > 180) bpm /= 2;
          this.bpm = Math.round(bpm);
        }
        this.onBeat();
      }
    }, 23);
  }

  stop() {
    clearInterval(this.timer);
    this.stream?.getTracks().forEach((t) => t.stop());
    this.ctx?.close();
    this.active = false;
  }
}

/** Tap tempo for the cockpit: median of the last taps. */
export class TapTempo {
  private taps: number[] = [];
  tap(): { bpm: number; anchor: number } | null {
    const now = Date.now();
    if (this.taps.length && now - this.taps[this.taps.length - 1] > 2500) this.taps = [];
    this.taps.push(now);
    if (this.taps.length > 8) this.taps.shift();
    if (this.taps.length < 4) return null;
    const gaps = this.taps.slice(1).map((t, i) => t - this.taps[i]).sort((a, b) => a - b);
    const med = gaps[Math.floor(gaps.length / 2)];
    return { bpm: Math.round((60000 / med) * 10) / 10, anchor: now };
  }
  get count() {
    return this.taps.length;
  }
}
