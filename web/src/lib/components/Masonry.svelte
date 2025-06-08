<script lang="ts">
	import Column from './Column.svelte';

	export let items: unknown[]        = [];
	export let columnWidths: string[]  = ['1fr', '1fr'];

	let columns: unknown[][] = [];

	/* simple round-robin distribution */
	$: {
		const n = columnWidths.length;
		columns = Array.from({ length: n }, () => []);
		items.forEach((item, i) => columns[i % n].push(item));
	}
</script>

<div class="grid gap-3" style="grid-template-columns:{columnWidths.join(' ')}">
	{#each columns as col}
		<Column items={col}/>
	{/each}
</div>
