// Package auth, used to hash and unhash user password
package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)


func HashPassword(password string) (string, error){

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 15)	
	if err != nil {
		return "", err
	}

	return string(hash), nil

}

func CheckPasswordHash(password, hash string) error {
		
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		return err
	}

	return nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject: userID.String(),
	})

	ss, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return ss, nil
}

func keyFunc(secret []byte) jwt.Keyfunc {
	return func (t *jwt.Token) (any, error) {

		if _, ok  :=  t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")	
		}

		return secret, nil
	};
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	
	claims := jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc([]byte(tokenSecret)))
	
	if err != nil {
		return uuid.Nil, err
	}
	
	stringID, err := token.Claims.GetSubject()
	if err != nil{
		return uuid.Nil, err
	}

	userID, err := uuid.Parse(stringID)
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

func GetBearerToken(header http.Header) (string, error) {
	str := header.Get("Authorization")
	tokenString := strings.Fields(str)[1]
	if str == "" || tokenString == "" || strings.Fields(str)[0] != "Bearer"{
		return "", errors.New("invalid authorization")
	}
	return tokenString, nil
}
