package auth

// PublicUser represents a user object that is safe to expose to the public.
type PublicUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

// User represents a user object with sensitive information.
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Role     string `json:"role"`
}

func (u *User) HasRole(role string) bool {
	return u.Role == role
}

func (u *User) ToPublicUser() *PublicUser {
	return &PublicUser{
		ID:       u.ID,
		Username: u.Username,
		Email:    u.Email,
		Role:     u.Role,
	}
}

// Roles
const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
	RoleViewer = "viewer"
)

var validRoles = map[string]bool{
	RoleAdmin:  true,
	RoleEditor: true,
	RoleViewer: true,
}

func IsValidRole(role string) bool {
	return validRoles[role]
}
