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
)

type OrgsHandler struct {
    config *config.Config
    db     database.DatabaseInterface
}

func NewOrgsHandler(cfg *config.Config, db database.DatabaseInterface) *OrgsHandler {
    return &OrgsHandler{config: cfg, db: db}
}

// POST /api/orgs
func (h *OrgsHandler) CreateOrganization(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{
        Name string `json:"name"`
        Description string `json:"description"`
        Avatar string `json:"avatar"`
        DefaultSpaces []struct{ Name, Description string; IsDefault bool } `json:"default_spaces"`
        InviteEmails []string `json:"invite_emails"`
    }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if strings.TrimSpace(req.Name) == "" { utils.WriteBadRequestResponse(w, "Name required"); return }

    org := &models.Organization{ Name: req.Name, Description: req.Description, Avatar: req.Avatar, OwnerID: user.ID }
    if err := h.db.CreateOrganization(org); err != nil { utils.WriteInternalServerErrorResponse(w, "Create org failed: "+err.Error()); return }

    // Create optional default spaces
    for _, s := range req.DefaultSpaces {
        _ = h.db.CreateSpace(&models.Space{ OrganizationID: org.ID, Name: s.Name, Description: s.Description, IsDefault: s.IsDefault })
    }

    // Create invitations async-like (no email send here)
    for _, email := range req.InviteEmails {
        email = strings.TrimSpace(email)
        if email == "" { continue }
        inv := &models.OrganizationInvitation{ OrganizationID: org.ID, Email: email, InviterID: user.ID, Status: models.InvitationPending, ExpiresAt: time.Now().Add(14*24*time.Hour) }
        if err := h.db.CreateInvitation(inv); err != nil { fmt.Printf("[warn] failed to create invitation for %s: %v\n", email, err) }
    }

    utils.WriteSuccessResponse(w, map[string]interface{}{ "organization": org })
}

// GET /api/orgs
func (h *OrgsHandler) ListMyOrganizations(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    orgs, err := h.db.ListUserOrganizations(user.ID)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "organizations": orgs })
}

// GET /api/orgs/{orgID}/members
func (h *OrgsHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
    // Simple listing; authorization checks omitted for brevity
    orgID := r.URL.Query().Get("org_id")
    if orgID == "" { utils.WriteBadRequestResponse(w, "org_id required"); return }
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
    // Minimal authorization: user must belong to org
    orgs, _ := h.db.ListUserOrganizations(user.ID)
    allowed := false
    for _, o := range orgs { if o.ID == req.OrganizationID { allowed = true; break } }
    if !allowed { utils.WriteUnauthorizedResponse(w, "Not a member of organization"); return }
    space := &models.Space{ OrganizationID: req.OrganizationID, Name: req.Name, Description: req.Description, IsDefault: req.IsDefault }
    if err := h.db.CreateSpace(space); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "space": space })
}

// GET /api/orgs/{orgID}/spaces
func (h *OrgsHandler) ListSpaces(w http.ResponseWriter, r *http.Request) {
    orgID := r.URL.Query().Get("org_id")
    if orgID == "" { utils.WriteBadRequestResponse(w, "org_id required"); return }
    spaces, err := h.db.ListSpacesByOrganization(orgID)
    if err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    utils.WriteSuccessResponse(w, map[string]interface{}{ "spaces": spaces })
}

// PUT /api/spaces/{spaceID}/permissions
func (h *OrgsHandler) SetSpacePermission(w http.ResponseWriter, r *http.Request) {
    var req struct{ SpaceID, UserID string; CanEdit bool }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if req.SpaceID == "" || req.UserID == "" { utils.WriteBadRequestResponse(w, "space_id and user_id required"); return }
    if err := h.db.SetSpacePermission(req.SpaceID, req.UserID, req.CanEdit); err != nil { utils.WriteInternalServerErrorResponse(w, err.Error()); return }
    perms, _ := h.db.GetSpacePermissions(req.SpaceID)
    utils.WriteSuccessResponse(w, map[string]interface{}{ "permissions": perms })
}

// POST /api/orgs/{orgID}/invite
func (h *OrgsHandler) InviteMember(w http.ResponseWriter, r *http.Request) {
    user, err := middleware.RequireUser(r.Context())
    if err != nil { utils.WriteUnauthorizedResponse(w, "Authentication required"); return }
    var req struct{ OrganizationID string; Email string }
    if err := utils.ParseJSONBody(r, &req); err != nil { utils.WriteBadRequestResponse(w, "Invalid body"); return }
    if req.OrganizationID == "" || req.Email == "" { utils.WriteBadRequestResponse(w, "org_id and email required"); return }
    inv := &models.OrganizationInvitation{ OrganizationID: req.OrganizationID, Email: req.Email, InviterID: user.ID, Status: models.InvitationPending, ExpiresAt: time.Now().Add(14*24*time.Hour) }
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

