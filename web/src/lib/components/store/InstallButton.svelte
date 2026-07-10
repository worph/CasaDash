<script lang="ts">
  import { installApp } from '../../stores/store'
  import { apps } from '../../stores/apps'
  import { sanitizeProject } from '../../project'
  import { t } from '../../i18n'

  let {
    id,
    installed = false,
    size = 'small',
  }: { id: string; installed?: boolean; size?: 'small' | 'normal' } = $props()

  // Progress is not owned here — it rides the live "apps" channel, the same
  // source the dashboard tile reads. This button just kicks the install off and
  // reflects whatever the channel reports, so it stays in sync with the tile
  // (and keeps advancing even if this store panel is closed and reopened).
  let starting = $state(false) // optimistic: click → channel confirms
  let error = $state('')

  const projectId = $derived(sanitizeProject(id))
  const entry = $derived($apps.find((a) => a.id === projectId))
  const installing = $derived(starting || entry?.installing === true)
  const download = $derived(entry?.download ?? 0)
  const start = $derived(entry?.start ?? 0)
  const pct = $derived(Math.round(download < 100 ? download : start))
  const failed = $derived(entry?.install_error ?? '')
  const isInstalled = $derived(installed || (!!entry && !entry.installing && !entry.install_error))

  // Drop the optimistic flag once the channel confirms the install is tracked.
  $effect(() => {
    if (entry?.installing || entry?.install_error) starting = false
  })

  async function install(e: MouseEvent) {
    e.stopPropagation()
    if (isInstalled || installing) return
    starting = true
    error = ''
    try {
      await installApp(id)
    } catch (err) {
      error = String(err)
      starting = false
    }
  }
</script>

{#if installing}
  <span class="pill installing" class:normal={size === 'normal'} title="{download < 100 ? $t('downloading') : $t('starting_up')} {pct}%">
    <span class="track"><span class="fill" style:width={`${pct}%`}></span></span>
    <span class="pct">{pct}%</span>
  </span>
{:else}
  <button
    class="pill"
    class:done={isInstalled}
    class:failed={!!failed || !!error}
    class:normal={size === 'normal'}
    disabled={isInstalled}
    onclick={install}
    title={failed || error}
  >
    {isInstalled ? $t('installed') : $t('install')}
  </button>
{/if}

<style>
  .pill {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    border: none;
    border-radius: 999px;
    font-size: 0.72rem;
    font-weight: 600;
    padding: 0.22rem 0.75rem;
    /* is-primary is-light: pale casablue bg + dark-blue text */
    background: hsla(216, 90%, 54%, 0.14);
    color: hsl(216, 72%, 42%);
    cursor: pointer;
    white-space: nowrap;
  }
  .pill:hover:not(:disabled) {
    background: hsla(216, 90%, 54%, 0.22);
  }
  .pill.normal {
    font-size: 0.85rem;
    padding: 0.4rem 1.2rem;
  }
  .pill.done {
    background: hsla(118, 70%, 45%, 0.16);
    color: hsl(118, 55%, 32%);
    cursor: default;
  }
  .pill.failed {
    background: hsla(6, 78%, 57%, 0.14);
    color: hsl(6, 60%, 45%);
  }
  .installing {
    background: hsla(216, 90%, 54%, 0.1);
    color: hsl(216, 72%, 42%);
  }
  .track {
    width: 46px;
    height: 4px;
    border-radius: 2px;
    background: hsla(216, 30%, 50%, 0.25);
    overflow: hidden;
  }
  .fill {
    display: block;
    height: 100%;
    background: var(--casablue);
    transition: width 0.3s ease;
  }
  .pct {
    font-variant-numeric: tabular-nums;
  }
</style>
