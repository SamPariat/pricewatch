package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
)

const SessionCookie = "pw_session"

// Session is a minimal stateless signed cookie — no session table, no JWT
// library. Payload is just an expiry timestamp; secret is
// config.SessionSecret. This is proportionate to PLAN.md's access model:
// single admin, one credential, one session at a time.
type Session struct {
	secret []byte
}

func NewSession(secret string) *Session {
	return &Session{secret: []byte(secret)}
}

// Issue returns a signed token good for ttl, to be set as a cookie value.
func (s *Session) Issue(ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	payload := make([]byte, 8)
	binary.BigEndian.PutUint64(payload, uint64(exp))
	sig := s.sign(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(sig)
}

// Valid reports whether token is a signature this Session issued and
// hasn't expired. Signature comparison is constant-time.
func (s *Session) Valid(token string) bool {
	payloadPart, sigPart, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(payloadPart)
	if err != nil || len(payload) != 8 {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigPart)
	if err != nil {
		return false
	}
	want := s.sign(payload)
	if subtle.ConstantTimeCompare(sig, want) != 1 {
		return false
	}
	exp := int64(binary.BigEndian.Uint64(payload))
	return time.Now().Unix() < exp
}

func (s *Session) sign(payload []byte) []byte {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

// RequireAuth rejects any request without a valid session cookie. The
// panel is internet-facing and guards a live Telegram bot token and every
// watch's configuration, so every /api route except /auth/login and the
// version/health checks sits behind this.
func RequireAuth(sess *Session) fiber.Handler {
	return func(c fiber.Ctx) error {
		token := c.Cookies(SessionCookie)
		if token == "" || !sess.Valid(token) {
			return fiber.NewError(fiber.StatusUnauthorized, "not authenticated")
		}
		return c.Next()
	}
}
