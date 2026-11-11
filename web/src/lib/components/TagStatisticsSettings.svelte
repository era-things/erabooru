<script lang="ts">
	import { onMount } from 'svelte';
	import { fetchTagStatisticsStatus, rebuildTagStatistics } from '$lib/api';
	import type { TagStatisticsStatus } from '$lib/api';

	let status = $state<TagStatisticsStatus | null>(null);
	let starting = $state(false);
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
			await rebuildTagStatistics();
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
		Recommendations learn from applied and rejected tags. Statistics update automatically; rebuild
		them to recalculate everything from your collection. The tag “tagme” is excluded.
	</p>
	<button
		class="rounded border px-3 py-1 disabled:opacity-50"
		disabled={starting || active || !status}
		onclick={rebuild}
		>{starting || active ? 'Rebuilding tag statistics...' : 'Rebuild tag statistics'}</button
	>
	<div aria-live="polite" class="space-y-1 text-sm text-gray-600">
		{#if active}
			<progress aria-label="Rebuilding tag statistics" class="block w-60"></progress>
			<p>
				{status?.state === 'running'
					? 'Recalculating tag statistics. Current recommendations remain available.'
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
