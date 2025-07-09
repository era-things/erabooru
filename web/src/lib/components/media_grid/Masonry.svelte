<script lang="ts">
	import Column from './Column.svelte';
	import { distributeVertically, distributeRoundRobin } from '$lib/masonryDistribution';
	import type { MediaPreviewItem } from '$lib/types/media';
	import { ElementRect } from 'runed';

	let {
		items = [],
		columnWidths = ['1fr', '1fr'],
		scrollPosition = 0
	} = $props<{
		items: MediaPreviewItem[];
		columnWidths: string[];
		scrollPosition: number;
	}>();

	let columns = $derived(
		items.length > columnWidths.length
			? distributeVertically(items, columnWidths.length)
			: distributeRoundRobin(items, columnWidths.length)
	);

	let el = $state<HTMLElement>();
	const rect = new ElementRect(() => el);

	let scrolledPercentage = $derived.by(() => {
		if (!rect.current || typeof window === 'undefined') return 0;

		const containerTop = rect.top;
		//const containerBottom = rect.top + rect.height;
		const viewportHeight = window.innerHeight;

		// How much of the container is above the viewport
		const scrolledIntoContainer = Math.max(0, scrollPosition - containerTop);

		// Total scrollable distance through the container
		const maxScrollThroughContainer = Math.max(0, rect.height - viewportHeight);

		return maxScrollThroughContainer > 0
			? clamp01(scrolledIntoContainer / maxScrollThroughContainer)
			: 0;
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
