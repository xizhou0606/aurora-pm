package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

func (s *Server) listUsers(c *gin.Context) {
	limit, offset := pagination(c, 500)
	var ids []uint
	if _, exists := c.Request.URL.Query()["ids"]; exists {
		var err error
		ids, err = parseUintListQuery(c, "ids")
		if err != nil {
			bad(c, err.Error())
			return
		}
	}
	search, err := pmUserSearchFromQuery(c)
	if err != nil {
		bad(c, err.Error())
		return
	}
	users, err := s.listActivePMUsers(ids, limit, offset, search)
	if err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	ok(c, pmUserRows(users))
}

func (s *Server) listActivePMUsers(ids []uint, limit, offset int, search pmUserSearch) ([]identitygorm.User, error) {
	query := s.db.Model(&identitygorm.User{}).Where("users.active = ?", true)
	if len(ids) > 0 {
		query = query.Where("(users.id IN ? OR "+pmUserFactIDSQL+" IN ?)", ids, ids)
	}
	if username := strings.ToLower(strings.TrimSpace(search.Username)); username != "" {
		query = query.Where("LOWER(users.username) = ?", username)
	}
	if value := strings.ToLower(strings.TrimSpace(search.Query)); value != "" {
		if search.Exact {
			query = query.Where("LOWER(users.username) = ? OR LOWER(users.display_name) = ?", value, value)
		} else {
			like := escapedLikePattern(value)
			query = query.Where(
				"LOWER(users.username) LIKE ? ESCAPE '\\' OR LOWER(users.display_name) LIKE ? ESCAPE '\\'",
				like, like,
			)
		}
	}
	var users []identitygorm.User
	if err := query.Select("users.*").Order(pmUserFactIDSQL + " ASC").
		Limit(limit).Offset(offset).Find(&users).Error; err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		if _, err := pmUsersByReferenceID(users); err != nil {
			return nil, err
		}
	}
	return users, nil
}

type pmUserSearch struct {
	Query    string
	Username string
	Exact    bool
}

func pmUserSearchFromQuery(c *gin.Context) (pmUserSearch, error) {
	search := pmUserSearch{
		Query: strings.TrimSpace(c.Query("q")), Username: strings.TrimSpace(c.Query("username")),
	}
	if raw, exists := c.GetQuery("exact"); exists {
		exact, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return pmUserSearch{}, errors.New("exact must be true or false")
		}
		search.Exact = exact
	}
	return search, nil
}

func (s *Server) listTeams(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var teams []identitygorm.Team
	if s.db.Where("workspace_id = ?", workspace.ID).Order("id DESC").Find(&teams).Error != nil {
		dbFail(c)
		return
	}
	rows := make([]gin.H, 0, len(teams))
	for _, team := range teams {
		var members, projects int64
		memberQuery := s.db.Model(&identitygorm.TeamMember{}).
			Joins("JOIN users u ON u.id = team_members.user_id").
			Where("team_members.workspace_id = ? AND team_members.team_id = ? AND u.active = ?", workspace.ID, team.ID, true)
		memberQuery.Count(&members)
		s.db.Model(&projectgorm.ProjectSet{}).Where("workspace_id = ? AND team_id = ?", workspace.ID, team.ID).Count(&projects)
		rows = append(rows, gin.H{
			"id": team.ID, "workspace_id": team.WorkspaceID, "name": team.Name,
			"description": team.Description, "owner_user_id": team.OwnerUserID,
			"member_count": members, "project_count": projects,
		})
	}
	ok(c, rows)
}

func (s *Server) createTeam(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		OwnerUserID *uint  `json:"owner_user_id"`
	}
	if c.ShouldBindJSON(&body) != nil || strings.TrimSpace(body.Name) == "" {
		bad(c, "name required")
		return
	}
	var ownerFactID *uint
	var owner identitygorm.User
	if body.OwnerUserID != nil {
		var err error
		owner, err = s.activePMUserByReferenceID(*body.OwnerUserID)
		if err != nil {
			respondPMUserRegistryValidationError(c, err)
			return
		}
		ownerID := *body.OwnerUserID
		ownerFactID = &ownerID
	}
	team := identitygorm.Team{
		WorkspaceID: workspace.ID, Name: strings.TrimSpace(body.Name),
		Description: strings.TrimSpace(body.Description), OwnerUserID: ownerFactID,
	}
	teamCreateFailed := false
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&team).Error; err != nil {
			teamCreateFailed = true
			return err
		}
		if ownerFactID == nil {
			return nil
		}
		return tx.Create(&identitygorm.TeamMember{
			WorkspaceID: workspace.ID, TeamID: team.ID, UserID: owner.ID, Role: "admin",
		}).Error
	}); err != nil && teamCreateFailed {
		bad(c, "team could not be created")
		return
	} else if err != nil {
		dbFail(c)
		return
	}
	created(c, team)
}

func (s *Server) listTeamMembers(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	teamID, okID := paramID(c, "id")
	if !okID {
		return
	}
	if !s.teamInWorkspace(teamID, workspace.ID) {
		fail(c, http.StatusNotFound, "team not found")
		return
	}
	var members []identitygorm.TeamMember
	if s.db.Model(&identitygorm.TeamMember{}).
		Where("team_members.workspace_id = ? AND team_members.team_id = ?", workspace.ID, teamID).
		Select("team_members.*").Find(&members).Error != nil {
		dbFail(c)
		return
	}
	userIDs := make([]uint, 0, len(members))
	roles := make(map[uint]string, len(members))
	for _, member := range members {
		userIDs = append(userIDs, member.UserID)
		roles[member.UserID] = member.Role
	}
	users, err := s.findActivePMUsersByReferenceIDs(userIDs)
	if err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	rows := make([]gin.H, 0, len(members))
	for _, member := range members {
		user, exists := users[member.UserID]
		if !exists {
			continue
		}
		row := pmUserJSON(user)
		row["role"] = roles[member.UserID]
		rows = append(rows, row)
	}
	ok(c, rows)
}

func (s *Server) listCoreProjects(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	query := s.db.Where("workspace_id = ?", workspace.ID)
	if teamID := queryUint(c, "team_id"); teamID > 0 {
		if !s.teamInWorkspace(teamID, workspace.ID) {
			bad(c, "team_id does not belong to workspace")
			return
		}
		query = query.Where("team_id = ?", teamID)
	}
	// Project sets are project data: this endpoint must enforce the same
	// membership scope as /project-sets, not just the workspace boundary.
	if !s.applyProjectDataFilter(c, query, "project_sets", "id") {
		return
	}
	var projects []projectgorm.ProjectSet
	if query.Order("id DESC").Find(&projects).Error != nil {
		dbFail(c)
		return
	}
	ok(c, projects)
}

func (s *Server) teamInWorkspace(teamID, workspaceID uint) bool {
	var count int64
	s.db.Model(&identitygorm.Team{}).Where("id = ? AND workspace_id = ?", teamID, workspaceID).Count(&count)
	return count == 1
}

func pmUserJSON(user identitygorm.User) gin.H {
	id := factUserID(user)
	return gin.H{
		"id": id, "user_id": id, "pm_user_id": user.ID, "username": user.Username,
		"display_name": user.DisplayName, "email": user.Email, "avatar_url": user.AvatarURL,
		"role": user.Role, "platform_role": user.Role, "active": user.Active,
	}
}
