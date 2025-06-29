<script lang="ts">
	import TabNav from '$lib/components/TabNav.svelte';
        import { api } from '$lib/client';

	async function regenerate() {
		if (!confirm('Are you sure you want to regenerate the reverse index?')) {
			return;
		}
                const { error } = await api.POST('/admin/regenerate');
                if (!error) {
                        alert('Regeneration complete');
                } else {
                        alert('Failed to regenerate');
                }
	}
</script>

<TabNav active="settings" />
<div class="p-4">
	<button class="rounded border px-3 py-1" on:click={regenerate}> Regenerate reverse index </button>
</div>
