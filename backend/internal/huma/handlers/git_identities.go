package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/getarcaneapp/arcane/backend/internal/services"
	"github.com/getarcaneapp/arcane/types/base"
)

type GitIdentityHandler struct {
	service *services.GitIdentityService
}

func RegisterGitIdentities(api huma.API, service *services.GitIdentityService) {
	h := &GitIdentityHandler{
		service: service,
	}

	huma.Register(api, huma.Operation{
		OperationID: "list-git-identities",
		Method:      http.MethodGet,
		Path:        "/settings/git-identities",
		Summary:     "List Git Identities",
		Description: "List all saved Git credentials profiles",
		Tags:        []string{"Settings", "Git"},
		Security: []map[string][]string{
			{"BearerAuth": {}},
			{"ApiKeyAuth": {}},
		},
	}, h.ListIdentities)

	huma.Register(api, huma.Operation{
		OperationID: "create-git-identity",
		Method:      http.MethodPost,
		Path:        "/settings/git-identities",
		Summary:     "Create Git Identity",
		Description: "Save a new Git credentials profile",
		Tags:        []string{"Settings", "Git"},
		Security: []map[string][]string{
			{"BearerAuth": {}},
			{"ApiKeyAuth": {}},
		},
	}, h.CreateIdentity)

	huma.Register(api, huma.Operation{
		OperationID: "delete-git-identity",
		Method:      http.MethodDelete,
		Path:        "/settings/git-identities/{id}",
		Summary:     "Delete Git Identity",
		Description: "Delete a Git credentials profile",
		Tags:        []string{"Settings", "Git"},
		Security: []map[string][]string{
			{"BearerAuth": {}},
			{"ApiKeyAuth": {}},
		},
	}, h.DeleteIdentity)

	huma.Register(api, huma.Operation{
		OperationID: "update-git-identity",
		Method:      http.MethodPut,
		Path:        "/settings/git-identities/{id}",
		Summary:     "Update Git Identity",
		Description: "Update a Git credentials profile",
		Tags:        []string{"Settings", "Git"},
		Security: []map[string][]string{
			{"BearerAuth": {}},
			{"ApiKeyAuth": {}},
		},
	}, h.UpdateIdentity)

	huma.Register(api, huma.Operation{
		OperationID: "test-git-identity",
		Method:      http.MethodPost,
		Path:        "/settings/git-identities/test",
		Summary:     "Test Git Identity",
		Description: "Test Git credentials against GitHub API",
		Tags:        []string{"Settings", "Git"},
		Security: []map[string][]string{
			{"BearerAuth": {}},
			{"ApiKeyAuth": {}},
		},
	}, h.TestIdentity)
}

// Types

type GitIdentityResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

type ListGitIdentitiesOutput struct {
	Body base.ApiResponse[[]GitIdentityResponse]
}

type CreateGitIdentityInput struct {
	Body struct {
		Name     string `json:"name" doc:"Name of the identity"`
		Username string `json:"username" doc:"Git username"`
		Token    string `json:"token" doc:"Personal Access Token (PAT)"`
	}
}

type CreateGitIdentityOutput struct {
	Body base.ApiResponse[GitIdentityResponse]
}

type DeleteGitIdentityInput struct {
	ID string `path:"id"`
}

type DeleteGitIdentityOutput struct {
	Body base.ApiResponse[base.MessageResponse]
}

// Handlers

func (h *GitIdentityHandler) ListIdentities(ctx context.Context, input *struct{}) (*ListGitIdentitiesOutput, error) {
	identities, err := h.service.ListIdentities(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	resp := make([]GitIdentityResponse, 0)
	for _, id := range identities {
		resp = append(resp, GitIdentityResponse{
			ID:        id.ID,
			Name:      id.Name,
			Username:  id.Username,
			CreatedAt: id.CreatedAt,
		})
	}

	return &ListGitIdentitiesOutput{
		Body: base.ApiResponse[[]GitIdentityResponse]{
			Success: true,
			Data:    resp,
		},
	}, nil
}

func (h *GitIdentityHandler) CreateIdentity(ctx context.Context, input *CreateGitIdentityInput) (*CreateGitIdentityOutput, error) {
	identity, err := h.service.CreateIdentity(ctx, input.Body.Name, input.Body.Username, input.Body.Token)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &CreateGitIdentityOutput{
		Body: base.ApiResponse[GitIdentityResponse]{
			Success: true,
			Data: GitIdentityResponse{
				ID:        identity.ID,
				Name:      identity.Name,
				Username:  identity.Username,
				CreatedAt: identity.CreatedAt,
			},
		},
	}, nil
}

func (h *GitIdentityHandler) DeleteIdentity(ctx context.Context, input *DeleteGitIdentityInput) (*DeleteGitIdentityOutput, error) {
	err := h.service.DeleteIdentity(ctx, input.ID)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &DeleteGitIdentityOutput{
		Body: base.ApiResponse[base.MessageResponse]{
			Success: true,
			Data: base.MessageResponse{
				Message: "Identity deleted successfully",
			},
		},
	}, nil
}

type TestGitIdentityInput struct {
	Body struct {
		Username string `json:"username" doc:"Git username"`
		Token    string `json:"token" doc:"Personal Access Token (PAT)"`
	}
}

type TestGitIdentityOutput struct {
	Body base.ApiResponse[base.MessageResponse]
}

func (h *GitIdentityHandler) TestIdentity(ctx context.Context, input *TestGitIdentityInput) (*TestGitIdentityOutput, error) {
	// Simple validation against GitHub API
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, huma.Error500InternalServerError("failed to create request")
	}
	req.SetBasicAuth(input.Body.Username, input.Body.Token)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("connection failed: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, huma.Error401Unauthorized("Invalid credentials")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, huma.Error500InternalServerError(fmt.Sprintf("GitHub API check failed with status: %d", resp.StatusCode))
	}

	return &TestGitIdentityOutput{
		Body: base.ApiResponse[base.MessageResponse]{
			Success: true,
			Data: base.MessageResponse{
				Message: "Connection successful!",
			},
		},
	}, nil
}

type UpdateGitIdentityInput struct {
	ID   string `path:"id"`
	Body struct {
		Name     string `json:"name" doc:"Name of the identity"`
		Username string `json:"username" doc:"Git username"`
		Token    string `json:"token,omitempty" doc:"Personal Access Token (PAT) - leave empty to keep existing"`
	}
}

type UpdateGitIdentityOutput struct {
	Body base.ApiResponse[GitIdentityResponse]
}

func (h *GitIdentityHandler) UpdateIdentity(ctx context.Context, input *UpdateGitIdentityInput) (*UpdateGitIdentityOutput, error) {
	identity, err := h.service.UpdateIdentity(ctx, input.ID, input.Body.Name, input.Body.Username, input.Body.Token)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}

	return &UpdateGitIdentityOutput{
		Body: base.ApiResponse[GitIdentityResponse]{
			Success: true,
			Data: GitIdentityResponse{
				ID:        identity.ID,
				Name:      identity.Name,
				Username:  identity.Username,
				CreatedAt: identity.CreatedAt,
			},
		},
	}, nil
}
