<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { PAGE_SIZE } from '$lib/constants';
	import { buildSearchUrl } from '$lib/utils/searchParams';

	const rawQuery = $derived(page.url.searchParams.get('q') ?? '');
	const vectorFlag = $derived(page.url.searchParams.get('vector') === '1');
	const vectorParam = $derived(page.url.searchParams.get('vector_q'));
	const hasVectorParam = $derived(page.url.searchParams.has('vector_q'));
	const vectorQuery = $derived(hasVectorParam ? (vectorParam ?? '') : vectorFlag ? rawQuery : '');
	const q = $derived(!hasVectorParam && vectorFlag ? '' : rawQuery);
	const vectorSearch = $derived(vectorQuery.trim().length > 0 && (vectorFlag || hasVectorParam));
	let currentPage = $derived(Number(page.url.searchParams.get('page') || '1'));
	const pageSize = $derived(Number(page.url.searchParams.get('page_size') || PAGE_SIZE));
	let total = $state(1);
	let totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
	let pageInputRaw = $state('');
	let pageInputDirty = $state(false);
	const pageInput = $derived(pageInputDirty ? pageInputRaw : String(currentPage));

	const vectorParamForUrl = $derived(vectorSearch ? vectorQuery : undefined);

	function buildUrl(targetPage: number): string {
		return buildSearchUrl({
			page: targetPage,
			pageSize,
			query: q,
			vectorQuery: vectorParamForUrl
		});
	}

	function prev() {
		if (currentPage > 1) {
			pageInputDirty = false;
			goto(buildUrl(currentPage - 1));
		}
	}

	function next() {
		pageInputDirty = false;
		goto(buildUrl(currentPage + 1));
	}

	function submitPageInput(event: SubmitEvent) {
		event.preventDefault();
		const parsed = Number(pageInput);
		if (!Number.isFinite(parsed)) return;
		const clamped = Math.min(Math.max(Math.trunc(parsed), 1), totalPages);
		if (clamped === currentPage) {
			pageInputDirty = false;
			return;
		}
		pageInputDirty = false;
		goto(buildUrl(clamped));
	}

	function handlePageInput(event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		pageInputRaw = target.value;
		pageInputDirty = true;
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
		<form class="flex items-center gap-2 text-sm" onsubmit={submitPageInput}>
			<label class="text-gray-600" for="page-input">Go to</label>
			<input
				id="page-input"
				class="w-16 rounded border px-2 py-1"
				type="number"
				min="1"
				max={totalPages}
				value={pageInput}
				inputmode="numeric"
				on:input={handlePageInput}
			/>
			<button type="submit" class="rounded border px-2 py-1">Go</button>
		</form>
		{#if currentPage < totalPages}
			<button class="rounded border px-3 py-1" onclick={next}>Next</button>
		{/if}
	</div>
</div>
