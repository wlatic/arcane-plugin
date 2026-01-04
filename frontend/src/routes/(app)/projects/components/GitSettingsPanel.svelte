<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import * as Tabs from '$lib/components/ui/tabs';
	import * as Table from '$lib/components/ui/table';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { ArcaneButton } from '$lib/components/arcane-button';
	import { projectService } from '$lib/services/project-service';
	import { gitIdentityService } from '$lib/services/git-identity-service';
	import type { GitIdentity } from '$lib/types/git-identity.type';
	import type { GitLogEntry } from '$lib/types/project.type';
	import { toast } from 'svelte-sonner';
	import { onMount } from 'svelte';

	let {
		projectId,
		repoUrl = $bindable(),
		branch = $bindable(),
		path = $bindable(),
		authUser = $bindable(),
		authToken = $bindable(),
		syncMode = $bindable(),
		identityId = $bindable(),
		pollInterval = $bindable(),
		onPullSuccess
	} = $props<{
		projectId: string;
		repoUrl: string;
		branch: string;
		path: string;
		authUser: string;
		authToken: string;
		syncMode: 'manual' | 'pull' | 'push';
		identityId: string | null;
		pollInterval: number;
		onPullSuccess?: () => void;
	}>();

	let isLoading = $state({
		pull: false,
		push: false,
		status: false,
		identities: true,
		logs: false,
		validate: false
	});

	let identities = $state<GitIdentity[]>([]);
	let gitStatus = $state<string | null>(null);
	let logs = $state<GitLogEntry[]>([]);
	let validationStatus = $state<{ success: boolean; message: string } | null>(null);

	// "manual" or "saved"
	// let effectiveAuthMode = $derived(identityId ? 'saved' : 'manual');
	let authMode = $state<'manual' | 'saved'>('manual');

	onMount(async () => {
		if (identityId) {
			authMode = 'saved';
		}

		try {
			identities = await gitIdentityService.list();
		} catch (e) {
			console.error('Failed to list git identities', e);
		} finally {
			isLoading.identities = false;
		}

		// Auto-load history and status since it's the default tab now
		checkStatus(false);
		loadHistory();
	});

	async function checkStatus(showToast = true) {
		isLoading.status = true;
		try {
			gitStatus = await projectService.getProjectGitStatus(projectId);
			if (showToast) {
				if (gitStatus) {
					toast.success(gitStatus);
				} else {
					gitStatus = 'Synced'; // Default if empty
					toast.success('Project is synced');
				}
			} else if (!gitStatus) {
				gitStatus = 'Synced';
			}
		} catch (e: any) {
			if (showToast) toast.error('Failed to get git status: ' + e.message);
			gitStatus = 'Error checking status';
		} finally {
			isLoading.status = false;
		}
	}

	async function loadHistory() {
		isLoading.logs = true;
		try {
			logs = await projectService.getProjectGitLog(projectId);
		} catch (e: any) {
			// specific error handling if needed, usually assume UI shows empty if failed
			console.error('Failed to load git logs', e);
		} finally {
			isLoading.logs = false;
		}
	}

	async function pull() {
		isLoading.pull = true;
		try {
			const msg = await projectService.syncProjectFromGit(projectId);
			toast.success(msg || 'Git pull successful');
			checkStatus(false);
			loadHistory(); // Refresh history
			onPullSuccess?.();
		} catch (e: any) {
			toast.error('Git pull failed: ' + e.message);
		} finally {
			isLoading.pull = false;
		}
	}

	async function push() {
		isLoading.push = true;
		try {
			const msg = await projectService.pushProjectToGit(projectId, 'Manual push from UI');
			toast.success(msg || 'Git push successful');
			checkStatus(false);
			loadHistory(); // Refresh history
		} catch (e: any) {
			toast.error('Git push failed: ' + e.message);
		} finally {
			isLoading.push = false;
		}
	}

	function handleAuthModeChange(e: Event) {
		const target = e.target as HTMLSelectElement;
		authMode = target.value as 'manual' | 'saved';
		if (authMode === 'manual') {
			identityId = null;
		} else if (identities.length > 0 && !identityId) {
			identityId = identities[0].id;
		}
	}

	async function testConnection() {
		isLoading.validate = true;
		validationStatus = null;
		try {
			// Basic validation before call
			if (!repoUrl) {
				toast.error('Repository URL is required');
				return;
			}

			await projectService.validateGitConnection({
				repoUrl,
				branch,
				authMode,
				authUser,
				authToken,
				identityId
			});
			toast.success('Connection verified successfully');
			validationStatus = { success: true, message: 'Connection verified' };
		} catch (e: any) {
			console.error('Git validation error', e);
			// Extract message if it's nested
			const msg = e.response?.data?.error || e.message || 'Validation failed';
			toast.error('Validation failed: ' + msg);
			validationStatus = { success: false, message: 'Validation failed' };
		} finally {
			isLoading.validate = false;
		}
	}
</script>

