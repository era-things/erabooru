<script lang="ts">
	export let items: any[] = [];
</script>

<div class="flex flex-col gap-3">
	{#each items as item}
		{#if typeof item === 'object' && ('src' in item || 'url' in item)}
			<!-- fall back to `url` if `src` absent -->
			{#if 'id' in item}
				<a href={`/media/${(item as any).id}`} class="block">
					<img
						src={(item as any).src ?? (item as any).url}
						alt={(item as any).alt ?? ''}
						class="h-auto w-full rounded-md object-contain shadow"
						loading="lazy"
					/>
				</a>
			{:else}
				<img
					src={(item as any).src ?? (item as any).url}
					alt={(item as any).alt ?? ''}
					class="h-auto w-full rounded-md object-contain shadow"
					loading="lazy"
				/>
			{/if}
		{:else}
			{item}
		{/if}
	{/each}
</div>
