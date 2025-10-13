<script lang="ts">
	import { fetchTagSuggestions } from '$lib/api';
	import type { TagCount } from '$lib/types/media';

	let {
		value = $bindable(''),
		placeholder = '',
		disabled = false,
		inputClass = 'rounded border px-2 py-1',
		containerClass = '',
		id,
		name,
		autofocus = false,
		oncommit
	} = $props<{
		value: string;
		placeholder?: string;
		disabled?: boolean;
		inputClass?: string;
		containerClass?: string;
		id?: string;
		name?: string;
		autofocus?: boolean;
		oncommit?: (event: { value: string; nativeEvent: KeyboardEvent }) => void;
	}>();

	const fallbackId = `tag-assist-${Math.random().toString(36).slice(2, 10)}`;
	const resolvedId = $derived(id ?? fallbackId);

	let inputRef: HTMLInputElement | null = $state(null);
	let suggestions = $state<TagCount[]>([]);
	let highlighted = $state(-1);
	let focused = $state(false);
	let lastTerm = $state('');
	let tokenStart = $state(0);
	let tokenEnd = $state(0);
	let hasMinusPrefix = $state(false);
	let requestToken = $state(0);

	function clearSuggestions() {
		requestToken += 1;
		suggestions = [];
		highlighted = -1;
		lastTerm = '';
	}

	function parseToken(text: string, cursor: number) {
		const clampedCursor = Math.max(0, Math.min(cursor, text.length));
		const beforeCursor = text.slice(0, clampedCursor);
		const start = (() => {
			const idx = beforeCursor.lastIndexOf(' ');
			return idx === -1 ? 0 : idx + 1;
		})();
		const afterCursorIndex = text.indexOf(' ', clampedCursor);
		const end = afterCursorIndex === -1 ? text.length : afterCursorIndex;
		const hasMinus = text.slice(start, start + 1) === '-';
		const typedBeforeCursor = hasMinus
			? text.slice(start + 1, clampedCursor)
			: text.slice(start, clampedCursor);
		return { start, end, hasMinus, typedBeforeCursor };
	}

	async function loadSuggestions(term: string) {
		const currentRequest = requestToken + 1;
		requestToken = currentRequest;
		try {
			const results = await fetchTagSuggestions(term);
			if (currentRequest !== requestToken) {
				return;
			}
			suggestions = results;
			highlighted = results.length > 0 ? 0 : -1;
		} catch (err) {
			if (currentRequest === requestToken) {
				clearSuggestions();
			}
			console.error('tag suggestion fetch failed', err);
		}
	}

	function updateContext(cursor?: number) {
		if (!focused || disabled) {
			return;
		}
		const activeCursor =
			cursor ?? (inputRef ? (inputRef.selectionStart ?? inputRef.value.length) : value.length);
		const { start, end, hasMinus, typedBeforeCursor } = parseToken(value, activeCursor);
		tokenStart = start;
		tokenEnd = end;
		hasMinusPrefix = hasMinus;
		if (typedBeforeCursor.trim() === '') {
			clearSuggestions();
			return;
		}
		if (typedBeforeCursor === lastTerm) {
			return;
		}
		lastTerm = typedBeforeCursor;
		loadSuggestions(typedBeforeCursor);
	}

	function applySuggestion(suggestion: TagCount) {
		if (!inputRef) return;
		const prefix = value.slice(0, tokenStart);
		const suffix = value.slice(tokenEnd);
		const replacement = `${hasMinusPrefix ? '-' : ''}${suggestion.name}`;
		value = `${prefix}${replacement}${suffix}`;
		const newCursor = prefix.length + replacement.length;
		requestAnimationFrame(() => {
			inputRef?.setSelectionRange(newCursor, newCursor);
		});
		tokenStart = prefix.length;
		tokenEnd = prefix.length + replacement.length;
		clearSuggestions();
	}

	function handleInput(event: Event) {
		const target = event.currentTarget as HTMLInputElement;
		updateContext(target.selectionStart ?? undefined);
	}

	function handleFocus(event: FocusEvent) {
		focused = true;
		const target = event.currentTarget as HTMLInputElement;
		updateContext(target.selectionStart ?? undefined);
	}

	function handleBlur() {
		focused = false;
		clearSuggestions();
	}

	function moveHighlight(delta: number) {
		if (!suggestions.length) return;
		const count = suggestions.length;
		highlighted = (((highlighted + delta) % count) + count) % count;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Tab' && suggestions.length) {
			event.preventDefault();
			moveHighlight(event.shiftKey ? -1 : 1);
			return;
		}
		if (event.key === 'ArrowDown' && suggestions.length) {
			event.preventDefault();
			moveHighlight(1);
			return;
		}
		if (event.key === 'ArrowUp' && suggestions.length) {
			event.preventDefault();
			moveHighlight(-1);
			return;
		}
		if (event.key === 'Enter') {
			// First priority: apply suggestion if one is highlighted
			if (suggestions.length && highlighted >= 0) {
				event.preventDefault();
				applySuggestion(suggestions[highlighted]);
				return;
			}
			// Second priority: trigger commit callback if provided
			if (oncommit) {
				oncommit({ value, nativeEvent: event });
			}
			// Don't prevent default if no commit handler - let form submit naturally
			return;
		}
		if (event.key === 'Escape' && suggestions.length) {
			event.preventDefault();
			clearSuggestions();
		}
	}

	$effect(() => {
		if (disabled) {
			clearSuggestions();
		}
	});
</script>

<div class={`relative ${containerClass}`}>
	<input
		bind:this={inputRef}
		bind:value
		{placeholder}
		{disabled}
		id={resolvedId}
		{name}
		class={inputClass}
		autocomplete="off"
		autocapitalize="none"
		spellcheck={false}
		aria-autocomplete="list"
		aria-expanded={suggestions.length > 0}
		aria-activedescendant={highlighted >= 0 ? `${resolvedId}-option-${highlighted}` : undefined}
		oninput={handleInput}
		onfocus={handleFocus}
		onblur={handleBlur}
		onkeydown={handleKeydown}
		{...autofocus ? { autofocus: true } : {}}
	/>
	{#if suggestions.length}
		<ul
			class="absolute right-0 left-0 z-20 mt-1 max-h-56 overflow-auto rounded border border-gray-200 bg-white text-sm shadow-lg"
			role="listbox"
		>
			{#each suggestions as suggestion, index (suggestion.name)}
				<li
					id={`${resolvedId}-option-${index}`}
					role="option"
					aria-selected={index === highlighted}
					class={`flex cursor-pointer items-center justify-between gap-3 px-3 py-1 ${
						index === highlighted ? 'bg-blue-50 text-blue-900' : ''
					}`}
					onmousedown={(e) => {
						e.preventDefault();
						applySuggestion(suggestion);
					}}
					onmousemove={() => (highlighted = index)}
				>
					<span class="truncate">{suggestion.name}</span>
					<span class="text-xs text-gray-500">{suggestion.count.toLocaleString()}</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>
