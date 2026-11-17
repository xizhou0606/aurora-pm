package auth

import "fmt"

const PMSessionIssuer = "pm"

// Principal is the stable PM identity projection used by downstream access
// checks. Authorization never consumes role or workspace claims from a token.
type Principal struct {
	Issuer      string `json:"issuer"`
	Subject     string `json:"subject"`
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
}

func PrincipalFromIdentity(identity Identity) Principal {
	return Principal{
		Issuer: identity.Issuer, Subject: identity.Subject, UserID: identity.UserID,
		Username: identity.Username, DisplayName: identity.DisplayName, Email: identity.Email,
	}
}

func PMPrincipal(userID uint, username, displayName, email string) Principal {
	return Principal{
		Issuer: PMSessionIssuer, Subject: fmt.Sprintf("user:%d", userID), UserID: userID,
		Username: username, DisplayName: displayName, Email: email,
	}
}
