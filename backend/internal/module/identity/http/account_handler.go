package identityhttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type AccountHandler struct {
	service       *identity.AccountService
	passwordReset *identity.PasswordResetService
}

func NewAccountHandler(service *identity.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

func (handler *AccountHandler) SetPasswordResetService(service *identity.PasswordResetService) {
	handler.passwordReset = service
}

func (handler *AccountHandler) RegisterPublicRoutes(core *gin.RouterGroup) {
	core.POST("/auth/external-registration/complete", handler.completeRegistration)
	core.POST("/auth/password-reset/complete", handler.completePasswordReset)
}

func (handler *AccountHandler) completePasswordReset(c *gin.Context) {
	if handler.passwordReset == nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": "PASSWORD_RESET_UNAVAILABLE", "error": "password reset is unavailable"})
		return
	}
	var body struct {
		Token        string `json:"token"`
		Password     string `json:"new_password"`
		Confirmation string `json:"new_password_confirmation"`
	}
	if c.ShouldBindJSON(&body) != nil {
		accountError(c, identity.ErrPasswordResetInvalid)
		return
	}
	if err := handler.passwordReset.Complete(c.Request.Context(), body.Token, body.Password, body.Confirmation); err != nil {
		accountError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AccountHandler) RegisterAuthenticatedRoutes(group *gin.RouterGroup) {
	group.GET("/auth/me/account", handler.account)
	group.PATCH("/auth/me/profile", handler.profile)
	group.PUT("/auth/me/password", handler.password)
	group.POST("/auth/me/identities/:provider/bind/start", handler.bindStart)
	group.DELETE("/auth/me/identities/:provider", handler.unbind)
}

func (handler *AccountHandler) bindStart(c *gin.Context) {
	actorID, ok := accountActorID(c)
	if !ok {
		return
	}
	var body struct {
		ReturnTo string `json:"return_to"`
	}
	if c.ShouldBindJSON(&body) != nil {
		accountError(c, identity.ErrIdentityProviderInvalid)
		return
	}
	location, err := handler.service.StartBind(c.Request.Context(), actorID, c.Param("provider"), body.ReturnTo)
	if err != nil {
		accountError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"authorization_url": location}})
}

func (handler *AccountHandler) completeRegistration(c *gin.Context) {
	var body struct {
		RegistrationCode     string `json:"registration_code"`
		Username             string `json:"username"`
		DisplayName          string `json:"display_name"`
		Password             string `json:"password"`
		PasswordConfirmation string `json:"password_confirmation"`
	}
	if c.ShouldBindJSON(&body) != nil {
		accountError(c, identity.ErrExternalRegistrationInvalid)
		return
	}
	code, err := handler.service.CompleteRegistration(
		c.Request.Context(), body.RegistrationCode, body.Username, body.DisplayName,
		body.Password, body.PasswordConfirmation,
	)
	if err != nil {
		accountError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"login_code": code}})
}

func (handler *AccountHandler) account(c *gin.Context) {
	actorID, ok := accountActorID(c)
	if !ok {
		return
	}
	account, err := handler.service.Account(c.Request.Context(), actorID)
	if err != nil {
		accountError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"user":                  account.User,
		"display_name_editable": account.DisplayNameEditable, "password_configured": account.PasswordConfigured,
		"identities":    account.Identities,
		"email_binding": account.EmailBinding,
	}})
}

func (handler *AccountHandler) profile(c *gin.Context) {
	actorID, ok := accountActorID(c)
	if !ok {
		return
	}
	var body struct {
		DisplayName string `json:"display_name"`
	}
	if c.ShouldBindJSON(&body) != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"code": "IDENTITY_PROFILE_INVALID", "error": "invalid profile"})
		return
	}
	user, err := handler.service.UpdateProfile(c.Request.Context(), actorID, body.DisplayName)
	if err != nil {
		accountError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"user": user}})
}

