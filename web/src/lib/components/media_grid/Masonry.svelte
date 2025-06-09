<script lang="ts">
	import Column from './Column.svelte';
    import { distributeByHeight } from '$lib/masonryDistribution';

	export let items: {height: number}[] = [];
	export let columnWidths: string[] = ['1fr', '1fr'];

	let columns: unknown[][] = [];

	/* simple round-robin distribution */
	$: {
        columns = distributeByHeight(items, columnWidths.length);
	}
</script>

<div class="grid gap-3" style="grid-template-columns:{columnWidths.join(' ')}">
	{#each columns as col}
		<Column items={col} />
	{/each}
</div>
