package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	adminCookie    = "admin_auth"
	adminCookieTTL = time.Hour
	RoleMain       = "main"
	RoleView       = "view"
)

// The admin cookie used to hold the bare role ("main"/"view"), so anyone could
// type admin_auth=main into devtools and get the full portal. It now carries
// role|expiry|HMAC, signed with a server-side secret the browser never sees.
func adminSecret() []byte {
	if s := os.Getenv("ADMIN_COOKIE_SECRET"); s != "" {
		return []byte(s)
	}
	return []byte(os.Getenv("SESSION_SECRET"))
}

func signAdmin(payload string) string {
	mac := hmac.New(sha256.New, adminSecret())
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func setAdminCookie(c *gin.Context, role string) {
	exp := strconv.FormatInt(time.Now().Add(adminCookieTTL).Unix(), 10)
	payload := role + "|" + exp
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(adminCookie, payload+"|"+signAdmin(payload), int(adminCookieTTL.Seconds()), "/", "", true, true)
}

// Role returns the verified admin role on the request, or "" when the cookie
// is missing, expired or has been tampered with.
func Role(c *gin.Context) string {
	raw, err := c.Cookie(adminCookie)
	if err != nil {
		return ""
	}
	parts := strings.Split(raw, "|")
	if len(parts) != 3 {
		return ""
	}
	payload := parts[0] + "|" + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(signAdmin(payload))) {
		return ""
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return ""
	}
	if parts[0] != RoleMain && parts[0] != RoleView {
		return ""
	}
	return parts[0]
}

func HandleLogout(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(adminCookie, "", -1, "/", "", true, true)
	c.Redirect(http.StatusFound, "/admin/login")
}
