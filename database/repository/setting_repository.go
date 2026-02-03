package repository

import (
	"x-ui/database/model"

	"gorm.io/gorm"
)

// SettingRepository 定义 Setting 数据访问接口
type SettingRepository interface {
	FindAll() ([]*model.Setting, error)
	FindAllExcept(excludeKey string) ([]*model.Setting, error)
	FindByKey(key string) (*model.Setting, error)
	Create(setting *model.Setting) error
	Update(setting *model.Setting) error
	DeleteAll() error

	GetDB() *gorm.DB
}

// settingRepository 实现 SettingRepository 接口
type settingRepository struct {
	db *gorm.DB
}

// NewSettingRepository 创建新的 SettingRepository 实例
func NewSettingRepository(db *gorm.DB) SettingRepository {
	return &settingRepository{
		db: db,
	}
}

// GetDB 返回当前数据库连接
func (r *settingRepository) GetDB() *gorm.DB {
	return r.db
}

// FindAll 查找所有设置
func (r *settingRepository) FindAll() ([]*model.Setting, error) {
	var settings []*model.Setting
	err := r.db.Model(model.Setting{}).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// FindAllExcept 查找除指定 key 外的所有设置
func (r *settingRepository) FindAllExcept(excludeKey string) ([]*model.Setting, error) {
	var settings []*model.Setting
	err := r.db.Model(model.Setting{}).Not("key = ?", excludeKey).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	return settings, nil
}

// FindByKey 根据 key 查找设置
func (r *settingRepository) FindByKey(key string) (*model.Setting, error) {
	setting := &model.Setting{}
	err := r.db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

// Create 创建新设置
func (r *settingRepository) Create(setting *model.Setting) error {
	return r.db.Create(setting).Error
}

// Update 更新设置
func (r *settingRepository) Update(setting *model.Setting) error {
	return r.db.Save(setting).Error
}

// DeleteAll 删除所有设置
func (r *settingRepository) DeleteAll() error {
	return r.db.Where("1 = 1").Delete(model.Setting{}).Error
}
