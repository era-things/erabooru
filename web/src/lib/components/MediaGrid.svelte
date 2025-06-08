<script lang="ts">
	import { onMount } from 'svelte';

	interface MediaItem {
		id: number;
		url: string;
		width: number;
		height: number;
	}

	let items: MediaItem[] = [];
	const apiBase = 'http://localhost/api';

	onMount(async () => {
		try {
			const res = await fetch(`${apiBase}/media`);
			if (res.ok) {
				const data = await res.json();
				items = data.media as MediaItem[];
			} else {
				console.error('failed to fetch media', res.status);
			}
		} catch (err) {
			console.error('media fetch error', err);
		}
	});
</script>

<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6">
	{#each items as item (item.id)}
		<img src={item.url} alt="media" class="aspect-square object-cover" />
	{/each}
</div>
