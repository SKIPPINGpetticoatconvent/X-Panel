package repository

import (
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/xray"

	"gorm.io/gorm"
)

// InboundRepository 定义 Inbound 数据访问接口
type InboundRepository interface {
	// 基础 CRUD 操作
	FindByID(id int) (*model.Inbound, error)
	FindByUserID(userId int) ([]*model.Inbound, error)
	FindAll() ([]*model.Inbound, error)
	FindByTag(tag string) (*model.Inbound, error)
	Create(inbound *model.Inbound) error
	Update(inbound *model.Inbound) error
	Delete(id int) error

	// 查询操作
	Search(query string) ([]*model.Inbound, error)
	GetAllTags() ([]string, error)
	GetAllIDs() ([]int, error)
	CheckPortExist(listen string, port int, ignoreId int) (bool, error)

	// 事务支持
	WithTx(tx *gorm.DB) InboundRepository
	GetDB() *gorm.DB
}

// inboundRepository 实现 InboundRepository 接口
type inboundRepository struct {
	db *gorm.DB
}

// NewInboundRepository 创建新的 InboundRepository 实例
func NewInboundRepository() InboundRepository {
	return &inboundRepository{
		db: database.GetDB(),
	}
}

// WithTx 返回使用指定事务的新 Repository 实例
func (r *inboundRepository) WithTx(tx *gorm.DB) InboundRepository {
	return &inboundRepository{db: tx}
}

// GetDB 返回当前数据库连接
func (r *inboundRepository) GetDB() *gorm.DB {
	return r.db
}

