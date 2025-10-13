<script lang="ts">
	import TabNav from '$lib/components/TabNav.svelte';
	import {
		regenerateReverseIndex,
		downloadMediaTags,
		importMediaTags,
		fetchHiddenTagFilters,
		createHiddenTagFilter,
		selectHiddenTagFilter,
		deleteHiddenTagFilter
	} from '$lib/api';
	import type { HiddenTagFilter } from '$lib/api';
	import { onMount } from 'svelte';
	import TagAssistInput from '$lib/components/TagAssistInput.svelte';

	let fileInput: HTMLInputElement;
	let filters = $state<HiddenTagFilter[]>([]);
	let activeFilterId = $state<number | null>(null);
	let filtersLoading = $state(false);
	let newFilterValue = $state('');

	async function regenerate() {
		if (!confirm('Are you sure you want to regenerate the reverse index?')) {
			return;
		}
		try {
			await regenerateReverseIndex();
			alert('Regeneration complete');
		} catch (err) {
			alert(`Failed to regenerate: ${err}`);
		}
	}

	async function exportTags() {
		try {
			const blob = await downloadMediaTags();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = 'tags_export.ndjson.gz';
			a.click();
			URL.revokeObjectURL(url);
		} catch (err) {
			alert(`Failed to export: ${err}`);
		}
	}

	async function importTags(event: Event) {
		const input = event.target as HTMLInputElement;
		const file = input.files && input.files[0];
		if (!file) return;
		try {
			await importMediaTags(file);
			alert('Import complete');
		} catch (err) {
			alert(`Failed to import: ${err}`);
		}
		input.value = '';
	}

	async function loadHiddenFilters() {
		filtersLoading = true;
		try {
			const data = await fetchHiddenTagFilters();
			filters = data.filters;
			activeFilterId = data.active?.id ?? null;
		} catch (err) {
			console.error(err);
			alert(`Failed to load hidden tag filters: ${err}`);
		} finally {
			filtersLoading = false;
		}
	}

	function labelFor(filter: HiddenTagFilter): string {
		return filter.value === '' ? 'Empty' : filter.value;
	}

	async function addHiddenFilter() {
		const trimmed = newFilterValue.trim();
		if (!trimmed) {
			alert('Hidden tag filter cannot be empty.');
			return;
		}
		try {
			filtersLoading = true;
			await createHiddenTagFilter(trimmed);
			newFilterValue = '';
		} catch (err) {
			console.error(err);
			alert(`Failed to create hidden tag filter: ${err}`);
		} finally {
			await loadHiddenFilters();
		}
	}

	async function setActiveHiddenFilter(filter: HiddenTagFilter) {
		if (filtersLoading || activeFilterId === filter.id) {
			return;
		}
		try {
			filtersLoading = true;
			await selectHiddenTagFilter(filter.id);
			activeFilterId = filter.id;
		} catch (err) {
			console.error(err);
			alert(`Failed to select hidden tag filter: ${err}`);
			await loadHiddenFilters();
		} finally {
			filtersLoading = false;
		}
	}

	async function removeHiddenFilter(filter: HiddenTagFilter) {
		if (filter.is_default) {
			return;
		}
		if (!confirm(`Remove hidden tag filter "${labelFor(filter)}"?`)) {
			return;
		}
		try {
			filtersLoading = true;
			await deleteHiddenTagFilter(filter.id);
		} catch (err) {
			console.error(err);
			alert(`Failed to delete hidden tag filter: ${err}`);
		} finally {
			await loadHiddenFilters();
		}
	}

	onMount(() => {
		loadHiddenFilters();
	});
</script>

<TabNav active="settings" />
<div class="space-y-6 p-4">
	<div class="flex flex-wrap gap-2">
		<button class="rounded border px-3 py-1" onclick={regenerate}>
			Regenerate reverse index
		</button>
		<button class="rounded border px-3 py-1" onclick={exportTags}> Export tags </button>
		<button class="rounded border px-3 py-1" onclick={() => fileInput.click()}>
			Import tags
		</button>
		<input type="file" bind:this={fileInput} accept=".gz" class="hidden" onchange={importTags} />
	</div>

	<section class="space-y-4 rounded border border-gray-200 p-4">
		<div class="flex items-center justify-between">
			<h2 class="text-lg font-semibold">Hidden tag filters</h2>
			<button
				class="rounded border px-3 py-1 text-sm"
				onclick={loadHiddenFilters}
				disabled={filtersLoading}
			>
				Refresh
			</button>
		</div>
		<p class="text-sm text-gray-600">
			Create tag expressions that will be applied to every search. Use the default “Empty” filter to
			remove all restrictions.
		</p>
		<div class="flex flex-wrap gap-2">
			<TagAssistInput
				bind:value={newFilterValue}
				placeholder="e.g. human -disturbing -nudity"
				inputClass="min-w-[16rem] flex-1 rounded border px-3 py-2"
				disabled={filtersLoading}
				oncommit={() => addHiddenFilter()}
			/>
			<button class="rounded border px-3 py-2" onclick={addHiddenFilter} disabled={filtersLoading}>
				Add filter
			</button>
		</div>
		<div class="space-y-2">
			{#if filtersLoading && filters.length === 0}
				<p class="text-sm text-gray-600">Loading filters…</p>
			{:else if filters.length === 0}
				<p class="text-sm text-gray-600">No custom hidden tag filters yet.</p>
			{:else}
				{#each filters as filter (filter.id)}
					<div
						class={`flex items-center justify-between gap-2 rounded border px-3 py-2 ${
							activeFilterId === filter.id ? 'border-blue-500 bg-blue-50' : 'border-gray-200'
						}`}
					>
						<button
							class={`flex-1 text-left ${filtersLoading ? 'opacity-70' : ''}`}
							onclick={() => setActiveHiddenFilter(filter)}
							disabled={filtersLoading}
						>
							<span class="font-medium">{labelFor(filter)}</span>
							{#if filter.value !== ''}
								<span class="block text-xs text-gray-600">{filter.value}</span>
							{/if}
						</button>
						<button
							class="rounded border px-2 py-1 text-sm"
							onclick={() => removeHiddenFilter(filter)}
							disabled={filtersLoading || filter.is_default}
						>
							Remove
						</button>
					</div>
				{/each}
			{/if}
		</div>
	</section>
</div>
