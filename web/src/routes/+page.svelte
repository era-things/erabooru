<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PAGE_SIZE } from '$lib/constants';

	const rawQuery = $derived(page.url.searchParams.get('q') ?? '');
	const vectorFlag = $derived(page.url.searchParams.get('vector') === '1');
	const vectorParam = $derived(page.url.searchParams.get('vector_q'));
	const hasVectorParam = $derived(page.url.searchParams.has('vector_q'));
	const vectorQuery = $derived(() => {
		if (hasVectorParam) {
			return vectorParam ?? '';
		}
		if (vectorFlag) {
			return rawQuery;
		}
		return '';
	});
	const q = $derived(() => {
		if (!hasVectorParam && vectorFlag) {
			return '';
		}
		return rawQuery;
	});
	const vectorSearch = $derived(() => {
		if (vectorQuery.trim().length === 0) {
			return false;
		}
		return vectorFlag || hasVectorParam;
	});
	let currentPage = $derived(Number(page.url.searchParams.get('page') || '1'));
	const pageSize = $derived(Number(page.url.searchParams.get('page_size') || PAGE_SIZE));
	let total = $state(1);
	let totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));

	function buildUrl(targetPage: number): string {
		const params = new URLSearchParams({
			page: targetPage.toString(),
			page_size: pageSize.toString()
		});
		if (q) {
			params.set('q', q);
		}
		if (vectorSearch) {
			params.set('vector', '1');
			const trimmedVector = vectorQuery.trim();
			if (trimmedVector) {
				params.set('vector_q', trimmedVector);
			}
		}
		return `/?${params.toString()}`;
	}

	function prev() {
		if (currentPage > 1) {
			goto(buildUrl(currentPage - 1));
		}
	}

	function next() {
		goto(buildUrl(currentPage + 1));
	}
</script>

<div class="h-screen">
	<TabNav active="media" />
	<MediaGrid query={q} {vectorQuery} page={currentPage} {pageSize} bind:total />
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
