package documenthttp

import (
	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"log/slog"
)

func (handler *Handler) ConfigureUserDisplays(reader identity.UserDisplayReader) {
	handler.displays = reader
}

func writeData(c *gin.Context, status int, data any) { c.JSON(status, gin.H{"data": data}) }

// Called only after the service has authorized the returned resource. Profiles
// describe historical authors; they do not confer membership or edit access.
func (handler *Handler) writeData(c *gin.Context, status int, data any) {
	refs := map[uint][]**identity.UserDisplay{}
	node := func(n *document.Node) { refs[n.CreatedBy] = append(refs[n.CreatedBy], &n.Creator) }
	comment := func(v *document.Comment) { refs[v.AuthorUserID] = append(refs[v.AuthorUserID], &v.Author) }
	thread := func(v *document.CommentThread) {
		for i := range v.Comments {
			comment(&v.Comments[i])
		}
	}
	switch v := data.(type) {
	case document.Tree:
		for i := range v.Nodes {
			node(&v.Nodes[i])
		}
		data = &v
	case document.Node:
		node(&v)
		data = &v
	case document.CreateNodeResult:
		node(&v.Node)
		data = &v
	case document.CommentThreadPage:
		for i := range v.Items {
			thread(&v.Items[i])
		}
		data = &v
	case document.CommentThread:
		thread(&v)
		data = &v
	case document.Comment:
		comment(&v)
		data = &v
	}
	if handler.displays != nil && len(refs) > 0 {
		ids := make([]uint, 0, len(refs))
		for id := range refs {
			if id > 0 {
				ids = append(ids, id)
			}
		}
		profiles, err := handler.displays.ReadUserDisplays(c.Request.Context(), ids)
		if err != nil {
			slog.WarnContext(c.Request.Context(), "document author profile lookup failed")
			writeData(c, status, data)
			return
		}
		for id, targets := range refs {
			if profile, ok := profiles[id]; ok {
				for _, target := range targets {
					*target = &profile
				}
			}
		}
	}
	writeData(c, status, data)
}
