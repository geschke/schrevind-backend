package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/geschke/schrevind/config"
	"github.com/geschke/schrevind/pkg/cors"
	"github.com/geschke/schrevind/pkg/db"
	"github.com/geschke/schrevind/pkg/grrt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/sessions"
)

type GroupsController struct {
	DB          *db.DB
	Store       sessions.Store
	SessionName string
	G           *grrt.Grrt
}

// NewGroupsController constructs and returns a new instance.
func NewGroupsController(database *db.DB, store sessions.Store, sessionName string, g *grrt.Grrt) *GroupsController {
	return &GroupsController{
		DB:          database,
		Store:       store,
		SessionName: sessionName,
		G:           g,
	}
}

// Options handles the CORS preflight request.
func (ct GroupsController) Options(c *gin.Context) {
	_ = cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins)
}

type addGroupRequest struct {
	Name string `json:"Name"`
}

type updateGroupRequest struct {
	Name *string `json:"Name"`
}

type groupMemberAddRequest struct {
	ContextGroupID int64                `json:"ContextGroupID"`
	Members        []groupMemberAddItem `json:"members"`
}

type groupMemberAddItem struct {
	UserID int64  `json:"UserID"`
	Role   string `json:"Role"`
}

type groupMemberRemoveRequest struct {
	ContextGroupID int64   `json:"ContextGroupID"`
	UserIDs        []int64 `json:"UserIDs"`
}

// ensureAuthorized performs its package-specific operation.
func (ct GroupsController) ensureAuthorized(c *gin.Context) bool {
	if ct.DB == nil || ct.DB.SQL == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_NOT_INITIALIZED"})
		return false
	}
	if ct.Store == nil || strings.TrimSpace(ct.SessionName) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "AUTH_NOT_CONFIGURED"})
		return false
	}

	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil || sess.IsNew {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return false
	}
	if _, ok := sess.Values["id"]; !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return false
	}

	return true
}

// currentSessionUserID performs its package-specific operation.
func (ct GroupsController) currentSessionUserID(c *gin.Context) (int64, bool) {
	sess, _ := ct.Store.Get(c.Request, ct.SessionName)
	if sess == nil {
		return 0, false
	}
	raw, ok := sess.Values["id"]
	if !ok {
		return 0, false
	}
	id, ok := raw.(int64)
	if !ok {
		return 0, false
	}
	return id, true
}

// parseGroupID performs its package-specific operation.
func parseGroupID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return 0, false
	}
	return id, true
}

func (ct GroupsController) ensureMemberActionContext(c *gin.Context, sessionUserID, contextGroupID int64, action string) bool {
	if contextGroupID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_GROUP_ID"})
		return false
	}

	if contextGroupID == db.SystemGroupID {
		allowed, err := ct.G.Can(sessionUserID, action)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return false
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_CONTEXT_GROUP"})
			return false
		}
		return true
	}

	inGroup, err := ct.DB.IsUserInGroup(contextGroupID, sessionUserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return false
	}
	if !inGroup {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_CONTEXT_GROUP"})
		return false
	}

	return true
}

