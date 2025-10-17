<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import TabNav from '$lib/components/TabNav.svelte';
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import PaginationControls from '$lib/components/PaginationControls.svelte';
	import { PAGE_SIZE } from '$lib/constants';
	import { fetchMediaDetail, deleteMedia, updateMediaTags } from '$lib/api';
	import type { MediaDetail } from '$lib/types/media';
	import { isFormatVideo } from '$lib/utils/media_utils';
	import TagAssistInput from '$lib/components/TagAssistInput.svelte';

	let media = $state<MediaDetail | null>(null);
	let tagsInput = $state('');
	let edit = $state(false);
	let vectorSearchQuery = $state<string | null>(null);
	let similarPage = $state(1);
	const similarPageSize = Number(PAGE_SIZE);
	let similarTotal = $state(0);
	let lastVectorQuery = $state<string | null>(null);
	const similarTotalPages = $derived(
		Math.max(1, Math.ceil(Math.max(similarTotal, 0) / similarPageSize))
	);
	const hasVectorSearch = $derived(
		typeof vectorSearchQuery === 'string' && vectorSearchQuery.length > 0
	);

	function applyMediaDetail(detail: MediaDetail) {
		media = detail;
		tagsInput = detail.tags.map((t) => t.name.replace(/ /g, '_')).join(' ');
		const vector = detail.vectors?.find((entry) => entry.name === 'vision') ?? detail.vectors?.[0];
		vectorSearchQuery = vector && vector.value.length ? `media:${vector.name}:${detail.id}` : null;
	}

	$effect(() => {
		const id = page.params.id;
		if (!id) return;

		async function loadMedia() {
			try {
				const detail = await fetchMediaDetail(id);
				applyMediaDetail(detail);
				edit = false;
			} catch (err) {
				console.error('failed to load media', err);
			}
		}

		loadMedia();
	});

	$effect(() => {
		if (vectorSearchQuery !== lastVectorQuery) {
			lastVectorQuery = vectorSearchQuery;
			similarPage = 1;
			similarTotal = 0;
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
			const detail = await fetchMediaDetail(media.id);
			applyMediaDetail(detail);
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

	$effect(() => {
		if (similarPage > similarTotalPages) {
			similarPage = similarTotalPages;
		}
	});

	function goToSimilar(pageNumber: number) {
		if (pageNumber === similarPage) return;
		similarPage = pageNumber;
	}
</script>

<TabNav active="media" />

{#if media}
	<div class="flex flex-col gap-6 p-4">
		<div class="flex flex-row gap-6">
			<div class="flex w-60 flex-col gap-4">
				<div class="text-sm">
					<p class="font-semibold">Tags</p>
					{#if media.tags.length}
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
					{:else}
						<p class="text-gray-500">No tags yet.</p>
					{/if}
				</div>
				<div class="flex flex-col gap-3">
					<button class="rounded bg-blue-500 px-4 py-2 text-white" on:click={() => (edit = !edit)}
						>{edit ? 'Cancel' : 'Edit'} tags</button
					>
					{#if edit}
						<div class="flex flex-col gap-2">
							<label for="tags-input" class="font-semibold">Tags</label>
							<TagAssistInput
								id="tags-input"
								bind:value={tagsInput}
								inputClass="w-full rounded border px-2 py-1"
							/>
							<button
								class="self-start rounded bg-green-500 px-4 py-2 text-white"
								on:click={saveTags}>Save changes</button
							>
						</div>
					{/if}
				</div>
			</div>

			<div class="flex flex-1 flex-col items-center">
				{#if isFormatVideo(media.format)}
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
				<button class="rounded bg-red-500 px-4 py-2 text-white" on:click={remove}>Delete</button>
			</div>
		</div>

		<div class="flex flex-col gap-4 border-t pt-4">
			<h2 class="text-lg font-semibold">Similar media</h2>
			{#if hasVectorSearch}
				{#key vectorSearchQuery}
					<MediaGrid
						query=""
						vectorQuery={vectorSearchQuery ?? undefined}
						page={similarPage}
						pageSize={similarPageSize}
						bind:total={similarTotal}
					/>
				{/key}
				<div class="flex flex-wrap items-center justify-center gap-4">
					<PaginationControls
						currentPage={similarPage}
						totalPages={similarTotalPages}
						onSelectPage={goToSimilar}
					/>
				</div>
			{:else}
				<p class="text-sm text-gray-500">No similar media yet.</p>
			{/if}
		</div>
	</div>
{/if}
