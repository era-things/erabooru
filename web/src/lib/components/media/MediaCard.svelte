<script lang="ts">
	import type { MediaItem } from '$lib/types/media';
	import { isFormatVideo } from '$lib/utils/media_utils';
	let {item} = $props<{item: MediaItem}>();

	function getBorderStyle(format:string){
		if (isFormatVideo(format)) {
			return ' border-2 border-dashed border-neutral-900 ';
		} else {
			return '';
		}
	}
</script>

<a href={`/media/${item.id}`} class="group relative block">
	<img 
		src={item.url} 
		alt={"media " + item.id} 
		class={"h-auto w-full rounded-md object-contain shadow" + getBorderStyle(item.format)}
		loading="lazy" 
	/>
	<div
		class="absolute inset-0 rounded-md bg-black opacity-0 opacity-3 transition-opacity duration-200 group-hover:opacity-12"
	></div>
</a>
