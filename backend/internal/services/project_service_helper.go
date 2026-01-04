package services

import (
	"context"
	"fmt"

	"github.com/getarcaneapp/arcane/backend/internal/models"
)

func (s *ProjectService) resolveGitCredentials(ctx context.Context, project *models.Project) error {
	if project.GitIdentityID != nil && *project.GitIdentityID != "" {
		username, token, err := s.gitIdentityService.GetIdentityCredentials(ctx, *project.GitIdentityID)
		if err != nil {
			return fmt.Errorf("failed to resolve git identity: %w", err)
		}
		project.GitAuthUser = &username
		project.GitAuthToken = &token
	}
	return nil
}
