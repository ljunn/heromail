package store

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *PostgresStore) GetSystemConfig(key string) (map[string]string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, false, errors.New("系统配置键不能为空")
	}
	var row sqlSystemConfig
	if err := s.db.Where("key = ?", key).First(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	config := make(map[string]string)
	if err := s.decryptJSON(row.EncryptedValue, &config); err != nil {
		return nil, false, errors.New("系统配置解密失败")
	}
	return config, true, nil
}

func (s *PostgresStore) SaveSystemConfig(key string, value map[string]string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("系统配置键不能为空")
	}
	encrypted, err := s.encryptJSON(value)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var row sqlSystemConfig
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if findErr := tx.Where("key = ?", key).First(&row).Error; errors.Is(findErr, gorm.ErrRecordNotFound) {
			row = sqlSystemConfig{Key: key, EncryptedValue: encrypted, CreatedAt: now, UpdatedAt: now}
			return tx.Create(&row).Error
		} else if findErr != nil {
			return findErr
		}
		return tx.Model(&row).Updates(map[string]any{"encrypted_value": encrypted, "updated_at": now}).Error
	})
	return err
}

var _ SystemConfigRepository = (*PostgresStore)(nil)
