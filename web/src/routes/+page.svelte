<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/stores';

	$: query = $page.url.searchParams.get('q') ?? '';
</script>

<div class="h-screen">
	<TabNav active="media" />
	<MediaGrid {query} />
</div>
