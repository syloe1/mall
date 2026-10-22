package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"

	"mall/pkg/core"
)

var testSecret = []byte("test-secret-key-for-unit-test")
var wrongSecret = []byte("wrong-secret-key")

// makeToken signs claims with the given secret using HS256.
func makeToken(secret []byte, claims Claims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString(secret)
	return s
}

// newJWTCtx creates a gin test context with a proper *http.Request.
func newJWTCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c, w
}

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("missing_authorization_header", func(t *testing.T) {
		c, w := newJWTCtx()
		// no Authorization header set

		JWTAuth(testSecret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), core.MsgUnauthorized)
	})

	t.Run("empty_bearer_token", func(t *testing.T) {
		c, w := newJWTCtx()
		c.Request.Header.Set("Authorization", "Bearer ")
		// TrimPrefix("Bearer ") produces "" which != original header,
		// so the code skips the "no prefix" branch and tries JWT parse
		// on the empty string, which fails → "token无效"

		JWTAuth(testSecret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), core.MsgUnauthorized)
	})

	t.Run("malformed_token", func(t *testing.T) {
		c, w := newJWTCtx()
		c.Request.Header.Set("Authorization", "Bearer x.y.z")

		JWTAuth(testSecret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), core.MsgUnauthorized)
	})

	t.Run("wrong_secret", func(t *testing.T) {
		c, w := newJWTCtx()
		token := makeToken(wrongSecret, Claims{
			UserId:    1,
			Username:  "test",
			Role:      "admin",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		c.Request.Header.Set("Authorization", "Bearer "+token)

		JWTAuth(testSecret)(c) // validate with different secret

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), core.MsgUnauthorized)
	})

	t.Run("expired_token", func(t *testing.T) {
		c, w := newJWTCtx()
		token := makeToken(testSecret, Claims{
			UserId:    1,
			Username:  "test",
			Role:      "user",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			},
		})
		c.Request.Header.Set("Authorization", "Bearer "+token)

		JWTAuth(testSecret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), core.MsgUnauthorized)
	})

	t.Run("refresh_token_rejected", func(t *testing.T) {
		c, w := newJWTCtx()
		token := makeToken(testSecret, Claims{
			UserId:    2,
			Username:  "refresh-user",
			Role:      "user",
			TokenType: "refresh",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		c.Request.Header.Set("Authorization", "Bearer "+token)

		JWTAuth(testSecret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), core.MsgUnauthorized)
	})

	t.Run("valid_access_token", func(t *testing.T) {
		c, _ := newJWTCtx()
		token := makeToken(testSecret, Claims{
			UserId:    42,
			Username:  "alice",
			Role:      "admin",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		c.Request.Header.Set("Authorization", "Bearer "+token)

		JWTAuth(testSecret)(c)

		assert.False(t, c.IsAborted(), "valid access token should pass")
		userID, _ := c.Get("userID")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		assert.Equal(t, uint(42), userID)
		assert.Equal(t, "alice", username)
		assert.Equal(t, "admin", role)
	})

	t.Run("no_bearer_prefix_raw_token_passes", func(t *testing.T) {
		c, _ := newJWTCtx()
		token := makeToken(testSecret, Claims{
			UserId:    7,
			Username:  "bob",
			Role:      "user",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		// no "Bearer " prefix, just the raw token
		c.Request.Header.Set("Authorization", token)

		JWTAuth(testSecret)(c)

		assert.False(t, c.IsAborted(), "raw token without Bearer prefix should pass")
		userID, _ := c.Get("userID")
		assert.Equal(t, uint(7), userID)
	})

	t.Run("not_before_future_token", func(t *testing.T) {
		c, w := newJWTCtx()
		token := makeToken(testSecret, Claims{
			UserId:    1,
			Username:  "future",
			Role:      "user",
			TokenType: "access",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)), // not yet valid
			},
		})
		c.Request.Header.Set("Authorization", "Bearer "+token)

		JWTAuth(testSecret)(c)

		assert.True(t, c.IsAborted())
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
