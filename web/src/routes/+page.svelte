<script lang="ts">
        import MediaGrid from '$lib/components/MediaGrid.svelte';
        import TabNav from '$lib/components/TabNav.svelte';
        import { page } from '$app/state';
</script>

<div class="h-screen ">
        <TabNav active="media" />
        <MediaGrid query={page.url.searchParams.get('q') ?? ''} />
</div>
