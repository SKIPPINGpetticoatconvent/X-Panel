package repository

import (
	"x-ui/database/model"

	"gorm.io/gorm"
)

// UserRepository 定义 User 数据访问接口
type UserRepository interface {
	FindFirst() (*model.User, error)
	FindByUsername(username string) (*model.User, error)
	Create(user *model.User) error
	Update(user *model.User) error
	UpdatePassword(id int, hashedPassword string) error

	GetDB() *gorm.DB
}

// userRepository 实现 UserRepository 接口
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建新的 UserRepository 实例
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{
		db: db,
	}
}

// GetDB 返回当前数据库连接
func (r *userRepository) GetDB() *gorm.DB {
	return r.db
}

// FindFirst 查找第一个用户
func (r *userRepository) FindFirst() (*model.User, error) {
	user := &model.User{}
	err := r.db.Model(model.User{}).First(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindByUsername 根据用户名查找用户
func (r *userRepository) FindByUsername(username string) (*model.User, error) {
	user := &model.User{}
	err := r.db.Model(model.User{}).Where("username = ?", username).First(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

// Create 创建新用户
func (r *userRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

// Update 更新用户
func (r *userRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// UpdatePassword 更新用户密码
func (r *userRepository) UpdatePassword(id int, hashedPassword string) error {
	return r.db.Model(model.User{}).Where("id = ?", id).Update("password", hashedPassword).Error
}
