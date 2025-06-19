<script lang="ts">
	import MediaCard from '../media/MediaCard.svelte';
	export let items: unknown[] = [];
</script>

<div class="flex flex-col gap-3">
	{#each items as item, index (index)}
		{#if typeof item === 'object' && ('src' in item || 'url' in item)}
			<MediaCard id={item.id} src={item.src || item.url} alt={item.alt || ''} />
		{:else}
			{item}
		{/if}
	{/each}
</div>
