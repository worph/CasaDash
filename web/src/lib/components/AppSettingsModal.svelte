<script lang="ts">
  import {
    getConfig,
    setConfig,
    setWebUI,
    setNote,
    getServices,
    checkUpdate,
    applyUpdate,
    type WebUI,
    type AppService,
    type UpdateStatus,
  } from '../stores/apps'
  import { openStream } from '../live/stream'
  import { renderSize } from '../format'

  let {
    id,
    name,
    managed,
    onclose,
  }: { id: string; name: string; managed: boolean; onclose: () => void } = $props()

  // Managed apps get the config-editing pages (incl. Tips + note, which live in
  // the app folder); every app gets logs + stats.
  type Tab = 'tips' | 'webui' | 'compose' | 'override' | 'update' | 'logs' | 'stats'
  const tabs = $derived<Tab[]>(
    managed
      ? ['tips', 'webui', 'compose', 'override', 'update', 'logs', 'stats']
      : ['logs', 'stats'],
  )
  const tabLabel: Record<Tab, string> = {
    tips: 'Tips',
    webui: 'Web UI',
    compose: 'Compose',
    override: 'Override',
    update: 'Update',
    logs: 'Logs',
    stats: 'Stats',
  }
  let tab = $state<Tab>(managed ? 'tips' : 'logs')

  // --- Config (base compose / override / web-UI / tips / note) ---
  let baseCompose = $state('')
  let override = $state('')
  let webui = $state<WebUI>({ scheme: '', host: '', port: '', path: '', url: '' })
  let tips = $state('')
  let note = $state('')
  let configLoaded = $state(false)

  async function loadConfig() {
    const c = await getConfig(id)
    baseCompose = c.base
    override = c.override
    webui = c.webui
    tips = c.tips
    note = c.note
    configLoaded = true
  }
  $effect(() => {
    if (managed && !configLoaded) loadConfig()
  })

  // Note editor (the user's own scratchpad, persisted per-app).
  let savingNote = $state(false)
  let noteMsg = $state('')
  async function saveNote() {
    savingNote = true
    noteMsg = ''
    try {
      await setNote(id, note)
      noteMsg = 'Saved.'
    } catch (e) {
      noteMsg = String(e)
    } finally {
      savingNote = false
    }
  }
  // Override editor
  let savingOverride = $state(false)
  let overrideMsg = $state('')
  async function saveOverride() {
    savingOverride = true
    overrideMsg = ''
    try {
      await setConfig(id, override)
      overrideMsg = 'Saved & recreated.'
    } catch (e) {
      overrideMsg = String(e)
    } finally {
      savingOverride = false
    }
  }

  // --- Update (pull a fresher compose from the app's reference store) ---
  let update = $state<UpdateStatus | null>(null)
  let checkingUpdate = $state(false)
  let applyingUpdate = $state(false)
  let updateMsg = $state('')
  let updateChecked = $state(false) // one-shot: don't re-auto-check on error

  async function runCheckUpdate() {
    checkingUpdate = true
    updateChecked = true
    updateMsg = ''
    try {
      update = await checkUpdate(id)
    } catch (e) {
      updateMsg = String(e)
    } finally {
      checkingUpdate = false
    }
  }
  // Auto-check the first time the Update tab is opened.
  $effect(() => {
    if (tab === 'update' && !updateChecked) runCheckUpdate()
  })

  async function runApplyUpdate() {
    applyingUpdate = true
    updateMsg = ''
    try {
      const applied = await applyUpdate(id)
      updateMsg = applied ? 'Updated & recreated.' : 'Already up to date.'
      await runCheckUpdate() // refresh the status after applying
    } catch (e) {
      updateMsg = String(e)
    } finally {
      applyingUpdate = false
    }
  }

  // Web-UI (opening URL) form
  let savingWebui = $state(false)
  let webuiMsg = $state('')
  // A client-side approximation of the click URL (server resolves ${domain}).
  const previewUrl = $derived(
    (() => {
      const host = webui.host.trim()
      if (!host) return ''
      const scheme = webui.scheme.trim() || 'https'
      const port = webui.port.trim() ? `:${webui.port.trim()}` : ''
      let path = webui.path.trim() || '/'
      if (!path.startsWith('/')) path = '/' + path
      return `${scheme}://${host}${port}${path}`
    })(),
  )
  async function saveWebUI() {
    savingWebui = true
    webuiMsg = ''
    try {
      await setWebUI(id, {
        scheme: webui.scheme,
        host: webui.host,
        port: webui.port,
        path: webui.path,
      })
      await loadConfig() // refresh override + resolved URL
      webuiMsg = 'Saved & recreated.'
    } catch (e) {
      webuiMsg = String(e)
    } finally {
      savingWebui = false
    }
  }

  // --- Services (multi-service picker, shared by logs + stats) ---
  let services = $state<AppService[]>([])
  let selected = $state('')
  let servicesLoaded = $state(false)
  $effect(() => {
    getServices(id)
      .then((s) => {
        services = s
        if (s.length && !selected) selected = s[0].service
      })
      .catch(() => {
        services = []
      })
      .finally(() => (servicesLoaded = true))
  })
  const selectedSvc = $derived(services.find((s) => s.service === selected))

  function healthLabel(h: string): string {
    if (h === 'healthy') return 'Healthy'
    if (h === 'unhealthy') return 'Unhealthy'
    if (h === 'starting') return 'Starting…'
    return 'No health check'
  }

  // --- Logs (streamed while the logs tab is active, for the selected service) ---
  let logLines = $state<string[]>([])
  $effect(() => {
    if (tab !== 'logs' || !selected) return
    const svc = selected
    logLines = []
    const close = openStream(
      `/api/apps/${encodeURIComponent(id)}/logs?service=${encodeURIComponent(svc)}`,
      (line) => {
        logLines = [...logLines.slice(-500), line]
      },
    )
    return close
  })

  // --- Stats (streamed while the stats tab is active, for the selected service) ---
  let stat = $state<{
    cpu_percent: number
    mem_usage: number
    mem_limit: number
    health: string
  } | null>(null)
  $effect(() => {
    if (tab !== 'stats' || !selected) return
    const svc = selected
    stat = null
    const close = openStream(
      `/api/apps/${encodeURIComponent(id)}/stats?service=${encodeURIComponent(svc)}`,
      (raw) => {
        try {
          stat = JSON.parse(raw)
        } catch {
          /* ignore */
        }
      },
    )
    return close
  })
  // Prefer the live health from the stats frame, fall back to the services list.
  const liveHealth = $derived(stat?.health || selectedSvc?.health || '')
