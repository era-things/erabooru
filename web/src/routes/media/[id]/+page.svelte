<script lang="ts">
	import { onDestroy } from 'svelte';
	import { shouldPollMedia } from '$lib/utils/mediaLifecycle';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import TabNav from '$lib/components/TabNav.svelte';
	import MediaGrid from '$lib/components/MediaGrid.svelte';
	import { fetchMediaDetail, deleteMedia, updateMediaTags, ApiError } from '$lib/api';
	import type { MediaDetail } from '$lib/types/media';
	import { isFormatVideo } from '$lib/utils/media_utils';
	import TagAssistInput from '$lib/components/TagAssistInput.svelte';

	let media = $state<MediaDetail | null>(null);
	let navigationHeight = $state(0);
	let tagsInput = $state('');
	let edit = $state(false);
	let mutation = $state<'idle' | 'saving' | 'deleting'>('idle');
	let routeGeneration = 0;
	let mutationRevision = 0;
	onDestroy(() => {
		routeGeneration++;
	});
	let mediaLoadError = $state('');
	let vectorSearchQuery = $state<string | null>(null);
	const hasVectorSearch = $derived(
		typeof vectorSearchQuery === 'string' && vectorSearchQuery.length > 0
	);

	function applyMediaDetail(detail: MediaDetail) {
		media = detail;
		if (!edit) tagsInput = detail.tags.map((t) => t.name.replace(/ /g, '_')).join(' ');
		const embedding =
			detail.embeddings.find((entry) => entry.space === 'vision' && entry.status === 'ready') ??
			detail.embeddings.find((entry) => entry.status === 'ready');
		vectorSearchQuery = embedding ? `media:${embedding.space}:${detail.id}` : null;
	}

	$effect(() => {
		const id = page.params.id;
		if (!id) return;
		const generation = ++routeGeneration;
		mutationRevision++;
		mutation = 'idle';
		const controller = new AbortController();
		let cancelled = false;
		let failedAttempts = 0;
		let timer: ReturnType<typeof setTimeout> | undefined;
		media = null;
		mediaLoadError = '';
		vectorSearchQuery = null;
		edit = false;

		async function loadMedia() {
			try {
				const revision = mutationRevision;
				const detail = await fetchMediaDetail(id, controller.signal);
				if (cancelled || generation !== routeGeneration || page.params.id !== id) return;
				if (revision !== mutationRevision || mutation !== 'idle') {
					timer = setTimeout(loadMedia, 1000);
					return;
				}
				failedAttempts = 0;
				applyMediaDetail(detail);
				mediaLoadError = '';
				if (shouldPollMedia(detail)) timer = setTimeout(loadMedia, 1000);
			} catch (err) {
				if (cancelled || page.params.id !== id) return;
				if (mutation !== 'idle') {
					timer = setTimeout(loadMedia, 1000);
					return;
				}
				if (!(err instanceof ApiError && err.status === 404) && ++failedAttempts < 3) {
					timer = setTimeout(loadMedia, 1000);
				} else {
					console.error('failed to load media', err);
					mediaLoadError =
						err instanceof ApiError && err.status === 404
							? 'Media not found.'
							: 'Could not load media. Please try again.';
				}
			}
		}

		void loadMedia();
		return () => {
			cancelled = true;
			controller.abort();
			clearTimeout(timer);
		};
	});

	function isCurrent(id: string, generation: number) {
		return generation === routeGeneration && page.params.id === id;
	}

	async function remove() {
		if (!media || mutation !== 'idle') return;
		if (!confirm('Delete this media?')) return;
		const id = media.id;
		const generation = routeGeneration;
		mutation = 'deleting';
		mutationRevision++;
		try {
			await deleteMedia(id);
			if (isCurrent(id, generation)) await goto('/');
		} catch (err) {
			if (isCurrent(id, generation)) {
				console.error('delete failed', err);
				alert('Delete failed');
			}
		} finally {
			if (isCurrent(id, generation)) mutation = 'idle';
		}
	}

	async function saveTags() {
		if (!media || mutation !== 'idle') return;
		const id = media.id;
		const generation = routeGeneration;
		const tags = tagsInput.split(/\s+/).filter((t) => t.length > 0);
		mutation = 'saving';
		mutationRevision++;
		try {
			await updateMediaTags(id, tags);
			if (!isCurrent(id, generation)) return;
			const detail = await fetchMediaDetail(id);
			if (!isCurrent(id, generation)) return;
			edit = false;
			applyMediaDetail(detail);
		} catch (err) {
			if (isCurrent(id, generation)) {
				console.error('failed to save tags', err);
				alert('Failed to save');
			}
		} finally {
			if (isCurrent(id, generation)) {
				mutationRevision++;
				mutation = 'idle';
			}
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

<div class="media-navigation" bind:clientHeight={navigationHeight}>
	<TabNav active="media" />
</div>

{#if mediaLoadError}
	<p role="alert" class="p-4 text-sm text-red-600">{mediaLoadError}</p>
{:else if !media}
	<p role="status" class="p-4 text-sm text-gray-500">Loading media...</p>
{:else if media.status === 'failed'}
	<p role="alert" class="p-4 text-sm text-red-600">
		Media processing failed. Try repairing the collection in Settings.
	</p>
{:else if media.status !== 'ready'}
	<p role="status" class="p-4 text-sm text-gray-500">
		{media.status === 'retrying' ? 'Processing interrupted. Retrying...' : 'Processing media...'}
	</p>
{/if}

{#if media && media.status !== 'ready'}
	<button
		class="m-4 rounded bg-red-500 px-4 py-2 text-white"
		disabled={mutation !== 'idle'}
		onclick={remove}
	>
		{mutation === 'deleting' ? 'Deleting...' : 'Delete media'}
	</button>
{/if}

{#if media && media.status === 'ready'}
	<div
		class="flex flex-col px-4 pb-4"
		style={`--media-height: calc(100dvh - ${navigationHeight}px - 2rem); --media-aspect-ratio: ${media.width / media.height}`}
	>
		<div class="flex flex-row items-start gap-6 py-4">
			<div class="flex w-60 shrink-0 flex-col gap-4">
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
					<button
						class="rounded bg-blue-500 px-4 py-2 text-white"
						disabled={mutation !== 'idle'}
						onclick={() => (edit = !edit)}>{edit ? 'Cancel' : 'Edit'} tags</button
					>
					{#if edit}
						<div class="flex flex-col gap-2">
							<label for="tags-input" class="font-semibold">Tags</label>
							<TagAssistInput
								id="tags-input"
								bind:value={tagsInput}
								disabled={mutation !== 'idle'}
								inputClass="w-full rounded border px-2 py-1"
							/>
							<button
								class="self-start rounded bg-green-500 px-4 py-2 text-white"
								disabled={mutation !== 'idle'}
								onclick={saveTags}>{mutation === 'saving' ? 'Saving...' : 'Save changes'}</button
							>
						</div>
					{/if}
				</div>
				<div class="text-sm">
					{#if media.original_name}
						<p class="break-all">Original name: {media.original_name}</p>
					{/if}
					<p>Format: {media.format}</p>
					<p>Size: {(media.size / 1024 / 1024).toFixed(2)} MB</p>
					<p>Dimensions: {media.width}×{media.height}</p>
					{#each media.dates as d (d.name)}
						<p>
							{d.name} date: {formatDate(d.value)}
						</p>
					{/each}
				</div>
				<button
					class="self-start rounded bg-[#b85c5c] p-2 text-white hover:bg-[#a34d4d] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[#b85c5c] disabled:opacity-50"
					aria-label={mutation === 'deleting' ? 'Deleting media...' : 'Delete media'}
					title={mutation === 'deleting' ? 'Deleting media...' : 'Delete media'}
					aria-busy={mutation === 'deleting'}
					disabled={mutation !== 'idle'}
					onclick={remove}
				>
					<svg
						xmlns="http://www.w3.org/2000/svg"
						width="20"
						height="20"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="1.75"
						stroke-linecap="round"
						stroke-linejoin="round"
						aria-hidden="true"
					>
						<path
							d="M3 6h18M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M5 6l1 14a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1l1-14M10 10v7M14 10v7"
						/>
					</svg>
				</button>
			</div>

			<div class="flex min-w-0 flex-1 flex-col items-center">
				{#if isFormatVideo(media.format)}
					<!-- svelte-ignore a11y_media_has_caption -->
					<video controls loop playsinline src={media.url} class="media-file"></video>
				{:else}
					<!-- svelte-ignore a11y_missing_attribute -->
					<img src={media.url} class="media-file" />
				{/if}
			</div>
		</div>

		<div class="flex flex-col gap-4 border-t pt-4">
			<h2 class="text-lg font-semibold">Similar media</h2>
			{#if hasVectorSearch}
				{#key vectorSearchQuery}
					<MediaGrid query="" vectorQuery={vectorSearchQuery ?? undefined} />
				{/key}
			{:else if shouldPollMedia(media)}
				<p role="status" class="text-sm text-gray-500">Preparing similar media...</p>
			{:else if media.embeddings.some((entry) => entry.status === 'failed')}
				<p role="alert" class="text-sm text-red-600">Embedding generation failed.</p>
			{:else}
				<p class="text-sm text-gray-500">
					{media.embedding_availability === 'unavailable'
						? 'Similarity search is unavailable on this server.'
						: 'No embedding is available. Try repairing the collection in Settings.'}
				</p>
			{/if}
		</div>
	</div>
{/if}

<style>
	.media-file {
		width: min(100%, calc(var(--media-height) * var(--media-aspect-ratio)));
		height: auto;
		aspect-ratio: var(--media-aspect-ratio);
		object-fit: contain;
	}

	.media-navigation :global(> div) {
		margin-bottom: 0;
	}
</style>
