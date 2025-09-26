package handlers

import (
    "fmt"
    "net/http"
    "strings"
    "time"

    "tab-sync-backend-refactor/pkg/config"
    "tab-sync-backend-refactor/pkg/database"
    "tab-sync-backend-refactor/pkg/middleware"
    "tab-sync-backend-refactor/pkg/models"
    "tab-sync-backend-refactor/pkg/utils"
    chiRoute "github.com/go-chi/chi/v5"
)

type OrgsHandler struct {
    config *config.Config
    db     database.DatabaseInterface
}

func NewOrgsHandler(cfg *config.Config, db database.DatabaseInterface) *OrgsHandler {
    return &OrgsHandler{config: cfg, db: db}
}

// ==== helpers: membership/role checks ====
func (h *OrgsHandler) getUserRoleInOrg(userID, orgID string) (models.OrgMemberRole, bool) {
    // owner fast-path
    if org, err := h.db.GetOrganization(orgID); err == nil {
        if org.OwnerID == userID {
            return models.RoleOwner, true
        }
    }
    // check memberships
    members, err := h.db.ListOrganizationMembers(orgID)
    if err != nil {
        return "", false
    }
    for _, m := range members {
        if m.UserID == userID {
            return m.Role, true
        }
    }
    return "", false
}

func (h *OrgsHandler) requireOrgMember(w http.ResponseWriter, userID, orgID string) (models.OrgMemberRole, bool) {
    role, ok := h.getUserRoleInOrg(userID, orgID)
    if !ok {
        utils.WriteForbiddenResponse(w, "Not a member of organization")
        return "", false
    }
    return role, true
}

func (h *OrgsHandler) requireOwner(w http.ResponseWriter, userID, orgID string) bool {
    role, ok := h.getUserRoleInOrg(userID, orgID)
    if !ok || role != models.RoleOwner {
        utils.WriteForbiddenResponse(w, "Owner privileges required")
        return false
    }
    return true
}

// POST /api/orgs
func (h *OrgsHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{
        Name string `json:"name"`
        Description string `json:"description"`
        Avatar string `json:"avatar"`
        Color string `json:"color"`
        DefaultSpaces []struct{ Name, Description string; IsDefault bool } `json:"default_spaces"`
        InviteEmails []string `json:"invite_emails"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if strings.TrimSpace(req.Name) == "" { utils.WriteBadRequestResponse(w, "Name required"); return }

    // Default color if not provided
    color := strings.TrimSpace(req.Color)
    if color == "" { color = "#3b82f6" }
    org := &models.Organization{ Name: req.Name, Description: req.Description, Avatar: req.Avatar, Color: color, OwnerID: user.ID }
    if err := h.db.CreateOrganization(org); err != nil { utils.WriteInternalServerErrorResponse(w, "Create org failed: "+err.Error()); return }

    // Create optional default spaces
    for _, s := range req.DefaultSpaces {
        _ = h.db.CreateSpace(&models.Space{ OrganizationID: org.ID, Name: s.Name, Description: s.Description, IsDefault: s.IsDefault })
    }

    // Create invitations async-like (no email send here)
    for _, email := range req.InviteEmails {
        email = strings.TrimSpace(email)
        if email == "" { continue }
        // 生成安全的 URL-safe token
        tok, err := utils.GenerateURLToken(24)
        if err != nil { fmt.Printf("[warn] failed to generate token for %s: %v\n", email, err); continue }
        inv := &models.OrganizationInvitation{ OrganizationID: org.ID, Email: email, InviterID: user.ID, Token: tok, Status: models.InvitationPending, ExpiresAt: time.Now().Add(14*24*time.Hour) }
        if err := h.db.CreateInvitation(inv); err != nil { fmt.Printf("[warn] failed to create invitation for %s: %v\n", email, err) }
    }

    utils.WriteSuccessResponse(w, map[string]interface{}{ "organization": org })
}

// PUT /api/orgs/{id}
func (h *OrgsHandler) UpdateOrganization(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    orgID := chiRoute.URLParam(r, "id")
    if strings.TrimSpace(orgID) == "" { utils.WriteBadRequestResponse(w, "organization id required"); return }
    // Ensure membership and role
    role, ok := h.requireOrgMember(w, user.ID, orgID)
    if !ok { return }
    if role != models.RoleOwner && role != models.RoleAdmin {
        utils.WriteForbiddenResponse(w, "Only owner/admin can update organization")
        return
    }
    // Parse patch
    var req struct{
        Name string `json:"name"`
        Description string `json:"description"`
        Avatar string `json:"avatar"`
        Color string `json:"color"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    // Load current org (optional)
    org, err := h.db.GetOrganization(orgID)
    if err != nil { utils.WriteNotFoundResponse(w, "organization not found"); return }
    // Apply patch values (only non-empty)
    if strings.TrimSpace(req.Name) != "" { org.Name = req.Name }
    if strings.TrimSpace(req.Description) != "" { org.Description = req.Description }
    if strings.TrimSpace(req.Avatar) != "" { org.Avatar = req.Avatar }
    if strings.TrimSpace(req.Color) != "" { org.Color = req.Color }
    if err := h.db.UpdateOrganization(org); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"organization": org})
}

