package login

import (
	"errors"
	"net/http"
	"os"
	"strings"

	models "ccs.quizportal/Models"
	"ccs.quizportal/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func GenerateUserUUID(email string) models.UID {
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	u := uuid.NewSHA1(namespace, []byte(email))
	var arr [7]byte
	copy(arr[:], u[:7])
	return &arr
}

func StoreAuthUser(email string, token string, uid models.UID) error {
	var user models.AuthUser
	found, err := db.AUTH.GetExists(map[string]interface{}{"userEmail": email}, &user)
	if err != nil {
		return err
	}

	if found {
		filter := map[string]interface{}{"userEmail": email}
		update := map[string]interface{}{"$set": map[string]interface{}{"sessionToken": token}}
		_, err := db.AUTH.Coll.UpdateOne(db.AUTH.Context, filter, update)
		return err
	} else {
		authUser := models.AuthUser{
			UserId:       uid,
			Email:        email,
			SessionToken: token,
		}
		_, err := db.AUTH.Coll.InsertOne(db.AUTH.Context, authUser)
		return err
	}
}

// SessionToken pulls the JWT from the Authorization header first, then the
// session_token cookie.
//
// The header path exists because the frontend and this API are served from
// different sites (vercel.app vs onrender.com), which makes the session cookie a
// third-party cookie. Edge, Safari, Firefox and every incognito mode drop those,
// so cookie-only auth silently fails for a large share of candidates.
func SessionToken(c *gin.Context) (string, error) {
	if h := c.GetHeader("Authorization"); h != "" {
		if token, ok := strings.CutPrefix(h, "Bearer "); ok && token != "" {
			return strings.TrimSpace(token), nil
		}
	}
	token, err := c.Cookie("session_token")
	if err != nil || token == "" {
		return "", errors.New("no session token")
	}
	return token, nil
}

// session is a request's verified identity, cached on the gin context so the
// middleware and the handler behind it share one JWT parse and one DB lookup.
type session struct {
	Email string
	Name  string
	User  models.AuthUser
}

const sessionKey = "ccs_session"

// loadSession validates the JWT and checks it is still the account's current
// token. Logging out (or signing in again elsewhere) replaces the stored token,
// so an old copy stops working instead of staying valid for its full 12 hours.
func loadSession(c *gin.Context) (*session, int, error) {
	if v, ok := c.Get(sessionKey); ok {
		return v.(*session), 0, nil
	}

	tokenStr, err := SessionToken(c)
	if err != nil {
		return nil, http.StatusUnauthorized, errors.New("Missing session token")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return nil, http.StatusInternalServerError, errors.New("JWT secret not set")
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, http.StatusUnauthorized, errors.New("Invalid or expired session token")
	}

	email, ok := claims["email"].(string)
	if !ok || email == "" {
		return nil, http.StatusUnauthorized, errors.New("email not found in token")
	}
	name, _ := claims["name"].(string)

	var user models.AuthUser
	found, err := db.AUTH.GetExists(map[string]interface{}{"userEmail": email}, &user)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("could not load your account")
	}
	if !found || user.UserId == nil {
		return nil, http.StatusUnauthorized, errors.New("user not found")
	}
	if user.SessionToken != tokenStr {
		return nil, http.StatusUnauthorized, errors.New("This session has ended (you logged out or signed in again elsewhere). Please sign in again.")
	}

	s := &session{Email: email, Name: name, User: user}
	c.Set(sessionKey, s)
	return s, 0, nil
}

func GetUIDFromSession(c *gin.Context) (models.UID, error) {
	s, _, err := loadSession(c)
	if err != nil {
		return nil, err
	}
	return s.User.UserId, nil
}

// Verify_token is a standalone endpoint (not middleware): the frontend calls it
// after the OAuth redirect to find out whether the session it just got is
// usable, so it answers with JSON instead of redirecting a browser around.
func Verify_token(c *gin.Context) {
	s, status, err := loadSession(c)
	if err != nil {
		c.AbortWithStatusJSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"authenticated": true, "email": s.Email, "name": s.Name})
}
