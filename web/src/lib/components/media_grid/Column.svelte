<script lang="ts">
	import MediaCard from '../media/MediaCard.svelte';
	import type { MediaItem } from '$lib/types/media';
	export let items: MediaItem[] = [];
</script>

<div class="flex flex-col gap-3">
	{#each items as item, index (index)}
		{#if typeof item === 'object' && ('src' in item || 'url' in item)}
			<MediaCard item={item}/>
		{:else}
			{item}
		{/if}
	{/each}
</div>
