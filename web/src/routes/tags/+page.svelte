<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import TabNav from '$lib/components/TabNav.svelte';
	import { fetchTags, fetchTagReview, sendTagFeedback } from '$lib/api';
	import type { TagFeedbackAction } from '$lib/api';
	import type { TagCount, MediaItem } from '$lib/types/media';
	import type { TagReviewResponse } from '$lib/api';
	import { createRequest } from '$lib/utils/request.svelte';
	import { mediaBorderClass } from '$lib/utils/media_utils';
	import { createScrollSnapshot } from '$lib/utils/scrollSnapshot';

	const disabledTags = new Set(['tagme']);

	let tags: TagCount[] = $state([]);
	let active = $state('');
	let representatives: MediaItem[] = $state([]);
	let media: MediaItem[] = $state([]);
	let selected: string[] = $state([]);
	let nextCursor = $state('');
	const request = createRequest<TagReviewResponse>();
	const loading = $derived(request.loading);
	let saving = $state(false);
	let feedbackError = $state('');
	const error = $derived(
		feedbackError || (request.error ? 'Could not load media. Try again.' : '')
	);
	let tagsError = $state('');
	let notice = $state('');

	export const snapshot = createScrollSnapshot(
		() =>
			$state.snapshot({
				tags,
				active,
				representatives,
				media,
				selected,
				nextCursor,
				notice,
				feedbackError
			}),
		(saved) => {
			const tag = page.url.searchParams.get('tag') ?? '';
			if (saved.active !== (disabledTags.has(tag) ? '' : tag)) return false;
			request.cancel();
			({ tags, active, representatives, media, selected, nextCursor, notice, feedbackError } =
				saved);
			return true;
		}
	);

	onMount(async () => {
		try {
			tags = await fetchTags();
		} catch {
			tagsError = 'Could not load tags. Reload the page to try again.';
		}
	});
	$effect(() => {
		const tag = page.url.searchParams.get('tag') ?? '';
		// Finish saving feedback for the old view before restoring history.
		if (saving) return;
		untrack(() => {
			const next = disabledTags.has(tag) ? '' : tag;
			if (next !== active) load(next);
		});
	});

	function selectTag(tag: string) {
		if (saving || disabledTags.has(tag)) return;
		if (tag === active) return load(tag);
		const url = new URL(page.url);
		url.searchParams.set('tag', tag);
		return goto(url, { noScroll: true, keepFocus: true });
	}

	function load(tag: string, more = false) {
		if (saving || (more && loading)) return;
		active = tag;
		feedbackError = '';
		if (!more) {
			media = [];
			representatives = [];
			selected = [];
			nextCursor = '';
			notice = '';
		}
		if (!tag) {
			request.cancel();
			return;
		}
		return request.run(
			(signal) => fetchTagReview(tag, more ? nextCursor : '', signal),
			(data) => {
				if (!more) representatives = data.representatives;
				const existing = new Set(media.map((item) => item.id));
				media = more
					? [...media, ...data.media.filter((item) => !existing.has(item.id))]
					: data.media;
				nextCursor = data.next_cursor;
			}
		);
	}

	function toggle(id: string) {
		selected = selected.includes(id) ? selected.filter((value) => value !== id) : [...selected, id];
	}

	async function apply(action: TagFeedbackAction) {
		if (saving || loading || !selected.length) return;
		saving = true;
		feedbackError = '';
		request.clearError();
		notice = '';
		const ids = [...selected];
		const succeeded = new Set<string>();
		// Bound concurrent requests while retaining failed selections for retry.
		for (let i = 0; i < ids.length; i += 5) {
			await Promise.all(
				ids.slice(i, i + 5).map(async (id) => {
					try {
						await sendTagFeedback(id, active, action);
						succeeded.add(id);
					} catch {
						/* Keep this item selected for retry. */
					}
				})
			);
		}
		media = media.filter((item) => !succeeded.has(item.id));
		selected = selected.filter((id) => !succeeded.has(id));
		if (action === 'accept')
			tags = tags.map((tag) =>
				tag.name === active ? { ...tag, count: tag.count + succeeded.size } : tag
			);
		notice = `${succeeded.size} ${succeeded.size === 1 ? 'file' : 'files'} ${action === 'accept' ? 'tagged' : 'rejected'}.`;
		if (selected.length)
			feedbackError = 'Some changes could not be saved. Those files are still selected; try again.';
		saving = false;
	}
</script>

<TabNav active="tags" />

