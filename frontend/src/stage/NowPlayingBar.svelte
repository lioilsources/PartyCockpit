<script lang="ts">
  import type { MusicSnapshot } from '../lib/types';
  let { music }: { music: MusicSnapshot | null } = $props();
  const np = $derived(music?.nowPlaying);
</script>

{#if np}
  <div class="bar">
    {#if np.artworkUrl}<img src={np.artworkUrl} alt="" />{:else}<div class="art">♪</div>{/if}
    <div class="t">
      <strong>{np.title}</strong>
      <span>{np.artist}{np.proposedByName ? ` · přidal(a) ${np.proposedByName}` : ''}</span>
    </div>
  </div>
{/if}

<style>
  .bar {
    position: absolute;
    left: 2.2vw;
    bottom: 2.2vw;
    display: flex;
    align-items: center;
    gap: 1vw;
    background: rgba(11, 6, 19, 0.72);
    backdrop-filter: blur(10px);
    padding: 0.6vw 1.4vw 0.6vw 0.6vw;
    border-radius: 999px;
    max-width: 50vw;
    z-index: 20;
  }
  img,
  .art {
    width: 3.2vw;
    height: 3.2vw;
    border-radius: 50%;
    object-fit: cover;
    background: var(--pink);
    display: grid;
    place-items: center;
    font-size: 1.6vw;
  }
  .t {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  strong,
  span {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  strong {
    font-size: 1.25vw;
  }
  span {
    font-size: 0.95vw;
    color: var(--muted);
  }
</style>