// FindByID 根据 ID 查找 Inbound
func (r *inboundRepository) FindByID(id int) (*model.Inbound, error) {
	inbound := &model.Inbound{}
	err := r.db.Model(model.Inbound{}).Preload("ClientStats").First(inbound, id).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

// FindByUserID 根据用户 ID 查找所有 Inbound
func (r *inboundRepository) FindByUserID(userId int) ([]*model.Inbound, error) {
	var inbounds []*model.Inbound
	err := r.db.Model(model.Inbound{}).Preload("ClientStats").Where("user_id = ?", userId).Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

// FindAll 查找所有 Inbound
func (r *inboundRepository) FindAll() ([]*model.Inbound, error) {
	var inbounds []*model.Inbound
	err := r.db.Model(model.Inbound{}).Preload("ClientStats").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

// FindByTag 根据 Tag 查找 Inbound
func (r *inboundRepository) FindByTag(tag string) (*model.Inbound, error) {
	inbound := &model.Inbound{}
	err := r.db.Model(model.Inbound{}).Where("tag = ?", tag).First(inbound).Error
	if err != nil {
		return nil, err
	}
	return inbound, nil
}

// Create 创建新的 Inbound
func (r *inboundRepository) Create(inbound *model.Inbound) error {
	return r.db.Create(inbound).Error
}

// Update 更新 Inbound
func (r *inboundRepository) Update(inbound *model.Inbound) error {
	return r.db.Save(inbound).Error
}

// Delete 删除 Inbound
func (r *inboundRepository) Delete(id int) error {
	return r.db.Delete(model.Inbound{}, id).Error
}

// Search 搜索 Inbound
func (r *inboundRepository) Search(query string) ([]*model.Inbound, error) {
	var inbounds []*model.Inbound
	err := r.db.Model(model.Inbound{}).Preload("ClientStats").Where("remark like ?", "%"+query+"%").Find(&inbounds).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return inbounds, nil
}

// GetAllTags 获取所有 Inbound 的 Tag
func (r *inboundRepository) GetAllTags() ([]string, error) {
	var tags []string
	err := r.db.Model(model.Inbound{}).Select("tag").Find(&tags).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	return tags, nil
}

// GetAllIDs 获取所有 Inbound 的 ID
func (r *inboundRepository) GetAllIDs() ([]int, error) {
	var ids []int
	err := r.db.Model(model.Inbound{}).Pluck("id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// CheckPortExist 检查端口是否已存在
func (r *inboundRepository) CheckPortExist(listen string, port int, ignoreId int) (bool, error) {
	query := r.db.Model(model.Inbound{})

	if listen == "" || listen == "0.0.0.0" || listen == "::" || listen == "::0" {
		query = query.Where("port = ?", port)
	} else {
		query = query.Where("port = ?", port).
			Where(
				r.db.Where("listen = ?", listen).
					Or("listen = ?", "").
					Or("listen = ?", "0.0.0.0").
					Or("listen = ?", "::").
					Or("listen = ?", "::0"),
			)
	}

	if ignoreId > 0 {
		query = query.Where("id != ?", ignoreId)
	}

	var count int64
	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ClientTrafficRepository 定义 ClientTraffic 数据访问接口
type ClientTrafficRepository interface {
	FindByEmail(email string) (*xray.ClientTraffic, error)
	FindByInboundID(inboundId int) ([]*xray.ClientTraffic, error)
	FindByTgID(tgId int64) ([]*xray.ClientTraffic, error)
	Create(traffic *xray.ClientTraffic) error
	Update(traffic *xray.ClientTraffic) error
	Delete(id int) error
	ResetByEmail(email string) error
	ResetByInboundID(inboundId int) error
	UpdateTraffic(email string, upload, download int64) error
	GetLastOnline() (map[string]int64, error)

	WithTx(tx *gorm.DB) ClientTrafficRepository
}

// clientTrafficRepository 实现 ClientTrafficRepository 接口
type clientTrafficRepository struct {
	db *gorm.DB
}

// NewClientTrafficRepository 创建新的 ClientTrafficRepository 实例
func NewClientTrafficRepository() ClientTrafficRepository {
	return &clientTrafficRepository{
		db: database.GetDB(),
	}
}

// WithTx 返回使用指定事务的新 Repository 实例
func (r *clientTrafficRepository) WithTx(tx *gorm.DB) ClientTrafficRepository {
	return &clientTrafficRepository{db: tx}
}

// FindByEmail 根据 Email 查找 ClientTraffic
func (r *clientTrafficRepository) FindByEmail(email string) (*xray.ClientTraffic, error) {
	var traffics []*xray.ClientTraffic
	err := r.db.Model(xray.ClientTraffic{}).Where("email = ?", email).Find(&traffics).Error
	if err != nil {
		return nil, err
	}
	if len(traffics) == 0 {
		return nil, nil
	}
	return traffics[0], nil
}

// FindByInboundID 根据 InboundID 查找所有 ClientTraffic
func (r *clientTrafficRepository) FindByInboundID(inboundId int) ([]*xray.ClientTraffic, error) {
	var traffics []*xray.ClientTraffic
	err := r.db.Model(xray.ClientTraffic{}).Where("inbound_id = ?", inboundId).Find(&traffics).Error
	if err != nil {
		return nil, err
	}
	return traffics, nil
}

// FindByTgID 根据 TgID 查找所有 ClientTraffic
func (r *clientTrafficRepository) FindByTgID(tgId int64) ([]*xray.ClientTraffic, error) {
	var traffics []*xray.ClientTraffic
	err := r.db.Model(xray.ClientTraffic{}).Where("tg_id = ?", tgId).Find(&traffics).Error
	if err != nil {
		return nil, err
	}
	return traffics, nil
}

// Create 创建新的 ClientTraffic
func (r *clientTrafficRepository) Create(traffic *xray.ClientTraffic) error {
	return r.db.Create(traffic).Error
}

// Update 更新 ClientTraffic
func (r *clientTrafficRepository) Update(traffic *xray.ClientTraffic) error {
	return r.db.Save(traffic).Error
}

// Delete 删除 ClientTraffic
func (r *clientTrafficRepository) Delete(id int) error {
	return r.db.Delete(xray.ClientTraffic{}, id).Error
}

// ResetByEmail 根据 Email 重置流量
func (r *clientTrafficRepository) ResetByEmail(email string) error {
	return r.db.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"up":     0,
			"down":   0,
			"enable": true,
		}).Error
}

// ResetByInboundID 根据 InboundID 重置流量
func (r *clientTrafficRepository) ResetByInboundID(inboundId int) error {
	whereText := "inbound_id = ?"
	if inboundId == -1 {
		whereText = "inbound_id > ?"
		inboundId = 0
	}
	return r.db.Model(xray.ClientTraffic{}).
		Where(whereText, inboundId).
		Updates(map[string]interface{}{
			"up":     0,
			"down":   0,
			"enable": true,
		}).Error
}

// UpdateTraffic 更新流量统计
func (r *clientTrafficRepository) UpdateTraffic(email string, upload, download int64) error {
	return r.db.Model(xray.ClientTraffic{}).
		Where("email = ?", email).
		Updates(map[string]interface{}{
			"up":   gorm.Expr("up + ?", upload),
			"down": gorm.Expr("down + ?", download),
		}).Error
}

// GetLastOnline 获取所有客户端的最后在线时间
func (r *clientTrafficRepository) GetLastOnline() (map[string]int64, error) {
	var rows []xray.ClientTraffic
	err := r.db.Model(&xray.ClientTraffic{}).Select("email, last_online").Find(&rows).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	result := make(map[string]int64)
	for _, row := range rows {
		result[row.Email] = row.LastOnline
	}
	return result, nil
}

// ClientIPRepository 定义 ClientIP 数据访问接口
type ClientIPRepository interface {
	FindByEmail(email string) (*model.InboundClientIps, error)
	Create(clientIps *model.InboundClientIps) error
	Update(clientIps *model.InboundClientIps) error
	DeleteByEmail(email string) error
	ClearIPs(email string) error

	WithTx(tx *gorm.DB) ClientIPRepository
}

// clientIPRepository 实现 ClientIPRepository 接口
type clientIPRepository struct {
	db *gorm.DB
}

// NewClientIPRepository 创建新的 ClientIPRepository 实例
func NewClientIPRepository() ClientIPRepository {
	return &clientIPRepository{
		db: database.GetDB(),
	}
}

// WithTx 返回使用指定事务的新 Repository 实例
func (r *clientIPRepository) WithTx(tx *gorm.DB) ClientIPRepository {
	return &clientIPRepository{db: tx}
}

// FindByEmail 根据 Email 查找 ClientIPs
func (r *clientIPRepository) FindByEmail(email string) (*model.InboundClientIps, error) {
	clientIps := &model.InboundClientIps{}
	err := r.db.Model(model.InboundClientIps{}).Where("client_email = ?", email).First(clientIps).Error
	if err != nil {
		return nil, err
	}
	return clientIps, nil
}

// Create 创建新的 ClientIPs 记录
func (r *clientIPRepository) Create(clientIps *model.InboundClientIps) error {
	return r.db.Create(clientIps).Error
}

// Update 更新 ClientIPs 记录
func (r *clientIPRepository) Update(clientIps *model.InboundClientIps) error {
	return r.db.Save(clientIps).Error
}

// DeleteByEmail 根据 Email 删除 ClientIPs 记录
func (r *clientIPRepository) DeleteByEmail(email string) error {
	return r.db.Where("client_email = ?", email).Delete(model.InboundClientIps{}).Error
}

// ClearIPs 清空指定 Email 的 IP 记录
func (r *clientIPRepository) ClearIPs(email string) error {
	return r.db.Model(model.InboundClientIps{}).
		Where("client_email = ?", email).
		Update("ips", "").Error
}
