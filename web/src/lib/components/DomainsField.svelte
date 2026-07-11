<script lang="ts">
  import { settings, saveDomains, type Domain } from '../stores/settings'
  import { t } from '../i18n'

  // The two domains a Yundera box answers on besides its own: both are wildcard
  // DNS services that encode the public IP in the hostname, so an app stays
  // reachable with no DNS set up at all. They differ only in TLS — nip.io goes
  // through the gateway's certificate, sslip.io gets its own from Let's Encrypt —
  // which is exactly what `directives` carries.
  const PRESETS: Domain[] = [
    { name: 'sslip', domain: '${APP_PUBLIC_IP_DASH}.sslip.io' },
    { name: 'nip', domain: '${APP_PUBLIC_IP_DASH}.nip.io', directives: { import: 'gateway_tls' } },
  ]

  let busy = $state(false)
  let error = $state('')
  let adding = $state(false)
  let name = $state('')
  let host = $state('')

  const current = $derived($settings.domains ?? [])
  const unused = $derived(PRESETS.filter((p) => !current.some((d) => d.domain === p.domain)))

  async function apply(list: Domain[]) {
    busy = true
    error = ''
    try {
      await saveDomains(list)
    } catch (e) {
      error = String(e)
    } finally {
      busy = false
    }
  }

  function add(d: Domain) {
    apply([...current, d])
  }

  function remove(d: Domain) {
    apply(current.filter((x) => x.domain !== d.domain))
  }

  function addCustom() {
    if (!name.trim() || !host.trim()) return
    add({ name: name.trim(), domain: host.trim() })
    name = ''
    host = ''
    adding = false
  }
</script>

<div class="field">
  <span>{$t('domains')}</span>
  <p class="hint">{$t('domains_hint')}</p>

  {#each current as d (d.domain)}
    <div class="row">
      <span class="name">{d.name}</span>
      <code title={d.domain}>{d.domain}</code>
      <button class="trash" aria-label={$t('remove')} disabled={busy} onclick={() => remove(d)}>✕</button>
    </div>
  {/each}

  <div class="add">
    {#each unused as p (p.domain)}
      <button class="chip" disabled={busy} onclick={() => add(p)}>+ {p.name}</button>
    {/each}
    {#if !adding}
      <button class="chip" disabled={busy} onclick={() => (adding = true)}>+ {$t('custom')}</button>
    {/if}
  </div>

  {#if adding}
    <div class="custom">
      <input placeholder={$t('domain_name')} bind:value={name} />
      <input
        placeholder="lan.example.com"
        bind:value={host}
        onkeydown={(e) => e.key === 'Enter' && addCustom()}
      />
      <button class="go" disabled={busy} onclick={addCustom}>{busy ? '…' : $t('add')}</button>
    </div>
  {/if}

  {#if busy}<p class="note">{$t('republishing')}</p>{/if}
  {#if error}<p class="err">{error}</p>{/if}
</div>

<style>
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    font-size: 0.8rem;
    color: var(--grey-600);
  }
  .hint {
    margin: 0;
    font-size: 0.72rem;
    line-height: 1.35;
    color: var(--grey-600);
  }
  .row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }
  .name {
    color: var(--grey-800);
    font-size: 0.82rem;
  }
  code {
    flex: 1;
    font-size: 0.7rem;
    color: var(--grey-600);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .trash {
    border: none;
    background: none;
    color: var(--red);
    font-size: 0.8rem;
    cursor: pointer;
  }
  .trash:disabled {
    opacity: 0.3;
    cursor: default;
  }
  .add {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }
  .chip {
    border: 1px dashed #cfcfcf;
    background: none;
    color: var(--casablue);
    border-radius: 999px;
    padding: 0.2rem 0.6rem;
    font-size: 0.75rem;
    cursor: pointer;
  }
  .chip:disabled {
    opacity: 0.5;
    cursor: default;
  }
  .custom {
    display: flex;
    gap: 0.3rem;
  }
  .custom input {
    min-width: 0;
    flex: 1;
    height: 1.8rem;
    border: 1px solid #cfcfcf;
    border-radius: 4px;
    padding: 0 0.4rem;
    font-size: 0.75rem;
  }
  .go {
    border: none;
    background: var(--casablue);
    color: #fff;
    border-radius: 6px;
    padding: 0 0.6rem;
    font-size: 0.75rem;
  }
  .note {
    margin: 0;
    font-size: 0.72rem;
    color: var(--grey-600);
  }
  .err {
    margin: 0;
    font-size: 0.72rem;
    color: var(--red);
  }
</style>
