package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestMakeValidJWT(t *testing.T) {
	userID, _ := uuid.NewUUID()
	tokenSecret := "secret"
	expiresIn := 2 * time.Minute
	tokenString, err := MakeJWT(userID, tokenSecret, expiresIn)
	assert.Nil(t, err)
	resultUser, err := ValidateJWT(tokenString, tokenSecret)
	assert.Nil(t, err)
	assert.Equal(t, userID, resultUser)
}

func TestInvalidSecretJWT(t *testing.T) {
	userID, _ := uuid.NewUUID()
	tokenSecret := "secret"
	differentSecret := "terces"
	expiresIn := 2 * time.Minute
	tokenString, err := MakeJWT(userID, tokenSecret, expiresIn)
	assert.Nil(t, err)
	resultUser, err := ValidateJWT(tokenString, differentSecret)
	assert.Error(t, err)
	assert.NotEqual(t, userID, resultUser)
}

func TestExpiredJWT(t *testing.T) {
	userID, _ := uuid.NewUUID()
	tokenSecret := "secret"
	expiresIn := 1 * time.Nanosecond
	tokenString, err := MakeJWT(userID, tokenSecret, expiresIn)
	assert.Nil(t, err)
	resultUser, err := ValidateJWT(tokenString, tokenSecret)
	assert.NotNil(t, err)
	assert.EqualError(t, err, "token has invalid claims: token is expired")
	assert.NotEqual(t, userID, resultUser)
}
