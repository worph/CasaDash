<script lang="ts">
  import { clickOutside } from '../actions'
  import { appAction, openApp, type App } from '../stores/apps'
  import { removeLink, type Link } from '../stores/links'
  import { storeOpen, settingsApp, uninstallTarget } from '../stores/ui'
  import { t } from '../i18n'

  export type TileData =
    | { kind: 'system'; id: string; name: string }
    | { kind: 'app'; id: string; app: App }
    | { kind: 'link'; id: string; link: Link }

  let { tile }: { tile: TileData } = $props()

  let menuOpen = $state(false)
  let imgFailed = $state(false)

  const name = $derived(
    tile.kind === 'app' ? tile.app.name : tile.kind === 'link' ? tile.link.name : tile.name,
  )
  const icon = $derived(
    tile.kind === 'app' ? tile.app.icon : tile.kind === 'link' ? (tile.link.icon ?? '') : '',
  )
  const stopped = $derived(tile.kind === 'app' && tile.app.status === 'stopped')

  function open() {
    if (tile.kind === 'system') storeOpen.set(true)
    else if (tile.kind === 'app') openApp(tile.app)
    else window.open(tile.link.url, '_blank', 'noopener')
  }

  async function act(action: 'start' | 'stop' | 'restart') {
    menuOpen = false
    if (tile.kind === 'app') await appAction(tile.app.id, action)
  }

  function remove() {
    menuOpen = false
    if (tile.kind === 'app') {
      uninstallTarget.set({ id: tile.app.id, name })
    } else if (tile.kind === 'link') {
      removeLink(tile.link.id)
    }
  }
</script>

<div class="tile" class:stopped>
  <div class="glass"></div>

  {#if tile.kind !== 'system'}
    <button
      class="burger"
      aria-label="Menu"
      onclick={(e) => {
        e.stopPropagation()
        menuOpen = !menuOpen
      }}
    >
      ⋮
    </button>
  {/if}

  {#if menuOpen}
    <div class="menu" use:clickOutside={() => (menuOpen = false)}>
      <button onclick={() => { menuOpen = false; open() }}>{$t('open')}</button>
      {#if tile.kind === 'app'}
        <button
          onclick={() => {
            menuOpen = false
            if (tile.kind === 'app')
              settingsApp.set({ id: tile.app.id, name: tile.app.name, managed: tile.app.managed })
          }}>{$t('settings')}</button
        >
        {#if tile.app.status === 'stopped'}
          <button onclick={() => act('start')}>{$t('start')}</button>
        {:else}
          <button onclick={() => act('restart')}>{$t('restart')}</button>
          <button onclick={() => act('stop')}>{$t('stop')}</button>
        {/if}
        <button class="danger" onclick={remove}>{$t('uninstall')}</button>
      {:else if tile.kind === 'link'}
        <button class="danger" onclick={remove}>{$t('remove')}</button>
      {/if}
    </div>
  {/if}

  <button class="body" onclick={open}>
    <div class="icon">
      {#if tile.kind === 'system'}
        <svg viewBox="0 0 24 24" width="34" height="34" fill="currentColor" aria-hidden="true"><path d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z"/></svg>
      {:else if icon && !imgFailed}
        <img src={icon} alt="" onerror={() => (imgFailed = true)} />
      {:else}
        <span class="letter">{name.charAt(0).toUpperCase()}</span>
      {/if}
    </div>
    <span class="title one-line">{name}</span>
  </button>

  {#if tile.kind === 'app' && !tile.app.managed}
    <span class="badge">{$t('unmanaged')}</span>
  {/if}
  {#if tile.kind === 'app' && tile.app.status !== 'stopped'}
    <span class="dot" class:partial={tile.app.status === 'partial'} title={tile.app.status}></span>
  {/if}
</div>

<style>
  .tile {
    position: relative;
    aspect-ratio: 1 / 1;
    border-radius: var(--radius-card);
    transition: box-shadow 0.2s;
  }
  .tile:hover {
    box-shadow: 0 0 17px 0 rgba(0, 0, 0, 0.2);
  }
  .body {
    position: relative;
    z-index: 1;
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    background: none;
    border: none;
    color: inherit;
    padding: 0.5rem;
  }
  .icon {
    width: 64px;
    height: 64px;
    border-radius: var(--radius-icon);
    display: grid;
    place-items: center;
    overflow: hidden;
    background: var(--casablue);
    color: #fff;
  }
  .icon img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: var(--radius-icon);
  }
  .stopped .icon img,
  .stopped .icon {
    filter: grayscale(1);
    opacity: 0.7;
  }
  .letter {
    font-size: 2rem;
    font-weight: 600;
  }
  .title {
    color: var(--grey-100);
    max-width: 90%;
    text-align: center;
  }
  .burger {
    position: absolute;
    top: 0.5rem;
    right: 0.5rem;
    z-index: 3;
    width: 1.6rem;
    height: 1.6rem;
    border: none;
    border-radius: 6px;
    background: rgba(0, 0, 0, 0.25);
    color: #fff;
    font-size: 1rem;
    line-height: 1;
    opacity: 0;
    transition: opacity 0.15s;
  }
  .tile:hover .burger {
    opacity: 1;
  }
  .menu {
    position: absolute;
    top: 2rem;
    right: 0.5rem;
    z-index: 5;
    background: #fff;
    border-radius: 10px;
    padding: 4px;
    min-width: 8.5rem;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25);
    display: flex;
    flex-direction: column;
  }
  .menu button {
    text-align: left;
    height: 2rem;
    padding: 0 0.6rem;
    border: none;
    background: none;
    border-radius: 5px;
    font-size: 0.875rem;
    color: var(--grey-800);
  }
  .menu button:hover {
    background: hsla(208, 16%, 96%, 1);
  }
  .menu button.danger {
    color: var(--red);
  }
  .menu button.danger:hover {
    background: hsla(18, 98%, 94%, 1);
  }
  .badge {
    position: absolute;
    bottom: 0.4rem;
    left: 0.5rem;
    z-index: 2;
    font-size: 0.6rem;
    color: var(--grey-100);
    background: rgba(0, 0, 0, 0.35);
    border-radius: 4px;
    padding: 0 0.3rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
  }
  .dot {
    position: absolute;
    top: 0.6rem;
    left: 0.6rem;
    z-index: 2;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--status-running);
  }
  .dot.partial {
    background: var(--yellow);
  }
</style>