func uniquePositiveUserIDs(userIDs []int64) ([]int64, string) {
	seen := make(map[int64]struct{}, len(userIDs))
	out := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid <= 0 {
			return nil, "INVALID_USER_ID"
		}
		if _, ok := seen[uid]; ok {
			return nil, "DUPLICATE_USER_ID"
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out, ""
}

func (ct GroupsController) groupAdminCountAfterAdd(groupID int64, members []groupMemberAddItem) (int, error) {
	count, err := ct.DB.CountGroupAdminMemberships(groupID)
	if err != nil {
		return 0, err
	}

	for _, member := range members {
		current, found, err := ct.DB.GetMembership(db.EntityTypeGroup, groupID, member.UserID)
		if err != nil {
			return 0, err
		}
		currentAdmin := found && current.Role == db.RoleGroupAdmin
		nextAdmin := member.Role == db.RoleGroupAdmin

		switch {
		case currentAdmin && !nextAdmin:
			count--
		case !currentAdmin && nextAdmin:
			count++
		}
	}

	return count, nil
}

func (ct GroupsController) groupAdminCountAfterRemove(groupID int64, userIDs []int64) (int, error) {
	count, err := ct.DB.CountGroupAdminMemberships(groupID)
	if err != nil {
		return 0, err
	}

	for _, uid := range userIDs {
		current, found, err := ct.DB.GetMembership(db.EntityTypeGroup, groupID, uid)
		if err != nil {
			return 0, err
		}
		if found && current.Role == db.RoleGroupAdmin {
			count--
		}
	}

	return count, nil
}

// GET /api/groups/list
func (ct GroupsController) GetList(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.Can(sessionUserID, "group:list")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	items, err := ct.DB.ListGroups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   int64(len(items)),
		"items":   items,
	})
}

// GET /api/groups/:id
func (ct GroupsController) GetByID(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	groupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "group:view", groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	item, found, err := ct.DB.GetGroupByID(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "GROUP_NOT_FOUND"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/groups/add
func (ct GroupsController) PostAdd(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.Can(sessionUserID, "group:create")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	var req addGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_NAME"})
		return
	}

	item := db.Group{Name: name}
	if err := ct.DB.CreateGroupWithDefaultCurrenciesAndAdmin(&item, sessionUserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionCreate,
		EntityType: db.EntityTypeGroup,
		EntityID:   item.ID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    item,
	})
}

// POST /api/groups/update/:id
func (ct GroupsController) PostUpdate(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	groupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "group:edit", groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	existing, found, err := ct.DB.GetGroupByID(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "GROUP_NOT_FOUND"})
		return
	}

	var req updateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}

	if req.Name != nil {
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if existing.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_NAME"})
		return
	}

	if err := ct.DB.UpdateGroup(&existing); err != nil {
		// UpdateGroup returns an error for the system group.
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_SYSTEM_GROUP"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionUpdate,
		EntityType: db.EntityTypeGroup,
		EntityID:   groupID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"item":    existing,
	})
}

// POST /api/groups/delete/:id
func (ct GroupsController) PostDelete(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	groupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "group:delete", groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	if err := ct.DB.DeleteGroup(groupID); err != nil {
		// DeleteGroup returns an error for the system group.
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN_SYSTEM_GROUP"})
		return
	}

	_ = ct.DB.WriteAuditLog(&db.AuditLog{
		UserID:     sessionUserID,
		Action:     db.ActionDelete,
		EntityType: db.EntityTypeGroup,
		EntityID:   groupID,
	})

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "GROUP_DELETED",
	})
}

// GET /api/groups/:id/members
func (ct GroupsController) GetMembers(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	groupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "member:list", groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	items, err := ct.DB.ListGroupMembersByGroupID(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   int64(len(items)),
		"items":   items,
	})
}

