package dbs

import "time"

type User struct {
	ID                int64      `gorm:"column:id;primaryKey"`
	Name              string     `gorm:"column:name;not null"`
	Surname           string     `gorm:"column:surname;not null"`
	Email             string     `gorm:"column:email;uniqueIndex;not null"`
	Phone             string     `gorm:"column:phone"`
	PhoneContact      string     `gorm:"column:phone_contact"`
	Country           string     `gorm:"column:country"`
	Province          string     `gorm:"column:province"`
	City              string     `gorm:"column:city"`
	Street            string     `gorm:"column:street"`
	Number            string     `gorm:"column:number"`
	DNI               string     `gorm:"column:dni;uniqueIndex;not null"`
	BirthDate         time.Time  `gorm:"column:birth_date;not null"`
	Password          string     `gorm:"column:password;not null"`
	Status            string     `gorm:"column:status;not null;default:active"`
	BankAlias         *string    `gorm:"column:bank_alias"`
	PasswordChangedAt *time.Time `gorm:"column:password_changed_at"`
	PhotoKey          *string    `gorm:"column:photo_key"`
	PhotoUpdatedAt    *time.Time `gorm:"column:photo_updated_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}
