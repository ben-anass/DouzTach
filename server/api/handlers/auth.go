package handlers

import (
	"encoding/json"
	"github.com/ben-anass/DouzTach/internal/auth"
	"github.com/ben-anass/DouzTach/server/users"
	"net/http"
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
	Name     string `json:"name"`
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
		req.Name,
		req.Email,
		req.Password,
	)
	if err != nil {
		http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User registered successfully",
		"name":    user.Name,
		"email":   user.Email,
	})
}
