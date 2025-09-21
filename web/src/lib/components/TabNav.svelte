<script lang="ts">
	/**
	 * Page currently shown so we can highlight the active tab
	 */
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { PAGE_SIZE } from '$lib/constants';

	let tagQuery: string = $state('');
	let textQuery: string = $state('');
	let active: 'media' | 'upload' | 'tags' | 'settings' = $props();

	onMount(() => {
		const unsubscribe = page.subscribe(($page) => {
			const params = $page.url.searchParams;
			const query = params.get('q') ?? '';
			const vector = params.get('vector') === '1';
			if (vector) {
				if (textQuery !== query) {
					textQuery = query;
				}
				if (tagQuery !== '') {
					tagQuery = '';
				}
			} else {
				if (tagQuery !== query) {
					tagQuery = query;
				}
				if (textQuery !== '') {
					textQuery = '';
				}
			}
		});
		return unsubscribe;
	});

	function searchTags(event: Event) {
		event.preventDefault();
		const params = new URLSearchParams({ page: '1', page_size: PAGE_SIZE.toString() });
		const trimmed = tagQuery.trim();
		if (trimmed) {
			params.set('q', trimmed);
		}
		goto(`/?${params.toString()}`);
	}

	function searchText(event: Event) {
		event.preventDefault();
		const params = new URLSearchParams({ page: '1', page_size: PAGE_SIZE.toString(), vector: '1' });
		const trimmed = textQuery.trim();
		if (trimmed) {
			params.set('q', trimmed);
		} else {
			params.delete('q');
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
					placeholder="Search tags"
					bind:value={tagQuery}
					class="rounded border px-2 py-1"
				/>
			</form>
			<form onsubmit={searchText}>
				<input
					type="text"
					name="vector-search"
					placeholder="Search by text"
					bind:value={textQuery}
					class="rounded border px-2 py-1"
				/>
			</form>
		</div>
	</nav>
</div>
