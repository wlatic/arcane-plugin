<script lang="ts">
	import { onMount } from 'svelte';
	import { projectService } from '$lib/services/project-service';
	import { settingsService } from '$lib/services/settings-service';
	import type { VolumeReport, VolumeReportItem } from '$lib/types/project.type';
	import { ArcaneButton } from '$lib/components/arcane-button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import Checkbox from '$lib/components/ui/checkbox/checkbox.svelte';
	import * as Table from '$lib/components/ui/table';
	import { CheckIcon, AlertIcon, InfoIcon, SecurityIcon } from '$lib/icons';
	import { m } from '$lib/paraglide/messages';
	import settingsStore from '$lib/stores/config-store';

	import { ArcaneTooltip, ArcaneTooltipTrigger, ArcaneTooltipContent } from '$lib/components/arcane-tooltip';

	let report = $state<VolumeReport | null>(null);
	let isLoading = $state(true);
	let allowedPath = $state('');
	let isSavingSettings = $state(false);

	let sortedItems = $derived.by(() => {
		if (!report?.items) return [];
		const statusPriority: Record<string, number> = { warning: 0, backup_warning: 1, overridden: 2, unknown: 3, ok: 4 };
		// Create a shallow copy to sort
		return [...report.items].sort((a, b) => {
			const scoreA = statusPriority[a.status] ?? 99;
			const scoreB = statusPriority[b.status] ?? 99;
			if (scoreA !== scoreB) return scoreA - scoreB;
			return a.source.localeCompare(b.source);
		});
	});

	onMount(async () => {
		await loadData();
	});

	async function loadData() {
		isLoading = true;
		try {
			const [reportData, settings] = await Promise.all([projectService.getVolumeReport(), settingsService.getSettings()]);
			report = reportData;
			allowedPath = settings.backup_safe_paths || '';
		} catch (error) {
			console.error('Failed to load volume report:', error);
		} finally {
			isLoading = false;
		}
	}

	async function saveSettings() {
		isSavingSettings = true;
		try {
			await settingsService.updateSettings({ backup_safe_paths: allowedPath });
			// Refresh report to reflect status changes based on new path
			const reportData = await projectService.getVolumeReport();
			report = reportData;
		} catch (error) {
			console.error('Failed to save settings:', error);
		} finally {
			isSavingSettings = false;
		}
	}

	async function toggleOverride(item: VolumeReportItem, checked: boolean) {
		try {
			// Optimistic update
			item.is_overridden = checked;

			// Re-evaluating status on client side is tricky now with backup_warning
			// because we don't know if ZFS failed or not just from frontend state easily.
			// But we can approximate functionality for instant feedback.

			if (checked) {
				item.status = 'overridden';
			} else {
				// Revert to original state (approximated)
				// Ideally we should just refresh data, but let's try to be smart.
				// If we reverted override, we definitely aren't 'overridden'.
				// We don't know if it should be 'warning' or 'backup_warning' or 'ok' without backend logic.
				// Simplest is to set it to 'warning' temporarily until refresh if we suspect it's bad.
				// Or... just trigger refresh immediately and rely on backend.
				item.status = 'warning'; // Temporary placeholder
			}

			await projectService.toggleVolumeOverride(item.projectId, item.override_key, checked);

			// Refresh to ensure sync with backend logic (Backup Warning vs Warning vs OK)
			const reportData = await projectService.getVolumeReport();
			report = reportData;
		} catch (error) {
			console.error('Failed to toggle override:', error);
			// Revert on error
			item.is_overridden = !checked;
		}
	}

	function getStatusColor(status: string) {
		switch (status) {
			case 'ok':
				return 'text-green-500 bg-green-500/10 border-green-500/20';
			case 'warning':
				return 'text-red-500 bg-red-500/10 border-red-500/20';
			case 'backup_warning':
				return 'text-orange-500 bg-orange-500/10 border-orange-500/20';
			case 'overridden':
				return 'text-blue-500 bg-blue-500/10 border-blue-500/20';
			case 'unknown':
				return 'text-gray-500 bg-gray-500/10 border-gray-500/20';
			default:
				return 'text-muted-foreground';
		}
	}

	function getStatusLabel(status: string) {
		switch (status) {
			case 'ok':
				return 'OK';
			case 'warning':
				return 'Warning';
			case 'backup_warning':
				return 'Backup Warning';
			case 'overridden':
				return 'Overridden';
			case 'unknown':
				return 'Unknown';
			default:
				return status;
		}
	}
