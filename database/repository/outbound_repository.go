package repository

import (
	"x-ui/database/model"

	"gorm.io/gorm"
)

// OutboundRepository 定义 Outbound 数据访问接口
type OutboundRepository interface {
	FindAll() ([]*model.OutboundTraffics, error)
	Create(traffic *model.OutboundTraffics) error
	Update(traffic *model.OutboundTraffics) error
	ResetTraffic(tag string) error
	ResetAllTraffics() error

	WithTx(tx *gorm.DB) OutboundRepository
	GetDB() *gorm.DB
}

// outboundRepository 实现 OutboundRepository 接口
type outboundRepository struct {
	db *gorm.DB
}

// NewOutboundRepository 创建新的 OutboundRepository 实例
func NewOutboundRepository(db *gorm.DB) OutboundRepository {
	return &outboundRepository{
		db: db,
	}
}

// WithTx 返回使用指定事务的新 Repository 实例
func (r *outboundRepository) WithTx(tx *gorm.DB) OutboundRepository {
	return &outboundRepository{db: tx}
}

// GetDB 返回当前数据库连接
func (r *outboundRepository) GetDB() *gorm.DB {
	return r.db
}

// FindAll 查找所有出站流量记录
func (r *outboundRepository) FindAll() ([]*model.OutboundTraffics, error) {
	var traffics []*model.OutboundTraffics
	err := r.db.Model(model.OutboundTraffics{}).Find(&traffics).Error
	if err != nil {
		return nil, err
	}
	return traffics, nil
}

// Create 创建新的出站流量记录
func (r *outboundRepository) Create(traffic *model.OutboundTraffics) error {
	return r.db.Create(traffic).Error
}

// Update 更新出站流量记录
func (r *outboundRepository) Update(traffic *model.OutboundTraffics) error {
	return r.db.Save(traffic).Error
}

// ResetTraffic 重置指定 tag 的流量
func (r *outboundRepository) ResetTraffic(tag string) error {
	return r.db.Model(model.OutboundTraffics{}).
		Where("tag = ?", tag).
		Updates(map[string]interface{}{"up": 0, "down": 0}).Error
}

// ResetAllTraffics 重置所有出站流量
func (r *outboundRepository) ResetAllTraffics() error {
	return r.db.Model(model.OutboundTraffics{}).
		Where("1 = 1").
		Updates(map[string]interface{}{"up": 0, "down": 0}).Error
}
