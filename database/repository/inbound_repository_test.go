package repository

import (
	"testing"

	"x-ui/database"
	"x-ui/database/model"

	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) {
	err := database.InitDB(":memory:")
	assert.NoError(t, err)
}

func TestInboundRepository_CRUD(t *testing.T) {
	setupTestDB(t)
	repo := NewInboundRepository()

	// Create
	inbound := &model.Inbound{
		UserId:   1,
		Remark:   "test-inbound",
		Port:     10000,
		Protocol: "vless",
		Enable:   true,
		Tag:      "test-tag",
	}
	err := repo.Create(inbound)
	assert.NoError(t, err)
	assert.Greater(t, inbound.Id, 0)

	// FindByID
	found, err := repo.FindByID(inbound.Id)
	assert.NoError(t, err)
	assert.Equal(t, "test-inbound", found.Remark)

	// FindByTag
	foundByTag, err := repo.FindByTag("test-tag")
	assert.NoError(t, err)
	assert.Equal(t, inbound.Id, foundByTag.Id)

	// Update
	inbound.Remark = "updated-inbound"
	err = repo.Update(inbound)
	assert.NoError(t, err)

	updated, err := repo.FindByID(inbound.Id)
	assert.NoError(t, err)
	assert.Equal(t, "updated-inbound", updated.Remark)

	// FindAll
	all, err := repo.FindAll()
	assert.NoError(t, err)
	assert.Len(t, all, 1)

	// Search
	searched, err := repo.Search("updated")
	assert.NoError(t, err)
	assert.Len(t, searched, 1)

	// GetAllTags
	tags, err := repo.GetAllTags()
	assert.NoError(t, err)
	assert.Contains(t, tags, "test-tag")

	// GetAllIDs
	ids, err := repo.GetAllIDs()
	assert.NoError(t, err)
	assert.Contains(t, ids, inbound.Id)

	// CheckPortExist
	exists, err := repo.CheckPortExist("", 10000, 0)
	assert.NoError(t, err)
	assert.True(t, exists)

	notExists, err := repo.CheckPortExist("", 10001, 0)
	assert.NoError(t, err)
	assert.False(t, notExists)

	// Delete
	err = repo.Delete(inbound.Id)
	assert.NoError(t, err)

	_, err = repo.FindByID(inbound.Id)
	assert.Error(t, err)
}

func TestInboundRepository_CheckPortExist_WithListen(t *testing.T) {
	setupTestDB(t)
	repo := NewInboundRepository()

	// Create inbound with specific listen address
	inbound := &model.Inbound{
		UserId:   1,
		Remark:   "test-listen",
		Listen:   "127.0.0.1",
		Port:     20000,
		Protocol: "vless",
		Enable:   true,
		Tag:      "test-listen-tag",
	}
	err := repo.Create(inbound)
	assert.NoError(t, err)

	// Same port, different listen - should not exist
	exists, err := repo.CheckPortExist("192.168.1.1", 20000, 0)
	assert.NoError(t, err)
	assert.False(t, exists)

	// Same port, same listen - should exist
	exists, err = repo.CheckPortExist("127.0.0.1", 20000, 0)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Same port, ignore self - should not exist
	exists, err = repo.CheckPortExist("127.0.0.1", 20000, inbound.Id)
	assert.NoError(t, err)
	assert.False(t, exists)
}
