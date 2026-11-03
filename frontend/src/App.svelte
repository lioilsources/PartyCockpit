<script lang="ts">
  import Home from './Home.svelte';
  import Stage from './stage/Stage.svelte';
  import Cockpit from './cockpit/Cockpit.svelte';
  import Guest from './guest/Guest.svelte';

  // /stage/<code>  /cockpit/<code>  /join/<code>  /
  const [, section = '', raw = ''] = location.pathname.split('/');
  const code = raw.toUpperCase();
</script>

{#if section === 'stage' && code}
  <Stage {code} />
{:else if section === 'cockpit' && code}
  <Cockpit {code} />
{:else if section === 'join' && code}
  <Guest {code} />
{:else}
  <Home />
{/if}
