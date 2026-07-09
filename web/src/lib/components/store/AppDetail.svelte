<script lang="ts">
  import { fetchStoreApp, type StoreApp } from '../../stores/store'
  import { t } from '../../i18n'
  import { renderMarkdown } from '../../markdown'
  import InstallButton from './InstallButton.svelte'

  let {
    id,
    installed = false,
    onback,
  }: { id: string; installed?: boolean; onback: () => void } = $props()

  let app = $state<StoreApp | null>(null)
  let loading = $state(true)
  let error = $state('')

  $effect(() => {
    loading = true
    fetchStoreApp(id)
      .then((a) => (app = a))
      .catch((e) => (error = String(e)))
      .finally(() => (loading = false))
  })
</script>

<div class="detail">
  <button class="back" onclick={onback}>‹ {$t('back')}</button>

  {#if loading}
    <p class="muted">{$t('loading')}</p>
  {:else if app}
    <header class="app-header">
      <img class="plate" src={app.icon} alt="" />
      <div class="meta">
        <h1>{app.name}</h1>
        <p class="tagline">{app.tagline}</p>
        <InstallButton {id} {installed} size="normal" />
      </div>
    </header>

    <nav class="info">
      <div class="item">
        <span class="label">{$t('category')}</span>
        <span class="value">{app.category || '—'}</span>
      </div>
      <div class="item">
        <span class="label">{$t('developer')}</span>
        <span class="value">{app.developer || '—'}</span>
      </div>
      {#if app.min_memory}
        <div class="item">
          <span class="label">Require memory</span>
          <span class="value">{app.min_memory} MB</span>
        </div>
      {/if}
    </nav>

    {#if app.screenshots?.length}
      <div class="shots">
        {#each app.screenshots as s}
          <img src={s} alt="screenshot" loading="lazy" />
        {/each}
      </div>
    {/if}

    {#if app.description}
      <!-- eslint-disable-next-line svelte/no-at-html-tags -->
      <div class="description markdown">{@html renderMarkdown(app.description)}</div>
    {/if}
  {:else}
    <p class="error">{error || 'Not found'}</p>
  {/if}
</div>

<style>
  .detail {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
    color: var(--grey-800);
  }
  .back {
    align-self: flex-start;
    background: none;
    border: none;
    color: var(--casablue);
    font-size: 0.95rem;
    cursor: pointer;
    padding: 0;
  }
  .app-header {
    display: flex;
    gap: 1.5rem;
    align-items: center;
    padding-bottom: 1rem;
    border-bottom: 1px solid #cfcfcf;
  }
  .plate {
    width: 128px;
    height: 128px;
    flex: none;
    object-fit: cover;
    border-radius: 18.75%;
    background: linear-gradient(180deg, #f7fafc 0%, #f0f2f5 100%);
    box-shadow: 1px 2px 4px rgba(0, 0, 0, 0.2);
  }
  .meta {
    flex: 1;
    min-width: 0;
  }
  h1 {
    margin: 0 0 0.4rem;
    font-size: 1.5rem;
    font-weight: 600;
    color: #29343d;
  }
  .tagline {
    margin: 0 0 0.9rem;
    color: hsl(0, 0%, 45%);
    font-size: 0.9rem;
  }
  .info {
    display: flex;
    gap: 3rem;
    padding: 0.5rem 0 1.25rem;
    border-bottom: 1px solid #cfcfcf;
  }
  .item {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    text-align: center;
  }
  .label {
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: hsl(0, 0%, 48%);
  }
  .value {
    font-size: 0.95rem;
    color: #29343d;
  }
  .shots {
    display: flex;
    gap: 1.5rem;
    overflow-x: auto;
    padding-bottom: 0.5rem;
    scroll-snap-type: x mandatory;
  }
  .shots img {
    flex: 0 0 calc(33.333% - 1rem);
    min-width: 280px;
    aspect-ratio: 16 / 9;
    object-fit: cover;
    border-radius: 8px;
    scroll-snap-align: start;
  }
  .description {
    color: #3a444d;
    font-size: 0.875rem;
    line-height: 1.5rem;
  }
  .markdown :global(h3),
  .markdown :global(h4),
  .markdown :global(h5) {
    color: #29343d;
    font-weight: 600;
    margin: 1rem 0 0.4rem;
  }
  .markdown :global(p) {
    margin: 0 0 0.75rem;
  }
  .markdown :global(ul) {
    margin: 0 0 0.75rem;
    padding-left: 1.25rem;
  }
  .markdown :global(li) {
    margin: 0.2rem 0;
  }
  .markdown :global(code) {
    background: hsla(208, 16%, 94%, 1);
    padding: 0 0.25rem;
    border-radius: 4px;
    font-size: 0.82rem;
  }
  .markdown :global(a) {
    color: var(--casablue);
  }
  .markdown :global(strong) {
    color: #29343d;
  }
  .muted {
    color: var(--grey-600);
  }
  .error {
    color: var(--red);
  }
</style>
