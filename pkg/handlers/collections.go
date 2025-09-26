package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "strconv"
    "time"

    "tab-sync-backend-refactor/pkg/config"
    "tab-sync-backend-refactor/pkg/database"
    "tab-sync-backend-refactor/pkg/middleware"
    "tab-sync-backend-refactor/pkg/models"
    "tab-sync-backend-refactor/pkg/utils"

    chiRoute "github.com/go-chi/chi/v5"
)

type CollectionsHandler struct {
    config *config.Config
    db     database.DatabaseInterface
}

func NewCollectionsHandler(cfg *config.Config, db database.DatabaseInterface) *CollectionsHandler {
    return &CollectionsHandler{config: cfg, db: db}
}

// helper: require edit permission on a space (owner/admin or explicit can_edit)
func (h *CollectionsHandler) requireSpaceEdit(w http.ResponseWriter, userID, spaceID string) (spaceOrgID string, ok bool) {
    // get space to determine org
    space, err := h.db.GetSpaceByID(spaceID)
    if err != nil { utils.WriteNotFoundResponse(w, "space not found"); return "", false }
    spaceOrgID = space.OrganizationID
    // owner/admin?
    members, err := h.db.ListOrganizationMembers(spaceOrgID)
    if err == nil {
        for _, m := range members {
            if m.UserID == userID && (m.Role == models.RoleOwner || m.Role == models.RoleAdmin) {
                return spaceOrgID, true
            }
        }
    }
    // explicit permission
    perms, err := h.db.GetSpacePermissions(spaceID)
    if err == nil {
        for _, p := range perms {
            if p.UserID == userID && p.CanEdit { return spaceOrgID, true }
        }
    }
    utils.WriteForbiddenResponse(w, "No edit permission for this space")
    return "", false
}

// GET /api/collections?space_id=
func (h *CollectionsHandler) ListCollections(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    spaceID := r.URL.Query().Get("space_id")
    if strings.TrimSpace(spaceID) == "" { utils.WriteBadRequestResponse(w, "space_id required"); return }
    // must be org member to view
    space, err := h.db.GetSpaceByID(spaceID)
    if err != nil { utils.WriteNotFoundResponse(w, "space not found"); return }
    // basic membership check
    members, _ := h.db.ListOrganizationMembers(space.OrganizationID)
    allowed := false
    for _, m := range members { if m.UserID == user.ID { allowed = true; break } }
    if !allowed { utils.WriteForbiddenResponse(w, "Not a member of organization"); return }
    list, err := h.db.ListCollectionsBySpace(spaceID)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }

    // Optional pagination and incremental filtering
    page := 1
    pageSize := 20
    if v := r.URL.Query().Get("page"); v != "" { if n, e := strconv.Atoi(v); e == nil && n > 0 { page = n } }
    if v := r.URL.Query().Get("page_size"); v != "" { if n, e := strconv.Atoi(v); e == nil && n > 0 && n <= 200 { pageSize = n } }
    // since in milliseconds epoch or RFC3339
    var sinceTime time.Time
    if sv := r.URL.Query().Get("since"); sv != "" {
        if ms, e := strconv.ParseInt(sv, 10, 64); e == nil {
            sinceTime = time.Unix(0, ms*int64(time.Millisecond))
        } else {
            if t, e2 := time.Parse(time.RFC3339, sv); e2 == nil { sinceTime = t }
        }
    }

    filtered := make([]models.Collection, 0, len(list))
    var maxUpdated, maxDeleted int64
    for _, c := range list {
        // Incremental: include updated rows OR tombstones newer than since
        if !sinceTime.IsZero() {
            include := c.UpdatedAt.After(sinceTime)
            if c.DeletedAt != nil && c.DeletedAt.After(sinceTime) { include = true }
            if include { filtered = append(filtered, c) }
        } else {
            // No since: only active rows
            if c.DeletedAt == nil { filtered = append(filtered, c) }
        }
        if ts := c.UpdatedAt.UnixMilli(); ts > maxUpdated { maxUpdated = ts }
        if c.DeletedAt != nil {
            if td := c.DeletedAt.UnixMilli(); td > maxDeleted { maxDeleted = td }
        }
    }
    total := len(filtered)
    start := (page - 1) * pageSize
    if start < 0 { start = 0 }
    if start > total { start = total }
    end := start + pageSize
    if end > total { end = total }
    pageItems := filtered[start:end]

    // Set ETag header (weak)
    etag := fmt.Sprintf("W/\"collections:%s:%d:%d:%d\"", spaceID, total, maxUpdated, maxDeleted)
    w.Header().Set("ETag", etag)

    utils.WriteSuccessResponse(w, map[string]interface{}{
        "collections": pageItems,
        "total":       total,
        "next_since":  maxUpdated,
        "page":        page,
        "page_size":   pageSize,
    })
}

