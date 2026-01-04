<script lang="ts">
	import { z } from 'zod/v4';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Label } from '$lib/components/ui/label';
	import type { PageData } from './$types';
	import type { Settings } from '$lib/types/settings.type';
	import { m } from '$lib/paraglide/messages';
	import { SecurityIcon, InfoIcon } from '$lib/icons';
	import TextInputWithLabel from '$lib/components/form/text-input-with-label.svelte';
	import settingsStore from '$lib/stores/config-store';
	import { SettingsPageLayout } from '$lib/layouts';
	import { createSettingsForm } from '$lib/utils/settings-form.util';
	import * as Alert from '$lib/components/ui/alert';

	let { data }: { data: PageData } = $props();
	const currentSettings = $derived<Settings>($settingsStore || data.settings!);
	const isReadOnly = $derived.by(() => $settingsStore.uiConfigDisabled);

	const formSchema = z.object({
		opsModeEnabled: z.boolean(),
		backup_safe_paths: z.string().optional().default('')
	});

	const formDefaults = $derived({
		opsModeEnabled: currentSettings.opsModeEnabled,
		backup_safe_paths: currentSettings.backup_safe_paths ?? ''
	});

	let { formInputs } = $derived(
		createSettingsForm({
			schema: formSchema,
			currentSettings: formDefaults,
			getCurrentSettings: () => ({
				opsModeEnabled: ($settingsStore || data.settings!).opsModeEnabled,
				backup_safe_paths: ($settingsStore || data.settings!).backup_safe_paths ?? ''
			}),
			successMessage: m.general_settings_saved()
		})
	);
</script>

<SettingsPageLayout
	title="Resilience / Ops"
	description="Manage advanced operational features and resilience settings"
	icon={SecurityIcon}
	pageType="form"
	showReadOnlyTag={isReadOnly}
>
	{#snippet mainContent()}
		<fieldset disabled={isReadOnly} class="relative space-y-8">
			<!-- Ops Mode Section -->
			<div class="space-y-4">
				<h3 class="text-lg font-medium">Operations Mode</h3>
				<div class="bg-card rounded-lg border shadow-sm">
					<div class="space-y-6 p-6">
						<div class="grid gap-4 md:grid-cols-[1fr_1.5fr] md:gap-8">
							<div>
								<Label class="text-base">Enable Advanced Operations</Label>
								<p class="text-muted-foreground mt-1 text-sm">
									Enables Git integration (pull/push/audit) and ZFS snapshot reporting.
								</p>
							</div>
							<div class="space-y-4">
								<div class="flex items-center gap-2">
									<Switch id="opsModeEnabledSwitch" bind:checked={$formInputs.opsModeEnabled.value} />
									<Label for="opsModeEnabledSwitch" class="font-normal">
										{$formInputs.opsModeEnabled.value ? 'Enabled' : 'Disabled'}
									</Label>
								</div>

								<Alert.Root variant="warning">
									<InfoIcon class="size-4" />
									<Alert.Title>Advanced Feature Requirements</Alert.Title>
									<Alert.Description>
										Enables advanced resilience features. ZFS reporting requires access to Docker/ZFS tooling on the host.
									</Alert.Description>
								</Alert.Root>
							</div>
						</div>
					</div>
				</div>
			</div>

			<!-- Backup Safe Paths Section -->
			<div class="space-y-4">
				<h3 class="text-lg font-medium">Storage Safety</h3>
				<div class="bg-card rounded-lg border shadow-sm">
					<div class="space-y-6 p-6">
						<div class="grid gap-4 md:grid-cols-[1fr_1.5fr] md:gap-8">
							<div>
								<Label class="text-base">Backup Safe Paths</Label>
								<p class="text-muted-foreground mt-1 text-sm">
									Define which host paths are considered "safe" (e.g., backed up) for bind mounts in Volume Reports.
								</p>
							</div>
							<div>
								<TextInputWithLabel
									bind:value={$formInputs.backup_safe_paths.value}
									error={$formInputs.backup_safe_paths.error}
									label="Safe Paths (Comma Separated)"
									placeholder="/mnt/user/appdata, /opt/docker"
									helpText="Comma-separated list of absolute paths."
									type="text"
								/>
							</div>
						</div>
					</div>
				</div>
			</div>
		</fieldset>
	{/snippet}
</SettingsPageLayout>
