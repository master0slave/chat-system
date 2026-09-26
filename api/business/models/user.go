package models

type Role string

const (
	RoleCustomer Role = "customer"
	RoleAgent    Role = "agent"
)

func (r Role) Valid() bool { return r == RoleCustomer || r == RoleAgent }

type User struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}
