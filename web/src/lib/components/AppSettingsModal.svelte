<script lang="ts">
  import { getConfig, setConfig } from '../stores/apps'
  import { openStream } from '../live/stream'
  import { renderSize } from '../format'

  let {
    id,
    name,
    managed,
    onclose,
  }: { id: string; name: string; managed: boolean; onclose: () => void } = $props()

  type Tab = 'config' | 'logs' | 'stats'
  let tab = $state<Tab>(managed ? 'config' : 'logs')

  // --- Config ---
  let override = $state('')
  let baseCompose = $state('')
  let configLoaded = $state(false)
  let saving = $state(false)
  let saveMsg = $state('')

  $effect(() => {
    if (managed && !configLoaded) {
      getConfig(id).then((c) => {
        baseCompose = c.base
        override = c.override
        configLoaded = true
      })
    }
  })

  async function save() {
    saving = true
    saveMsg = ''
    try {
      await setConfig(id, override)
      saveMsg = 'Saved and recreated.'
    } catch (e) {
      saveMsg = String(e)
    } finally {
      saving = false
    }
  }

  // --- Logs (streamed while the logs tab is active) ---
  let logLines = $state<string[]>([])
  $effect(() => {
    if (tab !== 'logs') return
    logLines = []
    const close = openStream(`/api/apps/${encodeURIComponent(id)}/logs`, (line) => {
      logLines = [...logLines.slice(-500), line]
    })
    return close
  })

  // --- Stats (streamed while the stats tab is active) ---
  let stat = $state<{ cpu_percent: number; mem_usage: number; mem_limit: number } | null>(null)
  $effect(() => {
    if (tab !== 'stats') return
    stat = null
    const close = openStream(`/api/apps/${encodeURIComponent(id)}/stats`, (raw) => {
      try {
        stat = JSON.parse(raw)
      } catch {
        /* ignore */
      }
    })
    return close
  })
</script>

<div class="backdrop" onclick={onclose} role="presentation">
  <div class="modal" onclick={(e) => e.stopPropagation()} role="presentation">
    <header>
      <h2>{name}</h2>
      <button class="close" aria-label="Close" onclick={onclose}>✕</button>
    </header>

    <nav class="tabs">
      {#if managed}
        <button class:active={tab === 'config'} onclick={() => (tab = 'config')}>Settings</button>
      {/if}
      <button class:active={tab === 'logs'} onclick={() => (tab = 'logs')}>Logs</button>
      <button class:active={tab === 'stats'} onclick={() => (tab = 'stats')}>Stats</button>
    </nav>

    <div class="content">
      {#if tab === 'config'}
        <p class="hint">
          Edits are saved to a separate <code>docker-compose.override.yml</code> — the original
          compose is never modified.
        </p>
        <label class="lbl">Override compose</label>
        <textarea bind:value={override} spellcheck="false" placeholder="services:\n  ..."></textarea>
        <details class="base">
          <summary>View base compose (read-only)</summary>
          <pre>{baseCompose}</pre>
        </details>
        <div class="actions">
          <span class="msg">{saveMsg}</span>
          <button class="primary" disabled={saving} onclick={save}>
            {saving ? 'Saving…' : 'Save & recreate'}
          </button>
        </div>
      {:else if tab === 'logs'}
        <pre class="logs">{logLines.join('\n')}</pre>
      {:else}
        <div class="stats">
          {#if stat}
            <div class="stat">
              <span class="k">CPU</span>
              <div class="bar"><div class="fill" style:width={`${Math.min(100, stat.cpu_percent)}%`}></div></div>
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
    </div>
  </div>
</div>

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
  code {
    background: rgba(255, 255, 255, 0.1);
    padding: 0 0.25rem;
    border-radius: 4px;
  }
  .lbl {
    font-size: 0.8rem;
    color: var(--grey-400);
    margin-bottom: 0.25rem;
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
  .base {
    margin: 0.5rem 0;
    font-size: 0.8rem;
    color: var(--grey-400);
  }
  .base pre {
    max-height: 12rem;
    overflow: auto;
    background: rgba(0, 0, 0, 0.3);
    padding: 0.6rem;
    border-radius: 8px;
    font-size: 0.75rem;
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
  .stat .k {
    color: var(--grey-400);
    font-size: 0.85rem;
  }
  .stat .v {
    text-align: right;
    font-size: 0.85rem;
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
