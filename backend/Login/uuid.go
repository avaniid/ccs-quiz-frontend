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

func GetUIDFromSession(c *gin.Context) (models.UID, error) {
	tokenStr, err := SessionToken(c)
	if err != nil {
		return nil, errors.New("session_token missing")
	}

	claims := jwt.MapClaims{}
	_, err = jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		return nil, errors.New("invalid session_token")
	}

	email, ok := claims["email"].(string)
	if !ok {
		return nil, errors.New("email not found in token")
	}

	var user models.AuthUser
	found, err := db.AUTH.GetExists(map[string]interface{}{"userEmail": email}, &user)
	if err != nil || !found {
		return nil, errors.New("user not found")
	}

	return user.UserId, nil
}

// Verify_token is a standalone endpoint (not middleware): the frontend calls it
// after the OAuth redirect to find out whether the session cookie it just got is
// usable, so it answers with JSON instead of redirecting a browser around.
func Verify_token(c *gin.Context) {
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

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(jwtSecret), nil
	})

	if err != nil || !token.Valid {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired session token"})
		return
	}

	email, _ := claims["email"].(string)
	name, _ := claims["name"].(string)
	c.JSON(http.StatusOK, gin.H{"authenticated": true, "email": email, "name": name})
}
