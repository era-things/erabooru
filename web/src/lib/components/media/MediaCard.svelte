<script lang="ts">
	import type { MediaPreviewItem } from '$lib/types/media';
	import { mediaBorderClass } from '$lib/utils/media_utils';
	let { item } = $props<{ item: MediaPreviewItem }>();

	const needsCrop = $derived(item.cropped);
</script>

<a href={`/media/${item.id}`} class="group relative block">
	<div
		class="relative w-full overflow-hidden rounded-md"
		style={`aspect-ratio:${item.width}/${item.height}`}
	>
		<img
			src={item.url}
			alt={'media ' + item.id}
			class={'h-full w-full ' +
				(needsCrop ? 'object-cover' : '') +
				' shadow ' +
				mediaBorderClass(item.format)}
			loading="lazy"
		/>
		{#if needsCrop}
			<div
				class="pointer-events-none absolute inset-x-0 bottom-0 h-10 bg-gradient-to-b from-transparent to-black/50"
			></div>
		{/if}
	</div>
	<div
		class="absolute inset-0 rounded-md bg-black opacity-0 opacity-3 transition-opacity duration-200 group-hover:opacity-12"
	></div>
</a>