</script>

<div class="backdrop" onclick={onclose} role="presentation">
  <div class="modal" onclick={(e) => e.stopPropagation()} role="presentation">
    <header>
      <h2>{name}</h2>
      <button class="close" aria-label="Close" onclick={onclose}>✕</button>
    </header>

    <nav class="tabs">
      {#each tabs as t (t)}
        <button class:active={tab === t} onclick={() => (tab = t)}>{tabLabel[t]}</button>
      {/each}
    </nav>

    <div class="content">
      {#if tab === 'tips'}
        {#if !configLoaded}
          <p class="hint">Loading…</p>
        {:else}
          {#if tips.trim()}
            <p class="hint">Setup guidance shipped with this app by the store.</p>
            <pre class="tips">{tips}</pre>
          {:else}
            <p class="hint">The store didn't ship any setup tips for this app.</p>
          {/if}
          <p class="hint note-hint">
            Your <strong>note</strong> — a private scratchpad for this app (credentials reminders,
            reverse-proxy details, whatever you need). Saved alongside the app; never shown to anyone else.
          </p>
          <textarea
            bind:value={note}
            spellcheck="false"
            placeholder="Write a note for this app…"
          ></textarea>
          <div class="actions">
            <span class="msg">{noteMsg}</span>
            <button class="primary" disabled={savingNote} onclick={saveNote}>
              {savingNote ? 'Saving…' : 'Save note'}
            </button>
          </div>
        {/if}
      {:else if tab === 'webui'}
        <p class="hint">
          The <strong>opening URL</strong> is where the tile's <em>Open</em> button points. This is
          a friendlier shortcut for the <code>x-compose-app</code> web-UI fields — saving writes them
          into the <code>docker-compose.override.yml</code>.
        </p>
        <div class="form">
          <label>
            <span>Scheme</span>
            <select bind:value={webui.scheme}>
              <option value="">https (default)</option>
              <option value="https">https</option>
              <option value="http">http</option>
            </select>
          </label>
          <label>
            <span>Host</span>
            <input
              bind:value={webui.host}
              spellcheck="false"
              placeholder={`${id}-\${domain}`}
            />
          </label>
          <label>
            <span>Port</span>
            <input bind:value={webui.port} spellcheck="false" placeholder="(none — 443 via gateway)" />
          </label>
          <label>
            <span>Path</span>
            <input bind:value={webui.path} spellcheck="false" placeholder="/" />
          </label>
        </div>
        <p class="hint">
          <code>{'${domain}'}</code> resolves to the deployment domain when the app is brought up.
          {#if webui.url}
            Current URL: <a href={webui.url} target="_blank" rel="noopener">{webui.url}</a>
          {:else if previewUrl}
            Preview: <span class="mono">{previewUrl}</span>
          {:else}
            No opening URL set — the tile has no <em>Open</em> action.
          {/if}
        </p>
        <div class="actions">
          <span class="msg">{webuiMsg}</span>
          <button class="primary" disabled={savingWebui} onclick={saveWebUI}>
            {savingWebui ? 'Saving…' : 'Save & recreate'}
          </button>
        </div>
      {:else if tab === 'compose'}
        <p class="hint">
          The strict <code>docker-compose.yml</code> as shipped by the store — <strong>read-only</strong>.
          CasaDash never modifies it, so updates stay clean.
        </p>
        <pre class="code readonly">{baseCompose || (configLoaded ? '(empty)' : 'Loading…')}</pre>
      {:else if tab === 'override'}
        <p class="hint">
          Your edits, layered on top via Compose override semantics. The running stack is
          <code>docker-compose.yml</code> + this override.
        </p>
        <textarea
          bind:value={override}
          spellcheck="false"
          placeholder={'services:\n  app:\n    ports:\n      - "8080:80"'}
        ></textarea>
        <div class="actions">
          <span class="msg">{overrideMsg}</span>
          <button class="primary" disabled={savingOverride} onclick={saveOverride}>
            {savingOverride ? 'Saving…' : 'Save & recreate'}
          </button>
        </div>
      {:else if tab === 'update'}
        <p class="hint">
          Pulls a fresher <code>docker-compose.yml</code> from the store this app was installed
          from and re-applies it (<code>docker compose up -d</code>). Your
          <code>docker-compose.override.yml</code> and <code>.env</code> are left untouched.
        </p>
        {#if checkingUpdate && update === null}
          <p class="hint">Checking the store…</p>
        {:else if update && !update.has_ref}
          <p class="hint">
            This app has no store reference recorded, so CasaDash can't check for updates.
            Reinstall it from the store to enable updates.
          </p>
        {:else if update}
          <div class="update-box">
            <div class="update-row">
              <span class="k">Reference store</span>
              <span class="v mono">{update.store || '(merged catalog)'}</span>
            </div>
            <div class="update-row">
              <span class="k">Store app</span>
              <span class="v mono">{update.store_app_id}</span>
            </div>
            <div class="update-row">
              <span class="k">Status</span>
              {#if update.error}
                <span class="v badge health-unhealthy">Couldn't check</span>
              {:else if update.available}
                <span class="v badge health-starting">Update available</span>
              {:else}
                <span class="v badge health-healthy">Up to date</span>
              {/if}
            </div>
            {#if update.error}
              <p class="hint">{update.error}</p>
            {/if}
          </div>
        {/if}
        <div class="actions">
          <span class="msg">{updateMsg}</span>
          <button
            disabled={checkingUpdate || applyingUpdate}
            onclick={runCheckUpdate}
          >
            {checkingUpdate ? 'Checking…' : 'Check again'}
          </button>
          <button
            class="primary"
            disabled={applyingUpdate || checkingUpdate || !update?.available}
            onclick={runApplyUpdate}
          >
            {applyingUpdate ? 'Updating…' : 'Update now'}
          </button>
        </div>
      {:else if tab === 'logs'}
        {@render servicePicker()}
        {#if !servicesLoaded}
          <p class="hint">Loading…</p>
        {:else if !services.length}
          <p class="hint">No running containers — start the app to see logs.</p>
        {:else}
          <pre class="logs">{logLines.join('\n')}</pre>
        {/if}
      {:else if tab === 'stats'}
        {@render servicePicker()}
        {#if !servicesLoaded}
          <p class="hint">Loading…</p>
        {:else if !services.length}
          <p class="hint">No running containers — start the app to see stats.</p>
        {:else}
          <div class="stats">
            <div class="stat health">
              <span class="k">Health</span>
              <span class="badge health-{liveHealth || 'none'}">{healthLabel(liveHealth)}</span>
            </div>
            {#if stat}
              <div class="stat">
                <span class="k">CPU</span>
                <div class="bar">
                  <div class="fill" style:width={`${Math.min(100, stat.cpu_percent)}%`}></div>
                </div>
                <span class="v">{stat.cpu_percent}%</span>
              </div>
              <div class="stat">
                <span class="k">Memory</span>
                <div class="bar">
                  <div
                    class="fill"
                    style:width={`${stat.mem_limit ? Math.min(100, (stat.mem_usage / stat.mem_limit) * 100) : 0}%`}
                  ></div>
                </div>
                <span class="v">{renderSize(stat.mem_usage)}</span>
              </div>
            {:else}
              <p class="hint">Waiting for stats…</p>
            {/if}
          </div>
        {/if}
      {/if}
    </div>
  </div>
</div>

{#snippet servicePicker()}
  {#if services.length > 1}
    <div class="picker">
      <label for="svc-select">Service</label>
      <select id="svc-select" bind:value={selected}>
        {#each services as s (s.service)}
          <option value={s.service}>
            {s.service}{s.health ? ` · ${s.health}` : ''}{s.state !== 'running' ? ` · ${s.state}` : ''}
          </option>
        {/each}
      </select>
    </div>
  {/if}
{/snippet}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 95;
    background: rgba(0, 0, 0, 0.5);
    display: grid;
    place-items: center;
  }
  .modal {
    width: min(94vw, 820px);
    height: min(88vh, 680px);
    background: rgba(28, 30, 34, 0.92);
    backdrop-filter: blur(14px);
    border-radius: 14px;
    padding: 1.1rem 1.35rem;
    color: var(--grey-100);
    display: flex;
    flex-direction: column;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  h2 {
    margin: 0;
    font-size: 1.15rem;
  }
  .close {
    background: none;
    border: none;
    color: var(--grey-200);
    font-size: 1.1rem;
  }
  .tabs {
    display: flex;
    gap: 0.25rem;
    margin: 0.75rem 0;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }
  .tabs button {
    background: none;
    border: none;
    color: var(--grey-400);
    padding: 0.5rem 0.9rem;
    border-bottom: 2px solid transparent;
    font-size: 0.9rem;
  }
  .tabs button.active {
    color: var(--grey-100);
    border-bottom-color: var(--casablue);
  }
  .content {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  .hint {
    color: var(--grey-400);
    font-size: 0.8rem;
    margin: 0 0 0.5rem;
  }
  .hint a {
    color: var(--casablue);
  }
  code {
    background: rgba(255, 255, 255, 0.1);
    padding: 0 0.25rem;
    border-radius: 4px;
  }
  .mono {
    font-family: ui-monospace, monospace;
  }
  /* Web-UI form */
  .form {
    display: grid;
    gap: 0.6rem;
    margin-bottom: 0.5rem;
  }
  .form label {
    display: grid;
    grid-template-columns: 5rem 1fr;
    align-items: center;
    gap: 0.75rem;
  }
  .form label span {
    font-size: 0.8rem;
    color: var(--grey-400);
  }
  .form input,
  .form select {
    background: rgba(0, 0, 0, 0.35);
    color: var(--grey-100);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    padding: 0.5rem 0.6rem;
    font-family: ui-monospace, monospace;
    font-size: 0.8rem;
  }
  textarea {
    flex: 1;
    min-height: 8rem;
    background: rgba(0, 0, 0, 0.35);
    color: var(--grey-100);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    padding: 0.6rem;
    font-family: ui-monospace, monospace;
    font-size: 0.8rem;
    resize: none;
  }
  .code {
    flex: 1;
    overflow: auto;
    background: rgba(0, 0, 0, 0.3);
    padding: 0.6rem;
    border-radius: 8px;
    font-size: 0.75rem;
    margin: 0;
    white-space: pre;
  }
  .code.readonly {
    border: 1px solid rgba(255, 255, 255, 0.08);
  }
  /* Tips (store guidance, read-only) */
  .tips {
    max-height: 40%;
    overflow: auto;
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 8px;
    padding: 0.6rem 0.75rem;
    margin: 0 0 0.75rem;
    font-family: inherit;
    font-size: 0.82rem;
    line-height: 1.45;
    color: var(--grey-200);
    white-space: pre-wrap;
    word-break: break-word;
  }
  .note-hint {
    margin-top: 0.25rem;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.5rem;
  }
  .msg {
    font-size: 0.8rem;
    color: var(--grey-400);
  }
  .primary {
    background: var(--casablue);
    color: #fff;
    border: none;
    border-radius: 8px;
    padding: 0.5rem 1.1rem;
    font-size: 0.875rem;
  }
  .actions button:not(.primary) {
    background: rgba(255, 255, 255, 0.08);
    color: var(--grey-100);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    padding: 0.5rem 1.1rem;
    font-size: 0.875rem;
  }
  .actions button:disabled {
    opacity: 0.5;
  }
  /* Update tab */
  .update-box {
    display: flex;
    flex-direction: column;
    gap: 0.6rem;
    background: rgba(0, 0, 0, 0.25);
    border: 1px solid rgba(255, 255, 255, 0.08);
    border-radius: 8px;
    padding: 0.75rem 0.9rem;
    margin-bottom: 0.5rem;
  }
  .update-row {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: center;
    gap: 0.75rem;
  }
  .update-row .k {
    color: var(--grey-400);
    font-size: 0.8rem;
  }
  .update-row .v {
    font-size: 0.82rem;
    word-break: break-all;
  }
  /* Service picker */
  .picker {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    margin-bottom: 0.6rem;
  }
  .picker label {
    font-size: 0.8rem;
    color: var(--grey-400);
  }
  .picker select {
    background: rgba(0, 0, 0, 0.35);
    color: var(--grey-100);
    border: 1px solid rgba(255, 255, 255, 0.15);
    border-radius: 8px;
    padding: 0.35rem 0.6rem;
    font-size: 0.8rem;
  }
  .logs {
    flex: 1;
    overflow: auto;
    background: rgba(0, 0, 0, 0.4);
    border-radius: 8px;
    padding: 0.6rem;
    font-family: ui-monospace, monospace;
    font-size: 0.75rem;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
  }
  .stats {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    padding-top: 0.5rem;
  }
  .stat {
    display: grid;
    grid-template-columns: 4rem 1fr 6rem;
    align-items: center;
    gap: 0.75rem;
  }
  .stat.health {
    grid-template-columns: 4rem 1fr;
  }
  .stat .k {
    color: var(--grey-400);
    font-size: 0.85rem;
  }
  .stat .v {
    text-align: right;
    font-size: 0.85rem;
  }
  .badge {
    justify-self: start;
    font-size: 0.75rem;
    padding: 0.15rem 0.55rem;
    border-radius: 999px;
    border: 1px solid transparent;
  }
  .health-healthy {
    color: var(--status-running);
    border-color: var(--status-running);
  }
  .health-unhealthy {
    color: var(--red);
    border-color: var(--red);
  }
  .health-starting {
    color: var(--yellow);
    border-color: var(--yellow);
  }
  .health-none {
    color: var(--grey-400);
    border-color: rgba(255, 255, 255, 0.2);
  }
  .bar {
    height: 8px;
    background: rgba(255, 255, 255, 0.15);
    border-radius: 4px;
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background: var(--casablue);
    transition: width 0.4s;
  }
</style>
