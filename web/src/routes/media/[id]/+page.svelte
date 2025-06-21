<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import { goto } from '$app/navigation';
	import TabNav from '$lib/components/TabNav.svelte';

	interface MediaDetail {
		id: number;
		url: string;
		width: number;
		height: number;
		format: string;
		size: number;
		tags: string[];
	}

	let media: MediaDetail | null = null;
	let tagsInput = '';
	let edit = false;
	const apiBase = 'http://localhost/api';

	onMount(async () => {
		const id = get(page).params.id;
		try {
			const res = await fetch(`${apiBase}/media/${id}`);
			if (res.ok) {
				media = await res.json();
				tagsInput = media?.tags.map((t) => t.replace(/ /g, '_')).join(' ') ?? '';
			} else {
				console.error('failed to load media', res.status, res.statusText);
			}
		} catch (err) {
			console.error('network error', err);
		}
	});

	async function remove() {
		if (!media) return;
		if (!confirm('Delete this image?')) return;
		const res = await fetch(`${apiBase}/media/${media.id}`, { method: 'DELETE' });
		if (res.ok) {
			goto('/');
		} else {
			alert('Delete failed');
		}
	}

	async function saveTags() {
		if (!media) return;
		const tags = tagsInput.split(/\s+/).filter((t) => t.length > 0);
		const res = await fetch(`${apiBase}/media/${media.id}/tags`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ tags })
		});
		if (res.ok) {
			media.tags = tags;
			tagsInput = media.tags.map((t) => t.replace(/ /g, '_')).join(' ');
			edit = false;
		} else {
			alert('Failed to save');
		}
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
			</div>
			{#if media.tags.length}
				<div class="text-sm">
					<p class="font-semibold">Tags:</p>
					<ul class="ml-4 list-disc">
						{#each media.tags as t (t)}
							<li>{t}</li>
						{/each}
					</ul>
				</div>
			{/if}
			<button class="rounded bg-red-500 px-4 py-2 text-white" on:click={remove}>Delete</button>
		</div>

                <div class="flex flex-1 items-center justify-center">
                        {#if ['mp4','webm','avi','mkv'].includes(media.format)}
                                <!-- svelte-ignore a11y_media_has_caption -->
                                <video 
									controls loop playsinline
									src={media.url} 
									class="object-contain" 
									style="max-width:75vw; max-height:75vh"></video>
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
