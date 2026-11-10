<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { onDestroy, onMount } from 'svelte';
  import type { BeatClock } from '../../lib/beat';

  // Kruh (D17): circular pong for the players of the last lottery draw. Each
  // player guards an arc of the ring with a paddle steered from their phone
  // (0..1 along the arc). A ball through your arc costs a life; three lives,
  // last one standing wins. The ball speeds up on the beat. A player whose
  // phone went quiet gets a slow autopilot, so one dropped phone doesn't stop
  // the game.
  type Player = { slot: number; nickname: string; color: string };
  let { params, clock }: { params: Record<string, any>; clock: BeatClock } = $props();

  const players: Player[] = (params.players ?? []).slice(0, 8);
  const N = Math.max(2, players.length);
  const LIVES = 3;
  const ARC = (Math.PI * 2) / N;
  const PADDLE = ARC * 0.34;
  const QUIET_MS = 6000;

  type Seat = Player & { target: number; x: number; lives: number; lastInput: number; hitAt: number };
  const seats: Seat[] = players.map((p) => ({ ...p, target: 0.5, x: 0.5, lives: LIVES, lastInput: 0, hitAt: 0 }));
  type Ball = { x: number; y: number; vx: number; vy: number; waitUntil: number };
  let balls: Ball[] = [];
  let speed = 0.5; // in radii per second
  let winner = $state<Seat | null>(null);
  let countdown = $state(3);

  let canvas: HTMLCanvasElement;
  let ctx: CanvasRenderingContext2D;
  let raf = 0;
  let last = 0;
  let pulse = 0;

  export function input(slot: number, x: number) {
    const s = seats.find((p) => p.slot === slot);
    if (!s) return;
    s.target = x;
    s.lastInput = performance.now();
  }

  const arcStart = (i: number) => -Math.PI / 2 + i * ARC - ARC / 2;
  const paddleAngle = (s: Seat, i: number) => arcStart(i) + PADDLE / 2 + s.x * (ARC - PADDLE);
  const alive = () => seats.filter((s) => s.lives > 0);
  const norm = (a: number) => ((a % (Math.PI * 2)) + Math.PI * 2) % (Math.PI * 2);

  function serve(b: Ball, now: number, delay = 1200) {
    const a = Math.random() * Math.PI * 2;
    b.x = 0;
    b.y = 0;
    b.vx = Math.cos(a);
    b.vy = Math.sin(a);
    b.waitUntil = now + delay;
  }

  function step(now: number) {
    const dt = Math.min(0.05, (now - last) / 1000);
    last = now;
    if (winner || countdown > 0) return;
    const R = 1;
    const rBall = 0.035;
    for (const [i, s] of seats.entries()) {
      // autopilot follows the nearest ball, a little slow and late
      if (s.lives > 0 && now - s.lastInput > QUIET_MS && balls.length) {
        const b = balls[0];
        const a = norm(Math.atan2(b.y, b.x) - arcStart(i));
        const want = Math.min(1, Math.max(0, (a - PADDLE / 2) / (ARC - PADDLE)));
        s.target += (want - s.target) * 0.04;
      }
      s.x += (s.target - s.x) * Math.min(1, dt * 14);
    }
    for (const b of balls) {
      if (now < b.waitUntil) continue;
      b.x += b.vx * speed * dt;
      b.y += b.vy * speed * dt;
      const d = Math.hypot(b.x, b.y);
      if (d < R - rBall) continue;
      const phi = Math.atan2(b.y, b.x);
      const owner = Math.floor(norm(phi - arcStart(0)) / ARC) % N;
      const s = seats[owner];
      const nx = b.x / d;
      const ny = b.y / d;
      const bounce = (deflect: number) => {
        const dot = b.vx * nx + b.vy * ny;
        let vx = b.vx - 2 * dot * nx;
        let vy = b.vy - 2 * dot * ny;
        const c = Math.cos(deflect);
        const sn = Math.sin(deflect);
        [vx, vy] = [vx * c - vy * sn, vx * sn + vy * c];
        // never skim along the wall
        const inward = -(vx * nx + vy * ny);
        if (inward < 0.35) {
          vx -= nx * (0.35 - inward);
          vy -= ny * (0.35 - inward);
        }
        const l = Math.hypot(vx, vy);
        b.vx = vx / l;
        b.vy = vy / l;
        b.x = nx * (R - rBall - 0.002);
        b.y = ny * (R - rBall - 0.002);
      };
      if (!s || s.lives <= 0) {
        bounce(0);
        continue;
      }
      const off = Math.atan2(Math.sin(phi - paddleAngle(s, owner)), Math.cos(phi - paddleAngle(s, owner)));
      if (Math.abs(off) <= PADDLE / 2 + 0.03) {
        bounce((off / (PADDLE / 2)) * 0.45);
        s.hitAt = now;
        speed = Math.min(1.6, speed * 1.02);
      } else {
        s.lives--;
        s.hitAt = -now; // negative = got scored on
        if (alive().length <= 1) {
          winner = alive()[0] ?? null;
          return;
        }
        serve(b, now);
      }
    }
  }

  function draw(now: number) {
    const W = canvas.width;
    const H = canvas.height;
    const R = Math.min(W, H) * 0.36;
    const cx = W / 2;
    const cy = H / 2 + H * 0.02;
    ctx.clearRect(0, 0, W, H);
    pulse = Math.max(0, pulse - 0.04);
    // arena
    ctx.beginPath();
    ctx.arc(cx, cy, R * (1 + pulse * 0.01), 0, Math.PI * 2);
    ctx.fillStyle = 'rgba(255,255,255,0.03)';
    ctx.fill();
    for (const [i, s] of seats.entries()) {
      const a0 = arcStart(i);
      const out = s.lives <= 0;
      const scored = s.hitAt < 0 && now + s.hitAt < 500;
      // the arc a player guards
      ctx.beginPath();
      ctx.arc(cx, cy, R, a0 + 0.01, a0 + ARC - 0.01);
      ctx.strokeStyle = out ? 'rgba(255,255,255,0.18)' : scored ? '#ff5a5a' : s.color + '55';
      ctx.lineWidth = R * 0.012;
      ctx.stroke();
      if (!out) {
        // paddle
        const pa = paddleAngle(s, i);
        const glow = s.hitAt > 0 && now - s.hitAt < 200;
        ctx.beginPath();
        ctx.arc(cx, cy, R, pa - PADDLE / 2, pa + PADDLE / 2);
        ctx.strokeStyle = s.color;
        ctx.lineWidth = R * (glow ? 0.06 : 0.045);
        ctx.lineCap = 'round';
        ctx.shadowColor = s.color;
        ctx.shadowBlur = glow ? 40 : 18;
        ctx.stroke();
        ctx.shadowBlur = 0;
        ctx.lineCap = 'butt';
      }
      // name and lives outside the ring
      const mid = a0 + ARC / 2;
      const lx = cx + Math.cos(mid) * R * 1.17;
      const ly = cy + Math.sin(mid) * R * 1.17;
      ctx.textAlign = 'center';
      ctx.textBaseline = 'middle';
      ctx.font = `700 ${Math.round(R * 0.075)}px 'Space Grotesk', sans-serif`;
      ctx.fillStyle = out ? 'rgba(255,255,255,0.35)' : s.color;
      const bot = !out && countdown === 0 && now - s.lastInput > QUIET_MS ? ' 🤖' : '';
      ctx.fillText(s.nickname + bot, lx, ly);
      ctx.font = `${Math.round(R * 0.06)}px sans-serif`;
      ctx.fillText(out ? '✖' : '●'.repeat(s.lives) + '○'.repeat(LIVES - s.lives), lx, ly + R * 0.085);
    }
    // balls
    for (const b of balls) {
      const wait = now < b.waitUntil;
      ctx.beginPath();
      ctx.arc(cx + b.x * R, cy + b.y * R, R * 0.035 * (wait ? 0.7 + 0.3 * Math.sin(now / 90) : 1), 0, Math.PI * 2);
      ctx.fillStyle = '#fff';
      ctx.shadowColor = '#fff';
      ctx.shadowBlur = 24;
      ctx.fill();
      ctx.shadowBlur = 0;
    }
  }

  function frame(now: number) {
    raf = requestAnimationFrame(frame);
    if (canvas.width !== innerWidth * devicePixelRatio) {
      canvas.width = innerWidth * devicePixelRatio;
      canvas.height = innerHeight * devicePixelRatio;
    }
    step(now);
    draw(now);
  }

  const off = clock.on(() => {
    pulse = 1;
    if (!winner && countdown === 0) speed = Math.min(1.6, speed * 1.01);
  });

  let cdTimer = 0;
  onMount(() => {
    ctx = canvas.getContext('2d')!;
    last = performance.now();
    balls = Array.from({ length: N >= 5 ? 2 : 1 }, () => ({ x: 0, y: 0, vx: 1, vy: 0, waitUntil: 0 }));
    cdTimer = window.setInterval(() => {
      countdown--;
      if (countdown <= 0) {
        clearInterval(cdTimer);
        const now = performance.now();
        // the autopilot (and its 🤖) only takes over phones that stay quiet from here on
        for (const s of seats) s.lastInput = Math.max(s.lastInput, now);
        balls.forEach((b, i) => serve(b, now, i * 900));
      }
    }, 1000);
    raf = requestAnimationFrame(frame);
  });
  onDestroy(() => {
    off();
    cancelAnimationFrame(raf);
    clearInterval(cdTimer);
  });
