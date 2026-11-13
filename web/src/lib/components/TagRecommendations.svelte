<script lang="ts">
	import { fetchTagRecommendations } from '$lib/api';
	import type { TagRecommendationsResponse, TagFeedbackAction } from '$lib/api';
	import type { MediaDetail } from '$lib/types/media';
	import { createRequest } from '$lib/utils/request.svelte';

	let {
		media,
		disabled = false,
		onfeedback
	}: {
		media: MediaDetail;
		disabled?: boolean;
		onfeedback: (tag: string, action: TagFeedbackAction) => Promise<void>;
	} = $props();
	const request = createRequest<TagRecommendationsResponse>();
	const data = $derived(request.current);
	const loading = $derived(request.loading);
	let pending = $state(false);
	let feedbackError = $state('');
	const error = $derived(
		feedbackError || (request.error ? 'Could not load tag recommendations.' : '')
	);
	let negativeOpen = $state(false);
	let lowEvidenceOpen = $state(false);
	let currentMediaID = '';

	$effect(() => {
		const id = media.id;
		if (currentMediaID !== id) {
			currentMediaID = id;
			negativeOpen = false;
			lowEvidenceOpen = false;
		}
		// Refresh after edits or embedding completion, including on the same media.
		void media;
		load();
	});

	function load() {
		feedbackError = '';
		return request.run((signal) => fetchTagRecommendations(media.id, signal));
	}

	async function feedback(tag: string, action: TagFeedbackAction) {
		if (disabled || pending) return;
		const id = media.id;
		pending = true;
		feedbackError = '';
		try {
			await onfeedback(tag, action);
		} catch {
			if (media.id === id) feedbackError = 'Could not save tag feedback. Please try again.';
		} finally {
			pending = false;
		}
	}
</script>

{#snippet recommendationList(tags: TagRecommendationsResponse['recommended'])}
	<ul class="space-y-0.5">
		{#each tags as tag (tag.name)}
			<li class="flex items-center gap-1.5">
				<div class="min-w-0 flex-1">
					<a
						class="break-all hover:underline {tag.muted ? 'text-gray-500' : 'text-blue-500'}"
						href={`/?q=${encodeURIComponent(tag.name)}`}>{tag.name} ({tag.examples})</a
					>
				</div>
				<button
					class="inline-flex size-5 shrink-0 items-center justify-center rounded border border-gray-200 text-xs leading-none hover:bg-green-50 disabled:opacity-50"
					aria-label={`Add ${tag.name}`}
					title={`Add ${tag.name}`}
					disabled={disabled || pending}
					onclick={() => feedback(tag.name, 'accept')}>+</button
				>
				<button
					class="inline-flex size-5 shrink-0 items-center justify-center rounded border border-gray-200 text-xs leading-none hover:bg-red-50 disabled:opacity-50"
					aria-label={`Reject ${tag.name}`}
					title={`This media is not ${tag.name}`}
					disabled={disabled || pending}
					onclick={() => feedback(tag.name, 'reject')}>−</button
				>
			</li>
		{/each}
	</ul>
{/snippet}

<section class="space-y-2 text-sm" aria-label="Tag recommendations">
	<p class="font-semibold">Recommended tags</p>
	{#if loading}
		<p role="status" class="text-gray-500">Finding tags...</p>
	{:else if data}
		{#if !data.embedding_ready}
			<p class="text-gray-500">Recommendations will appear when the embedding is ready.</p>
		{:else if data.recommended.length === 0}
			<p class="text-gray-500">No recommendations to show.</p>
		{:else}
			{@render recommendationList(data.recommended)}
		{/if}
		{#if data.low_evidence.length > 0}
			<details class="text-gray-600" bind:open={lowEvidenceOpen}>
				<summary class="cursor-pointer">Other suggestions ({data.low_evidence.length})</summary>
				<div class="mt-2 space-y-2">
					{@render recommendationList(data.low_evidence)}
				</div>
			</details>
		{/if}

		<details class="text-gray-600" bind:open={negativeOpen}>
			<summary class="cursor-pointer">Negative tags ({data.negative.length})</summary>
			{#if data.negative.length === 0}
				<p class="mt-2 text-gray-500">No rejected tags.</p>
			{:else}
				<ul class="mt-2 space-y-1">
					{#each data.negative as tag (tag)}
						<li class="flex items-center gap-2">
							<span class="min-w-0 flex-1 break-all">{tag}</span>
							<button
								class="rounded border border-gray-200 px-2 py-0.5 hover:bg-gray-100 disabled:opacity-50"
								aria-label={`Undo rejection of ${tag}`}
								disabled={disabled || pending}
								onclick={() => feedback(tag, 'clear')}>Undo</button
							>
						</li>
					{/each}
				</ul>
			{/if}
		</details>
	{/if}
	{#if error}
		<p role="alert" class="text-red-700">{error}</p>
		{#if !data && !loading}<button class="underline" onclick={load}>Retry</button>{/if}
	{/if}
</section>
