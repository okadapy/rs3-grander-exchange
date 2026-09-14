package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/rs3-market/backend/shared/models"
)

type Service struct {
	DB     *gorm.DB
	Secret []byte
	Expiry time.Duration
}

func New(db *gorm.DB, secret string, expiry time.Duration) *Service {
	return &Service{DB: db, Secret: []byte(secret), Expiry: expiry}
}

func (s *Service) Migrate() error {
	return s.DB.AutoMigrate(&models.ChatUser{}, &models.ChatMessage{})
}

func (s *Service) Register(username, password string) (*models.ChatUser, error) {
	if len(username) < 3 || len(username) > 32 {
		return nil, errors.New("username must be 3-32 chars")
	}
	if len(password) < 6 {
		return nil, errors.New("password too short")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}
	user := &models.ChatUser{Username: username, PasswordHash: string(hash), CreatedAt: time.Now().UTC()}
	if err := s.DB.Create(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) Login(username, password string) (string, error) {
	var u models.ChatUser
	if err := s.DB.Where("username = ?", username).First(&u).Error; err != nil {
		return "", errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": u.ID,
		"usr": u.Username,
		"exp": time.Now().Add(s.Expiry).Unix(),
		"iat": time.Now().Unix(),
	})
	return tok.SignedString(s.Secret)
}

func (s *Service) Verify(token string) (uint, string, error) {
	t, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("bad signing method")
		}
		return s.Secret, nil
	})
	if err != nil || !t.Valid {
		return 0, "", errors.New("invalid token")
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return 0, "", errors.New("invalid claims")
	}
	sub, _ := claims["sub"].(float64)
	usr, _ := claims["usr"].(string)
	return uint(sub), usr, nil
}
