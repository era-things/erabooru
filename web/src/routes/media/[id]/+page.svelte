<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import TabNav from '$lib/components/TabNav.svelte';
	import { fetchMediaDetail, deleteMedia, updateMediaTags, fetchSimilarMedia } from '$lib/api';
	import type { MediaDetail, MediaItem } from '$lib/types/media';
	import { isFormatVideo } from '$lib/utils/media_utils';
	import TagAssistInput from '$lib/components/TagAssistInput.svelte';

	let media = $state<MediaDetail | null>(null);
	let tagsInput = $state('');
	let edit = $state(false);
	let similar = $state<MediaItem[]>([]);
	let similarLoading = $state(false);

	// Make the loading reactive to page.params.id changes
	$effect(() => {
		const id = page.params.id;
		if (!id) return;

		async function loadMedia() {
			try {
				media = await fetchMediaDetail(id);
				tagsInput = media?.tags.map((t) => t.name.replace(/ /g, '_')).join(' ') ?? '';
				edit = false; // Reset edit mode when navigating

				const vector =
					media?.vectors?.find((entry) => entry.name === 'vision') ?? media?.vectors?.[0];
				if (media && vector && vector.value.length) {
					similarLoading = true;
					try {
						const results = await fetchSimilarMedia(vector.value, 5, media.id, vector.name);
						similar = results.filter((item) => item.id !== media!.id);
					} catch (err) {
						console.error('failed to load similar media', err);
					} finally {
						similarLoading = false;
					}
				} else {
					similar = [];
				}
			} catch (err) {
				console.error('failed to load media', err);
			}
		}

		loadMedia();
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

	function similarBorderClass(format: string): string {
		return isFormatVideo(format)
			? 'border-2 border-dashed border-neutral-900'
			: 'border border-gray-200';
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
			<button class="rounded bg-red-500 px-4 py-2 text-white" onclick={remove}>Delete</button>
		</div>

		<div class="flex flex-1 flex-col">
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
			<div class="mt-4 flex justify-center">
				<button class="rounded bg-blue-500 px-4 py-2 text-white" onclick={() => (edit = !edit)}
					>Edit</button
				>
			</div>
			{#if edit}
				<div class="mt-4 flex flex-col items-center gap-2">
					<div class="flex w-1/2 flex-col rounded px-2 py-1">
						<label for="tags-input" class="ml-4 self-start font-semibold">Tags</label>
						<TagAssistInput
							id="tags-input"
							bind:value={tagsInput}
							inputClass="w-full rounded border px-2 py-1"
						/>
					</div>
					<button class="rounded bg-green-500 px-4 py-2 text-white" onclick={saveTags}
						>Save changes</button
					>
				</div>
			{/if}
		</div>

		<div class="flex w-52 flex-col gap-3">
			<h2 class="text-base font-semibold">Similar media</h2>
			{#if similarLoading}
				<p class="text-sm text-gray-500">Loading…</p>
			{:else if !similar.length}
				<p class="text-sm text-gray-500">No similar media yet.</p>
			{:else}
				<div class="flex flex-col gap-3">
					{#each similar as item (item.id)}
						<a href={`/media/${item.id}`} class="block">
							<div
								class={`aspect-square w-full overflow-hidden rounded bg-gray-100 ${similarBorderClass(item.format)}`}
							>
								<img
									src={item.url}
									alt={`Similar media ${item.id}`}
									class="h-full w-full object-cover"
								/>
							</div>
						</a>
					{/each}
				</div>
			{/if}
		</div>
	</div>
{/if}
