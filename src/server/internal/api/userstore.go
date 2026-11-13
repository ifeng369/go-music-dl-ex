package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/glebarez/sqlite"
)

const (
	SessionCookieName = "musicdl_session"
	sessionTTL        = 7 * 24 * time.Hour
)

// AppUser is an account managed by this application, independent from the
// upstream go-music-dl single-admin settings.
type AppUser struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `gorm:"size:255;not null" json:"-"`
	Role         string    `gorm:"size:16;not null;default:user" json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (AppUser) TableName() string { return "app_users" }

type appSession struct {
	Token     string    `gorm:"primaryKey;size:64"`
	UserID    uint      `gorm:"index;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
}

func (appSession) TableName() string { return "app_sessions" }

type appUserLibrary struct {
	UserID    uint   `gorm:"primaryKey"`
	Data      string `gorm:"type:text"`
	UpdatedAt time.Time
}

func (appUserLibrary) TableName() string { return "app_user_libraries" }

type appSetting struct {
	Key   string `gorm:"primaryKey;size:64"`
	Value string `gorm:"type:text"`
}

func (appSetting) TableName() string { return "app_settings" }

type appDownloadRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"-"`
	SongID    string    `gorm:"size:128" json:"songId"`
	Title     string    `gorm:"size:255" json:"title"`
	Artist    string    `gorm:"size:255" json:"artist"`
	Album     string    `gorm:"size:255" json:"album"`
	Source    string    `gorm:"size:32" json:"source"`
	Cover     string    `gorm:"size:1024" json:"cover"`
	Filename  string    `gorm:"size:255" json:"filename"`
	Path      string    `gorm:"size:1024" json:"path"`
	Size      int64     `json:"size"`
	Status    string    `gorm:"size:32" json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func (appDownloadRecord) TableName() string { return "app_download_records" }

// UserStore persists users, sessions, per-user libraries and app settings in a
// dedicated SQLite database. It deliberately does not touch the upstream core
// settings database.
type UserStore struct {
	db *gorm.DB
}

func NewUserStore(path string) (*UserStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open user database: %w", err)
	}
	if err := db.AutoMigrate(&AppUser{}, &appSession{}, &appUserLibrary{}, &appSetting{}, &appDownloadRecord{}, &appSyncVersion{}); err != nil {
		return nil, fmt.Errorf("migrate user database: %w", err)
	}
	store := &UserStore{db: db}
	if err := store.ensureDefaultAdmin(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *UserStore) ensureDefaultAdmin() error {
	var count int64
	if err := s.db.Model(&AppUser{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	username := strings.TrimSpace(os.Getenv("MUSIC_DL_ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("MUSIC_DL_ADMIN_PASSWORD")
	if password == "" {
		password = "admin123"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := AppUser{Username: username, PasswordHash: string(hash), Role: "admin"}
	if err := s.db.Create(&user).Error; err != nil {
		return err
	}
	fmt.Printf("[musicdl] created default administrator %q (set MUSIC_DL_ADMIN_PASSWORD before first run to change the password)\n", username)
	return nil
}

func (s *UserStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}

func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return errors.New("用户名长度需为 3-32 个字符")
	}
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' {
			continue
		}
		return errors.New("用户名只能包含字母、数字、下划线、短横线和点")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少需要 6 个字符")
	}
	if len(password) > 128 {
		return errors.New("密码过长")
	}
	return nil
}

func (s *UserStore) Authenticate(username, password string) (*AppUser, error) {
	var user AppUser
	if err := s.db.Where("username = ?", normalizeUsername(username)).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, errors.New("用户名或密码错误")
	}
	return &user, nil
}

func (s *UserStore) GetUser(id uint) (*AppUser, error) {
	var user AppUser
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) ListUsers() ([]AppUser, error) {
	var users []AppUser
	err := s.db.Order("id asc").Find(&users).Error
	return users, err
}

