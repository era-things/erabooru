<script lang="ts">
        export let data : {query: string};
        import MediaGrid from '$lib/components/MediaGrid.svelte';
        import TabNav from '$lib/components/TabNav.svelte';
	import type { LoadEvent } from '@sveltejs/kit';

        export const load = ({ url }: LoadEvent) => {
                return {
                        query: url.searchParams.get('q') ?? ''
                };
        };

        let query = data.query;
        // $: query = $page.url.searchParams.get('q') ?? '';
</script>

<div class="h-screen ">
        <TabNav active="media" />
        <MediaGrid {query} />
</div>
