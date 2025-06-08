<script lang="ts">
	import { onMount } from 'svelte';
	import Masonry from './Masonry.svelte';   // the Masonry component from the previous example

	interface MediaItem {
		id: number;
		url: string;
		width: number;
		height: number;
	}

	/* Raw objects straight from the API */
	let media: MediaItem[] = [];

	/* Re-mapped to the shape Masonry / Column expect */
	let photos: { src: string; alt: string }[] = [];

	/* Pick any mix of fixed / fluid column sizes */
	const columnWidths = ['1fr', '1fr', '1fr', '1fr'];

	const apiBase = 'http://localhost/api';

	onMount(async () => {
		try {
			const res = await fetch(`${apiBase}/media`);
			if (res.ok) {
				const data = await res.json();
				media  = data.media as MediaItem[];
				photos = media.map(m => ({ src: m.url, alt: `media ${m.id}` }));
			} else {
				console.error('media fetch error', res.status, res.statusText);
			}
		} catch (err) {
			console.error('network error', err);
		}
	});
</script>

<!-- Drop-in replacement for the old grid -->
<Masonry items={photos} {columnWidths} />
