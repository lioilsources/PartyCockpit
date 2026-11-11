<script lang="ts">
  import { api } from '../../lib/api';
  import { rng } from '../../lib/rand';
  import type { Asset, Episode } from '../../lib/types';
  import Strip from './Strip.svelte';

  // Webtoon reader (D7, D23): 1–4 episodes side by side, each column scrolling
  // at its own pace. Episode "party" is tonight's strip from approved photos (D18).
  //   params.episodes  episode ids (params.episode, one id, still works)
  //   params.speed     px per second at 1080p (default 60)
  //   params.lang      bubble language (default cs)
  //   params.loops     animated panels where the pack has them (default true)
  //   params.seed      fixes each column's pace and start (Studio show)
  let {
    params,
    code,
    queue,
    title,
    startedAt = 0,
  }: { params: Record<string, any>; code: string; queue: Asset[]; title: string; startedAt?: number } = $props();

  const asked: string[] = (Array.isArray(params.episodes) ? params.episodes : params.episode ? [params.episode] : [])
    .filter((x: unknown) => typeof x === 'string' && x)
    .slice(0, 4);
  let ids = $state<string[]>(asked);
  let err = $state('');
  if (!asked.length) {
    api
      .get<Episode[]>('/api/webtoon')
      .then((eps) => {
        ids = eps.slice(0, 1).map((e) => e.id);
        if (!ids.length) err = 'Žádné epizody. Stáhni balík ve Studiu nebo dej panely do data/webtoon/.';
      })
      .catch((e) => (err = e.message));
  }

  const speed = Math.min(400, Math.max(10, Number(params.speed) || 60));
  const lang = typeof params.lang === 'string' && params.lang ? params.lang : 'cs';
  const loops = params.loops !== false;
  // each column its own pace (±15 %) and a staggered start
  const rand = rng(params.seed, 1);
  const pace = asked.map(() => 0.85 + rand() * 0.3);
  const widths = ['46vw', '44vw', '30vw', '23vw'];
</script>

<div class="wt" style:--w={widths[Math.max(0, ids.length - 1)]}>
  {#each ids as id, i (`${i}:${id}`)}
    <div class="slot">
      <Strip {id} speed={speed * (pace[i] ?? 1)} delay={i * 2.5} {lang} {loops} {startedAt} {code} {queue} {title} />
    </div>
  {:else}
    <p class="muted">{err || 'načítám…'}</p>
  {/each}
</div>

<style>
  .wt {
    position: absolute;
    inset: 0;
    background: #111;
    display: flex;
    justify-content: center;
    gap: 1.5vw;
  }
  .slot {
    width: var(--w);
    height: 100%;
  }
  p.muted {
    margin-top: 40vh;
    font-size: 1.6vw;
  }
</style>
