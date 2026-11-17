package feishu

import (
	"context"
	"fmt"
	"strconv"

	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
)

// OrgDepartment is one Feishu contact department projection.
type OrgDepartment struct {
	OpenID      string
	ParentID    string
	Name        string
	Order       int64
	MemberCount int
}

// OrgDepartmentUser is one Feishu user directly belonging to a department.
// DepartmentID is the user's primary department ("0"/empty at tenant root).
type OrgDepartmentUser struct {
	OpenID       string
	Name         string
	DepartmentID string
	JobTitle     string
}

// ListFeishuDepartments walks the whole department tree from the tenant root
// using fetch_child so one paginated call chain covers every department.
func (r *SDKReader) ListFeishuDepartments(ctx context.Context) ([]OrgDepartment, error) {
	if r == nil || r.client == nil {
		return nil, ErrFeishuWriterUnavailable
	}
	departments := []OrgDepartment{}
	pageToken := ""
	for {
		builder := larkcontact.NewChildrenDepartmentReqBuilder().
			DepartmentId("0").
			DepartmentIdType("open_department_id").
			FetchChild(true).
			PageSize(50)
		if pageToken != "" {
			builder = builder.PageToken(pageToken)
		}
		response, err := r.client.Contact.V3.Department.Children(ctx, builder.Build())
		if err != nil {
			return nil, fmt.Errorf("list Feishu departments: %w", err)
		}
		if response != nil && response.Code != 0 {
			return nil, fmt.Errorf("list Feishu departments: code %d: %s", response.Code, response.Msg)
		}
		if response == nil || response.Data == nil {
			break
		}
		for _, item := range response.Data.Items {
			if item.OpenDepartmentId == nil || item.Name == nil {
				continue
			}
			department := OrgDepartment{
				OpenID:   *item.OpenDepartmentId,
				ParentID: "0",
				Name:     *item.Name,
			}
			if item.ParentDepartmentId != nil {
				department.ParentID = *item.ParentDepartmentId
			}
			if item.Order != nil {
				if order, parseErr := strconv.ParseInt(*item.Order, 10, 64); parseErr == nil {
					department.Order = order
				}
			}
			if item.MemberCount != nil {
				department.MemberCount = *item.MemberCount
			}
			departments = append(departments, department)
		}
		if response.Data.HasMore == nil || !*response.Data.HasMore || response.Data.PageToken == nil {
			break
		}
		pageToken = *response.Data.PageToken
	}
	return departments, nil
}

// ListFeishuDepartmentUsers pages the users directly belonging to one
// department. Resigned accounts are skipped so the directory never maps a
// PM member to a departed Feishu profile.
func (r *SDKReader) ListFeishuDepartmentUsers(
	ctx context.Context,
	departmentID string,
) ([]OrgDepartmentUser, error) {
	if r == nil || r.client == nil {
		return nil, ErrFeishuWriterUnavailable
	}
	users := []OrgDepartmentUser{}
	pageToken := ""
	for {
		builder := larkcontact.NewFindByDepartmentUserReqBuilder().
			DepartmentId(departmentID).
			DepartmentIdType("open_department_id").
			PageSize(50)
		if pageToken != "" {
			builder = builder.PageToken(pageToken)
		}
		response, err := r.client.Contact.V3.User.FindByDepartment(ctx, builder.Build())
		if err != nil {
			return nil, fmt.Errorf("list Feishu department users: %w", err)
		}
		if response != nil && response.Code != 0 {
			return nil, fmt.Errorf("list Feishu department users: code %d: %s", response.Code, response.Msg)
		}
		if response == nil || response.Data == nil {
			break
		}
		for _, item := range response.Data.Items {
			if item.OpenId == nil || item.Name == nil {
				continue
			}
			user := OrgDepartmentUser{OpenID: *item.OpenId, Name: *item.Name}
			if len(item.DepartmentIds) > 0 {
				user.DepartmentID = item.DepartmentIds[0]
			}
			if item.JobTitle != nil {
				user.JobTitle = *item.JobTitle
			}
			users = append(users, user)
		}
		if response.Data.HasMore == nil || !*response.Data.HasMore || response.Data.PageToken == nil {
			break
		}
		pageToken = *response.Data.PageToken
	}
	return users, nil
}
