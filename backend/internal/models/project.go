package models

import "time"

type ProjectStatus string

const (
	ProjectStatusRunning          ProjectStatus = "running"
	ProjectStatusStopped          ProjectStatus = "stopped"
	ProjectStatusPartiallyRunning ProjectStatus = "partially running"
	ProjectStatusUnknown          ProjectStatus = "unknown"
	ProjectStatusDeploying        ProjectStatus = "deploying"
	ProjectStatusStopping         ProjectStatus = "stopping"
	ProjectStatusRestarting       ProjectStatus = "restarting"
	ProjectStatusGitConflict      ProjectStatus = "git_conflict"
)

type Project struct {
	Name         string        `json:"name" sortable:"true"`
	DirName      *string       `json:"dir_name"`
	Path         string        `json:"path"`
	Status       ProjectStatus `json:"status" sortable:"true"`
	StatusReason *string       `json:"status_reason"`
	RunningCount int           `json:"running_count" sortable:"true"`

	GitRepoURL      *string      `json:"git_repo_url"`
	GitBranch       *string      `json:"git_branch"`
	GitPath         *string      `json:"git_path"`
	GitAuthUser     *string      `json:"git_auth_user"`
	GitAuthToken    *string      `json:"git_auth_token"`
	GitSyncMode     string       `json:"git_sync_mode" gorm:"default:'manual'"`
	GitIdentityID   *string      `json:"git_identity_id"`
	GitIdentity     *GitIdentity `json:"git_identity" gorm:"foreignKey:GitIdentityID"`
	GitPollInterval int          `json:"git_poll_interval" gorm:"default:5"`
	GitLastSyncAt   *time.Time   `json:"git_last_sync_at"`

	VolumeOverrides *string `json:"volume_overrides"`

	// Docker config
	ServiceCount int `json:"service_count" gorm:"default:0"`

	BaseModel
}

func (Project) TableName() string {
	return "projects"
}
