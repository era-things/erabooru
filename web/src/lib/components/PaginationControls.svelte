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

	let pageInputValueRaw = $state('');
	let pageInputDirty = $state(false);
	let lastExternalPage = $state(currentPage);

	const pageInputValue = $derived(pageInputDirty ? pageInputValueRaw : String(currentPage));

	$effect(() => {
		if (currentPage !== lastExternalPage) {
			pageInputDirty = false;
			lastExternalPage = currentPage;
		}
	});

	function handleInput(event: InputEvent & { currentTarget: EventTarget & HTMLInputElement }) {
		pageInputValueRaw = event.currentTarget.value;
		pageInputDirty = true;
	}

	function selectPage(page: number) {
		if (page === currentPage) {
			pageInputDirty = false;
			return;
		}

		pageInputDirty = false;
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
		const parsed = Number(pageInputValue);
		if (!Number.isFinite(parsed)) {
			return;
		}

		const target = Math.min(Math.max(Math.trunc(parsed), 1), Math.max(totalPages, 1));

		selectPage(target);
	}
</script>

<div class="flex flex-wrap items-center gap-2 text-sm">
	{#each controls as control (control.kind === 'page' ? `page-${control.page}` : `${control.kind}-${control.page}`)}
		{#if control.kind === 'page'}
			{#if control.current}
				<span class="font-semibold">{control.page}</span>
			{:else if buildLink}
				<a
					href={buildLink(control.page)}
					class="rounded border px-2 py-1"
					on:click={(event) => handleTrigger(event, control.page)}
				>
					{control.page}
				</a>
			{:else}
				<button
					type="button"
					class="rounded border px-2 py-1"
					on:click={() => selectPage(control.page)}
				>
					{control.page}
				</button>
			{/if}
		{:else if buildLink}
			<a
				href={buildLink(control.page)}
				class="rounded border px-2 py-1"
				aria-label={CONTROL_ARIA[control.kind]}
				on:click={(event) => handleTrigger(event, control.page)}
			>
				{CONTROL_LABELS[control.kind]}
			</a>
		{:else}
			<button
				type="button"
				class="rounded border px-2 py-1"
				aria-label={CONTROL_ARIA[control.kind]}
				on:click={() => selectPage(control.page)}
			>
				{CONTROL_LABELS[control.kind]}
			</button>
		{/if}
	{/each}
	<form class="flex items-center gap-2" on:submit|preventDefault={submit}>
		<label class="flex items-center gap-2 text-gray-600">
			<span>Go to</span>
			<input
				class="w-16 rounded border px-2 py-1 text-black"
				type="number"
				min="1"
				max={Math.max(totalPages, 1)}
				inputmode="numeric"
				value={pageInputValue}
				on:input={handleInput}
			/>
		</label>
		<button type="submit" class="rounded border px-2 py-1">Go</button>
	</form>
</div>
