<script lang="ts">
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';

	const q = page.url.searchParams.get('q') ?? '';
	let currentPage = Number(page.url.searchParams.get('page') || '1');

	function prev() {
		if (currentPage > 1) {
			goto(`/?q=${encodeURIComponent(q)}&page=${currentPage - 1}`);
		}
	}

	function next() {
		goto(`/?q=${encodeURIComponent(q)}&page=${currentPage + 1}`);
	}
</script>

<div class="h-screen">
	<TabNav active="media" />
	<MediaGrid query={q} page={currentPage} />
	<div class="my-4 flex justify-center gap-4">
		<button class="rounded border px-3 py-1" on:click={prev} disabled={currentPage === 1}
			>Prev</button
		>
		<button class="rounded border px-3 py-1" on:click={next}>Next</button>
	</div>
</div>
