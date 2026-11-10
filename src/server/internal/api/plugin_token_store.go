package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

const pluginTokenTTL = 365 * 24 * time.Hour

// appPluginToken is a long-lived, revocable credential used by personal
// plugin download URLs. A matching appSession row is created so the normal
// API authentication middleware accepts the token as well.
type appPluginToken struct {
	Token     string    `gorm:"primaryKey;size:64" json:"token"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func (appPluginToken) TableName() string { return "app_plugin_tokens" }

func (s *UserStore) ensurePluginTokenTable() error {
	return s.db.AutoMigrate(&appPluginToken{})
}

func randomPluginToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func (s *UserStore) createPluginSession(token string, userID uint) error {
	expires := time.Now().Add(pluginTokenTTL)
	return s.db.Where("token = ?", token).Assign(appSession{
		UserID:    userID,
		ExpiresAt: expires,
	}).FirstOrCreate(&appSession{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expires,
		CreatedAt: time.Now(),
	}).Error
}

// EnsurePluginToken returns the user's stable plugin token, creating it when
// missing. It also refreshes the matching session so the token keeps working.
func (s *UserStore) EnsurePluginToken(userID uint) (string, error) {
	if err := s.ensurePluginTokenTable(); err != nil {
		return "", err
	}
	var record appPluginToken
	err := s.db.Where("user_id = ? AND expires_at > ?", userID, time.Now()).First(&record).Error
	if err == nil {
		if err := s.createPluginSession(record.Token, userID); err != nil {
			return "", err
		}
		return record.Token, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	token, err := randomPluginToken()
	if err != nil {
		return "", err
	}
	record = appPluginToken{
		Token:     token,
		UserID:    userID,
		ExpiresAt: time.Now().Add(pluginTokenTTL),
		CreatedAt: time.Now(),
	}
	if err := s.db.Create(&record).Error; err != nil {
		return "", err
	}
	if err := s.createPluginSession(token, userID); err != nil {
		return "", err
	}
	return token, nil
}

// ResetPluginToken invalidates the old personal plugin URL and returns a new
// token.
func (s *UserStore) ResetPluginToken(userID uint) (string, error) {
	if err := s.ensurePluginTokenTable(); err != nil {
		return "", err
	}
	var old []appPluginToken
	_ = s.db.Where("user_id = ?", userID).Find(&old).Error
	for _, record := range old {
		_ = s.db.Where("token = ?", record.Token).Delete(&appSession{}).Error
	}
	_ = s.db.Where("user_id = ?", userID).Delete(&appPluginToken{}).Error
	return s.EnsurePluginToken(userID)
}

// UserByPluginToken resolves a personal plugin token to its owner. Login
// session tokens are intentionally not accepted here.
func (s *UserStore) UserByPluginToken(token string) (*AppUser, error) {
	if token == "" {
		return nil, nil
	}
	if err := s.ensurePluginTokenTable(); err != nil {
		return nil, err
	}
	var record appPluginToken
	err := s.db.Where("token = ? AND expires_at > ?", token, time.Now()).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var user AppUser
	if err := s.db.First(&user, record.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}
