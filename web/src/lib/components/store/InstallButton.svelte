<script lang="ts">
  import { installAppStream } from '../../stores/store'
  import { loadApps } from '../../stores/apps'
  import { t } from '../../i18n'

  let {
    id,
    installed = false,
    size = 'small',
  }: { id: string; installed?: boolean; size?: 'small' | 'normal' } = $props()

  let installing = $state(false)
  let pct = $state(0)
  let error = $state('')

  function install(e: MouseEvent) {
    e.stopPropagation()
    if (installed || installing) return
    installing = true
    pct = 0
    error = ''
    const close = installAppStream(id, (ev) => {
      if (ev.phase === 'error') {
        error = ev.message
        installing = false
        close()
        return
      }
      pct = Math.max(pct, Math.round(ev.percent))
      if (ev.phase === 'done') {
        installed = true
        installing = false
        loadApps()
        close()
      }
    })
  }
</script>

{#if installing}
  <span class="pill installing" class:normal={size === 'normal'} title="{pct}%">
    <span class="track"><span class="fill" style:width={`${pct}%`}></span></span>
    <span class="pct">{pct}%</span>
  </span>
{:else}
  <button
    class="pill"
    class:done={installed}
    class:normal={size === 'normal'}
    disabled={installed}
    onclick={install}
    title={error}
  >
    {installed ? $t('installed') : $t('install')}
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
