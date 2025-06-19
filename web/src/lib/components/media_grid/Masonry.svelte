<script lang="ts">
	import Column from './Column.svelte';
	import { distributeVertically, distributeRoundRobin } from '$lib/masonryDistribution';

	export let items: { height: number; width: number }[] = [];
	export let columnWidths: string[] = ['1fr', '1fr'];

	let columns: unknown[][] = [];

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
