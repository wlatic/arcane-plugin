import BaseAPIService from './api-service';
import type { GitIdentity, CreateGitIdentityDto } from '../types/git-identity.type';

export class GitIdentityService extends BaseAPIService {
    async list(): Promise<GitIdentity[]> {
        return this.handleResponse(this.api.get('/settings/git-identities'));
    }

    async create(data: CreateGitIdentityDto): Promise<GitIdentity> {
        return this.handleResponse(this.api.post('/settings/git-identities', data));
    }

    async delete(id: string): Promise<void> {
        return this.handleResponse(this.api.delete(`/settings/git-identities/${id}`));
    }

    async update(id: string, data: { name: string; username: string; token?: string }): Promise<GitIdentity> {
        return this.handleResponse(this.api.put(`/settings/git-identities/${id}`, data));
    }

    async test(data: { username: string; token: string }): Promise<void> {
        return this.handleResponse(this.api.post('/settings/git-identities/test', data));
    }
}

export const gitIdentityService = new GitIdentityService();