func (handler *AccountHandler) password(c *gin.Context) {
	actorID, ok := accountActorID(c)
	if !ok {
		return
	}
	var body struct {
		Current      string `json:"current_password"`
		Next         string `json:"new_password"`
		Confirmation string `json:"new_password_confirmation"`
	}
	if c.ShouldBindJSON(&body) != nil {
		accountError(c, identity.ErrIdentityPasswordInvalid)
		return
	}
	if err := handler.service.UpdatePassword(c.Request.Context(), actorID, body.Current, body.Next, body.Confirmation); err != nil {
		accountError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (handler *AccountHandler) unbind(c *gin.Context) {
	actorID, ok := accountActorID(c)
	if !ok {
		return
	}
	if err := handler.service.Unbind(c.Request.Context(), actorID, c.Param("provider")); err != nil {
		accountError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func accountActorID(c *gin.Context) (uint, bool) {
	value, exists := c.Get(pmauth.PrincipalContextKey)
	principal, ok := value.(pmauth.Principal)
	if !exists || !ok || principal.UserID == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "IDENTITY_SESSION_REQUIRED", "error": "authentication required"})
		return 0, false
	}
	return principal.UserID, true
}

func accountError(c *gin.Context, err error) {
	status, code, message := http.StatusInternalServerError, "IDENTITY_OPERATION_FAILED", "identity operation failed"
	switch {
	case errors.Is(err, identity.ErrExternalRegistrationDisabled):
		status, code, message = http.StatusForbidden, "EXTERNAL_REGISTRATION_DISABLED", err.Error()
	case errors.Is(err, identity.ErrExternalRegistrationInvalid):
		status, code, message = http.StatusUnauthorized, "EXTERNAL_REGISTRATION_INVALID", err.Error()
	case errors.Is(err, identity.ErrIdentityUsernameInvalid), errors.Is(err, identity.ErrIdentityPasswordInvalid), errors.Is(err, identity.ErrIdentityPasswordMismatch), errors.Is(err, identity.ErrAccessNameInvalid):
		status, code, message = http.StatusBadRequest, "IDENTITY_VALIDATION_FAILED", err.Error()
	case errors.Is(err, identity.ErrIdentityUsernameConflict), errors.Is(err, identity.ErrLoginGoogleIdentityConflict), errors.Is(err, identity.ErrLoginTelegramIdentityConflict):
		status, code, message = http.StatusConflict, "IDENTITY_CONFLICT", err.Error()
	case errors.Is(err, identity.ErrIdentityCurrentPassword):
		status, code, message = http.StatusForbidden, "IDENTITY_CURRENT_PASSWORD_INVALID", err.Error()
	case errors.Is(err, identity.ErrIdentityDisplayNameReadOnly):
		status, code, message = http.StatusConflict, "IDENTITY_PROFILE_READ_ONLY", err.Error()
	case errors.Is(err, identity.ErrIdentityLoginMethodRequired):
		status, code, message = http.StatusConflict, "IDENTITY_LOGIN_METHOD_REQUIRED", err.Error()
	case errors.Is(err, identity.ErrIdentityProviderInvalid):
		status, code, message = http.StatusBadRequest, "IDENTITY_PROVIDER_INVALID", err.Error()
	case errors.Is(err, identity.ErrLoginProviderUnavailable):
		status, code, message = http.StatusServiceUnavailable, "AUTH_UPSTREAM_UNAVAILABLE", "authentication provider is unavailable"
	case errors.Is(err, identity.ErrLoginUserNotFound):
		status, code, message = http.StatusNotFound, "IDENTITY_NOT_FOUND", err.Error()
	case errors.Is(err, identity.ErrPasswordResetInvalid):
		status, code, message = http.StatusUnauthorized, "PASSWORD_RESET_INVALID", err.Error()
	}
	c.AbortWithStatusJSON(status, gin.H{"code": code, "error": strings.TrimSpace(message)})
}