</script>

<div class="container mx-auto space-y-8 py-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">Volume Report</h1>
			<p class="text-muted-foreground">Audit volume mounts and bind mounts across all projects.</p>
		</div>
	</div>

	{#if !$settingsStore.opsModeEnabled}
		<div class="flex min-h-[400px] flex-col items-center justify-center space-y-6 text-center">
			<div class="bg-muted rounded-full p-6">
				<SecurityIcon class="text-muted-foreground size-10" />
			</div>
			<div class="max-w-md space-y-2">
				<h2 class="text-xl font-semibold">Feature Disabled</h2>
				<p class="text-muted-foreground">
					Volume reporting and ZFS snapshot auditing are part of the advanced Operations Mode features, which are currently
					disabled.
				</p>
			</div>
			<ArcaneButton href="/settings/ops" tone="outline" action="base">Enable Ops Mode</ArcaneButton>
		</div>
	{:else}
		<!-- Settings Section -->
		<div class="bg-card space-y-4 rounded-lg border p-6">
			<h2 class="text-lg font-semibold">Configuration</h2>
			<div class="grid max-w-xl gap-4">
				<div class="grid gap-2">
					<Label for="allowed-path">Backup Safe Storage Path</Label>
					<div class="flex gap-2">
						<Input id="allowed-path" bind:value={allowedPath} placeholder="/mnt/user-data,/opt/backup" />
						<ArcaneButton action="save" loading={isSavingSettings} onclick={saveSettings}>Save</ArcaneButton>
					</div>
					<p class="text-muted-foreground text-xs">
						Comma-separated list of allowed paths for bind mounts. Mounts not matching these paths will be flagged as warnings.
					</p>
				</div>
			</div>
		</div>

		<!-- Report Table -->
		<div class="rounded-md border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Status</Table.Head>
						<Table.Head>Project</Table.Head>
						<Table.Head>Service</Table.Head>
						<Table.Head>Type</Table.Head>
						<Table.Head>Source</Table.Head>
						<Table.Head>Target</Table.Head>
						<Table.Head>Last Backup</Table.Head>
						<Table.Head class="text-right">Confirm Non-Persistent</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#if isLoading}
						<Table.Row>
							<Table.Cell colspan={8} class="h-24 text-center">Loading report...</Table.Cell>
						</Table.Row>
					{:else if sortedItems.length > 0}
						{#each sortedItems as item}
							<Table.Row>
								<Table.Cell>
									<div
										class={`focus:ring-ring inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:ring-2 focus:ring-offset-2 focus:outline-none ${getStatusColor(item.status)}`}
									>
										{getStatusLabel(item.status)}
									</div>
								</Table.Cell>
								<Table.Cell class="font-medium">{item.projectName}</Table.Cell>
								<Table.Cell>{item.serviceName}</Table.Cell>
								<Table.Cell class="capitalize">{item.volumeType}</Table.Cell>
								<Table.Cell class="max-w-[300px] font-mono text-xs">
									<ArcaneTooltip>
										<ArcaneTooltipTrigger>
											<div class="truncate">{item.source}</div>
										</ArcaneTooltipTrigger>
										<ArcaneTooltipContent>
											{item.source}
										</ArcaneTooltipContent>
									</ArcaneTooltip>
								</Table.Cell>
								<Table.Cell class="max-w-[300px] font-mono text-xs">
									<ArcaneTooltip>
										<ArcaneTooltipTrigger>
											<div class="truncate">{item.target}</div>
										</ArcaneTooltipTrigger>
										<ArcaneTooltipContent>
											{item.target}
										</ArcaneTooltipContent>
									</ArcaneTooltip>
								</Table.Cell>
								<Table.Cell class="font-mono text-xs">
									{#if item.latest_snapshot}
										<span class="text-green-600 dark:text-green-400">{item.latest_snapshot}</span>
									{:else}
										<span class="text-muted-foreground">-</span>
									{/if}
								</Table.Cell>
								<Table.Cell class="text-right">
									<div class="flex justify-end pr-4">
										<Checkbox checked={item.is_overridden} onCheckedChange={(v) => toggleOverride(item, v as boolean)} />
									</div>
								</Table.Cell>
							</Table.Row>
						{/each}
					{:else}
						<Table.Row>
							<Table.Cell colspan={8} class="h-24 text-center">No volumes found.</Table.Cell>
						</Table.Row>
					{/if}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
</div>