// GET /api/orgs
func (h *OrgsHandler) ListMyOrganizations(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    orgs, err := h.db.ListUserOrganizations(user.ID)
    if err != nil {
        fmt.Printf("[error] ListMyOrganizations failed for user=%s: %v\n", user.ID, err)
        utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    // Compute weak ETag: orgs:<user>:<count>:<maxUpdated>
    var maxUpdated int64
    for _, o := range orgs {
        if ts := o.UpdatedAt.UnixMilli(); ts > maxUpdated { maxUpdated = ts }
    }
    etag := fmt.Sprintf("W/\"orgs:%s:%d:%d\"", user.ID, len(orgs), maxUpdated)
    ifNone := r.Header.Get("If-None-Match")
    w.Header().Set("ETag", etag)
    if ifNone == etag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "organizations": orgs })
}

// GET /api/orgs/{orgID}/members
func (h *OrgsHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
    orgID := r.URL.Query().Get("org_id")
    if orgID == "" { utils.WriteBadRequestResponse(w, "org_id required"); return }
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    if _, ok := h.requireOrgMember(w, user.ID, orgID); !ok { return }
    members, err := h.db.ListOrganizationMembers(orgID)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "members": members })
}

// POST /api/orgs/{orgID}/spaces
func (h *OrgsHandler) CreateSpace(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{ OrganizationID, Name, Description string; IsDefault bool }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if req.OrganizationID == "" || strings.TrimSpace(req.Name) == "" { utils.WriteBadRequestResponse(w, "org_id and name required"); return }
    // Authorization: only owner (或未来扩展 admin) 可创建空间
    role, ok := h.requireOrgMember(w, user.ID, req.OrganizationID)
    if !ok { return }
    if role != models.RoleOwner && role != models.RoleAdmin {
        utils.WriteForbiddenResponse(w, "Only owner/admin can create spaces")
        return
    }
    space := &models.Space{ OrganizationID: req.OrganizationID, Name: req.Name, Description: req.Description, IsDefault: req.IsDefault }
    if err := h.db.CreateSpace(space); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "space": space })
}

// GET /api/orgs/{orgID}/spaces
func (h *OrgsHandler) ListSpaces(w http.ResponseWriter, r *http.Request) {
    orgID := r.URL.Query().Get("org_id")
    if orgID == "" { utils.WriteBadRequestResponse(w, "org_id required"); return }
    // require membership to browse
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    if _, ok := h.requireOrgMember(w, user.ID, orgID); !ok { return }
    spaces, err := h.db.ListSpacesByOrganization(orgID)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    var maxUpdated int64
    for _, s := range spaces {
        if ts := s.UpdatedAt.UnixMilli(); ts > maxUpdated { maxUpdated = ts }
    }
    etag := fmt.Sprintf("W/\"spaces:%s:%d:%d\"", orgID, len(spaces), maxUpdated)
    ifNone := r.Header.Get("If-None-Match")
    w.Header().Set("ETag", etag)
    if ifNone == etag {
        w.WriteHeader(http.StatusNotModified)
        return
    }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "spaces": spaces })
}

// PUT /api/spaces/{spaceID}/permissions
func (h *OrgsHandler) SetSpacePermission(w http.ResponseWriter, r *http.Request) {
    var req struct{ SpaceID, UserID string; CanEdit bool }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if req.SpaceID == "" || req.UserID == "" { utils.WriteBadRequestResponse(w, "space_id and user_id required"); return }
    // Only the organization owner of the space's organization can set permissions
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    space, err := h.db.GetSpaceByID(req.SpaceID)
    if err != nil { utils.WriteNotFoundResponse(w, "space not found"); return }
    if !h.requireOwner(w, user.ID, space.OrganizationID) { return }
    if err := h.db.SetSpacePermission(req.SpaceID, req.UserID, req.CanEdit); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    perms, _ := h.db.GetSpacePermissions(req.SpaceID)
    utils.WriteSuccessResponse(w, map[string]interface{}{ "permissions": perms })
}