// POST /api/groups/:id/members/add
func (ct GroupsController) PostMemberAdd(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	targetGroupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req groupMemberAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if len(req.Members) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_MEMBERS"})
		return
	}

	if !ct.ensureMemberActionContext(c, sessionUserID, req.ContextGroupID, "member:add") {
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "member:add", targetGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	seen := make(map[int64]struct{}, len(req.Members))
	for i := range req.Members {
		req.Members[i].Role = strings.TrimSpace(req.Members[i].Role)
		if req.Members[i].UserID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_USER_ID"})
			return
		}
		if _, ok := seen[req.Members[i].UserID]; ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "DUPLICATE_USER_ID"})
			return
		}
		seen[req.Members[i].UserID] = struct{}{}
		if req.Members[i].Role != "" && !db.IsValidGroupRole(req.Members[i].Role) {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_ROLE"})
			return
		}
	}

	finalAdminCount, err := ct.groupAdminCountAfterAdd(targetGroupID, req.Members)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if finalAdminCount <= 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_GROUP_ADMIN"})
		return
	}

	var addedCount, skippedCount int64
	for _, member := range req.Members {
		if member.Role != db.RoleGroupAdmin {
			continue
		}
		added, err := ct.DB.AddGroupMember(targetGroupID, member.UserID, member.Role)
		if err != nil {
			if errors.Is(err, db.ErrLastGroupAdmin) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_GROUP_ADMIN"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if added {
			addedCount++
			_ = ct.DB.WriteAuditLog(&db.AuditLog{
				UserID:     sessionUserID,
				Action:     db.ActionGrant,
				EntityType: db.EntityTypeGroup,
				EntityID:   targetGroupID,
				Detail:     strings.Join([]string{strconv.FormatInt(member.UserID, 10), member.Role}, ":"),
			})
		} else {
			skippedCount++
		}
	}
	for _, member := range req.Members {
		if member.Role == db.RoleGroupAdmin {
			continue
		}
		added, err := ct.DB.AddGroupMember(targetGroupID, member.UserID, member.Role)
		if err != nil {
			if errors.Is(err, db.ErrLastGroupAdmin) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_GROUP_ADMIN"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if added {
			addedCount++
			_ = ct.DB.WriteAuditLog(&db.AuditLog{
				UserID:     sessionUserID,
				Action:     db.ActionGrant,
				EntityType: db.EntityTypeGroup,
				EntityID:   targetGroupID,
				Detail:     strconv.FormatInt(member.UserID, 10),
			})
		} else {
			skippedCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"added":   addedCount,
		"skipped": skippedCount,
	})
}

// POST /api/groups/:id/members/remove
func (ct GroupsController) PostMemberRemove(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	targetGroupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	var req groupMemberRemoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "INVALID_JSON"})
		return
	}
	if len(req.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "MISSING_USER_IDS"})
		return
	}

	if !ct.ensureMemberActionContext(c, sessionUserID, req.ContextGroupID, "member:remove") {
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "member:remove", targetGroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	userIDs, message := uniquePositiveUserIDs(req.UserIDs)
	if message != "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": message})
		return
	}

	finalAdminCount, err := ct.groupAdminCountAfterRemove(targetGroupID, userIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if finalAdminCount <= 0 {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_GROUP_ADMIN"})
		return
	}

	var removedCount, notFoundCount int64
	for _, uid := range userIDs {
		removed, err := ct.DB.RemoveGroupMember(targetGroupID, uid)
		if err != nil {
			if errors.Is(err, db.ErrLastGroupAdmin) {
				c.JSON(http.StatusConflict, gin.H{"success": false, "message": "LAST_GROUP_ADMIN"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
			return
		}
		if removed {
			removedCount++
			_ = ct.DB.WriteAuditLog(&db.AuditLog{
				UserID:     sessionUserID,
				Action:     db.ActionRevoke,
				EntityType: db.EntityTypeGroup,
				EntityID:   targetGroupID,
				Detail:     strconv.FormatInt(uid, 10),
			})
		} else {
			notFoundCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"removed":   removedCount,
		"not_found": notFoundCount,
	})
}

// GET /api/groups/:id/depots
func (ct GroupsController) GetDepots(c *gin.Context) {
	if !cors.ApplyCORS(c, config.Cfg.WebUI.CORSAllowedOrigins) {
		return
	}
	if !ct.ensureAuthorized(c) {
		return
	}

	groupID, ok := parseGroupID(c)
	if !ok {
		return
	}

	sessionUserID, ok := ct.currentSessionUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "UNAUTHORIZED"})
		return
	}

	allowed, err := ct.G.CanDo(sessionUserID, db.EntityTypeGroup, "depot:list", groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "FORBIDDEN"})
		return
	}

	items, err := ct.DB.ListDepotsByGroupID(groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "DB_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"count":   int64(len(items)),
		"items":   items,
	})
}
