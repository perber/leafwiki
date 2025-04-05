package auth

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

const (
	RoleAdmin  = "admin"
	RoleEditor = "editor"
)

var validRoles = map[string]bool{
	RoleAdmin:  true,
	RoleEditor: true,
}

func IsValidRole(role string) bool {
	return validRoles[role]
}
