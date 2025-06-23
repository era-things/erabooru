<script lang="ts">
	import Column from './Column.svelte';
	import { distributeVertically, distributeRoundRobin } from '$lib/masonryDistribution';
	import type { MediaItem } from '$lib/types/media';

	let { items = [], columnWidths = ['1fr', '1fr'] } = $props<{
		items: MediaItem[];
		columnWidths: string[];
	}>();

	let columns = $derived(
		items.length > columnWidths.length
			? distributeVertically(items, columnWidths.length)
			: distributeRoundRobin(items, columnWidths.length)
	);
</script>

<div class="grid gap-3" style="grid-template-columns:{columnWidths.join(' ')}">
	{#each columns as col, index (index)}
		<Column items={col} />
	{/each}
</div>
