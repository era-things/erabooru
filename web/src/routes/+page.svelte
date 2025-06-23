<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PAGE_SIZE } from '$lib/constants';

    const q = $derived(page.url.searchParams.get('q') ?? '');
    let currentPage = $derived(Number(page.url.searchParams.get('page') || '1'));
    const pageSize = $derived(Number(page.url.searchParams.get('page_size') || PAGE_SIZE));
    let total = $state(0);
    let totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));


	function prev() {
		if (currentPage > 1) {
			goto(`/?q=${encodeURIComponent(q)}&page=${currentPage - 1}&page_size=${pageSize}`);
		}
	}

	function next() {
		goto(`/?q=${encodeURIComponent(q)}&page=${currentPage + 1}&page_size=${pageSize}`);
	}
</script>

<div class="h-screen">
	<TabNav active="media" />
	<MediaGrid query={q} page={currentPage} {pageSize} bind:total />
	<div class="my-4 flex items-center justify-center gap-4">
		{#if currentPage > 1}
			<button class="rounded border px-3 py-1" onclick={prev}>Prev</button>
		{/if}
		<span>Page {currentPage} of {totalPages}</span>
		{#if currentPage < totalPages}
			<button class="rounded border px-3 py-1" onclick={next}>Next</button>
		{/if}
	</div>
</div>
