<script lang="ts">
	export let items: any[] = [];
</script>

<div class="flex flex-col gap-3">
	{#each items as item}
		{#if typeof item === 'object' && ('src' in item || 'url' in item)}
			<!-- fall back to `url` if `src` absent -->
			<img
				src={(item as any).src ?? (item as any).url}
				alt={(item as any).alt ?? ''}
				class="w-full h-auto object-contain rounded-md shadow"
				loading="lazy"
			/>
		{:else}
			{item}
		{/if}
	{/each}
</div>
