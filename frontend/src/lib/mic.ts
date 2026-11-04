/**
 * MC microphone for live subtitles (M7, DECISIONS D15). Captures the mic,
 * resamples to 16 kHz mono PCM16, cuts utterances with a simple energy VAD and
 * streams them to /ws/parties/<code>/mic, which relays to the Translator.
 */

export type MicState = 'off' | 'starting' | 'listening' | 'speaking' | 'error';

const RATE = 16000;
const FRAME = 320; // 20 ms at 16 kHz
const PREROLL = 10; // frames kept before speech starts (200 ms)
const HANGOVER = 35; // silent frames that end an utterance (700 ms)
const MAX_FRAMES = 600; // 12 s, the Translator takes at most 30 s

// AudioWorklet: forwards raw input blocks to the main thread.
const worklet = `class Tap extends AudioWorkletProcessor {
  process(inputs) { const c = inputs[0] && inputs[0][0]; if (c) this.port.postMessage(c.slice(0)); return true; }
}
registerProcessor('pc-tap', Tap);`;

export class Mic {
  state: MicState = 'off';
  level = 0; // 0..1 for the meter
  error = '';
  private ws: WebSocket | null = null;
  private ctx: AudioContext | null = null;
  private stream: MediaStream | null = null;
  private node: AudioWorkletNode | null = null;
  private pending: Float32Array = new Float32Array(0);
  private acc: number[] = []; // 16 kHz samples not yet framed
  private ratio = 1;
  private pos = 0; // fractional read position for resampling
  private preroll: Int16Array<ArrayBuffer>[] = [];
  private inSpeech = false;
  private silent = 0;
  private frames = 0;
  private floor = 0.004;

  constructor(private onChange: () => void) {}

  async start(code: string, token: string, langA: string, langB: string) {
    if (this.state !== 'off' && this.state !== 'error') return;
    this.set('starting');
    try {
      this.stream = await navigator.mediaDevices.getUserMedia({
        audio: { channelCount: 1, echoCancellation: true, noiseSuppression: true, autoGainControl: true },
      });
      this.ctx = new AudioContext();
      this.ratio = this.ctx.sampleRate / RATE;
      const url = URL.createObjectURL(new Blob([worklet], { type: 'text/javascript' }));
      await this.ctx.audioWorklet.addModule(url);
      URL.revokeObjectURL(url);
      const src = this.ctx.createMediaStreamSource(this.stream);
      this.node = new AudioWorkletNode(this.ctx, 'pc-tap');
      this.node.port.onmessage = (e) => this.push(e.data as Float32Array);
      src.connect(this.node);

      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      const q = new URLSearchParams({ token });
      this.ws = new WebSocket(`${proto}://${location.host}/ws/parties/${code}/mic?${q}`);
      this.ws.binaryType = 'arraybuffer';
      this.ws.onopen = () => this.ws?.send(JSON.stringify({ type: 'start', langA, langB }));
      this.ws.onmessage = (e) => {
        const m = JSON.parse(e.data);
        if (m.type === 'ready') this.set('listening');
        else if (m.type === 'error') this.fail(m.message || 'chyba Translatoru');
      };
      this.ws.onclose = (e) => {
        if (this.state !== 'off') this.fail(e.reason || 'spojení s mikrofonem skončilo');
      };
    } catch (e: any) {
      this.fail(e?.message ?? String(e));
    }
  }

  stop() {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify({ type: 'stop' }));
    this.state = 'off';
    this.teardown();
    this.onChange();
  }

  private fail(msg: string) {
    this.error = msg;
    this.state = 'error';
    this.teardown();
    this.onChange();
  }

  private teardown() {
    this.ws?.close();
    this.ws = null;
    this.node?.disconnect();
    this.stream?.getTracks().forEach((t) => t.stop());
    this.ctx?.close().catch(() => {});
    this.ctx = null;
    this.inSpeech = false;
    this.preroll = [];
    this.level = 0;
  }

  private set(s: MicState) {
    this.state = s;
    this.onChange();
  }

  // linear resample to 16 kHz, then cut into 20 ms frames
  private push(block: Float32Array) {
    const buf = new Float32Array(this.pending.length + block.length);
    buf.set(this.pending);
    buf.set(block, this.pending.length);
    const out: number[] = [];
    let p = this.pos;
    while (p + 1 < buf.length) {
      const i = Math.floor(p);
      const f = p - i;
      out.push(buf[i] * (1 - f) + buf[i + 1] * f);
      p += this.ratio;
    }
    const keep = Math.floor(p);
    this.pending = buf.slice(keep);
    this.pos = p - keep;
    this.acc.push(...out);
    while (this.acc.length >= FRAME) this.frame(this.acc.splice(0, FRAME));
  }

  private frame(samples: number[]) {
    let sum = 0;
    const pcm = new Int16Array(FRAME);
    for (let i = 0; i < FRAME; i++) {
      const v = Math.max(-1, Math.min(1, samples[i]));
      sum += v * v;
      pcm[i] = v < 0 ? v * 0x8000 : v * 0x7fff;
    }
    const rms = Math.sqrt(sum / FRAME);
    const prevLevel = this.level;
    this.level = Math.min(1, rms * 8);
    // noise floor follows the quiet parts slowly, music in the room raises it
    if (!this.inSpeech) this.floor = this.floor * 0.995 + rms * 0.005;
    const speech = rms > Math.max(0.012, this.floor * 2.5);

    if (!this.inSpeech) {
      this.preroll.push(pcm);
      if (this.preroll.length > PREROLL) this.preroll.shift();
      if (speech) {
        this.inSpeech = true;
        this.silent = 0;
        this.frames = 0;
        for (const f of this.preroll) this.send(f);
        this.preroll = [];
        this.set('speaking');
      }
    } else {
      this.send(pcm);
      this.frames++;
      this.silent = speech ? 0 : this.silent + 1;
      if (this.silent >= HANGOVER || this.frames >= MAX_FRAMES) {
        this.inSpeech = false;
        if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify({ type: 'end' }));
        this.set('listening');
      }
    }
    if (Math.abs(prevLevel - this.level) > 0.05) this.onChange();
  }

  private send(pcm: Int16Array<ArrayBuffer>) {
    if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(pcm);
  }
}
