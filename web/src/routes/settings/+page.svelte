<script lang="ts">
	import TabNav from '$lib/components/TabNav.svelte';
	import { regenerateReverseIndex, downloadMediaTags } from '$lib/api';

	async function regenerate() {
		if (!confirm('Are you sure you want to regenerate the reverse index?')) {
			return;
		}
		try {
			await regenerateReverseIndex();
			alert('Regeneration complete');
		} catch (err) {
			alert(`Failed to regenerate: ${err}`);
		}
	}

	async function exportTags() {
		try {
			const blob = await downloadMediaTags();
			const url = URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = 'tags_export.ndjson.gz';
			a.click();
			URL.revokeObjectURL(url);
		} catch (err) {
			alert(`Failed to export: ${err}`);
		}
	}
</script>

<TabNav active="settings" />
<div class="flex gap-2 p-4">
	<button class="rounded border px-3 py-1" on:click={regenerate}> Regenerate reverse index </button>
	<button class="rounded border px-3 py-1" on:click={exportTags}> Export tags </button>
</div>
