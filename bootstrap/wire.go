//go:build wireinject
// +build wireinject

package bootstrap

import (
	"x-ui/database"
	"x-ui/database/repository"
	"x-ui/web/service"

	"github.com/google/wire"
)

func InitializeApp() (*App, error) {
	wire.Build(
		database.GetDBProvider,
		repository.RepositorySet,
		service.ServiceSet,
		NewApp,
	)
	return nil, nil
}
