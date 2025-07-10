<script lang="ts">
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import TabNav from '$lib/components/TabNav.svelte';
	import { fetchMediaDetail, deleteMedia, updateMediaTags } from '$lib/api';
	import type { MediaDetail } from '$lib/types/media';

	let media: MediaDetail | null = null;
	let tagsInput = '';
	let edit = false;

	onMount(async () => {
		const id = page.params.id;
		try {
			media = await fetchMediaDetail(id);
			tagsInput = media?.tags.map((t) => t.name.replace(/ /g, '_')).join(' ') ?? '';
		} catch (err) {
			console.error('failed to load media', err);
		}
	});

	async function remove() {
		if (!media) return;
		if (!confirm('Delete this image?')) return;
		try {
			await deleteMedia(media.id);
			goto('/');
		} catch (err) {
			console.error('delete failed', err);
			alert('Delete failed');
		}
	}

	async function saveTags() {
		if (!media) return;
		const tags = tagsInput.split(/\s+/).filter((t) => t.length > 0);
		try {
			await updateMediaTags(media.id, tags);
			media = await fetchMediaDetail(media.id);
			tagsInput = media.tags.map((t) => t.name.replace(/ /g, '_')).join(' ');
			edit = false;
		} catch (err) {
			console.error('failed to save tags', err);
			alert('Failed to save');
		}
	}

	function formatDate(date: string): string {
		return new Date(date).toLocaleDateString(undefined, {
			day: '2-digit',
			month: 'short',
			year: 'numeric'
		});
	}
</script>

<TabNav active="media" />

{#if media}
	<div class="flex flex-row gap-6 p-4">
		<div class="flex w-60 flex-col gap-4">
			<div class="text-sm">
				<p>Format: {media.format}</p>
				<p>Dimensions: {media.width}×{media.height}</p>
				<p>Size: {(media.size / 1024 / 1024).toFixed(2)} MB</p>
				{#each media.dates as d (d.name)}
					<p>
						{d.name} date: {formatDate(d.value)}
					</p>
				{/each}
			</div>
			{#if media.tags.length}
				<div class="text-sm">
					<p class="font-semibold">Tags:</p>
					<ul class="ml-4 list-disc">
						{#each media.tags as t (t.name)}
							<li>
								<a
									href={`/?q=${encodeURIComponent(t.name)}`}
									class="text-blue-500 visited:text-blue-500 hover:underline"
								>
									{t.name} ({t.count})
								</a>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
			<button class="rounded bg-red-500 px-4 py-2 text-white" on:click={remove}>Delete</button>
		</div>

		<div class="flex flex-1 items-center justify-center">
			{#if ['mp4', 'webm', 'avi', 'mkv'].includes(media.format)}
				<!-- svelte-ignore a11y_media_has_caption -->
				<video
					controls
					loop
					playsinline
					src={media.url}
					class="object-contain"
					style="max-width:75vw; max-height:75vh"
				></video>
			{:else}
				<!-- svelte-ignore a11y_missing_attribute -->
				<img src={media.url} class="object-contain" style="max-width:75vw; max-height:75vh" />
			{/if}
		</div>
	</div>
	<div class="mt-4 flex justify-center">
		<button class="rounded bg-blue-500 px-4 py-2 text-white" on:click={() => (edit = !edit)}
			>Edit</button
		>
	</div>
	{#if edit}
		<div class="mt-4 flex flex-col items-center gap-2">
			<label class="ml-4 self-start font-semibold">Tags</label>
			<input class="w-1/2 rounded border px-2 py-1" bind:value={tagsInput} />
			<button class="rounded bg-green-500 px-4 py-2 text-white" on:click={saveTags}
				>Save changes</button
			>
		</div>
	{/if}
{/if}
