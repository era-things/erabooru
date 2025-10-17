<script lang="ts">
	import { PaginationStateMachine } from '$lib/utils/paginationStateMachine';
	import type { PaginationControl } from '$lib/utils/paginationStateMachine';

	const CONTROL_LABELS: Record<
		Extract<PaginationControl['kind'], 'first' | 'prev' | 'next' | 'last'>,
		string
	> = {
		first: '<<',
		prev: '<',
		next: '>',
		last: '>>'
	};

	const CONTROL_ARIA: Record<
		Extract<PaginationControl['kind'], 'first' | 'prev' | 'next' | 'last'>,
		string
	> = {
		first: 'Go to first page',
		prev: 'Go to previous page',
		next: 'Go to next page',
		last: 'Go to last page'
	};

	let {
		currentPage = 1,
		totalPages = 1,
		onSelectPage = () => {},
		buildLink
	}: {
		currentPage?: number;
		totalPages?: number;
		onSelectPage?: (page: number) => void;
		buildLink?: (page: number) => string;
	} = $props();

	const machine = new PaginationStateMachine();

	const controls = $derived.by(() =>
		machine.compute({ current: Math.max(1, currentPage), total: Math.max(1, totalPages) })
	);

	let pageInputValue = $state('');

	function handleInput(event: Event & { currentTarget: EventTarget & HTMLInputElement }) {
		pageInputValue = event.currentTarget.value;
	}

	function selectPage(page: number) {
		if (page === currentPage) {
			pageInputValue = '';
			return;
		}

		pageInputValue = '';
		onSelectPage(page);
	}

	function handleTrigger(event: MouseEvent, page: number) {
		if (
			event.defaultPrevented ||
			event.button !== 0 ||
			event.metaKey ||
			event.ctrlKey ||
			event.shiftKey ||
			event.altKey
		) {
			return;
		}

		event.preventDefault();
		selectPage(page);
	}

	function submit() {
		const trimmed = pageInputValue.trim();
		if (!trimmed) {
			return;
		}
		
		const parsed = Number(trimmed);
		if (!Number.isFinite(parsed)) {
			return;
		}

		const target = Math.min(Math.max(Math.trunc(parsed), 1), Math.max(totalPages, 1));

		selectPage(target);
	}
</script>

<div class="flex flex-wrap items-center gap-2 text-m">
	{#each controls as control (control.kind === 'page' ? `page-${control.page}` : `${control.kind}-${control.page}`)}
		{#if control.kind === 'page'}
			{#if control.current}
				<span class="font-bold">{control.page}</span>
			{:else if buildLink}
				<a
					href={buildLink(control.page)}
					class="rounded border px-2 py-1"
					onclick={(event) => handleTrigger(event, control.page)}
				>
					{control.page}
				</a>
			{:else}
				<button
					type="button"
					class="rounded border px-2 py-1"
					onclick={() => selectPage(control.page)}
				>
					{control.page}
				</button>
			{/if}
		{:else if buildLink}
			<a
				href={buildLink(control.page)}
				class="rounded border px-2 py-1"
				aria-label={CONTROL_ARIA[control.kind]}
				onclick={(event) => handleTrigger(event, control.page)}
			>
				{CONTROL_LABELS[control.kind]}
			</a>
		{:else}
			<button
				type="button"
				class="rounded border px-2 py-1"
				aria-label={CONTROL_ARIA[control.kind]}
				onclick={() => selectPage(control.page)}
			>
				{CONTROL_LABELS[control.kind]}
			</button>
		{/if}
	{/each}
	<form class="flex items-center gap-2" onsubmit={(e) => { e.preventDefault(); submit(); }}>
		<label class="flex items-center gap-2 text-gray-600">
			<input
				class="w-12 h-6 rounded border border-gray-400 bg-gray-100 px-1 py-0.5 text-gray-900 placeholder-gray-500 [appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none"
				type="number"
				min="1"
				max={Math.max(totalPages, 1)}
				inputmode="numeric"
				placeholder="page"
				bind:value={pageInputValue}
				oninput={handleInput}
			/>
		</label>
		<button type="submit" class="rounded border h-6 border-gray-400 bg-gray-100 px-2 py-0.5 text-gray-900">Go</button>
	</form>
</div>
