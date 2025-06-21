<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import Masonry from './media_grid/Masonry.svelte'; // the Masonry component from the previous example
	import type { MediaItem } from '$lib/types/media'; 

	export let query: string = '';

	/* Objects straight from the API */
	let media: MediaItem[] = [];

	const apiBase = 'http://localhost/api';

	let screenWidth = 0;

	function updateWidth() {
		if (typeof window !== 'undefined') {
			screenWidth = window.innerWidth;
		}
	}
	
	async function load() {
		try {
			const url = query
				? `${apiBase}/media/previews?q=${encodeURIComponent(query)}`
				: `${apiBase}/media/previews`;
			const res = await fetch(url);
			if (res.ok) {
				const data = await res.json();
				media = data.media as MediaItem[];
			} else {
				console.error('media fetch error', res.status, res.statusText);
			}
		} catch (err) {
			console.error('network error', err);
		}
	}

	let mounted = false;
	let lastQuery = '';

	onMount(async () => {
		mounted = true;
		updateWidth();
		if (typeof window !== 'undefined') {
			window.addEventListener('resize', updateWidth);
		}
		lastQuery = query;
		await load();
	});

	$: if (mounted && query !== lastQuery) {
		lastQuery = query;
		load();
	}

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('resize', updateWidth);
		}
	});

	$: columnCount =
		screenWidth > 1600
			? 6
			: screenWidth > 1200
				? 5
				: screenWidth > 900
					? 4
					: screenWidth > 600
						? 3
						: 2;
	$: columnWidths = Array(columnCount).fill('1fr'); // Adjust column widths based on screen size
</script>

<!-- Drop-in replacement for the old grid -->
<Masonry items={media} {columnWidths} />

