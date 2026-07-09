<script lang="ts">
  import { uninstallTarget } from '../stores/ui'
  import { uninstallApp } from '../stores/apps'
  import { t } from '../i18n'

  let { target }: { target: { id: string; name: string } } = $props()

  let archive = $state(false)
  let busy = $state(false)
  let error = $state('')

  function close() {
    if (!busy) uninstallTarget.set(null)
  }

  async function confirm() {
    busy = true
    error = ''
    try {
      await uninstallApp(target.id, archive)
      uninstallTarget.set(null)
    } catch (e) {
      error = String(e)
      busy = false
    }
  }
</script>

<div class="backdrop" onclick={close} role="presentation">
  <div class="dialog" onclick={(e) => e.stopPropagation()} role="presentation">
    <h2>{$t('uninstall')} {target.name}?</h2>
    <p class="body">
      This removes the app and its containers.
    </p>

    <label class="check">
      <input type="checkbox" bind:checked={archive} disabled={busy} />
      <span>Archive &amp; remove app data</span>
    </label>
    <p class="note">
      {#if archive}
        The app's data folder will be zipped to a <code>.zip</code> in <code>AppData/</code>, then
        deleted.
      {:else}
        The app's data folder is kept untouched.
      {/if}
    </p>

    {#if error}<p class="error">{error}</p>{/if}

    <div class="actions">
      <button class="ghost" onclick={close} disabled={busy}>{$t('cancel')}</button>
      <button class="danger" onclick={confirm} disabled={busy}>
        {busy ? '…' : $t('uninstall')}
      </button>
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 110;
    background: rgba(0, 0, 0, 0.5);
    display: grid;
    place-items: center;
  }
  .dialog {
    width: min(92vw, 26rem);
    background: #fff;
    border-radius: 14px;
    padding: 1.25rem 1.4rem;
    color: var(--grey-800);
  }
  h2 {
    margin: 0 0 0.5rem;
    font-size: 1.1rem;
  }
  .body {
    margin: 0 0 0.9rem;
    color: var(--grey-600);
    font-size: 0.9rem;
  }
  .check {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
    cursor: pointer;
  }
  .note {
    margin: 0.4rem 0 0;
    font-size: 0.78rem;
    color: var(--grey-600);
  }
  code {
    background: hsla(208, 16%, 94%, 1);
    padding: 0 0.25rem;
    border-radius: 4px;
  }
  .error {
    color: var(--red);
    font-size: 0.8rem;
    margin: 0.6rem 0 0;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    margin-top: 1.1rem;
  }
  .actions button {
    padding: 0.5rem 1.1rem;
    border-radius: 8px;
    border: none;
    font-size: 0.875rem;
  }
  .ghost {
    background: hsla(208, 16%, 94%, 1);
    color: var(--grey-800);
  }
  .danger {
    background: var(--red);
    color: #fff;
  }
  button:disabled {
    opacity: 0.6;
  }
</style>
