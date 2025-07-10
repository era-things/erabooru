<script lang="ts">
	import { onMount } from 'svelte';
	import TabNav from '$lib/components/TabNav.svelte';
	import { fetchTags } from '$lib/api';
	import type { TagCount } from '$lib/types/media';

	let tags: TagCount[] = $state([]);

	onMount(async () => {
		try {
			tags = await fetchTags();
		} catch (err) {
			console.error('failed to load tags', err);
		}
	});
</script>

<TabNav active="tags" />

<table class="mx-auto mt-4">
	<thead>
		<tr>
			<th class="px-2 py-1 text-left">Tag</th>
			<th class="px-2 py-1 text-right">Count</th>
		</tr>
	</thead>
	<tbody>
		{#each tags as t (t.name)}
			<tr>
				<td class="px-2 py-1">
					<a
						href={`/?q=${encodeURIComponent(t.name)}`}
						class="text-blue-500 visited:text-blue-500 hover:underline"
					>
						{t.name}
					</a>
				</td>
				<td class="px-2 py-1 text-right">{t.count}</td>
			</tr>
		{/each}
	</tbody>
</table>