// POST /api/collections
func (h *CollectionsHandler) CreateCollection(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{
        SpaceID string `json:"space_id"`
        Name string `json:"name"`
        Description string `json:"description"`
        Color string `json:"color"`
        Icon string `json:"icon"`
        Position int `json:"position"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if strings.TrimSpace(req.SpaceID) == "" || strings.TrimSpace(req.Name) == "" {
        utils.WriteBadRequestResponse(w, "space_id and name required"); return
    }
    if _, ok := h.requireSpaceEdit(w, user.ID, req.SpaceID); !ok { return }
    c := &models.Collection{
        SpaceID: req.SpaceID,
        Name: req.Name,
        Description: req.Description,
        Color: req.Color,
        Icon: req.Icon,
        Position: req.Position,
    }
    if err := h.db.CreateCollection(c); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"collection": c})
}

// PUT /api/collections/{id}
func (h *CollectionsHandler) UpdateCollection(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    id := chiRoute.URLParam(r, "id")
    if strings.TrimSpace(id) == "" { utils.WriteBadRequestResponse(w, "id required"); return }
    var req struct{
        SpaceID string `json:"space_id"`
        Name *string `json:"name"`
        Description *string `json:"description"`
        Color *string `json:"color"`
        Icon *string `json:"icon"`
        Position *int `json:"position"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if strings.TrimSpace(req.SpaceID) == "" { utils.WriteBadRequestResponse(w, "space_id required"); return }
    // load existing
    existing, err := h.db.GetCollection(id)
    if err != nil { utils.WriteNotFoundResponse(w, "collection not found"); return }
    // permission against its (target) space
    if _, ok := h.requireSpaceEdit(w, user.ID, existing.SpaceID); !ok { return }
    // patch fields
    existing.SpaceID = req.SpaceID // allow move across spaces if permissions allow
    if req.Name != nil { existing.Name = *req.Name }
    if req.Description != nil { existing.Description = *req.Description }
    if req.Color != nil { existing.Color = *req.Color }
    if req.Icon != nil { existing.Icon = *req.Icon }
    if req.Position != nil { existing.Position = *req.Position }
    if err := h.db.UpdateCollection(existing); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"collection": existing})
}

// DELETE /api/collections/{id}
func (h *CollectionsHandler) DeleteCollection(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    id := chiRoute.URLParam(r, "id")
    spaceID := r.URL.Query().Get("space_id")
    if strings.TrimSpace(id) == "" { utils.WriteBadRequestResponse(w, "id required"); return }
    if strings.TrimSpace(spaceID) == "" {
        // try load collection to infer space id
        if c, e := h.db.GetCollection(id); e == nil { spaceID = c.SpaceID }
    }
    if strings.TrimSpace(spaceID) == "" { utils.WriteBadRequestResponse(w, "space_id required"); return }
    if _, ok := h.requireSpaceEdit(w, user.ID, spaceID); !ok { return }
    if err := h.db.DeleteCollection(id); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"deleted": true, "id": id})
}

// GET /api/collections/{id}/items
func (h *CollectionsHandler) ListItems(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    collectionID := chiRoute.URLParam(r, "id")
    if strings.TrimSpace(collectionID) == "" { utils.WriteBadRequestResponse(w, "collection id required"); return }
    coll, err := h.db.GetCollection(collectionID)
    if err != nil { utils.WriteNotFoundResponse(w, "collection not found"); return }
    // must be org member
    space, _ := h.db.GetSpaceByID(coll.SpaceID)
    if space == nil { utils.WriteNotFoundResponse(w, "space not found"); return }
    members, _ := h.db.ListOrganizationMembers(space.OrganizationID)
    allowed := false
    for _, m := range members { if m.UserID == user.ID { allowed = true; break } }
    if !allowed { utils.WriteForbiddenResponse(w, "Not a member of organization"); return }
    items, err := h.db.ListItemsByCollection(collectionID)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"items": items})
}


