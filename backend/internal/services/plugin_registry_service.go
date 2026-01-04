package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"

	"github.com/getarcaneapp/arcane/backend/internal/database"
	"github.com/getarcaneapp/arcane/backend/internal/models"
	"gorm.io/gorm"
)

type PluginRegistryService struct {
	db *database.DB
}

func NewPluginRegistryService(db *database.DB) *PluginRegistryService {
	return &PluginRegistryService{db: db}
}

func (s *PluginRegistryService) RegisterPlugin(envID, pluginID, baseURL string) (*models.PluginState, error) {
	var plugin models.PluginState
	err := s.db.Where("environment_id = ? AND plugin_id = ?", envID, pluginID).First(&plugin).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			secret, err := generateRandomSecret(32)
			if err != nil {
				return nil, err
			}

			plugin = models.PluginState{
				EnvironmentID: envID,
				PluginID:      pluginID,
				BaseURL:       baseURL,
				SharedSecret:  secret,
				Enabled:       false,
			}
			if err := s.db.Create(&plugin).Error; err != nil {
				return nil, err
			}
			return &plugin, nil
		}
		return nil, err
	}

	// Update BaseURL if changed, keep everything else
	if plugin.BaseURL != baseURL {
		plugin.BaseURL = baseURL
		if err := s.db.Save(&plugin).Error; err != nil {
			return nil, err
		}
	}

	return &plugin, nil
}

func (s *PluginRegistryService) GetPlugin(envID, pluginID string) (*models.PluginState, error) {
	var plugin models.PluginState
	err := s.db.Where("environment_id = ? AND plugin_id = ?", envID, pluginID).First(&plugin).Error
	if err != nil {
		return nil, err
	}
	return &plugin, nil
}

func (s *PluginRegistryService) ListPlugins(envID string) ([]models.PluginState, error) {
	var plugins []models.PluginState
	err := s.db.Where("environment_id = ?", envID).Find(&plugins).Error
	return plugins, err
}

func (s *PluginRegistryService) UpdatePlugin(envID, pluginID string, enabled *bool, config models.JSON) (*models.PluginState, error) {
	var plugin models.PluginState
	err := s.db.Where("environment_id = ? AND plugin_id = ?", envID, pluginID).First(&plugin).Error
	if err != nil {
		return nil, err
	}

	if enabled != nil {
		plugin.Enabled = *enabled
	}
	if config != nil {
		plugin.Config = config
	}

	if err := s.db.Save(&plugin).Error; err != nil {
		return nil, err
	}
	return &plugin, nil
}

func generateRandomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
