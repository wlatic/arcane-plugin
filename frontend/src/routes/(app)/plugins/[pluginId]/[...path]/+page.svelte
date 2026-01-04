<script lang="ts">
	import { page } from '$app/state';
	import VolumesPage from '../../../reports/volumes/+page.svelte';

	const pluginId = $derived(page.params.pluginId);
	const path = $derived(page.params.path);
</script>

{#if pluginId === 'resilience_ops' && path === 'reports/volumes'}
	<!-- Render existing Volumes component for the MVP Sidecar Plugin -->
	<VolumesPage />
{:else}
	<div class="p-8">
		<h1 class="mb-4 text-2xl font-bold">Plugin: {pluginId}</h1>
		<p class="mb-4">Path: {path}</p>

		<div class="bg-card text-card-foreground rounded-lg border p-4">
			<p>This is a placeholder for external plugin UI loading.</p>
			<!-- In a full implementation, this could use an iframe pointing to: -->
			<!-- <iframe src={`/api/environments/${$page.params.envId}/plugins/${pluginId}/ui/${path}`} class="w-full h-full" ... /> -->
		</div>
	</div>
{/if}
