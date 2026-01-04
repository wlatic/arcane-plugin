package services

import (
	"context"
	"fmt"
	"time"

	"github.com/getarcaneapp/arcane/backend/internal/database"
	"github.com/getarcaneapp/arcane/backend/internal/models"
	"github.com/getarcaneapp/arcane/backend/internal/utils"
)

type GitIdentityService struct {
	db *database.DB
}

func NewGitIdentityService(db *database.DB) *GitIdentityService {
	return &GitIdentityService{
		db: db,
	}
}

func (s *GitIdentityService) ListIdentities(ctx context.Context) ([]models.GitIdentity, error) {
	var identities []models.GitIdentity
	if err := s.db.WithContext(ctx).Order("created_at desc").Find(&identities).Error; err != nil {
		return nil, fmt.Errorf("failed to list git identities: %w", err)
	}

	// Decrypt tokens ? No, usually we don't return tokens in list, or we return masked.
	// For this use case, we might want to return them but masked in the handler.
	// But let's decrypt them here just in case the caller needs them,
	// OR we assume the handler handles masking.
	// Actually, for security, let's NOT decrypt in List. The user can't see the token anyway.
	// Be careful: if we return the encrypted string, the UI might display it.
	// Ideally we return a DTO without the token or with a "******" placeholder.
	// But since this is the service layer returning the model, let's keep it as is (encrypted in DB).
	// The handler should map to a response type.

	return identities, nil
}

func (s *GitIdentityService) GetIdentity(ctx context.Context, id string) (*models.GitIdentity, error) {
	var identity models.GitIdentity
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&identity).Error; err != nil {
		return nil, fmt.Errorf("failed to get git identity: %w", err)
	}

	// We decrypt the token when retrieving a single identity for internal use?
	// Or explicitly provide a method to get credentials.
	return &identity, nil
}

// GetIdentityCredentials returns the decrypted username and token for an identity
func (s *GitIdentityService) GetIdentityCredentials(ctx context.Context, id string) (string, string, error) {
	identity, err := s.GetIdentity(ctx, id)
	if err != nil {
		return "", "", err
	}

	token, err := utils.Decrypt(identity.Token)
	if err != nil {
		return "", "", fmt.Errorf("failed to decrypt token: %w", err)
	}

	return identity.Username, token, nil
}

func (s *GitIdentityService) CreateIdentity(ctx context.Context, name, username, token string) (*models.GitIdentity, error) {
	encryptedToken, err := utils.Encrypt(token)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt token: %w", err)
	}

	identity := models.GitIdentity{
		Name:      name,
		Username:  username,
		Token:     encryptedToken,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(&identity).Error; err != nil {
		return nil, fmt.Errorf("failed to create git identity: %w", err)
	}

	return &identity, nil
}

func (s *GitIdentityService) DeleteIdentity(ctx context.Context, id string) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&models.GitIdentity{}).Error; err != nil {
		return fmt.Errorf("failed to delete git identity: %w", err)
	}
	return nil
}

func (s *GitIdentityService) UpdateIdentity(ctx context.Context, id, name, username, token string) (*models.GitIdentity, error) {
	var identity models.GitIdentity
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&identity).Error; err != nil {
		return nil, fmt.Errorf("failed to find git identity: %w", err)
	}

	identity.Name = name
	identity.Username = username
	identity.UpdatedAt = time.Now()

	if token != "" {
		encryptedToken, err := utils.Encrypt(token)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt token: %w", err)
		}
		identity.Token = encryptedToken
	}

	if err := s.db.WithContext(ctx).Save(&identity).Error; err != nil {
		return nil, fmt.Errorf("failed to update git identity: %w", err)
	}

	return &identity, nil
}
