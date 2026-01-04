export interface GitIdentity {
    id: string;
    name: string;
    username: string;
    created_at: string;
    // Token is never returned to frontend
}

export interface CreateGitIdentityDto {
    name: string;
    username: string;
    token: string;
}
