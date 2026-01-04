import BaseAPIService from './api-service';
import { environmentStore } from '$lib/stores/environment.store.svelte';
import type { Project, ProjectStatusCounts, GitLogEntry, VolumeReport } from '$lib/types/project.type';
import type { SearchPaginationSortRequest, Paginated } from '$lib/types/pagination.type';
import { transformPaginationParams } from '$lib/utils/params.util';

export class ProjectService extends BaseAPIService {
	async getProjects(options?: SearchPaginationSortRequest): Promise<Paginated<Project>> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const params = transformPaginationParams(options);
		const res = await this.api.get(`/environments/${envId}/projects`, { params });
		return res.data;
	}

	async deployProject(projectId: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		return this.handleResponse(this.api.post(`/environments/${envId}/projects/${projectId}/up`));
	}

	async downProject(projectName: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		return this.handleResponse(this.api.post(`/environments/${envId}/projects/${projectName}/down`));
	}

	async createProject(projectName: string, composeContent: string, envContent?: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const payload = {
			name: projectName,
			composeContent,
			envContent
		};
		return this.handleResponse(this.api.post(`/environments/${envId}/projects`, payload));
	}

	async getProject(projectId: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const response = await this.handleResponse<{ project?: Project; success?: boolean }>(
			this.api.get(`/environments/${envId}/projects/${projectId}`)
		);

		return response.project ? response.project : (response as Project);
	}

	async getProjectStatusCounts(): Promise<ProjectStatusCounts> {
		const envId = await environmentStore.getCurrentEnvironmentId();

		const res = await this.api.get(`/environments/${envId}/projects/counts`);
		return res.data.data;
	}

	async updateProject(
		projectId: string,
		data: {
			name?: string;
			composeContent?: string;
			envContent?: string;
			gitRepoUrl?: string;
			gitBranch?: string;
			gitPath?: string;
			gitAuthUser?: string;
			gitAuthToken?: string;
			gitSyncMode?: 'manual' | 'pull' | 'push';
			gitIdentityId?: string | null;
			gitPollInterval?: number;
		}
	): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		return this.handleResponse(this.api.put(`/environments/${envId}/projects/${projectId}`, data));
	}

	async syncProjectFromGit(projectId: string): Promise<string> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		
		// Check for Resilience/Ops Plugin
		const { pluginsStore } = await import('$lib/stores/plugins.store.svelte');
		const pluginId = 'resilience_ops';
		const isPluginEnabled = pluginsStore.plugins.some(p => p.plugin_id === pluginId && p.enabled);

		if (isPluginEnabled) {
			const res = await this.handleResponse<{ message: string }>(
				this.api.post(`/environments/${envId}/plugins/${pluginId}/git/pull`, {
					project_id: projectId
				})
			);
			return res.message;
		}

		const res = await this.handleResponse<{ message: string }>(
			this.api.post(`/environments/${envId}/projects/${projectId}/git/pull`)
		);
		return res.message;
	}

	async pushProjectToGit(projectId: string, message?: string): Promise<string> {
		const envId = await environmentStore.getCurrentEnvironmentId();

		// Check for Resilience/Ops Plugin
		const { pluginsStore } = await import('$lib/stores/plugins.store.svelte');
		const pluginId = 'resilience_ops';
		const isPluginEnabled = pluginsStore.plugins.some(p => p.plugin_id === pluginId && p.enabled);

		if (isPluginEnabled) {
			const res = await this.handleResponse<{ message: string }>(
				this.api.post(`/environments/${envId}/plugins/${pluginId}/git/push`, {
					project_id: projectId,
					message: message
				})
			);
			return res.message;
		}

		const res = await this.handleResponse<{ message: string }>(
			this.api.post(`/environments/${envId}/projects/${projectId}/git/push`, null, {
				params: { message }
			})
		);
		return res.message;
	}

	async getProjectGitStatus(projectId: string): Promise<string> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const res = await this.api.get<any>(`/environments/${envId}/projects/${projectId}/git/status`);
		return res.data.data.message;
	}

	async updateProjectIncludeFile(projectId: string, relativePath: string, content: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const payload = {
			relativePath,
			content
		};
		return this.handleResponse(this.api.put(`/environments/${envId}/projects/${projectId}/includes`, payload));
	}

	async restartProject(projectId: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		return this.handleResponse(this.api.post(`/environments/${envId}/projects/${projectId}/restart`));
	}

	async redeployProject(projectName: string): Promise<Project> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		return this.handleResponse(this.api.post(`/environments/${envId}/projects/${projectName}/redeploy`));
	}

	private isDownloadingStatus(status?: string): boolean {
		if (!status) return false;
		const s = status.toLowerCase();
		return (
			s.includes('downloading') ||
			s.includes('extracting') ||
			s.includes('pull complete') ||
			s.includes('download complete') ||
			s.includes('pulling fs layer')
		);
	}

	private async streamProjectPull(projectId: string, onLine?: (data: any) => void): Promise<boolean> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const url = `/api/environments/${envId}/projects/${projectId}/pull`;

		const res = await fetch(url, { method: 'POST' });
		if (!res.ok || !res.body) {
			throw new Error(`Failed to start project image pull (${res.status})`);
		}

		const reader = res.body.getReader();
		const decoder = new TextDecoder();
		let buffer = '';
		let pulled = false;

		while (true) {
			const { value, done } = await reader.read();
			if (done) break;

			buffer += decoder.decode(value, { stream: true });
			const lines = buffer.split('\n');
			buffer = lines.pop() || '';

			for (const line of lines) {
				const trimmed = line.trim();
				if (!trimmed) continue;
				try {
					const obj = JSON.parse(trimmed);

					// Detect if any actual download happened
					if (!pulled) {
						const status = obj?.status as string | undefined;
						const total = obj?.progressDetail?.total as number | undefined;
						if (this.isDownloadingStatus(status) || (typeof total === 'number' && total > 0)) {
							pulled = true;
						}
					}

					onLine?.(obj);
				} catch {
					// ignore malformed line
				}
			}
		}
		return pulled;
	}

	pullProjectImages(projectId: string): Promise<void>;
	pullProjectImages(projectId: string, onLine: (data: any) => void): Promise<void>;
	async pullProjectImages(projectId: string, onLine?: (data: any) => void): Promise<void> {
		await this.streamProjectPull(projectId, onLine);
	}

	async deployProjectMaybePull(
		projectId: string,
		onPullLine?: (data: any) => void
	): Promise<{ pulled: boolean; project: Project }> {
		const pulled = await this.streamProjectPull(projectId, onPullLine);
		const project = await this.deployProject(projectId);
		return { pulled, project };
	}

	async destroyProject(projectName: string, removeVolumes = false, removeFiles = false): Promise<void> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		await this.handleResponse(
			this.api.delete(`/environments/${envId}/projects/${projectName}/destroy`, {
				data: {
					removeVolumes,
					removeFiles
				}
			})
		);
	}

	async getProjectGitLog(projectId: string): Promise<GitLogEntry[]> {
		const envId = await environmentStore.getCurrentEnvironmentId();
		const res = await this.api.get<any>(`/environments/${envId}/projects/${projectId}/git/log`);
		return res.data.data;
	}

	async validateGitConnection(data: {
		repoUrl: string;
		branch: string;
		authMode: 'manual' | 'saved';
		authUser?: string;
		authToken?: string;
		identityId?: string | null;
	}): Promise<void> {
		await this.handleResponse(this.api.post('/git/validate', data));
	}

	async getVolumeReport(): Promise<VolumeReport> {
		const envId = await environmentStore.getCurrentEnvironmentId();

		// Check for Resilience/Ops Plugin
		const { pluginsStore } = await import('$lib/stores/plugins.store.svelte');
		const pluginId = 'resilience_ops';
		const isPluginEnabled = pluginsStore.plugins.some(p => p.plugin_id === pluginId && p.enabled);

		if (isPluginEnabled) {
			const res = await this.api.get<{ body: VolumeReport }>(`/environments/${envId}/plugins/${pluginId}/reports/volumes`);
			return res.data.body;
		}

		const res = await this.api.get<any>('/reports/volumes');
		return res.data.data;
	}

	async toggleVolumeOverride(projectId: string, overrideKey: string, override: boolean): Promise<void> {
		await this.handleResponse(this.api.post(`/projects/${projectId}/volumes/override`, { override_key: overrideKey, override }));
	}
}

export const projectService = new ProjectService();