<div class="flex flex-col gap-4">
	<Tabs.Root value="history" class="w-full">
		<Tabs.List class="grid w-full grid-cols-2">
			<Tabs.Trigger
				value="history"
				onclick={() => {
					checkStatus(false);
					loadHistory();
				}}>History & Operations</Tabs.Trigger
			>
			<Tabs.Trigger value="settings">Settings</Tabs.Trigger>
		</Tabs.List>

		<Tabs.Content value="settings" class="mt-4">
			<Card.Root>
				<Card.Header>
					<Card.Title>Git Configuration</Card.Title>
					<Card.Description>Configure Git repository for this project.</Card.Description>
				</Card.Header>
				<Card.Content class="space-y-4">
					<div class="grid gap-2">
						<Label for="git-repo">Repository URL</Label>
						<Input id="git-repo" bind:value={repoUrl} placeholder="https://github.com/username/repo.git" />
					</div>
					<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
						<div class="grid gap-2">
							<Label for="git-branch">Branch</Label>
							<Input id="git-branch" bind:value={branch} placeholder="main" />
						</div>
						<div class="grid gap-2">
							<Label for="git-path">Path (optional)</Label>
							<Input id="git-path" bind:value={path} placeholder="Path within repo (e.g. /app)" />
						</div>
					</div>

					<div class="border-border mt-2 grid gap-2 border-t pt-4">
						<Label>Authentication Method</Label>
						<select
							class="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							value={authMode}
							onchange={handleAuthModeChange}
						>
							<option value="manual">Manual Credentials (Username/Token)</option>
							<option value="saved">Saved Identity</option>
						</select>
					</div>

					{#if authMode === 'saved'}
						<div class="grid gap-2">
							<Label for="git-identity">Select Identity</Label>
							{#if isLoading.identities}
								<div class="text-muted-foreground text-sm">Loading identities...</div>
							{:else if identities.length === 0}
								<div class="text-sm text-yellow-500">
									No saved identities found. <a href="/settings/git" class="underline">Create one in Settings</a> or use Manual mode.
								</div>
							{:else}
								<select
									id="git-identity"
									class="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
									bind:value={identityId}
								>
									{#each identities as identity}
										<option value={identity.id}>{identity.name} ({identity.username})</option>
									{/each}
								</select>
							{/if}
						</div>
					{:else}
						<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
							<div class="grid gap-2">
								<Label for="git-user">Username (optional)</Label>
								<Input id="git-user" bind:value={authUser} placeholder="Git username" />
							</div>
							<div class="grid gap-2">
								<Label for="git-token">Token (optional)</Label>
								<Input id="git-token" type="password" bind:value={authToken} placeholder="PAT or password" />
							</div>
						</div>
					{/if}

					<div class="flex items-center gap-4 pt-2">
						<ArcaneButton action="base" tone="outline-primary" size="sm" onclick={testConnection} loading={isLoading.validate}>
							Test Connection
						</ArcaneButton>
						{#if validationStatus}
							<span class="text-sm {validationStatus.success ? 'text-green-500' : 'text-red-500'}">
								{validationStatus.message}
							</span>
						{/if}
					</div>

					<div class="border-border mt-2 grid gap-2 border-t pt-4">
						<Label>Sync Mode</Label>
						<select
							class="border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex h-10 w-full rounded-md border px-3 py-2 text-sm file:border-0 file:bg-transparent file:text-sm file:font-medium focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
							bind:value={syncMode}
						>
							<option value="manual">Manual (No Auto-Sync)</option>
							<option value="pull">Auto Pull (Periodically pull changes)</option>
							<option value="push">Auto Push (Periodically push changes)</option>
						</select>
					</div>

					{#if syncMode !== 'manual'}
						<div class="grid gap-2">
							<Label for="git-poll-interval">Polling Interval (minutes)</Label>
							<Input id="git-poll-interval" type="number" bind:value={pollInterval} min="1" placeholder="5" />
						</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</Tabs.Content>

		<Tabs.Content value="history" class="mt-4">
			<Card.Root>
				<Card.Header>
					<Card.Title>Git Operations</Card.Title>
				</Card.Header>
				<Card.Content class="space-y-4">
					<div class="flex flex-wrap gap-2">
						<ArcaneButton action="base" onclick={() => checkStatus()} loading={isLoading.status}>Check Status</ArcaneButton>
						<ArcaneButton action="base" onclick={pull} loading={isLoading.pull}>Pull from Git</ArcaneButton>
						<ArcaneButton action="base" onclick={push} loading={isLoading.push}>Push to Git</ArcaneButton>
					</div>
					{#if gitStatus}
						<div class="bg-muted mt-4 rounded-md p-4 font-mono text-sm whitespace-pre-wrap">
							{gitStatus}
						</div>
					{/if}
				</Card.Content>
			</Card.Root>

			<Card.Root class="mt-4">
				<Card.Header>
					<div class="flex items-center justify-between">
						<Card.Title>Commit History</Card.Title>
						<ArcaneButton action="base" size="sm" tone="ghost" onclick={loadHistory} loading={isLoading.logs}
							>Refresh</ArcaneButton
						>
					</div>
				</Card.Header>
				<Card.Content>
					{#if isLoading.logs && logs.length === 0}
						<div class="text-muted-foreground py-4 text-center text-sm">Loading history...</div>
					{:else if logs.length > 0}
						<div class="rounded-md border">
							<Table.Root>
								<Table.Header>
									<Table.Row>
										<Table.Head>Hash</Table.Head>
										<Table.Head>Message</Table.Head>
										<Table.Head>Author</Table.Head>
										<Table.Head>Date</Table.Head>
									</Table.Row>
								</Table.Header>
								<Table.Body>
									{#each logs as log}
										<Table.Row>
											<Table.Cell class="font-mono text-xs">{log.hash}</Table.Cell>
											<Table.Cell>{log.message}</Table.Cell>
											<Table.Cell class="text-muted-foreground text-xs">{log.author}</Table.Cell>
											<Table.Cell class="text-muted-foreground text-xs">{log.date}</Table.Cell>
										</Table.Row>
									{/each}
								</Table.Body>
							</Table.Root>
						</div>
					{:else}
						<div class="text-muted-foreground py-4 text-center text-sm">No history available. (Check if repo is cloned)</div>
					{/if}
				</Card.Content>
			</Card.Root>
		</Tabs.Content>
	</Tabs.Root>
</div>
