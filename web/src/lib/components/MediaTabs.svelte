<script lang="ts">
	import MediaGrid from './MediaGrid.svelte';
	import UploadArea from './UploadArea.svelte';

	let active: 'media' | 'upload' = 'media';
</script>

<div>
	<div class="mb-4 border-b">
		<nav class="flex space-x-4">
			<button
				class="-mb-px border-b-2 px-3 py-2"
				class:!border-blue-500={active === 'media'}
				class:!text-blue-500={active === 'media'}
				class:border-transparent={active !== 'media'}
				class:text-gray-500={active !== 'media'}
				on:click={() => (active = 'media')}
			>
				Media
			</button>
			<button
				class="-mb-px border-b-2 px-3 py-2"
				class:!border-blue-500={active === 'upload'}
				class:!text-blue-500={active === 'upload'}
				class:border-transparent={active !== 'upload'}
				class:text-gray-500={active !== 'upload'}
				on:click={() => (active = 'upload')}
			>
				Upload
			</button>
		</nav>
	</div>

	{#if active === 'media'}
		<MediaGrid />
	{:else}
		<UploadArea />
	{/if}
</div>
