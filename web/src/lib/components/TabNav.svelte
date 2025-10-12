<script lang="ts">
	/**
	 * Page currently shown so we can highlight the active tab
	 */
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { get } from 'svelte/store';
	import { PAGE_SIZE } from '$lib/constants';

	let tagQuery: string = $state('');
	let vectorQuery: string = $state('');
	const tagActive = $derived(tagQuery.trim().length > 0);
	const vectorActive = $derived(vectorQuery.trim().length > 0);
	let active: 'media' | 'upload' | 'tags' | 'settings' = $props();

	$effect(() => {
		const params = get(page).url.searchParams;
		const rawQuery = params.get('q') ?? '';
		const vectorFlag = params.get('vector') === '1';
		const hasVectorParam = params.has('vector_q');
		const vectorParam = params.get('vector_q') ?? '';
		if (vectorFlag && !hasVectorParam) {
			tagQuery = '';
			vectorQuery = rawQuery;
		} else {
			tagQuery = rawQuery;
			vectorQuery = vectorParam;
		}
	});

	function submitSearch(event: Event) {
		event.preventDefault();
		const trimmedTag = tagQuery.trim();
		const trimmedVector = vectorQuery.trim();
		const params = new URLSearchParams({
			page: '1',
			page_size: PAGE_SIZE.toString()
		});
		if (trimmedTag) {
			params.set('q', trimmedTag);
		}
		if (trimmedVector) {
			params.set('vector', '1');
			params.set('vector_q', trimmedVector);
		}
		goto(`/?${params.toString()}`);
	}
</script>

<div class="mb-4 border-b">
	<nav class="flex items-center space-x-4">
		<a
			href="/"
			class="-mb-px border-b-2 px-3 py-2"
			class:!border-blue-500={active === 'media'}
			class:!text-blue-500={active === 'media'}
			class:border-transparent={active !== 'media'}
			class:text-gray-500={active !== 'media'}
		>
			Media
		</a>
		<a
			href="/upload"
			class="-mb-px border-b-2 px-3 py-2"
			class:!border-blue-500={active === 'upload'}
			class:!text-blue-500={active === 'upload'}
			class:border-transparent={active !== 'upload'}
			class:text-gray-500={active !== 'upload'}
		>
			Upload
		</a>
		<a
			href="/tags"
			class="-mb-px border-b-2 px-3 py-2"
			class:!border-blue-500={active === 'tags'}
			class:!text-blue-500={active === 'tags'}
			class:border-transparent={active !== 'tags'}
			class:text-gray-500={active !== 'tags'}
		>
			Tags
		</a>
		<a
			href="/settings"
			class="-mb-px border-b-2 px-3 py-2"
			class:!border-blue-500={active === 'settings'}
			class:!text-blue-500={active === 'settings'}
			class:border-transparent={active !== 'settings'}
			class:text-gray-500={active !== 'settings'}
		>
			Settings
		</a>
		<div class="ml-auto flex items-center gap-2">
			<form class="flex items-center gap-2" onsubmit={submitSearch}>
				<input
					type="text"
					name="tag-search"
					placeholder="Tag search"
					bind:value={tagQuery}
					class="rounded border px-2 py-1"
					class:border-blue-500={tagActive}
				/>
				<input
					type="text"
					name="vector-search"
					placeholder="Vector search"
					bind:value={vectorQuery}
					class="rounded border px-2 py-1"
					class:border-blue-500={vectorActive}
				/>
				<button type="submit" class="hidden" aria-hidden="true">Search</button>
			</form>
		</div>
	</nav>
</div>
