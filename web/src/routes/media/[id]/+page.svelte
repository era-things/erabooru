<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { get } from 'svelte/store';
	import { goto } from '$app/navigation';

	interface MediaDetail {
		id: number;
		url: string;
		width: number;
		height: number;
		format: string;
		size: number;
	}

	let media: MediaDetail | null = null;
	const apiBase = 'http://localhost/api';

	onMount(async () => {
		const id = get(page).params.id;
		try {
			const res = await fetch(`${apiBase}/media/${id}`);
			if (res.ok) {
				media = await res.json();
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
</script>

{#if media}
	<div class="flex flex-col items-center gap-4 p-4">
		<img src={media.url} alt="image" class="max-h-screen w-auto object-contain" />
		<div class="text-sm">
			<p>Format: {media.format}</p>
			<p>Dimensions: {media.width}×{media.height}</p>
			<p>Size: {media.size} bytes</p>
		</div>
		<button class="rounded bg-red-500 px-4 py-2 text-white" on:click={remove}> Delete </button>
	</div>
{/if}
