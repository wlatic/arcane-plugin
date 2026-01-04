import { projectService } from '$lib/services/project-service';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const project = await projectService.getProject(params.projectId);

	const editorState = {
		name: project.name || '',
		composeContent: project.composeContent || '',
		envContent: project.envContent || '',
		gitRepoUrl: project.gitRepoUrl || '',
		gitBranch: project.gitBranch || '',
		gitPath: project.gitPath || '',
		gitAuthUser: project.gitAuthUser || '',
		gitAuthToken: project.gitAuthToken || '',
		gitSyncMode: project.gitSyncMode || 'manual',
		originalName: project.name || '',
		originalComposeContent: project.composeContent || '',
		originalEnvContent: project.envContent || '',
		originalGitRepoUrl: project.gitRepoUrl || '',
		originalGitBranch: project.gitBranch || '',
		originalGitPath: project.gitPath || '',
		originalGitAuthUser: project.gitAuthUser || '',
		originalGitAuthToken: project.gitAuthToken || '',
		originalGitSyncMode: project.gitSyncMode || 'manual',
		gitIdentityId: project.gitIdentityId || null,
		originalGitIdentityId: project.gitIdentityId || null,
		gitPollInterval: project.gitPollInterval || 5,
		originalGitPollInterval: project.gitPollInterval || 5
	};

	return {
		projectId: params.projectId,
		project,
		editorState,
		error: null
	};
};
