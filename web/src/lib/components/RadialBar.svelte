<script lang="ts">
  // Ported verbatim from CasaOS-UI src/components/widgets/RadialBar.vue — a pure
  // SVG three-quarter-arc gauge. Keep the geometry/gradient identical for parity.
  let {
    percent = 0,
    label = '',
    extendContent = '',
    extendClickable = false,
    dotDiameter = '92px',
    circleBorderWidth = '5px',
    circleBackgroundColor = 'rgba(255, 255, 255, 0.4)',
    stopColorStart = '#33FFAA',
    stopColorEnd = '#FFD580',
    onextendclick = () => {},
  }: {
    percent?: number
    label?: string
    extendContent?: string
    extendClickable?: boolean
    dotDiameter?: string
    circleBorderWidth?: string
    circleBackgroundColor?: string
    stopColorStart?: string
    stopColorEnd?: string
    onextendclick?: () => void
  } = $props()

  // Unique gradient id per instance (multiple gauges on one page).
  const gid = 'grad-' + Math.random().toString(36).slice(2, 9)
  const inPercent = $derived((100 - Math.max(0, Math.min(100, percent))) * 0.75)
</script>

<div
  class="radial"
  style:--dot-diameter={dotDiameter}
  style:--circle-border-width={circleBorderWidth}
  style:--circle-background-color={circleBackgroundColor}
>
  <div class="container">
    <svg class="circle-container" viewBox="2 -3 28 38" xmlns="http://www.w3.org/2000/svg">
      <linearGradient id={gid} x1="0.17" x2="0.83" y1="0.13" y2="0.87">
        <stop style:stop-color={stopColorEnd} offset="0%" />
        <stop style:stop-color={stopColorStart} offset="100%" />
      </linearGradient>
      <circle class="bg" cx="16" cy="16" r="16" shape-rendering="geometricPrecision" />
      <circle
        class="progress"
        style:stroke-dashoffset={inPercent}
        style:stroke={`url(#${gid})`}
        cx="16"
        cy="16"
        r="16"
        shape-rendering="geometricPrecision"
      />
    </svg>
    <div class="overlay">
      <div class="per">{Math.round(percent)}</div>
      <div class="label">{label}</div>
    </div>
  </div>
  <div class="bar-content" class:is-clickable={extendClickable} onclick={() => extendClickable && onextendclick()} role="presentation">
    {extendContent}
  </div>
</div>

<style>
  .container {
    margin: auto;
    width: var(--dot-diameter);
    height: var(--dot-diameter);
    display: flex;
    flex-direction: column;
    align-items: center;
    position: relative;
  }
  .circle-container {
    width: var(--dot-diameter);
    height: var(--dot-diameter);
    transform: rotate(-225deg);
    fill: none;
    stroke: white;
    stroke-dasharray: 75 100;
    stroke-linecap: round;
  }
  .overlay {
    position: absolute;
    width: 100%;
    height: 100%;
    left: 0;
    top: 0;
    display: flex;
    justify-content: center;
    align-items: center;
    flex-direction: column;
  }
  .per {
    font-size: 1.5rem;
    font-weight: 500;
    color: var(--grey-200);
    position: relative;
    line-height: 2rem;
  }
  .per::after {
    content: '%';
    position: absolute;
    font-size: 0.875rem;
    color: var(--grey-400);
    bottom: 0.4rem;
    line-height: 1em;
    margin-left: 0.1rem;
  }
  .label {
    position: absolute;
    font-size: 0.875rem;
    font-weight: 400;
    color: var(--grey-200);
    bottom: 0;
    line-height: 1.25rem;
  }
  .bar-content {
    text-align: center;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--grey-200);
    line-height: 1.25rem;
    margin-top: 0.25rem;
  }
  .is-clickable {
    cursor: pointer;
  }
  .bg {
    fill: none;
    stroke: var(--circle-background-color);
    stroke-width: var(--circle-border-width);
    stroke-dasharray: 75 100;
    stroke-linecap: round;
  }
  .progress {
    fill: none;
    stroke-linecap: round;
    stroke-dasharray: 75 100;
    stroke-width: var(--circle-border-width);
    transition: stroke-dashoffset 1s ease-in-out;
  }
</style>