// PUT /api/orgs/spaces/{id}
func (h *OrgsHandler) UpdateSpace(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    spaceID := chiRoute.URLParam(r, "id")
    if strings.TrimSpace(spaceID) == "" { utils.WriteBadRequestResponse(w, "space id required"); return }
    space, err := h.db.GetSpaceByID(spaceID)
    if err != nil { utils.WriteNotFoundResponse(w, "space not found"); return }
    // owner/admin only
    role, ok := h.requireOrgMember(w, user.ID, space.OrganizationID)
    if !ok { return }
    if role != models.RoleOwner && role != models.RoleAdmin {
        utils.WriteForbiddenResponse(w, "Only owner/admin can update spaces")
        return
    }
    var req struct{ Name, Description string; IsDefault bool }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    space.Name = req.Name
    space.Description = req.Description
    space.IsDefault = req.IsDefault
    if err := h.db.UpdateSpace(space); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"space": space})
}

// DELETE /api/orgs/spaces/{id}
func (h *OrgsHandler) DeleteSpace(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    spaceID := chiRoute.URLParam(r, "id")
    if strings.TrimSpace(spaceID) == "" { utils.WriteBadRequestResponse(w, "space id required"); return }
    space, err := h.db.GetSpaceByID(spaceID)
    if err != nil { utils.WriteNotFoundResponse(w, "space not found"); return }
    // owner/admin only
    role, ok := h.requireOrgMember(w, user.ID, space.OrganizationID)
    if !ok { return }
    if role != models.RoleOwner && role != models.RoleAdmin {
        utils.WriteForbiddenResponse(w, "Only owner/admin can delete spaces")
        return
    }
    if err := h.db.DeleteSpace(spaceID); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{"deleted": true, "id": spaceID})
}

// POST /api/orgs/{orgID}/invite
func (h *OrgsHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{ OrganizationID string; Email string }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if req.OrganizationID == "" || req.Email == "" { utils.WriteBadRequestResponse(w, "org_id and email required"); return }
    // Only owner can invite
    if !h.requireOwner(w, user.ID, req.OrganizationID) { return }
    tok, err := utils.GenerateURLToken(24)
    if err != nil { utils.WriteInternalServerErrorResponse(w, "failed to generate token"); return }
    inv := &models.OrganizationInvitation{ OrganizationID: req.OrganizationID, Email: req.Email, InviterID: user.ID, Token: tok, Status: models.InvitationPending, ExpiresAt: time.Now().Add(14*24*time.Hour) }
    if err := h.db.CreateInvitation(inv); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "invitation": inv })
}

// GET /api/invitations/my
func (h *OrgsHandler) ListMyInvitations(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    invs, err := h.db.ListInvitationsByEmail(user.Email)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "invitations": invs })
}

// POST /api/invitations/accept
func (h *OrgsHandler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{ Token string }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if req.Token == "" { utils.WriteBadRequestResponse(w, "token required"); return }
    inv, err := h.db.GetInvitationByToken(req.Token)
    if err != nil { utils.WriteNotFoundResponse(w, "Invitation not found"); return }
    if inv.Status != models.InvitationPending || time.Now().After(inv.ExpiresAt) { utils.WriteBadRequestResponse(w, "Invitation invalid or expired"); return }

    // Add membership
    if err := h.db.AddOrganizationMember(&models.OrganizationMembership{ OrganizationID: inv.OrganizationID, UserID: user.ID, Role: models.RoleMember }); err != nil {
        utils.WriteInternalServerErrorResponse(w, "Failed to add membership: "+err.Error()); return
    }
    // Update invitation
    inv.Status = models.InvitationAccepted
    inv.AcceptedBy = &user.ID
    if err := h.db.UpdateInvitation(inv); err != nil { fmt.Printf("[warn] update invitation failed: %v\n", err) }

    utils.WriteSuccessResponse(w, map[string]interface{}{ "organization_id": inv.OrganizationID })
}
