package identityhttp

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

const (
	authLoginCodeInvalid     = "AUTH_LOGIN_CODE_INVALID"
	authLoginUserUnavailable = "AUTH_LOGIN_USER_UNAVAILABLE"
	authLoginFailed          = "AUTH_LOGIN_FAILED"
	authUpstreamUnavailable  = "AUTH_UPSTREAM_UNAVAILABLE"
)

type GoogleLoginHandler struct {
	service *identity.GoogleLoginService
	account *identity.AccountService
}

func (handler *GoogleLoginHandler) WithAccountService(account *identity.AccountService) *GoogleLoginHandler {
	handler.account = account
	return handler
}

func NewGoogleLoginHandler(service *identity.GoogleLoginService) *GoogleLoginHandler {
	return &GoogleLoginHandler{service: service}
}

// RegisterRoutes mounts public OAuth routes below the caller's core API group.
// Authentication middleware must not wrap these two browser entry points.
func (handler *GoogleLoginHandler) RegisterRoutes(core *gin.RouterGroup) {
	core.GET("/auth/google/start", handler.start)
	core.GET("/auth/google/callback", handler.callback)
}

func (handler *GoogleLoginHandler) start(c *gin.Context) {
	if handler == nil || handler.service == nil {
		googleLoginFailure(c, identity.ErrLoginProviderUnavailable)
		return
	}
	location, err := handler.service.Start(c.Request.Context(), c.Query("return_to"))
	if err != nil {
		googleLoginFailure(c, err)
		return
	}
	c.Redirect(http.StatusFound, location)
}

func (handler *GoogleLoginHandler) callback(c *gin.Context) {
	if handler == nil || handler.service == nil {
		googleLoginFailure(c, identity.ErrLoginProviderUnavailable)
		return
	}
	code := c.Query("code")
	if strings.TrimSpace(c.Query("error")) != "" {
		code = ""
	}
	result, err := handler.service.Complete(c.Request.Context(), c.Query("state"), code)
	if err != nil {
		if errors.Is(err, identity.ErrInvalidLoginState) && handler.account != nil {
			returnTo, bindErr := handler.account.CompleteGoogleBind(c.Request.Context(), c.Query("state"), code)
			if bindErr == nil {
				redirectBindingResult(c, returnTo, identity.FlowGoogle, "bound")
				return
			}
			if returnTo != "" {
				redirectBindingResult(c, returnTo, identity.FlowGoogle, "failed")
				return
			}
		}
		googleLoginFailure(c, err)
		return
	}
	location, err := url.Parse(identity.NormalizeReturnTo(result.ReturnTo))
	if err != nil {
		googleLoginFailure(c, identity.ErrInvalidLoginState)
		return
	}
	query := location.Query()
	if result.ExternalRegistrationCode != "" {
		query.Set("external_registration_code", result.ExternalRegistrationCode)
		query.Set("login_provider", identity.FlowGoogle)
	} else {
		query.Set("login_code", result.LoginCode)
	}
	location.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, location.String())
}

func googleLoginFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrLoginProviderUnavailable):
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": authUpstreamUnavailable, "error": "authentication provider is unavailable"})
	case errors.Is(err, identity.ErrLoginUserNotFound), errors.Is(err, identity.ErrLoginUserDisabled),
		errors.Is(err, identity.ErrLoginGoogleEmailUnverified), errors.Is(err, identity.ErrLoginGoogleEmailAmbiguous),
		errors.Is(err, identity.ErrLoginGoogleIdentityConflict), errors.Is(err, identity.ErrExternalRegistrationDisabled):
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": authLoginUserUnavailable, "error": "PM user is not provisioned for this login"})
	case errors.Is(err, identity.ErrInvalidLoginState), errors.Is(err, identity.ErrLoginGoogleIdentityInvalid):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": authLoginCodeInvalid, "error": "invalid or expired login state or code"})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": authLoginFailed, "error": "could not finish Google login"})
	}
}
