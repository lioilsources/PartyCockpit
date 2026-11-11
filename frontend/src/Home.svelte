<script lang="ts">
  import { api, store } from './lib/api';
  import type { Party } from './lib/types';

  let adminToken = $state(store.get('pc.admin') ?? '');
  let name = $state('');
  let err = $state('');
  let parties = $state<(Party & { cockpitToken: string; stageKey: string })[]>([]);
  let joinCode = $state('');

  async function load() {
    try {
      parties = await api.adminGet('/api/parties', adminToken);
      err = '';
    } catch (e: any) {
      if (e.status !== 401) err = e.message;
      parties = [];
    }
  }

  async function create() {
    err = '';
    try {
      store.set('pc.admin', adminToken);
      const r = await api.adminPost<{ party: Party; cockpitToken: string; stageKey: string }>('/api/parties', { name }, adminToken);
      store.set(`pc.cockpit.${r.party.code}`, r.cockpitToken);
      store.set(`pc.stage.key.${r.party.code}`, r.stageKey);
      location.href = `/cockpit/${r.party.code}`;
    } catch (e: any) {
      err = e.message;
    }
  }

  // prefix '/react': the second Cockpit implementation (D21), same token
  function cockpit(p: Party & { cockpitToken: string; stageKey: string }, prefix = '', page = 'cockpit') {
    store.set(`pc.cockpit.${p.code}`, p.cockpitToken);
    location.href = `${prefix}/${page}/${p.code}`;
  }

  $effect(() => {
    load();
  });
</script>

<main>
  <h1>Party<span>Cockpit</span></h1>
  <p class="muted">Velké plátno, telefony hostů, hudba z DJfy, obrázky ze Sparku.</p>

  <section>
    <h2>Připojit se jako host</h2>
    <form onsubmit={(e) => { e.preventDefault(); if (joinCode) location.href = `/join/${joinCode.trim().toUpperCase()}`; }}>
      <input placeholder="Kód party (z plátna)" bind:value={joinCode} autocapitalize="characters" />
      <button class="primary">Vstoupit</button>
    </form>
  </section>

  <section>
    <h2>Nová party</h2>
    <form onsubmit={(e) => { e.preventDefault(); create(); }}>
      <input placeholder="Název (např. Olinovy narozeniny)" bind:value={name} />
      <input placeholder="Admin token (PC_ADMIN_TOKEN, pokud je nastaven)" type="password" bind:value={adminToken} onchange={load} />
      <button class="primary">Založit party</button>
    </form>
    {#if err}<p class="err">{err}</p>{/if}
  </section>

  {#if parties.length}
    <section>
      <h2>Party</h2>
      <ul>
        {#each parties as p (p.code)}
          <li>
            <strong>{p.name}</strong> <code>{p.code}</code>
            <span class="muted">{new Date(p.createdAt).toLocaleString('cs')}</span>
            <button onclick={() => cockpit(p)}>Cockpit</button>
            <button onclick={() => cockpit(p, '/react')}>Cockpit (React)</button>
            <button onclick={() => cockpit(p, '', 'studio')}>🎬 Studio</button>
            <a href={`/stage/${p.code}?key=${p.stageKey}`} target="_blank">Plátno</a>
          </li>
        {/each}
      </ul>
    </section>
  {/if}
</main>

<style>
  main {
    max-width: 560px;
    margin: 0 auto;
    padding: 48px 16px;
  }
  h1 {
    font-size: 2.6rem;
    margin: 0 0 4px;
    letter-spacing: -0.03em;
  }
  h1 span {
    color: var(--pink);
  }
  h2 {
    font-size: 1rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--muted);
  }
  section {
    margin-top: 32px;
  }
  form {
    display: grid;
    gap: 10px;
  }
  ul {
    list-style: none;
    padding: 0;
    display: grid;
    gap: 8px;
  }
  li {
    display: flex;
    gap: 10px;
    align-items: center;
    flex-wrap: wrap;
    background: var(--panel);
    padding: 10px 12px;
    border-radius: var(--radius);
  }
  code {
    color: var(--yellow);
  }
</style>
