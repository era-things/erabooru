<script lang="ts">
	import Column from './Column.svelte';
	import { distributeRoundRobin } from '$lib/masonryDistribution';
	import type { MediaPreviewItem } from '$lib/types/media';
	import { ElementSize } from 'runed';
	import { innerHeight, scrollY } from 'svelte/reactivity/window';

	let {
		items = [],
		columnWidths = ['1fr', '1fr']
	}: {
		items: MediaPreviewItem[];
		columnWidths: string[];
	} = $props();

	let columns = $derived(distributeRoundRobin(items, columnWidths.length));

	let el = $state<HTMLElement>();
	const rect = new ElementSize(() => el);

	let scrolledPercentage = $derived.by(() => {
		const y = scrollY.current ?? 0;
		const maxScrollThroughContainer = rect.height - (innerHeight.current ?? 0);
		if (!el || maxScrollThroughContainer <= 0) return 0;

		// Measure in page coordinates now, including after history restoration or resizing.
		const containerTop = el.getBoundingClientRect().top + y;
		return clamp01((y - containerTop) / maxScrollThroughContainer);
	});

	function clamp01(value: number): number {
		return Math.min(1, Math.max(0, value));
	}
</script>

<div
	bind:this={el}
	class="grid items-start gap-3"
	style="grid-template-columns:{columnWidths.join(' ')}"
>
	{#each columns as col, index (index)}
		<Column items={col} {scrolledPercentage} maxHeight={rect.height} />
	{/each}
</div>
