<script lang="ts">
    export let id: number;
	export let src: string;
    export let alt: string = '';
</script>


<a href={`/media/${id}`} class="group block relative">
    <img
        src={src}
        alt={alt}
        class="h-auto w-full rounded-md object-contain shadow"
        loading="lazy"
    />
    <div class="absolute inset-0 bg-black opacity-0 group-hover:opacity-10 transition-opacity rounded-md duration-200"></div>
</a>

