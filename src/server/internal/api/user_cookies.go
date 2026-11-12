package api

import (
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// appUserCookie keeps each account's platform cookies separate. The upstream
// core keeps one global CookieManager, so a request scopes it to the signed-in
// user while it runs (see withUserCookies).
type appUserCookie struct {
	UserID    uint      `gorm:"primaryKey;autoIncrement:false"`
	Source    string    `gorm:"primaryKey;size:64"`
	Value     string    `gorm:"type:text"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (appUserCookie) TableName() string { return "app_user_cookies" }

func (s *UserStore) ensureUserCookieTable() error {
	return s.db.AutoMigrate(&appUserCookie{})
}

// UserCookies returns the cookie value per source for one account.
func (s *UserStore) UserCookies(userID uint) (map[string]string, error) {
	if err := s.ensureUserCookieTable(); err != nil {
		return nil, err
	}
	var rows []appUserCookie
	if err := s.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, row := range rows {
		out[row.Source] = row.Value
	}
	return out, nil
}

func (s *UserStore) SetUserCookie(userID uint, source, value string) error {
	if err := s.ensureUserCookieTable(); err != nil {
		return err
	}
	row := appUserCookie{UserID: userID, Source: source, Value: value, UpdatedAt: time.Now()}
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "source"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&row).Error
}

func (s *UserStore) DeleteUserCookie(userID uint, source string) error {
	if err := s.ensureUserCookieTable(); err != nil {
		return err
	}
	return s.db.Where("user_id = ? AND source = ?", userID, source).
		Delete(&appUserCookie{}).Error
}

func (s *UserStore) CountUserCookies() (int64, error) {
	if err := s.ensureUserCookieTable(); err != nil {
		return 0, err
	}
	var count int64
	err := s.db.Model(&appUserCookie{}).Count(&count).Error
	return count, err
}

func (s *UserStore) DB() *gorm.DB { return s.db }
