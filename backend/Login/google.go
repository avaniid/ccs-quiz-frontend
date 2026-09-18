package login

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markbates/goth/gothic"
)

// func HandleAuth(c *gin.Context) {
// 	gothic.BeginAuthHandler(c.Writer, c.Request)
// }

func HandleAuth(c *gin.Context) {
	q := c.Request.URL.Query()
	q.Add("provider", "google")
	c.Request.URL.RawQuery = q.Encode()
	gothic.BeginAuthHandler(c.Writer, c.Request)
}

func HandleAuthCallback(c *gin.Context) {
	jwtSecret := os.Getenv("JWT_SECRET")
	frontendRedirectUrl := os.Getenv("FRONTEND_REDIRECT_URL")

	if jwtSecret == "" || frontendRedirectUrl == "" {
		// A request handler must never call log.Fatal: it exits the process and
		// takes the whole backend down for every other candidate.
		log.Println("oauth callback: JWT_SECRET or FRONTEND_REDIRECT_URL missing")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	uid := GenerateUserUUID(user.Email)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": user.Email,
		"name":  user.Name,
		"exp":   time.Now().Add(time.Hour * 12).Unix(),
	})

	signedToken, err := token.SignedString([]byte(jwtSecret))

	if err != nil {
		log.Printf("oauth callback: could not sign token for %s: %v", user.Email, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not create session"})
		return
	}

	if err := StoreAuthUser(user.Email, signedToken, uid); err != nil {
		log.Printf("oauth callback: could not store user %s: %v", user.Email, err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not store your account"})
		return
	}

	setSessionCookie(c, signedToken, 3600*12)

	// Also hand the token over in the URL fragment. The cookie alone is not
	// enough: the frontend is on a different site, so browsers that block
	// third-party cookies never send it back. A fragment is never transmitted to
	// any server and the frontend clears it from the address bar on arrival.
	c.Redirect(http.StatusTemporaryRedirect, frontendRedirectUrl+"#token="+url.QueryEscape(signedToken))
}

// setSessionCookie writes the session_token cookie with the SameSite policy the
// current deployment needs. In production the frontend is served from a
// different site than this API, so the cookie must be SameSite=None + Secure or
// the browser drops it on every cross-site XHR. Locally we stay on Lax over
// plain http, which a Secure cookie would not survive.
func setSessionCookie(c *gin.Context, value string, maxAge int) {
	secure := gin.Mode() == gin.ReleaseMode
	if secure {
		c.SetSameSite(http.SameSiteNoneMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
	c.SetCookie("session_token", value, maxAge, "/", "", secure, true)
}

// AuthMiddleware guards the routes the SPA calls over XHR, so an unauthenticated
// request gets a 401 it can act on. Redirecting here would only produce a
// "headers already written" warning after the abort, and the browser would
// follow it into an opaque CORS failure instead of surfacing the 401.
func AuthMiddleware(c *gin.Context) {
	tokenStr, err := SessionToken(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing session token"})
		return
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "JWT secret not set"})
		return
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session token"})
		return
	}

	c.Next()
}

func Logout(c *gin.Context) {
	setSessionCookie(c, "", -1)
	c.JSON(200, gin.H{"message": "Logged out"})
}
