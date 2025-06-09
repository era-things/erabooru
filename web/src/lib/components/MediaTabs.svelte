<script lang="ts">
	import MediaGrid from './MediaGrid.svelte';
	import UploadArea from './UploadArea.svelte';
	import TabNav from './TabNav.svelte';

	let active: 'media' | 'upload' = 'media';
</script>

<div>
	<TabNav bind:active />

	{#if active === 'media'}
		<MediaGrid />
	{:else}
		<UploadArea />
	{/if}
</div>
