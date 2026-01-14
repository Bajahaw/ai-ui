package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func generateJWT(username string) (string, error) {
	if JWT_SECRET == "" {
		return "", fmt.Errorf("JWT_SECRET environment variable not set")
	}

	claims := jwt.MapClaims{
		"username": username,
		"exp":      time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(JWT_SECRET))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func extractClaims(token string) (map[string]any, error) {
	parsedToken, err := jwt.Parse(token, keyFunc)
	if err != nil {
		return nil, err
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok || !parsedToken.Valid {
		return nil, fmt.Errorf("Invalid token")
	}

	return claims, nil
}

// keyFunc provides the key for verifying the JWT signature
func keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("Unexpected signing method: %v", t.Header["alg"])
	}

	if JWT_SECRET == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable not set")
	}
	return []byte(JWT_SECRET), nil
}

func verifyUserCredentials(username, password string) error {
	user, err := users.GetByUsername(username)
	if err != nil {
		log.Debug("User not found", "username", username, "error", err)
		return fmt.Errorf("Invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.passHash), []byte(password))
	if err != nil {
		return fmt.Errorf("Invalid credentials")
	}

	return nil
}

func registerNewUser(username, password string) error {
	if len(password) < 8 || len(password) > 64 {
		return fmt.Errorf("Invalid password length")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user := &User{
		Username: username,
		passHash: string(hash),
	}

	err = users.Save(user)

	return err
}
