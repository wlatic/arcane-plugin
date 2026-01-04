<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import { gitIdentityService } from '$lib/services/git-identity-service';
	import { settingsService } from '$lib/services/settings-service';
	import type { GitIdentity } from '$lib/types/git-identity.type';
	import { ArcaneButton } from '$lib/components/arcane-button';
	import { PlusIcon, GitBranchIcon, TrashIcon, UserIcon } from '$lib/icons';
	import { toast } from 'svelte-sonner';

	let identities: GitIdentity[] = [];
	let loading = true;
	let error: string | null = null;
	let showCreateModal = false;

	let newIdentityName = '';
	let newIdentityUser = '';
	let newIdentityToken = '';
	let createLoading = false;
	let editingId: string | null = null;

	let pollingInterval = 5;
	let loadingSettings = true;

	onMount(async () => {
		loadIdentities();
		loadSettings();
	});

	async function loadSettings() {
		try {
			const settings = await settingsService.getSettings();
			// Access generic setting via any index to avoid TS error if types missing
			const val = (settings as any).gitPollingInterval;
			pollingInterval = val ? parseInt(val) : 5;
		} catch (e) {
			console.error('Failed to load git settings', e);
		} finally {
			loadingSettings = false;
		}
	}

	async function savePollingInterval() {
		try {
			await settingsService.updateSettings({ gitPollingInterval: pollingInterval.toString() } as any);
			toast.success('Polling interval updated');
		} catch (e) {
			toast.error('Failed to update polling interval');
		}
	}

	async function loadIdentities() {
		loading = true;
		try {
			identities = await gitIdentityService.list();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	function openCreateModal() {
		resetForm();
		editingId = null;
		showCreateModal = true;
	}

	function openEditModal(identity: GitIdentity) {
		newIdentityName = identity.name;
		newIdentityUser = identity.username;
		newIdentityToken = ''; // Don't show existing token
		editingId = identity.id;
		showCreateModal = true;
	}

	async function handleSave() {
		if (!newIdentityName || !newIdentityUser) return;
		// For create, token is required. For edit, it's optional.
		if (!editingId && !newIdentityToken) return;

		createLoading = true;
		try {
			if (editingId) {
				await gitIdentityService.update(editingId, {
					name: newIdentityName,
					username: newIdentityUser,
					token: newIdentityToken || undefined
				});
				toast.success('Identity updated');
			} else {
				await gitIdentityService.create({
					name: newIdentityName,
					username: newIdentityUser,
					token: newIdentityToken
				});
				toast.success('Identity created');
			}
			showCreateModal = false;
			resetForm();
			loadIdentities();
		} catch (e: any) {
			error = e.message;
			toast.error(error || 'Failed to save identity');
		} finally {
			createLoading = false;
		}
	}

	async function handleDelete(id: string) {
		if (!confirm('Are you sure you want to delete this identity?')) return;
		try {
			await gitIdentityService.delete(id);
			loadIdentities();
			toast.success('Identity deleted');
		} catch (e: any) {
			error = e.message;
			toast.error(error || 'Failed to delete identity');
		}
	}

	let testing = false;
	async function handleTest() {
		if (!newIdentityUser || (!newIdentityToken && !editingId)) {
			toast.error('Please enter username and token');
			return;
		}

		// If editing and no new token, we can't really test unless we ask backend to test with stored token?
		// But test endpoint expects token. For now require token to test.
		if (!newIdentityToken) {
			toast.error('Please enter a token to test connection');
			return;
		}

		testing = true;
		try {
			await gitIdentityService.test({
				username: newIdentityUser,
				token: newIdentityToken
			});
			toast.success('Connection successful!');
		} catch (e: any) {
			toast.error('Connection failed: ' + e.message);
		} finally {
			testing = false;
		}
	}

	function resetForm() {
		newIdentityName = '';
		newIdentityUser = '';
		newIdentityToken = '';
		editingId = null;
	}
</script>

<div class="space-y-6">
	<div class="flex items-center justify-between">
		<div>
			<h1 class="from-primary-400 to-secondary-400 bg-gradient-to-r bg-clip-text text-2xl font-bold text-transparent">
				Git Identities
			</h1>
			<p class="text-surface-400 mt-1">Manage Git credentials for your projects</p>
		</div>
		<ArcaneButton action="base" onclick={() => (showCreateModal = true)}>
			<PlusIcon class="mr-2 h-4 w-4" />
			Add Identity
		</ArcaneButton>
	</div>

	{#if error}
		<div class="rounded-lg border border-red-500/20 bg-red-500/10 p-4 text-red-400" transition:fade>
			{error}
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center py-12">
			<div class="border-primary-500 h-8 w-8 animate-spin rounded-full border-b-2"></div>
		</div>
	{:else if identities.length === 0}
		<div class="bg-surface-800/50 border-surface-700/50 rounded-lg border py-12 text-center">
			<GitBranchIcon class="text-surface-500 mx-auto mb-3 h-12 w-12" />
			<h3 class="text-surface-200 text-lg font-medium">No identities found</h3>
			<p class="text-surface-400 mt-1">Add a Git identity to use it in your projects.</p>
		</div>
	{:else}
		<div class="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3">
			{#each identities as identity (identity.id)}
				<div
					class="bg-surface-800/50 border-surface-700/50 hover:border-primary-500/50 group rounded-lg border p-4 transition-colors"
				>
					<div class="mb-3 flex items-start justify-between">
						<div class="flex items-center space-x-3">
							<div
								class="from-primary-500/20 to-secondary-500/20 flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br"
							>
								<GitBranchIcon class="text-primary-400 h-5 w-5" />
							</div>
							<div>
								<h3 class="text-surface-100 font-medium">{identity.name}</h3>
								<p class="text-surface-400 text-xs">Created {new Date(identity.created_at || '').toLocaleDateString()}</p>
							</div>
						</div>
						<div class="flex space-x-2">
							<button
								onclick={() => openEditModal(identity)}
								class="text-surface-500 hover:text-primary-400 p-1 opacity-0 transition-all group-hover:opacity-100"
								title="Edit Identity"
							>
								<!-- Pencil Icon -->
								<svg
									xmlns="http://www.w3.org/2000/svg"
									width="16"
									height="16"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									stroke-linecap="round"
									stroke-linejoin="round"
									class="lucide lucide-pencil"
									><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" /><path d="m15 5 4 4" /></svg
								>
							</button>
							<button
								onclick={() => handleDelete(identity.id)}
								class="text-surface-500 p-1 opacity-0 transition-all group-hover:opacity-100 hover:text-red-400"
								title="Delete Identity"
							>
								<TrashIcon class="h-4 w-4" />
							</button>
						</div>
					</div>

					<div class="bg-surface-900/50 text-surface-300 flex items-center space-x-2 rounded p-2 text-sm">
						<UserIcon class="text-surface-500 h-3 w-3" />
						<span>{identity.username}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if showCreateModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm" transition:fade>
		<div class="border-surface-700 w-full max-w-md rounded-lg border bg-neutral-900 p-6 shadow-2xl">
			<h2 class="text-surface-100 mb-4 text-xl font-bold">{editingId ? 'Edit Git Identity' : 'Add Git Identity'}</h2>

			<div class="space-y-4">
				<div>
					<label class="text-surface-300 mb-1 block text-sm font-medium">Name</label>
					<input
						type="text"
						bind:value={newIdentityName}
						placeholder="e.g. Work GitHub"
						class="bg-surface-900 border-surface-700 text-surface-200 focus:border-primary-500 w-full rounded-md border px-3 py-2 transition-colors focus:outline-none"
					/>
				</div>

				<div>
					<label class="text-surface-300 mb-1 block text-sm font-medium">Username</label>
					<input
						type="text"
						bind:value={newIdentityUser}
						placeholder="git-user"
						class="bg-surface-900 border-surface-700 text-surface-200 focus:border-primary-500 w-full rounded-md border px-3 py-2 transition-colors focus:outline-none"
					/>
				</div>

				<div>
					<label class="text-surface-300 mb-1 block text-sm font-medium">Personal Access Token</label>
					<input
						type="password"
						bind:value={newIdentityToken}
						placeholder="ghp_..."
						class="bg-surface-900 border-surface-700 text-surface-200 focus:border-primary-500 w-full rounded-md border px-3 py-2 transition-colors focus:outline-none"
					/>
					<p class="text-surface-500 mt-1 text-xs">Token is encrypted before storage.</p>
				</div>
			</div>

			<div class="mt-6 flex justify-between">
				<ArcaneButton
					action="base"
					tone="outline"
					onclick={handleTest}
					disabled={testing || !newIdentityUser || !newIdentityToken}
				>
					{testing ? 'Testing...' : 'Test Connection'}
				</ArcaneButton>
				<div class="flex space-x-3">
					<button
						onclick={() => (showCreateModal = false)}
						class="text-surface-300 hover:text-surface-100 px-4 py-2 transition-colors"
					>
						Cancel
					</button>
					<button
						onclick={handleSave}
						disabled={createLoading || !newIdentityName || !newIdentityUser || (!editingId && !newIdentityToken)}
						class="bg-primary-600 hover:bg-primary-500 rounded-md px-4 py-2 text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50"
					>
						{createLoading ? 'Saving...' : editingId ? 'Update Identity' : 'Save Identity'}
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
