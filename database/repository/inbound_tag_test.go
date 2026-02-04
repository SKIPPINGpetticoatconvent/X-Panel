package repository

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

// InboundTagTestSuite 测试入站标签唯一性功能
type InboundTagTestSuite struct {
	suite.Suite
	db   *gorm.DB
	repo InboundRepository
}

// SetupSuite 设置测试套件
func (suite *InboundTagTestSuite) SetupSuite() {
	err := database.InitDB(":memory:")
	assert.NoError(suite.T(), err)

	db := database.GetDB()
	suite.db = db
	suite.repo = NewInboundRepository(db)
}

// TestCheckTagExist 测试标签存在性检查
func (suite *InboundTagTestSuite) TestCheckTagExist() {
	// 创建第一个入站
	inbound1 := &model.Inbound{
		UserId:   1,
		Remark:   "test-inbound-1",
		Enable:   true,
		Listen:   "",
		Port:     10000,
		Protocol: model.VMESS,
		Settings: `{"clients": []}`,
		Tag:      "test-tag-1",
		Sniffing: `{"enabled": false}`,
	}

	err := suite.repo.Create(inbound1)
	assert.NoError(suite.T(), err)
	assert.Greater(suite.T(), inbound1.Id, 0)

	// 测试检查存在的标签
	exist, err := suite.repo.CheckTagExist("test-tag-1", 0)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exist, "应该能找到已存在的标签")

	// 测试检查不存在的标签
	exist, err = suite.repo.CheckTagExist("non-existent-tag", 0)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exist, "不应该找到不存在的标签")

	// 测试忽略当前入站ID
	exist, err = suite.repo.CheckTagExist("test-tag-1", inbound1.Id)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exist, "忽略当前入站ID时，应该返回false")

	// 创建第二个入站
	inbound2 := &model.Inbound{
		UserId:   1,
		Remark:   "test-inbound-2",
		Enable:   true,
		Listen:   "",
		Port:     10001,
		Protocol: model.VMESS,
		Settings: `{"clients": []}`,
		Tag:      "test-tag-2",
		Sniffing: `{"enabled": false}`,
	}

	err = suite.repo.Create(inbound2)
	assert.NoError(suite.T(), err)

	// 测试检查第二个入站的标签
	exist, err = suite.repo.CheckTagExist("test-tag-2", 0)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exist, "应该能找到第二个入站的标签")

	// 测试检查第一个入站的标签时忽略第二个入站ID
	exist, err = suite.repo.CheckTagExist("test-tag-1", inbound2.Id)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), exist, "忽略不相关的ID时，应该仍能找到标签")
}

// TestCheckTagExistWithEmptyTag 测试空标签的情况
func (suite *InboundTagTestSuite) TestCheckTagExistWithEmptyTag() {
	// 创建没有标签的入站
	inbound := &model.Inbound{
		UserId:   1,
		Remark:   "test-inbound-no-tag",
		Enable:   true,
		Listen:   "",
		Port:     10002,
		Protocol: model.VMESS,
		Settings: `{"clients": []}`,
		Tag:      "",
		Sniffing: `{"enabled": false}`,
	}

	err := suite.repo.Create(inbound)
	assert.NoError(suite.T(), err)

	// 测试空标签
	exist, err := suite.repo.CheckTagExist("", 0)
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), exist, "空标签不应该被认为是存在的")
}

// TestInboundTagTestSuite 运行入站标签测试
func TestInboundTagTestSuite(t *testing.T) {
	suite.Run(t, new(InboundTagTestSuite))
}
