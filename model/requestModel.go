package model

type LoginForm struct {
	Email    string `form:"email" validate:"required,email" json:"email"`
	Password string `form:"password" validate:"required" json:"password"`
}

type RestaurantSignup struct {
	Name           string `validate:"required" json:"name"`
	Description    string `gorm:"column:description" validate:"required" json:"description"`
	Address        string `gorm:"column:address" validate:"required" json:"address"`
	Email          string `gorm:"column:email" validate:"required,email" json:"email"`
	Password       string `gorm:"column:password" validate:"required" json:"password"`
	PhoneNumber    string `gorm:"column:phone_number" validate:"required" json:"phone_number"`
	ImageURL       string `gorm:"column:image_url" validate:"required" json:"image_url"`
	CertificateURL string `gorm:"column:certificate_url" validate:"required" json:"certificate_url"`
}

type AdminLoginRequest struct {
	Email string `json:"email" validate:"required,email"`
}