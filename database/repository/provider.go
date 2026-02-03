package repository

import (
	"github.com/google/wire"
)

// RepositorySet 包含所有 Repository 的 Provider
var RepositorySet = wire.NewSet(
	NewInboundRepository,
	NewOutboundRepository,
	NewSettingRepository,
	NewUserRepository,
	NewClientTrafficRepository,
	NewClientIPRepository,
)
