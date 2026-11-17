package api

import (
	"context"
	documentgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/document/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
	librarygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/notification"
	notificationgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/notification/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	accessgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
	"strconv"
	"strings"
)

// Only receipt-owned IDs reach this minimal projection. Detail and review
// authorization remain with their owners; no body or membership is exposed.
func inboxRepository(db *gorm.DB) *notificationgorm.MentionRepository {
	repo := notificationgorm.NewMentionRepository(db)
	repo.ConfigureInboxPresentation(func(ctx context.Context, workspaceID uint, items []notification.Mention) error {
		ids := []uint{}
		for _, item := range items {
			if item.RequestID > 0 {
				ids = append(ids, item.RequestID)
			}
		}
		requests, err := accessgorm.NewRepository(db).NotificationRequests(ctx, workspaceID, ids)
		if err != nil {
			return err
		}
		names, titles := map[uint]string{}, map[string]string{}
		directory := identitygorm.NewAccessGuideDirectory(db)
		for i := range items {
			item := &items[i]
			actor := item.ActorUserID
			if request, ok := requests[item.RequestID]; ok {
				item.RequestStatus = string(request.Status)
				if strings.HasSuffix(item.Scene, ".access_requested") {
					actor = request.ApplicantUserID
				}
				if item.ResourceType == "workspace" || item.ResourceType == "project_set" || item.ResourceType == "plan_unit" {
					item.GrantLevel = "member"
				} else {
					item.GrantLevel = string(request.GrantLevel)
				}
			}
			if _, ok := names[actor]; !ok {
				name, err := directory.GuideOwnerName(ctx, actor)
				if err != nil {
					return err
				}
				names[actor] = name
			}
			if names[actor] != "" {
				item.ActorName = names[actor]
			}
			key := item.ResourceType + ":" + item.ResourceID + ":" + strconv.FormatUint(uint64(item.DocumentID), 10)
			if _, ok := titles[key]; !ok {
				titles[key] = inboxResourceTitle(ctx, db, workspaceID, *item)
			}
			if titles[key] != "" {
				item.ResourceTitle = titles[key]
			}
		}
		return nil
	})
	return repo
}

func inboxResourceTitle(ctx context.Context, db *gorm.DB, workspaceID uint, item notification.Mention) string {
	id, _ := strconv.ParseUint(item.ResourceID, 10, 64)
	switch item.ResourceType {
	case "workspace":
		if uint(id) != workspaceID {
			return ""
		}
		title, _ := identitygorm.NewAccessGuideDirectory(db).GuideWorkspaceName(ctx, workspaceID)
		return title
	case "project_set":
		row, err := projectgorm.NewReadRepository(db).GetProjectSet(ctx, workspaceID, uint(id))
		if err == nil {
			return row.Name
		}
	case "plan_unit":
		parts := strings.Split(item.ResourceID, "/")
		if len(parts) != 2 {
			return ""
		}
		unitID, _ := strconv.ParseUint(parts[1], 10, 64)
		row, err := projectgorm.NewReadRepository(db).GetPlanUnit(ctx, workspaceID, uint(unitID))
		if err == nil && strconv.FormatUint(uint64(row.ProjectSetID), 10) == parts[0] {
			return row.Title
		}
	case "work_task":
		row, err := taskgorm.NewReadRepository(db).GetTask(ctx, workspaceID, task.Lookup{ID: uint(id)})
		if err == nil {
			return row.Title
		}
	case "quality_bug":
		row, err := qualitygorm.NewBugReadRepository(db).GetBug(ctx, workspaceID, uint(id))
		if err == nil {
			return row.Title
		}
	case "bitable", "rich_text":
		if item.ResourceID != "" {
			row, _, err := librarygorm.NewRepository(db).Get(ctx, workspaceID, documentlibrary.ResourceID(item.ResourceID))
			if err == nil {
				return row.Name
			}
		}
	}
	if item.DocumentID > 0 {
		repo := documentgorm.NewRepository(db)
		doc, err := repo.GetDocument(ctx, workspaceID, item.DocumentID)
		if err == nil {
			node, err := repo.GetNode(ctx, workspaceID, doc.NodeID)
			if err == nil {
				return node.Title
			}
		}
	}
	return ""
}
