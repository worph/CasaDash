<script lang="ts">
  import { wallpaper } from '../stores/settings'
</script>

<div id="background" style:background-image={`url(${$wallpaper})`}></div>
<div class="bottom-fade"></div>

<style>
  #background {
    position: fixed;
    inset: 0;
    z-index: -2;
    background-size: cover;
    background-position: center;
    transition: background-image 0.3s;
  }
  .bottom-fade {
    position: fixed;
    inset: 0;
    z-index: -1;
    pointer-events: none;
    background: linear-gradient(180deg, rgba(0, 0, 0, 0) 0%, rgba(0, 0, 0, 0.55) 100%);
  }
</style>
