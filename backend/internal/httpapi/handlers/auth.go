package handlers

import (
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"

	"github.com/sampariat/prices-reminder/internal/httpapi/middleware"
)

type loginRequest struct {
	Password string `json:"password"`
}

// Login godoc
// @Summary      Log in
// @Description  The only unauthenticated /api route besides /meta and the health checks. Compares against a bcrypt hash rather than a plain password (PLAN.md Prerequisites — ADMIN_PASSWORD_HASH), and always takes the same code path on failure so it doesn't distinguish "wrong password" from any other rejection at the network-timing level. On success, sets an httpOnly session cookie — see internal/httpapi/doc.go's CookieAuth security definition.
// @Tags         auth
// @Accept       json
// @Param        body  body  loginRequest  true  "Admin password"
// @Success      204   "no content — session cookie set"
// @Failure      400   {string}  string  "invalid request body"
// @Failure      401   {string}  string  "invalid password"
// @Router       /auth/login [post]
func (a *API) Login(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	if bcrypt.CompareHashAndPassword([]byte(a.AdminHash), []byte(req.Password)) != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "invalid password")
	}

	token := a.Session.Issue(a.SessionTTL)
	c.Cookie(&fiber.Cookie{
		Name:     middleware.SessionCookie,
		Value:    token,
		HTTPOnly: true,
		Secure:   secureCookies(c),
		SameSite: fiber.CookieSameSiteLaxMode,
		Expires:  time.Now().Add(a.SessionTTL),
		Path:     "/",
	})
	return c.SendStatus(fiber.StatusNoContent)
}

// Logout godoc
// @Summary  Log out
// @Description  Clears the session cookie. Not itself session-protected — logging out never requires already being logged in.
// @Tags     auth
// @Success  204  "no content"
// @Router   /auth/logout [post]
func (a *API) Logout(c fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     middleware.SessionCookie,
		Value:    "",
		HTTPOnly: true,
		Secure:   secureCookies(c),
		SameSite: fiber.CookieSameSiteLaxMode,
		Expires:  time.Now().Add(-time.Hour),
		Path:     "/",
	})
	return c.SendStatus(fiber.StatusNoContent)
}

// secureCookies is false only over plain HTTP in development — a Secure
// cookie is silently dropped by the browser over http://, which would
// make local login impossible if this were hardcoded true.
func secureCookies(c fiber.Ctx) bool {
	return c.Protocol() == "https"
}
