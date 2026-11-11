<script lang="ts">
  import Home from './Home.svelte';
  import Stage from './stage/Stage.svelte';
  import Cockpit from './cockpit/Cockpit.svelte';
  import Guest from './guest/Guest.svelte';
  import Recap from './recap/Recap.svelte';
  import Box from './Box.svelte';
  import Studio from './studio/Studio.svelte';

  // /stage/<code>  /cockpit/<code>  /studio/<code>  /join/<code>  /recap/<code>  /box  /
  const [, section = '', raw = ''] = location.pathname.split('/');
  const code = raw.toUpperCase();
</script>

{#if section === 'stage' && code}
  <Stage {code} />
{:else if section === 'cockpit' && code}
  <Cockpit {code} />
{:else if section === 'studio' && code}
  <Studio {code} />
{:else if section === 'join' && code}
  <Guest {code} />
{:else if section === 'recap' && code}
  <Recap {code} />
{:else if section === 'box'}
  <Box />
{:else}
  <Home />
{/if}
