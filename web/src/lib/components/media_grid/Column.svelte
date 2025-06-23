<script lang="ts">
	import MediaCard from '../media/MediaCard.svelte';
	import type { MediaItem } from '$lib/types/media';
	let { items = [] } = $props<{ items: MediaItem[] }>();
</script>

<div class="flex flex-col gap-3">
	{#each items as item, index (index)}
		{#if typeof item === 'object' && ('src' in item || 'url' in item)}
			<MediaCard {item} />
		{:else}
			{item}
		{/if}
	{/each}
</div>
