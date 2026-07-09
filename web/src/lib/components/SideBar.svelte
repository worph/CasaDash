<script lang="ts">
  import Clock from './widgets/Clock.svelte'
  import SystemStatus from './widgets/SystemStatus.svelte'
  import Storage from './widgets/Storage.svelte'
  import { settings } from '../stores/settings'

  const enabled = $derived($settings.widgets)
</script>

<aside class="sidebar">
  {#if enabled.clock}<Clock />{/if}
  {#if enabled.system}<SystemStatus />{/if}
  {#if enabled.storage}<Storage />{/if}
</aside>

<style>
  .sidebar {
    min-width: var(--sidebar-min);
  }
</style>