// POST /api/collections/{id}/items
func (h *CollectionsHandler) CreateItem(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    collectionID := chiRoute.URLParam(r, "id")
    if strings.TrimSpace(collectionID) == "" { utils.WriteBadRequestResponse(w, "collection id required"); return }
    coll, err := h.db.GetCollection(collectionID)
    if err != nil { utils.WriteNotFoundResponse(w, "collection not found"); return }
    if _, ok := h.requireSpaceEdit(w, user.ID, coll.SpaceID); !ok { return }
    var req struct {
        Title string `json:"title"`
        URL string `json:"url"`
        FavIconURL string `json:"fav_icon_url"`
        OriginalTitle string `json:"original_title"`
        AIGeneratedTitle string `json:"ai_generated_title"`
        Domain string `json:"domain"`
        Metadata map[string]interface{} `json:"metadata"`
        Position int `json:"position"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    metaJSON, _ := json.Marshal(req.Metadata)
    it := &models.CollectionItem{
        CollectionID: collectionID,
        Title: req.Title,
        URL: req.URL,
        FavIconURL: req.FavIconURL,
        OriginalTitle: req.OriginalTitle,
        AIGeneratedTitle: req.AIGeneratedTitle,
        Domain: req.Domain,
        Metadata: metaJSON,
        Position: req.Position,
    }
    if err := h.db.CreateCollectionItem(it); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"item": it})
}

// PUT /api/collection-items/{item_id}
func (h *CollectionsHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    itemID := chiRoute.URLParam(r, "item_id")
    if strings.TrimSpace(itemID) == "" { utils.WriteBadRequestResponse(w, "item id required"); return }
    var req struct {
        CollectionID string `json:"collection_id"`
        Title *string `json:"title"`
        URL *string `json:"url"`
        FavIconURL *string `json:"fav_icon_url"`
        OriginalTitle *string `json:"original_title"`
        AIGeneratedTitle *string `json:"ai_generated_title"`
        Domain *string `json:"domain"`
        Metadata map[string]interface{} `json:"metadata"`
        Position *int `json:"position"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if strings.TrimSpace(req.CollectionID) == "" { utils.WriteBadRequestResponse(w, "collection_id required"); return }
    coll, err := h.db.GetCollection(req.CollectionID)
    if err != nil { utils.WriteNotFoundResponse(w, "collection not found"); return }
    if _, ok := h.requireSpaceEdit(w, user.ID, coll.SpaceID); !ok { return }
    // build item for update
    metaJSON, _ := json.Marshal(req.Metadata)
    it := &models.CollectionItem{ ID: itemID, CollectionID: req.CollectionID, Metadata: metaJSON }
    if req.Title != nil { it.Title = *req.Title }
    if req.URL != nil { it.URL = *req.URL }
    if req.FavIconURL != nil { it.FavIconURL = *req.FavIconURL }
    if req.OriginalTitle != nil { it.OriginalTitle = *req.OriginalTitle }
    if req.AIGeneratedTitle != nil { it.AIGeneratedTitle = *req.AIGeneratedTitle }
    if req.Domain != nil { it.Domain = *req.Domain }
    if req.Position != nil { it.Position = *req.Position }
    if err := h.db.UpdateCollectionItem(it); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"item": it})
}

// DELETE /api/collection-items/{item_id}?collection_id=
func (h *CollectionsHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    itemID := chiRoute.URLParam(r, "item_id")
    collID := r.URL.Query().Get("collection_id")
    if strings.TrimSpace(itemID) == "" || strings.TrimSpace(collID) == "" { utils.WriteBadRequestResponse(w, "item_id and collection_id required"); return }
    coll, err := h.db.GetCollection(collID)
    if err != nil { utils.WriteNotFoundResponse(w, "collection not found"); return }
    if _, ok := h.requireSpaceEdit(w, user.ID, coll.SpaceID); !ok { return }
    if err := h.db.DeleteCollectionItem(itemID); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"deleted": true, "id": itemID})
}
