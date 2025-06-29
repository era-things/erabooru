<script lang="ts">
	import TabNav from '$lib/components/TabNav.svelte';
	import { regenerateReverseIndex } from '$lib/api';

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
</script>

<TabNav active="settings" />
<div class="p-4">
	<button class="rounded border px-3 py-1" on:click={regenerate}> Regenerate reverse index </button>
</div>
