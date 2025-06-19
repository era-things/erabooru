<script lang="ts">
	export let id: number;
	export let src: string;
	export let alt: string = '';
</script>

<a href={`/media/${id}`} class="group relative block">
	<img {src} {alt} class="h-auto w-full rounded-md object-contain shadow" loading="lazy" />
	<div
		class="absolute inset-0 rounded-md bg-black opacity-0 opacity-3 transition-opacity duration-200 group-hover:opacity-12"
	></div>
</a>
