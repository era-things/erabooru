<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
</script>

<div class="h-screen ">
	<TabNav active="media" />
	<MediaGrid />
</div>
