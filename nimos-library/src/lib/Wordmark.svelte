<script>
  let { height = 72 } = $props();
</script>

<span class="wordmark" style="font-size:{height}px" aria-label="Noumon">Noumon</span>

<style>
  .wordmark{
    display:block;
    color:var(--ink);
    font-weight:620;
    line-height:.9;
    letter-spacing:-.055em;
    user-select:none;
  }
</style>
