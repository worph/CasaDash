<script lang="ts">
  import { clickOutside } from '../actions'
  import { settings } from '../stores/settings'
  import { t, languages } from '../i18n'

  let open = $state(false)

  const wallpapers = [
    '/wallpapers/default_wallpaper.jpg',
    '/wallpapers/wallpaper01.jpg',
    '/wallpapers/wallpaper02.jpg',
  ]

  function setWallpaper(w: string) {
    settings.update((s) => ({ ...s, wallpaper: w }))
  }
  function setLanguage(code: string) {
    settings.update((s) => ({ ...s, language: code }))
  }
  function toggleWidget(key: string) {
    settings.update((s) => ({ ...s, widgets: { ...s.widgets, [key]: !s.widgets[key] } }))
  }
</script>

<header class="topbar">
  <div class="left">
    <div class="menu-wrap">
      <button class="picon" title={$t('settings')} aria-label={$t('settings')} onclick={() => (open = !open)}>
        <!-- control / sliders (CasaOS control-outline) -->
        <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round">
          <line x1="4" y1="7" x2="20" y2="7" /><circle cx="9" cy="7" r="2.2" fill="#fff" />
          <line x1="4" y1="17" x2="20" y2="17" /><circle cx="15" cy="17" r="2.2" fill="#fff" />
        </svg>
      </button>

      {#if open}
        <div class="dropdown" use:clickOutside={() => (open = false)}>
          <h3>{$t('settings')}</h3>

          <label class="field">
            <span>{$t('language')}</span>
            <select value={$settings.language} onchange={(e) => setLanguage((e.target as HTMLSelectElement).value)}>
              {#each languages as l}<option value={l.code}>{l.name}</option>{/each}
            </select>
          </label>

          <div class="field">
            <span>{$t('wallpaper')}</span>
            <div class="thumbs">
              {#each wallpapers as w}
                <button class="thumb" class:active={$settings.wallpaper === w} style:background-image={`url(${w})`} aria-label="wallpaper" onclick={() => setWallpaper(w)}></button>
              {/each}
            </div>
          </div>

          <div class="field">
            <span>{$t('widgets')}</span>
            <div class="toggles">
              {#each ['clock', 'system', 'storage'] as key}
                <label class="toggle">
                  <input type="checkbox" checked={$settings.widgets[key] ?? true} onchange={() => toggleWidget(key)} />
                  {$t(key === 'system' ? 'system_status' : key)}
                </label>
              {/each}
            </div>
          </div>
        </div>
      {/if}
    </div>

    <button class="picon" title={$t('wallpaper')} aria-label={$t('wallpaper')} onclick={() => (open = !open)}>
      <!-- wallpaper / image (CasaOS wallpaper-outline) -->
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.8">
        <rect x="3" y="4.5" width="18" height="15" rx="2.2" /><circle cx="8.5" cy="9.5" r="1.6" fill="currentColor" stroke="none" />
        <path d="M4 17l4.5-4.5 3.5 3.5 3-3L20 16.5" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
    </button>
  </div>

  <div class="spacer"></div>

  <a class="picon" href="https://github.com/Yundera" target="_blank" rel="noreferrer" title="GitHub" aria-label="GitHub">
    <svg viewBox="0 0 16 16" width="18" height="18" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z"/></svg>
  </a>
</header>

<style>
  /* White CasaOS-style navbar with left icon cluster. */
  .topbar {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    height: 3.25rem;
    z-index: 20;
    display: flex;
    align-items: center;
    padding: 0 1rem;
    background: #fff;
    border-bottom: 1px solid hsla(208, 16%, 90%, 1);
  }
  .left {
    display: flex;
    align-items: center;
    gap: 0.25rem;
  }
  .spacer {
    flex: 1;
  }
  .picon {
    display: grid;
    place-items: center;
    width: 2.25rem;
    height: 2.25rem;
    border-radius: 6px;
    background: transparent;
    border: none;
    color: #363636;
    cursor: pointer;
    transition: background 0.15s;
  }
  .picon:hover {
    background: rgba(0, 0, 0, 0.05);
  }
  .menu-wrap {
    position: relative;
  }
  .dropdown {
    position: absolute;
    left: 0;
    top: 2.75rem;
    width: 17rem;
    background: #fff;
    border-radius: 12px;
    padding: 1rem;
    box-shadow: 0 12px 30px rgba(0, 0, 0, 0.18);
    color: var(--grey-800);
    display: flex;
    flex-direction: column;
    gap: 1rem;
    border: 1px solid hsla(208, 16%, 90%, 1);
  }
  .dropdown h3 {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    font-size: 0.8rem;
    color: var(--grey-600);
  }
  select {
    background: #fff;
    border: 1px solid #cfcfcf;
    border-radius: 6px;
    color: var(--grey-800);
    padding: 0.4rem 0.5rem;
    font-size: 0.85rem;
  }
  .thumbs {
    display: flex;
    gap: 0.5rem;
  }
  .thumb {
    width: 3.4rem;
    height: 2.1rem;
    border-radius: 6px;
    background-size: cover;
    background-position: center;
    border: 2px solid transparent;
    cursor: pointer;
    padding: 0;
  }
  .thumb.active {
    border-color: var(--casablue);
  }
  .toggles {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    color: var(--grey-800);
    font-size: 0.85rem;
  }
</style>
