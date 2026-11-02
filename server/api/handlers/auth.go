package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/users"
)

type AuthHandler struct {
	userService *users.Service
	jwtService  *auth.JWTService
}

func NewAuthHandler(
	userService *users.Service,
	jwtService *auth.JWTService,
) *AuthHandler {
	return &AuthHandler{
		userService: userService,
		jwtService:  jwtService,
	}
}

type RegisterRequest struct {
	Username         string `json:"username"`
	Nickname         string `json:"nickname"`
	Email            string `json:"email"`
	Password         string `json:"password"`
	FavoriteLanguage string `json:"favorite_language"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req RegisterRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid request body",
		})
		return
	}

	user, err := h.userService.CreateUser(
		r.Context(),
		req.Username,
		req.Nickname,
		req.Email,
		req.Password,
		req.FavoriteLanguage,
	)
	if err != nil {
		http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message":           "User registered successfully",
		"username":          user.Username,
		"nickname":          user.Nickname,
		"email":             user.Email,
		"favorite_language": user.FavoriteLanguage.String,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid request body",
		})
		return
	}

	user, err := h.userService.LoginUser(
		r.Context(),
		req.Email,
		req.Password,
	)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid email or password",
		})
		return
	}

	token, err := h.jwtService.GenerateToken(user.ID.String())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "could not generate token",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Login successful",
		"token":   token,
		"user": map[string]string{
			"id":                user.ID.String(),
			"username":          user.Username,
			"nickname":          user.Nickname,
			"email":             user.Email,
			"favorite_language": user.FavoriteLanguage.String,
		},
	})
}
