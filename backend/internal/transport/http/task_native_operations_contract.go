package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

const maxNativeTaskBatchUpdate = 500

type nativeTaskCloneBody struct {
	TitleSuffix         *string `json:"title_suffix"`
	TargetRequirementID *uint   `json:"target_requirement_id"`
}

type nativeTaskBatchUpdates struct {
	Title            *string               `json:"title"`
	Description      *string               `json:"description"`
	Status           *string               `json:"status"`
	Priority         *string               `json:"priority"`
	AssigneeID       optionalValue[uint]   `json:"assignee_id"`
	ReporterUserID   optionalValue[uint]   `json:"reporter_user_id"`
	EstimatedMinutes optionalValue[int]    `json:"estimated_minutes"`
	StartsAt         optionalValue[string] `json:"starts_at"`
	DueAt            optionalValue[string] `json:"due_at"`
}

type nativeTaskBatchUpdateBody struct {
	IDs     []uint                 `json:"ids"`
	Updates nativeTaskBatchUpdates `json:"updates"`
	Atomic  *bool                  `json:"atomic"`
}

func (body nativeTaskBatchUpdates) taskBody() taskBody {
	return taskBody{
		Title: body.Title, Description: body.Description,
		Status: body.Status, Priority: body.Priority,
		AssigneeID: body.AssigneeID, ReporterUserID: body.ReporterUserID,
		EstimatedMinutes: body.EstimatedMinutes,
		StartsAt:         body.StartsAt, DueAt: body.DueAt,
	}
}

func (body nativeTaskBatchUpdates) empty() bool {
	return body.Title == nil && body.Description == nil && body.Status == nil && body.Priority == nil &&
		!body.AssigneeID.Set && !body.ReporterUserID.Set &&
		!body.EstimatedMinutes.Set && !body.StartsAt.Set && !body.DueAt.Set
}

func decodeStrictTaskNativeJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(io.LimitReader(c.Request.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func nativeTaskCloneSuffix(body nativeTaskCloneBody) string {
	if body.TitleSuffix == nil {
		return "（副本）"
	}
	return strings.TrimSpace(*body.TitleSuffix)
}
