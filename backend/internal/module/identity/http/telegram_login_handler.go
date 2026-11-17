package identityhttp

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
)

type TelegramLoginHandler struct {
	service *identity.TelegramLoginService
	account *identity.AccountService
}

func (handler *TelegramLoginHandler) WithAccountService(account *identity.AccountService) *TelegramLoginHandler {
	handler.account = account
	return handler
}

func NewTelegramLoginHandler(service *identity.TelegramLoginService) *TelegramLoginHandler {
	return &TelegramLoginHandler{service: service}
}

func (handler *TelegramLoginHandler) RegisterRoutes(core *gin.RouterGroup) {
	core.GET("/auth/telegram/start", handler.start)
	core.GET("/auth/telegram/callback", handler.callback)
}

func (handler *TelegramLoginHandler) start(c *gin.Context) {
	if handler == nil || handler.service == nil {
		telegramLoginFailure(c, identity.ErrLoginProviderUnavailable)
		return
	}
	location, err := handler.service.Start(c.Request.Context(), c.Query("return_to"))
	if err != nil {
		telegramLoginFailure(c, err)
		return
	}
	c.Redirect(http.StatusFound, location)
}

func (handler *TelegramLoginHandler) callback(c *gin.Context) {
	if handler == nil || handler.service == nil {
		telegramLoginFailure(c, identity.ErrLoginProviderUnavailable)
		return
	}
	code := c.Query("code")
	if strings.TrimSpace(c.Query("error")) != "" {
		code = ""
	}
	result, err := handler.service.Complete(c.Request.Context(), c.Query("state"), code)
	if err != nil {
		if errors.Is(err, identity.ErrInvalidLoginState) && handler.account != nil {
			returnTo, bindErr := handler.account.CompleteTelegramBind(c.Request.Context(), c.Query("state"), code)
			if bindErr == nil {
				redirectBindingResult(c, returnTo, identity.FlowTelegram, "bound")
				return
			}
			if returnTo != "" {
				redirectBindingResult(c, returnTo, identity.FlowTelegram, "failed")
				return
			}
		}
		telegramLoginFailure(c, err)
		return
	}
	location, err := url.Parse(identity.NormalizeReturnTo(result.ReturnTo))
	if err != nil {
		telegramLoginFailure(c, identity.ErrInvalidLoginState)
		return
	}
	query := location.Query()
	if result.ExternalRegistrationCode != "" {
		query.Set("external_registration_code", result.ExternalRegistrationCode)
		query.Set("login_provider", identity.FlowTelegram)
	} else {
		query.Set("login_code", result.LoginCode)
	}
	location.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, location.String())
}

func redirectBindingResult(c *gin.Context, returnTo, provider, result string) {
	location, _ := url.Parse(identity.NormalizeReturnTo(returnTo))
	query := location.Query()
	query.Set("identity_result", result)
	query.Set("login_provider", provider)
	location.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, location.String())
}

func telegramLoginFailure(c *gin.Context, err error) {
	switch {
	case errors.Is(err, identity.ErrLoginProviderUnavailable):
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"code": authUpstreamUnavailable, "error": "authentication provider is unavailable"})
	case errors.Is(err, identity.ErrLoginUserNotFound), errors.Is(err, identity.ErrLoginUserDisabled),
		errors.Is(err, identity.ErrLoginTelegramIdentityConflict), errors.Is(err, identity.ErrExternalRegistrationDisabled):
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": authLoginUserUnavailable, "error": "PM user is not provisioned for this login"})
	case errors.Is(err, identity.ErrInvalidLoginState), errors.Is(err, identity.ErrLoginTelegramIdentityInvalid):
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": authLoginCodeInvalid, "error": "invalid or expired login state or code"})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"code": authLoginFailed, "error": "could not finish Telegram login"})
	}
}
