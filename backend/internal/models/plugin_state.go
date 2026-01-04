package models

type PluginState struct {
	BaseModel
	EnvironmentID string `json:"environmentId" gorm:"column:environment_id"`
	PluginID      string `json:"pluginId" gorm:"column:plugin_id"`
	BaseURL       string `json:"baseUrl" gorm:"column:base_url"`
	Enabled       bool   `json:"enabled" gorm:"column:enabled"`
	Config        JSON   `json:"config" gorm:"column:config"`
	SharedSecret  string `json:"-" gorm:"column:shared_secret"`
}

func (PluginState) TableName() string {
	return "plugin_states"
}
