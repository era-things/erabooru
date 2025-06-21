<script lang="ts">
	import Column from './Column.svelte';
	import { distributeVertically, distributeRoundRobin } from '$lib/masonryDistribution';
	import type { MediaItem } from '$lib/types/media';

	export let items: MediaItem[] = [];
	export let columnWidths: string[] = ['1fr', '1fr'];

	let columns: MediaItem[][] = [];

	/* simple round-robin distribution */
	$: {
		columns =
			items.length > columnWidths.length
				? distributeVertically(items, columnWidths.length)
				: distributeRoundRobin(items, columnWidths.length);
	}
</script>

<div class="grid gap-3" style="grid-template-columns:{columnWidths.join(' ')}">
	{#each columns as col, index (index)}
		<Column items={col} />
	{/each}
</div>