<div class="flex flex-col gap-6 p-4 md:flex-row md:items-start">
	<aside class="w-full shrink-0 md:sticky md:top-4 md:w-64" aria-label="Tags">
		<p class="mb-3 text-sm text-gray-500">Click a row to review its tag.</p>
		{#if tagsError}<p role="alert" class="text-red-600">{tagsError}</p>{/if}
		<ul class="max-h-72 overflow-y-auto md:max-h-[calc(100vh-8rem)]">
			{#each tags as tag, index (tag.name)}
				{@const disabled = disabledTags.has(tag.name)}
				<li
					class="relative flex items-center justify-between gap-3 rounded px-3 py-2"
					class:bg-blue-100={!disabled && active === tag.name}
					class:bg-gray-50={!disabled && active !== tag.name && index % 2 === 0}
					class:bg-gray-100={disabled}
					title={disabled ? 'This tag cannot be selected for review.' : undefined}
				>
					<button
						type="button"
						class="absolute inset-0 rounded focus-visible:ring-2 focus-visible:ring-blue-500 enabled:hover:bg-blue-500/10"
						aria-label={`Review ${tag.name}`}
						aria-pressed={active === tag.name}
						disabled={saving || disabled}
						onclick={() => selectTag(tag.name)}
					></button>
					<a
						href={`/?q=${encodeURIComponent(tag.name)}`}
						class="pointer-events-auto relative text-blue-500 visited:text-blue-500 hover:underline"
						>{tag.name}</a
					>
					<span class="pointer-events-none text-sm text-gray-500">{tag.count}</span>
				</li>
			{/each}
		</ul>
	</aside>

	<main class="min-w-0 flex-1">
		{#if !active}
			<p class="py-12 text-center text-gray-500">Select a tag row to find more media to tag.</p>
		{:else}
			<h1 class="mb-4 text-xl font-semibold">{active}</h1>
			<section aria-label="Representative media">
				<h2 class="mb-3 font-medium">Representative media</h2>
				<div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
					{#each representatives as item (item.id)}
						<a
							href={`/media/${item.id}`}
							class="overflow-hidden rounded bg-gray-100 focus-visible:ring-2 focus-visible:ring-blue-500"
						>
							<img
								src={item.url}
								alt={item.original_name}
								class={`aspect-square w-full object-contain ${mediaBorderClass(item.format)}`}
							/>
						</a>
					{/each}
				</div>
				{#if !loading && !representatives.length}<p class="text-sm text-gray-500">
						No examples with ready embeddings yet.
					</p>{/if}
			</section>

			<hr class="my-6 border-gray-200" />

			<section aria-label="Suggested media" aria-busy={loading || saving}>
				<div
					class="sticky top-0 z-10 mt-1 mb-3 flex flex-wrap items-center justify-between gap-x-6 gap-y-3 bg-white py-2"
				>
					<h2 class="font-medium">Suggested media</h2>
					<div class="ml-auto flex flex-wrap items-center justify-end gap-3">
						<span class="text-sm whitespace-nowrap">{selected.length} selected</span>
						<button
							class="rounded bg-blue-600 px-3 py-1.5 text-sm text-white disabled:opacity-40"
							disabled={!selected.length || saving || loading}
							onclick={() => apply('accept')}>Add tag</button
						>
						<button
							class="rounded border border-gray-300 px-3 py-1.5 text-sm disabled:opacity-40"
							disabled={!selected.length || saving || loading}
							onclick={() => apply('reject')}>Reject</button
						>
						<button
							class="text-sm text-blue-500 disabled:opacity-40"
							disabled={!media.length || saving || loading}
							onclick={() => (selected = media.map((item) => item.id))}>Select all</button
						>
						<button
							class="text-sm text-gray-500 disabled:opacity-40"
							disabled={!selected.length || saving}
							onclick={() => (selected = [])}>Clear</button
						>
						{#if saving}<span class="text-sm text-gray-500">Saving…</span>{/if}
					</div>
				</div>
				{#if notice}<p role="status" class="mb-3 text-sm">{notice}</p>{/if}
				{#if error}
					<p role="alert" class="mb-3 text-red-600">{error}</p>
					{#if !selected.length}<button
							class="mb-3 text-blue-500"
							disabled={loading || saving}
							onclick={() => load(active)}>Try again</button
						>{/if}
				{/if}
				<div class="grid grid-cols-2 gap-3 lg:grid-cols-4 xl:grid-cols-5">
					{#each media as item (item.id)}
						<div
							class="relative overflow-hidden rounded border-2"
							class:border-blue-500={selected.includes(item.id)}
							class:border-transparent={!selected.includes(item.id)}
						>
							<button
								class="block w-full bg-gray-100 focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-inset"
								aria-label={`Select ${item.original_name}`}
								aria-pressed={selected.includes(item.id)}
								disabled={saving}
								onclick={() => toggle(item.id)}
							>
								<img
									src={item.url}
									alt={item.original_name}
									loading="lazy"
									class={`aspect-square w-full object-contain ${mediaBorderClass(item.format)}`}
								/>
								<span
									class="absolute top-2 left-2 flex h-6 w-6 items-center justify-center rounded border border-gray-400 bg-white text-blue-600"
									aria-hidden="true">{selected.includes(item.id) ? '✓' : ''}</span
								>
							</button>
							<a
								href={`/media/${item.id}`}
								class="block truncate px-2 py-1 text-xs text-blue-500 hover:underline"
								title={item.original_name}>View file</a
							>
						</div>
					{/each}
				</div>
				{#if loading}<p role="status" class="mt-4 text-gray-500">Loading media…</p>
				{:else if !media.length && !error}<p class="mt-4 text-gray-500">
						No suggestions available. Add tagged examples and wait for media processing to finish.
					</p>{/if}
				{#if nextCursor}<button
						class="mt-4 rounded border border-gray-300 px-4 py-2 disabled:opacity-40"
						disabled={loading || saving}
						onclick={() => load(active, true)}>Load more</button
					>{/if}
			</section>
		{/if}
	</main>
</div>
