package identityhttp

type permissionTreeNode struct {
	Code         string                      `json:"code"`
	Name         string                      `json:"name"`
	Type         string                      `json:"type"`
	Grantable    bool                        `json:"grantable"`
	Tools        []string                    `json:"tools,omitempty"`
	ObjectPolicy *permissionTreeObjectPolicy `json:"object_policy,omitempty"`
	Children     []permissionTreeNode        `json:"children,omitempty"`
}

type permissionTreeObjectPolicy struct {
	Code string `json:"code"`
	Kind string `json:"kind"`
}

type createRoleRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

type patchRoleRequest struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

type createServiceInterfaceRequest struct {
	Code          string `json:"code"`
	ActionCode    string `json:"action_code"`
	HTTPMethod    string `json:"http_method"`
	RouteTemplate string `json:"route_template"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Enabled       *bool  `json:"enabled"`
}

type bulkCreateServiceInterfaceRequest struct {
	Items []createServiceInterfaceRequest `json:"items"`
}

type patchServiceInterfaceRequest struct {
	Code          *string `json:"code"`
	ActionCode    *string `json:"action_code"`
	HTTPMethod    *string `json:"http_method"`
	RouteTemplate *string `json:"route_template"`
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Enabled       *bool   `json:"enabled"`
}

type replaceIDsRequest struct {
	RoleIDs             *[]uint `json:"role_ids"`
	MenuIDs             *[]uint `json:"menu_ids"`
	ServiceInterfaceIDs *[]uint `json:"service_interface_ids"`
}

type replaceActionCodesRequest struct {
	ActionCodes *[]string `json:"action_codes"`
}

type replaceUserAccessRequest struct {
	GroupID uint   `json:"group_id"`
	RoleIDs []uint `json:"role_ids"`
}

type createUserGroupRequest struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

type patchUserGroupRequest struct {
	Code        *string `json:"code"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Enabled     *bool   `json:"enabled"`
}

type replaceUserGroupRequest struct {
	GroupID uint `json:"group_id"`
}

type replaceUserStatusRequest struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type serviceInterfaceCandidateResponse struct {
	HTTPMethod           string `json:"http_method"`
	RouteTemplate        string `json:"route_template"`
	Registered           bool   `json:"registered"`
	ServiceInterfaceID   uint   `json:"service_interface_id,omitempty"`
	ServiceInterfaceCode string `json:"service_interface_code,omitempty"`
	ActionCode           string `json:"action_code,omitempty"`
}
