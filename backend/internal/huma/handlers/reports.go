package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/getarcaneapp/arcane/backend/internal/services"
	"github.com/getarcaneapp/arcane/types/base"
	"github.com/getarcaneapp/arcane/types/project"
)

type ReportsHandler struct {
	projectService  *services.ProjectService
	settingsService *services.SettingsService
}

func RegisterReports(api huma.API, projectService *services.ProjectService, settingsService *services.SettingsService) {
	handler := &ReportsHandler{
		projectService:  projectService,
		settingsService: settingsService,
	}

	huma.Register(api, huma.Operation{
		OperationID: "get-volume-report",
		Method:      http.MethodGet,
		Path:        "/reports/volumes",
		Summary:     "Get volume report",
		Tags:        []string{"Reports"},
	}, handler.GetVolumeReport)

	huma.Register(api, huma.Operation{
		OperationID: "toggle-volume-override",
		Method:      http.MethodPost,
		Path:        "/projects/{id}/volumes/override",
		Summary:     "Toggle volume override",
		Tags:        []string{"Reports", "Projects"},
	}, handler.ToggleVolumeOverride)
}

type GetVolumeReportOutput struct {
	Body base.ApiResponse[project.VolumeReport]
}

func (h *ReportsHandler) GetVolumeReport(ctx context.Context, input *struct{}) (*GetVolumeReportOutput, error) {
	if !h.settingsService.GetBoolSetting(ctx, "opsModeEnabled", false) {
		return nil, huma.Error403Forbidden("Resilience / Ops mode is disabled")
	}

	report, err := h.projectService.GetVolumeReport(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &GetVolumeReportOutput{
		Body: base.ApiResponse[project.VolumeReport]{
			Success: true,
			Data:    *report,
		},
	}, nil
}

type ToggleVolumeOverrideInput struct {
	ProjectID string `path:"id"`
	Body      struct {
		OverrideKey string `json:"override_key"`
		Override    bool   `json:"override"`
	}
}

type ToggleVolumeOverrideOutput struct {
	Body base.ApiResponse[base.MessageResponse]
}

func (h *ReportsHandler) ToggleVolumeOverride(ctx context.Context, input *ToggleVolumeOverrideInput) (*ToggleVolumeOverrideOutput, error) {
	if !h.settingsService.GetBoolSetting(ctx, "opsModeEnabled", false) {
		return nil, huma.Error403Forbidden("Resilience / Ops mode is disabled")
	}

	err := h.projectService.ToggleVolumeOverride(ctx, input.ProjectID, input.Body.OverrideKey, input.Body.Override)
	if err != nil {
		return nil, huma.Error500InternalServerError(err.Error())
	}
	return &ToggleVolumeOverrideOutput{
		Body: base.ApiResponse[base.MessageResponse]{
			Success: true,
			Data: base.MessageResponse{
				Message: "Override updated",
			},
		},
	}, nil
}
