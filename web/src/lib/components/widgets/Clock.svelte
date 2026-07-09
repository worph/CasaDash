<script lang="ts">
  import Widget from '../Widget.svelte'
  import { onMount } from 'svelte'

  let now = $state(new Date())
  let is24 = $state(localStorage.getItem('timeFormat') !== '12')

  onMount(() => {
    const t = setInterval(() => (now = new Date()), 1000)
    return () => clearInterval(t)
  })

  const time = $derived(
    is24
      ? now.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', hour12: false })
      : now.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit', hour12: true }),
  )
  const date = $derived(
    now.toLocaleDateString(undefined, {
      weekday: 'long',
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    }),
  )

  function toggle() {
    is24 = !is24
    localStorage.setItem('timeFormat', is24 ? '24' : '12')
  }
</script>

<Widget>
  <div class="clock" onclick={toggle} role="presentation">
    <div class="time">{time}</div>
    <div class="date">{date}</div>
  </div>
</Widget>

<style>
  .clock {
    cursor: pointer;
    user-select: none;
  }
  .time {
    font-size: 2rem;
    font-weight: 600;
    line-height: 1.125em;
    color: var(--grey-100);
  }
  .date {
    margin-top: 0.35rem;
    font-size: 0.875rem;
    font-weight: 400;
    color: var(--grey-400);
  }
</style>
