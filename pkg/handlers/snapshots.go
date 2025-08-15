package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"tab-sync-backend-refactor/pkg/config"
	"tab-sync-backend-refactor/pkg/database"
	"tab-sync-backend-refactor/pkg/middleware"
	"tab-sync-backend-refactor/pkg/models"
	"tab-sync-backend-refactor/pkg/utils"
)

// SnapshotHandler 快照处理器
type SnapshotHandler struct {
	config *config.Config
	db     database.DatabaseInterface
}

// NewSnapshotHandler 创建快照处理器
func NewSnapshotHandler(cfg *config.Config, db database.DatabaseInterface) *SnapshotHandler {
	return &SnapshotHandler{
		config: cfg,
		db:     db,
	}
}

// ListSnapshots 列出用户的所有快照
func (h *SnapshotHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	// 从认证中间件获取用户信息
	user, err := middleware.RequireUser(r.Context())
	if err != nil {
		utils.WriteUnauthorizedResponse(w, "Authentication required")
		return
	}

	// 获取快照列表
	snapshots, err := h.db.ListSnapshots(user.ID)
	if err != nil {
		utils.WriteInternalServerErrorResponse(w, "Failed to list snapshots: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, map[string]interface{}{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})
}

// CreateSnapshot 创建新快照
func (h *SnapshotHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	// 从认证中间件获取用户信息
	user, err := middleware.RequireUser(r.Context())
	if err != nil {
		utils.WriteUnauthorizedResponse(w, "Authentication required")
		return
	}

	// 解析请求体
	var req struct {
		Name      string              `json:"name"`
		TabGroups []models.TabGroup   `json:"tabGroups"`
	}

	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteBadRequestResponse(w, "Invalid request body")
		return
	}

	// 验证必填字段
	if req.Name == "" {
		utils.WriteBadRequestResponse(w, "Snapshot name is required")
		return
	}

	if len(req.TabGroups) == 0 {
		utils.WriteBadRequestResponse(w, "Tab groups are required")
		return
	}

	// 保存快照
	err = h.db.SaveSnapshot(user.ID, req.Name, req.TabGroups)
	if err != nil {
		utils.WriteInternalServerErrorResponse(w, "Failed to save snapshot: "+err.Error())
		return
	}

	utils.WriteCreatedResponse(w, map[string]interface{}{
		"message": "Snapshot created successfully",
		"name":    req.Name,
	})
}

// GetSnapshot 获取指定快照
func (h *SnapshotHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	// 从认证中间件获取用户信息
	user, err := middleware.RequireUser(r.Context())
	if err != nil {
		utils.WriteUnauthorizedResponse(w, "Authentication required")
		return
	}

	// 获取快照名称
	name := chi.URLParam(r, "name")
	if name == "" {
		utils.WriteBadRequestResponse(w, "Snapshot name is required")
		return
	}

	// 加载快照
	snapshot, err := h.db.LoadSnapshot(user.ID, name)
	if err != nil {
		utils.WriteNotFoundResponse(w, "Snapshot not found: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, snapshot)
}

// UpdateSnapshot 更新快照
func (h *SnapshotHandler) UpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	// 从认证中间件获取用户信息
	user, err := middleware.RequireUser(r.Context())
	if err != nil {
		utils.WriteUnauthorizedResponse(w, "Authentication required")
		return
	}

	// 获取快照名称
	name := chi.URLParam(r, "name")
	if name == "" {
		utils.WriteBadRequestResponse(w, "Snapshot name is required")
		return
	}

	// 解析请求体
	var req struct {
		TabGroups []models.TabGroup `json:"tabGroups"`
	}

	if err := utils.ParseJSONBody(r, &req); err != nil {
		utils.WriteBadRequestResponse(w, "Invalid request body")
		return
	}

	if len(req.TabGroups) == 0 {
		utils.WriteBadRequestResponse(w, "Tab groups are required")
		return
	}

	// 更新快照（实际上是保存，因为SaveSnapshot支持UPSERT）
	err = h.db.SaveSnapshot(user.ID, name, req.TabGroups)
	if err != nil {
		utils.WriteInternalServerErrorResponse(w, "Failed to update snapshot: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, map[string]interface{}{
		"message": "Snapshot updated successfully",
		"name":    name,
	})
}

// DeleteSnapshot 删除快照
func (h *SnapshotHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	// 从认证中间件获取用户信息
	user, err := middleware.RequireUser(r.Context())
	if err != nil {
		utils.WriteUnauthorizedResponse(w, "Authentication required")
		return
	}

	// 获取快照名称
	name := chi.URLParam(r, "name")
	if name == "" {
		utils.WriteBadRequestResponse(w, "Snapshot name is required")
		return
	}

	// 删除快照
	err = h.db.DeleteSnapshot(user.ID, name)
	if err != nil {
		utils.WriteNotFoundResponse(w, "Failed to delete snapshot: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, map[string]interface{}{
		"message": "Snapshot deleted successfully",
		"name":    name,
	})
}