</script>

<div class="kruh">
  <canvas bind:this={canvas}></canvas>
  <h1 class="title">{t('stage.kruh.title')}</h1>
  {#if players.length < 2}
    <div class="center"><p>{t('stage.kruh.need')}</p></div>
  {:else if countdown > 0}
    <div class="center"><span class="cd">{countdown}</span><p>{t('stage.kruh.hint')}</p></div>
  {:else if winner}
    <div class="center win" style:--c={winner.color}><p>{t('stage.kruh.winner')}</p><strong>{winner.nickname}</strong></div>
  {/if}
</div>

<style>
  .kruh {
    position: absolute;
    inset: 0;
    background: radial-gradient(circle at 50% 52%, #1a0d2e, #05020a 70%);
  }
  canvas {
    width: 100%;
    height: 100%;
    display: block;
  }
  .title {
    position: absolute;
    left: 3vw;
    top: 2.5vw;
    margin: 0;
    font-size: 3vw;
    letter-spacing: 0.3em;
    text-transform: uppercase;
    color: rgba(255, 255, 255, 0.35);
  }
  .center {
    position: absolute;
    inset: 0;
    display: grid;
    place-content: center;
    text-align: center;
    pointer-events: none;
  }
  .center p {
    margin: 0;
    font-size: 2vw;
    color: var(--muted);
  }
  .cd {
    font-size: 14vw;
    font-weight: 700;
    line-height: 1;
  }
  .win strong {
    font-size: 8vw;
    color: var(--c);
    text-shadow: 0 0 3vw var(--c);
    animation: pop 0.6s cubic-bezier(0.2, 1.6, 0.4, 1) both;
  }
  @keyframes pop {
    from {
      transform: scale(0.3);
      opacity: 0;
    }
  }
</style>
