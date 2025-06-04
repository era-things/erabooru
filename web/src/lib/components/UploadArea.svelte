<script lang="ts">
	let fileInput: HTMLInputElement | null = null;

	function trigger() {
		fileInput?.click();
	}
</script>

<div
	class="cursor-pointer rounded border-2 border-dashed border-gray-300 p-8 text-center"
	on:click={trigger}
	on:keydown={(e) => e.key === 'Enter' && trigger()}
	role="button"
	aria-label="Upload file"
	tabindex="0"
>
	<p class="text-gray-500">Click to upload a file</p>
	<input type="file" class="hidden" bind:this={fileInput} />
</div>
