<script lang="ts">
	/**
	 * Page currently shown so we can highlight the active tab
	 */
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { get } from 'svelte/store';
	import { PAGE_SIZE } from '$lib/constants';

	let tagQuery: string = $state('');
	let textQuery: string = $state('');
	let vectorMode: boolean = $state(false);
	const tagActive = $derived(!vectorMode && tagQuery.trim().length > 0);
	const textActive = $derived(vectorMode && textQuery.trim().length > 0);
	let active: 'media' | 'upload' | 'tags' | 'settings' = $props();

	$effect(() => {
		const params = get(page).url.searchParams;
		const current = params.get('q') ?? '';
		const isVector = params.get('vector') === '1';
		vectorMode = isVector;
		if (isVector) {
			textQuery = current;
			tagQuery = '';
		} else {
			tagQuery = current;
			textQuery = '';
		}
	});

	function searchTags(event: Event) {
		event.preventDefault();
		const trimmed = tagQuery.trim();
		const params = new URLSearchParams({
			page: '1',
			page_size: PAGE_SIZE
		});
		if (trimmed) {
			params.set('q', trimmed);
		}
		goto(`/?${params.toString()}`);
	}

	function searchText(event: Event) {
		event.preventDefault();
		const trimmed = textQuery.trim();
		const params = new URLSearchParams({
			page: '1',
			page_size: PAGE_SIZE,
			vector: '1'
		});
		if (trimmed) {
			params.set('q', trimmed);
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
			<form onsubmit={searchTags}>
				<input
					type="text"
					name="tag-search"
					placeholder="Tag search"
					bind:value={tagQuery}
					class="rounded border px-2 py-1"
					class:border-blue-500={tagActive}
				/>
			</form>
			<form onsubmit={searchText}>
				<input
					type="text"
					name="text-search"
					placeholder="Text search"
					bind:value={textQuery}
					class="rounded border px-2 py-1"
					class:border-blue-500={textActive}
				/>
			</form>
		</div>
	</nav>
</div>
