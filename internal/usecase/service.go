package usecase

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"sahata-worship-be/internal/domain"
	"strings"
	"time"
)

var ErrInvalid = errors.New("invalid input")
var ErrPending = errors.New("akun menunggu approval admin")
var ErrRejected = errors.New("registrasi akun ditolak admin")

type Service struct {
	store  domain.Store
	secret []byte
}

func New(s domain.Store, secret string) *Service { return &Service{store: s, secret: []byte(secret)} }
func (s *Service) Register(u domain.User, password string) (domain.User, string, error) {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	if u.Name == "" || u.Email == "" || len(password) < 8 {
		return u, "", ErrInvalid
	}
	h, e := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if e != nil {
		return u, "", e
	}
	u.PasswordHash = string(h)
	if u.Role == "" {
		u.Role = "Member"
	}
	u.Status = "pending"
	u, e = s.store.CreateUser(u)
	if e != nil {
		return u, "", e
	}
	return u, "", nil
}
func (s *Service) Login(email, password string) (domain.User, string, error) {
	u, e := s.store.UserByEmail(strings.ToLower(strings.TrimSpace(email)))
	if e != nil {
		return u, "", e
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return u, "", errors.New("invalid credentials")
	}
	if u.Status == "pending" {
		return u, "", ErrPending
	}
	if u.Status == "rejected" {
		return u, "", ErrRejected
	}
	t, e := s.token(u)
	return u, t, e
}
func (s *Service) token(u domain.User) (string, error) {
	c := jwt.MapClaims{"sub": fmt.Sprint(u.ID), "email": u.Email, "role": u.Role, "exp": time.Now().Add(24 * time.Hour).Unix(), "iat": time.Now().Unix()}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}
func (s *Service) ParseToken(raw string) (int64, error) {
	t, e := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, errors.New("invalid algorithm")
		}
		return s.secret, nil
	})
	if e != nil || !t.Valid {
		return 0, errors.New("invalid token")
	}
	var id int64
	_, e = fmt.Sscan(fmt.Sprint(t.Claims.(jwt.MapClaims)["sub"]), &id)
	return id, e
}
func (s *Service) Store() domain.Store { return s.store }
func ValidateRoom(x domain.Room) error {
	if strings.TrimSpace(x.Name) == "" || x.Date == "" || x.StartTime == "" || x.EndTime == "" || x.Code == "" {
		return ErrInvalid
	}
	switch x.Status {
	case "Scheduled", "Live", "Completed":
		return nil
	}
	return ErrInvalid
}
func IsNotFound(e error) bool { return errors.Is(e, sql.ErrNoRows) }
