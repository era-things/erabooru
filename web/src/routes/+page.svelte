<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/state';
	import { PAGE_SIZE } from '$lib/constants';

	const rawQuery = $derived(page.url.searchParams.get('q') ?? '');
	const vectorFlag = $derived(page.url.searchParams.get('vector') === '1');
	const vectorParam = $derived(page.url.searchParams.get('vector_q'));
	const hasVectorParam = $derived(page.url.searchParams.has('vector_q'));
	const vectorQuery = $derived(hasVectorParam ? (vectorParam ?? '') : vectorFlag ? rawQuery : '');
	const q = $derived(!hasVectorParam && vectorFlag ? '' : rawQuery);
	const pageSize = $derived(Number(page.url.searchParams.get('page_size') || PAGE_SIZE));
</script>

<div class="h-screen">
	<TabNav active="media" />
	<MediaGrid query={q} {vectorQuery} {pageSize} />
</div>
