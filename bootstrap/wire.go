//go:build wireinject
// +build wireinject

package bootstrap

import (
	"x-ui/database"
	"x-ui/database/repository"

	"github.com/google/wire"
)

func InitializeApp() (*App, error) {
	wire.Build(
		database.GetDBProvider,
		repository.RepositorySet,
		NewApp,
	)
	return nil, nil
}
