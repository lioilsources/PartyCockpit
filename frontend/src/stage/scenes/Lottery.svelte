<script lang="ts">
  import { t } from '../../lib/i18n.svelte';
  import { onDestroy } from 'svelte';

  // Lottery for the intermission games (D6): names spin, then the winners land.
  let { params }: { params: Record<string, any> } = $props();

  const candidates: string[] = $derived(params.candidates ?? []);
  const winners: { nickname: string }[] = $derived(params.winners ?? []);
  let shown = $state('');
  let revealed = $state(0);
  let timer: ReturnType<typeof setTimeout>;

  function spin(step = 0) {
    if (!candidates.length) return;
    shown = candidates[Math.floor(Math.random() * candidates.length)];
    const delay = 50 + step * step * 0.9;
    if (delay > 450) {
      revealed++;
      if (revealed < winners.length) timer = setTimeout(() => spin(0), 1600);
      return;
    }
    timer = setTimeout(() => spin(step + 1), delay);
  }

  $effect(() => {
    revealed = 0;
    if (winners.length) spin();
  });
  onDestroy(() => clearTimeout(timer));
</script>

<div class="lot">
  <h2>{params.game ? t('stage.lot.who', { game: params.game }) : t('stage.lot.title')}</h2>
  {#if winners.length === 0}
    <p class="muted">{t('stage.lot.wait')}</p>
  {:else}
    {#if revealed < winners.length}<div class="spin">{shown}</div>{/if}
    <div class="winners">
      {#each winners.slice(0, revealed) as w (w.nickname)}
        <div class="w">🎉 {w.nickname}</div>
      {/each}
    </div>
  {/if}
  <p class="pool">{t('stage.lot.pool', { n: candidates.length })}</p>
</div>

<style>
  .lot {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: radial-gradient(circle, #3a0f2c, #0b0613 70%);
  }
  h2 {
    font-size: 2.6vw;
    color: var(--yellow);
    margin: 0 0 4vh;
  }
  .spin {
    font-size: 7vw;
    font-weight: 700;
    color: var(--muted);
  }
  .winners {
    display: flex;
    gap: 3vw;
    flex-wrap: wrap;
    justify-content: center;
  }
  .w {
    font-size: 6vw;
    font-weight: 700;
    animation: pop 0.8s cubic-bezier(0.2, 1.6, 0.4, 1) both;
    text-shadow: 0 0 3vw rgba(255, 61, 139, 0.8);
  }
  .pool {
    position: absolute;
    bottom: 3vh;
    color: var(--muted);
    font-size: 1.2vw;
  }
  @keyframes pop {
    from {
      transform: scale(0.2);
      opacity: 0;
    }
  }
</style>
