<script lang="ts">
	import MediaCard from '../media/MediaCard.svelte';
	import type { MediaPreviewItem } from '$lib/types/media';
	import { ElementSize } from 'runed';
	import { browser } from '$app/environment';

	let {
		items = [],
		scrolledPercentage = 0,
		maxHeight
	} = $props<{
		items: MediaPreviewItem[];
		scrolledPercentage: number;
		maxHeight: number;
	}>();

	let el = $state<HTMLElement>();
	const rect = new ElementSize(() => el);

	// Calculate transform for bottom alignment effect
	let transform = $derived.by(() => {
		if (!browser || !rect.current) return '';

		const maxTranslate = Math.max(0, maxHeight - rect.height);

		if (maxTranslate <= 0) {
			return '';
		}

		const viewportHeight = window.innerHeight; // Height of visible area
		const columnHeight = rect.height;

		// Don't move column if it fits in the viewport
		if (columnHeight <= viewportHeight) {
			return '';
		}

		const translateY = scrolledPercentage * maxTranslate;

		return `translateY(${translateY}px)`;
	});
</script>

<div class="flex flex-col gap-3" bind:this={el} style="transform: {transform}">
	{#each items as item, index (index)}
		{#if typeof item === 'object' && ('src' in item || 'url' in item)}
			<MediaCard {item} />
		{:else}
			{item}
		{/if}
	{/each}
</div>
