<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchTagStatisticsStatus, rebuildTagStatistics } from '$lib/api';
	import type { TagStatisticsStatus } from '$lib/api';

	let status = $state<TagStatisticsStatus | null>(null);
	let starting = $state(false);
	let legacyProfile = $state('');
	let error = $state('');
	let refresh = $state(0);
	const active = $derived(
		status !== null &&
			['available', 'pending', 'running', 'retryable', 'scheduled'].includes(status.state)
	);
	let mounted = $state(false);
	onMount(() => {
		mounted = true;
	});

	$effect(() => {
		if (!mounted) return;
		void refresh;
		const controller = new AbortController();
		let timer: ReturnType<typeof setTimeout>;
		async function poll() {
			try {
				const result = await fetchTagStatisticsStatus(controller.signal);
				if (controller.signal.aborted) return;
				status = result;
				error = '';
				if (['available', 'pending', 'running', 'retryable', 'scheduled'].includes(result.state))
					timer = setTimeout(poll, 1500);
			} catch {
				if (!controller.signal.aborted) error = 'Could not check rebuild status.';
			}
		}
		void poll();
		return () => {
			controller.abort();
			clearTimeout(timer);
		};
	});

	async function rebuild() {
		if (starting || active) return;
		starting = true;
		error = '';
		try {
			await rebuildTagStatistics(legacyProfile);
			legacyProfile = '';
			refresh++;
		} catch {
			error = 'Could not start the rebuild. Please try again.';
		} finally {
			starting = false;
		}
	}
</script>

<section class="space-y-3 rounded border border-gray-200 p-4">
	<h2 class="text-lg font-semibold">Tag recommendations</h2>
	<p class="text-sm text-gray-600">
		Tag recommendations update automatically as you edit your collection. Rebuild to train the
		classifiers again using your current tags and feedback.
	</p>
	{#if status && status.legacy_embeddings > 0 && status.active_profile}
		<label class="block text-sm">
			<input
				type="checkbox"
				checked={legacyProfile === status.active_profile}
				disabled={active || starting}
				onchange={(event) => {
					legacyProfile = event.currentTarget.checked ? (status?.active_profile ?? '') : '';
				}}
			/>
			Include {status.legacy_embeddings} older embeddings in training. I confirm they were all created
			with the current model and preprocessing.
		</label>
	{/if}
	<button
		class="rounded border px-3 py-1 disabled:opacity-50"
		disabled={starting || active || !status}
		onclick={rebuild}
		>{starting || active
			? 'Rebuilding tag recommendations...'
			: 'Rebuild tag recommendations'}</button
	>
	<div aria-live="polite" class="space-y-1 text-sm text-gray-600">
		{#if active}
			<progress aria-label="Rebuilding tag recommendations" class="block w-60"></progress>
			<p>
				{status?.state === 'running'
					? 'Training tag classifiers. Current recommendations remain available.'
					: status?.state === 'retryable'
						? 'Rebuild interrupted. Waiting to retry...'
						: 'Rebuild queued...'}
			</p>
		{:else if status?.state === 'discarded' || status?.state === 'cancelled'}
			<p class="text-red-700">Rebuild did not complete. You can try again.</p>
		{:else if status?.rebuilt_at}
			<p>
				{status.tags} tags available. Last rebuilt {new Date(status.rebuilt_at).toLocaleString()}.
			</p>
		{/if}
	</div>
	{#if error}<p role="alert" class="text-sm text-red-700">{error}</p>
		<button class="text-sm underline" onclick={() => refresh++}>Refresh status</button>{/if}
</section>
