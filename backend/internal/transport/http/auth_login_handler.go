package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishulogin"
	"github.com/xizhou0606/aurora-pm/backend/internal/integration/saassso"
	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	pmauth "github.com/xizhou0606/aurora-pm/backend/internal/platform/auth"
)

type sessionExchangeBody struct {
	LoginCode string `json:"login_code"`
}

func (s *Server) feishuStart(c *gin.Context) {
	if s.loginInitErr != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "login provider is misconfigured")
		return
	}
	if s.feishuLogin == nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "login provider is misconfigured")
		return
	}
	state, err := s.loginService.Start(c.Request.Context(), identity.FlowFeishu, c.Query("return_to"))
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not start Feishu login")
		return
	}
	redirect, err := s.feishuLogin.AuthorizationURL(state)
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not start Feishu login")
		return
	}
	c.Redirect(http.StatusFound, redirect)
}

func (s *Server) feishuCallback(c *gin.Context) {
	if s.loginInitErr != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "login provider is misconfigured")
		return
	}
	if s.feishuLogin == nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "login provider is misconfigured")
		return
	}
	returnTo, err := s.loginService.ConsumeState(c.Request.Context(), identity.FlowFeishu, c.Query("state"))
	if err != nil {
		loginInvalid(c)
		return
	}
	if strings.TrimSpace(c.Query("error")) != "" || strings.TrimSpace(c.Query("code")) == "" {
		loginInvalid(c)
		return
	}
	providerIdentity, err := s.feishuLogin.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		loginProviderError(c, err)
		return
	}
	user, err := s.loginService.ResolveOrProvisionFeishuUser(c.Request.Context(), identity.FeishuIdentity{
		TenantKey:   providerIdentity.TenantKey,
		OpenID:      providerIdentity.OpenID,
		DisplayName: providerIdentity.DisplayName,
		AvatarURL:   providerIdentity.AvatarURL,
	})
	if err != nil {
		loginUserError(c, err)
		return
	}
	loginCode, err := s.loginService.IssueCode(c.Request.Context(), identity.FlowFeishu, user.ID)
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not finish Feishu login")
		return
	}
	redirectLoginCode(c, returnTo, loginCode)
}

func (s *Server) ssoStart(c *gin.Context) {
	if s.loginInitErr != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "login provider is misconfigured")
		return
	}
	if !s.cfg.SSO.Enabled || s.ssoLogin == nil {
		loginNotConfigured(c)
		return
	}
	state, err := s.loginService.Start(c.Request.Context(), identity.FlowSSO, c.Query("return_to"))
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not start SSO login")
		return
	}
	redirect, err := s.ssoLogin.AuthorizationURL(state)
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not start SSO login")
		return
	}
	c.Redirect(http.StatusFound, redirect)
}

func (s *Server) ssoCallback(c *gin.Context) {
	if s.loginInitErr != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "login provider is misconfigured")
		return
	}
	if !s.cfg.SSO.Enabled || s.ssoLogin == nil {
		loginNotConfigured(c)
		return
	}
	returnTo, err := s.loginService.ConsumeState(c.Request.Context(), identity.FlowSSO, c.Query("state"))
	if err != nil {
		loginInvalid(c)
		return
	}
	if strings.TrimSpace(c.Query("error")) != "" || strings.TrimSpace(c.Query("code")) == "" {
		loginInvalid(c)
		return
	}
	providerIdentity, err := s.ssoLogin.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		loginProviderError(c, err)
		return
	}
	user, err := s.loginService.ResolveSSOUser(c.Request.Context(), providerIdentity.Issuer, providerIdentity.Subject)
	if err != nil {
		loginUserError(c, err)
		return
	}
	loginCode, err := s.loginService.IssueCode(c.Request.Context(), identity.FlowSSO, user.ID)
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not finish SSO login")
		return
	}
	redirectLoginCode(c, returnTo, loginCode)
}

func (s *Server) exchangeSession(c *gin.Context) {
	var body sessionExchangeBody
	if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.LoginCode) == "" {
		loginInvalid(c)
		return
	}
	user, _, err := s.loginService.ExchangeCode(c.Request.Context(), body.LoginCode)
	if err != nil {
		loginInvalid(c)
		return
	}
	token, err := s.tokens.SignSession(user.ID, user.Username, user.DisplayName, user.Email)
	if err != nil {
		failCode(c, http.StatusInternalServerError, apiErrorAuthLoginFailed, "could not sign PM session")
		return
	}
	ttl := s.cfg.TokenTTL
	if ttl <= 0 {
		ttl = 12 * time.Hour
	}
	principal := pmauth.PMPrincipal(user.ID, user.Username, user.DisplayName, user.Email)
	ok(c, gin.H{
		"access_token": token,
		"token":        token,
		"expires_at":   time.Now().UTC().Add(ttl),
		"principal":    principal,
		"user":         user,
	})
}

func redirectLoginCode(c *gin.Context, returnTo, loginCode string) {
	parsed, err := url.Parse(identity.NormalizeReturnTo(returnTo))
	if err != nil {
		loginInvalid(c)
		return
	}
	query := parsed.Query()
	query.Set("login_code", loginCode)
	parsed.RawQuery = query.Encode()
	c.Redirect(http.StatusFound, parsed.String())
}

func loginNotConfigured(c *gin.Context) {
	failCode(c, http.StatusNotFound, apiErrorAuthLoginNotConfigured, "login method is not configured")
}

func loginInvalid(c *gin.Context) {
	failCode(c, http.StatusUnauthorized, apiErrorAuthLoginCodeInvalid, "invalid or expired login state or code")
}

func loginProviderError(c *gin.Context, err error) {
	if errors.Is(err, feishulogin.ErrUnavailable) || errors.Is(err, saassso.ErrUnavailable) {
		failCode(c, http.StatusServiceUnavailable, apiErrorAuthUpstreamUnavailable, "authentication provider is unavailable")
		return
	}
	loginInvalid(c)
}

func loginUserError(c *gin.Context, err error) {
	if errors.Is(err, identity.ErrLoginTenantNotRegistered) || errors.Is(err, identity.ErrLoginTenantDisabled) {
		failCode(c, http.StatusForbidden, apiErrorAuthLoginTenantUnavailable, "organization is not registered for PM login")
		return
	}
	if errors.Is(err, identity.ErrLoginUserDisabled) {
		failCode(c, http.StatusForbidden, apiErrorAuthLoginUserUnavailable, "PM user is disabled")
		return
	}
	failCode(c, http.StatusForbidden, apiErrorAuthLoginUserUnavailable, "PM user is not provisioned for this login")
}
