package api

import (
	"errors"

	"gorm.io/gorm"
)

// EmbeddedVersion is reported by the standalone android build.
func EmbeddedVersion() string { return "standalone" }

// EnsureLocalUser returns the personal account used by the standalone client,
// creating it on first launch. No password is stored because the app never
// exposes a login screen.
func EnsureLocalUser(store *UserStore) (*AppUser, error) {
	if store == nil {
		return nil, errors.New("user store unavailable")
	}
	var user AppUser
	err := store.DB().Where("username = ?", "local").First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = AppUser{Username: "local", Role: "admin", PasswordHash: ""}
		if createErr := store.DB().Create(&user).Error; createErr != nil {
			return nil, createErr
		}
		return &user, nil
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}
