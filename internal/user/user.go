package user

type Repository interface{}

type User struct {
	Username string
	Role     string
}
