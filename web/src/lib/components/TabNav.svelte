<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { PAGE_SIZE } from '$lib/constants';
	import TagAssistInput from '$lib/components/TagAssistInput.svelte';
	import { buildSearchUrl } from '$lib/utils/searchParams';

	let tagQuery: string = $state('');
	let vectorQuery: string = $state('');
	const tagActive = $derived(tagQuery.trim().length > 0);
	const vectorActive = $derived(vectorQuery.trim().length > 0);
	const tagInputClass = $derived(`rounded border px-2 py-1 ${tagActive ? 'border-blue-500' : ''}`);
	type TabKey = 'media' | 'upload' | 'tags' | 'settings';
	const navItems: { href: string; key: TabKey; label: string }[] = [
		{ href: '/', key: 'media', label: 'Media' },
		{ href: '/upload', key: 'upload', label: 'Upload' },
		{ href: '/tags', key: 'tags', label: 'Tags' },
		{ href: '/settings', key: 'settings', label: 'Settings' }
	];
	let active: TabKey = $props();

	$effect(() => {
		const params = page.url.searchParams;
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
		const url = buildSearchUrl({
			page: 1,
			pageSize: PAGE_SIZE,
			query: trimmedTag,
			vectorQuery: trimmedVector
		});
		goto(url);
	}
</script>

<div class="mb-4 border-b">
	<nav class="flex items-center space-x-4">
		{#each navItems as item (item.key)}
			<a
				href={item.href}
				class="-mb-px border-b-2 px-3 py-2"
				class:!border-blue-500={active === item.key}
				class:!text-blue-500={active === item.key}
				class:border-transparent={active !== item.key}
				class:text-gray-500={active !== item.key}
			>
				{item.label}
			</a>
		{/each}
		<div class="ml-auto flex items-center gap-2">
			<form class="flex items-center gap-2" onsubmit={submitSearch}>
				<TagAssistInput
					bind:value={tagQuery}
					name="tag-search"
					placeholder="Tag search"
					inputClass={tagInputClass}
					oncommit={() => submitSearch(new Event('submit'))}
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