func (s *UserStore) CreateUser(username, password, role string) (*AppUser, error) {
	username = normalizeUsername(username)
	if err := ValidateUsername(username); err != nil {
		return nil, err
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	if role != "admin" && role != "user" {
		role = "user"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := AppUser{Username: username, PasswordHash: string(hash), Role: role}
	if err := s.db.Create(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, errors.New("用户名已存在")
		}
		return nil, err
	}
	return &user, nil
}

func (s *UserStore) UpdatePassword(id uint, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	result := s.db.Model(&AppUser{}).Where("id = ?", id).Update("password_hash", string(hash))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return s.deleteSessionsForUser(id)
}

func (s *UserStore) DeleteUser(id uint) error {
	result := s.db.Delete(&AppUser{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	_ = s.deleteSessionsForUser(id)
	_ = s.db.Delete(&appUserLibrary{}, "user_id = ?", id).Error
	return nil
}

func (s *UserStore) CountAdmins() (int64, error) {
	var count int64
	err := s.db.Model(&AppUser{}).Where("role = ?", "admin").Count(&count).Error
	return count, err
}

func (s *UserStore) CreateSession(userID uint) (string, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", time.Time{}, err
	}
	token := hex.EncodeToString(raw)
	expires := time.Now().Add(sessionTTL)
	session := appSession{Token: token, UserID: userID, ExpiresAt: expires, CreatedAt: time.Now()}
	if err := s.db.Create(&session).Error; err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *UserStore) GetSession(token string) (*AppUser, error) {
	if token == "" {
		return nil, errors.New("empty session")
	}
	var session appSession
	if err := s.db.Where("token = ?", token).First(&session).Error; err != nil {
		return nil, err
	}
	if time.Now().After(session.ExpiresAt) {
		_ = s.db.Delete(&appSession{}, "token = ?", token).Error
		return nil, errors.New("session expired")
	}
	return s.GetUser(session.UserID)
}

func (s *UserStore) DeleteSession(token string) error {
	return s.db.Delete(&appSession{}, "token = ?", token).Error
}

func (s *UserStore) deleteSessionsForUser(userID uint) error {
	return s.db.Delete(&appSession{}, "user_id = ?", userID).Error
}

func (s *UserStore) GetLibrary(userID uint) (string, error) {
	var item appUserLibrary
	if err := s.db.First(&item, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", err
	}
	return item.Data, nil
}

func (s *UserStore) GetSetting(key, fallback string) string {
	var item appSetting
	if err := s.db.First(&item, "key = ?", key).Error; err != nil {
		return fallback
	}
	if strings.TrimSpace(item.Value) == "" {
		return fallback
	}
	return item.Value
}

func (s *UserStore) SetSetting(key, value string) error {
	return s.db.Save(&appSetting{Key: key, Value: value}).Error
}

func (s *UserStore) AddDownloadRecord(userID uint, item MusicItem, path, filename string, size int64) error {
	cover := item.Artwork
	if cover == "" {
		cover = item.Cover
	}
	record := &appDownloadRecord{
		UserID:   userID,
		SongID:   item.SourceID,
		Title:    item.Title,
		Artist:   item.Artist,
		Album:    item.Album,
		Source:   item.Source,
		Cover:    cover,
		Filename: filename,
		Path:     path,
		Size:     size,
		Status:   "done",
	}
	return s.db.Create(record).Error
}

func (s *UserStore) ListDownloadRecords(userID uint, limit int) ([]appDownloadRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var records []appDownloadRecord
	err := s.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&records).Error
	return records, err
}

func (s *UserStore) DeleteDownloadRecord(userID, id uint) error {
	return s.db.Where("user_id = ? AND id = ?", userID, id).Delete(&appDownloadRecord{}).Error
}

func (s *UserStore) ClearDownloadRecords(userID uint) error {
	return s.db.Where("user_id = ?", userID).Delete(&appDownloadRecord{}).Error
}

func (s *UserStore) FindDownloadRecord(userID uint, source, songID string) (*appDownloadRecord, error) {
	if songID == "" {
		return nil, nil
	}
	var record appDownloadRecord
	err := s.db.
		Where("user_id = ? AND source = ? AND song_id = ? AND status = ?", userID, source, songID, "done").
		Order("created_at DESC").
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *UserStore) UpdateDownloadRecordCover(userID, id uint, cover string) error {
	return s.db.Model(&appDownloadRecord{}).Where("user_id = ? AND id = ?", userID, id).Update("cover", cover).Error
}
