<script lang="ts">
  import { isVideo } from './types';

  // A still, an animated GIF or a looping muted video — whatever the file is.
  let {
    src,
    rate = 1,
    cls = '',
  }: { src: string; rate?: number; cls?: string } = $props();

  let video = $state<HTMLVideoElement | null>(null);
  $effect(() => {
    if (video) video.playbackRate = rate;
  });
</script>

{#if isVideo(src)}
  <video bind:this={video} class={cls} {src} autoplay loop muted playsinline></video>
{:else}
  <img class={cls} {src} alt="" />
{/if}
