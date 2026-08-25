package service

import (
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/SamPariat/pricewatch/internal/httpapi/middleware"
)

// AuthService owns the admin credential check and session-token issuance
// — not cookie-writing itself, since that's an HTTP-transport concern
// that stays in the controller (the same split this refactor applies to
// the frontend's login flow).
type AuthService struct {
	AdminHash  string
	Session    *middleware.Session
	SessionTTL time.Duration
}

// Login compares password against the bcrypt admin hash and, on success,
// issues a signed session token good for SessionTTL. Always takes the
// same code path on failure so it doesn't distinguish "wrong password"
// from any other rejection at the network-timing level.
func (s *AuthService) Login(password string) (token string, ttl time.Duration, ok bool) {
	if bcrypt.CompareHashAndPassword([]byte(s.AdminHash), []byte(password)) != nil {
		return "", 0, false
	}
	return s.Session.Issue(s.SessionTTL), s.SessionTTL, true
}
