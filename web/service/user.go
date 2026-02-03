package service

import (
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/database/repository"
	"x-ui/logger"
	"x-ui/util/common"
	"x-ui/util/crypto"

	"github.com/xlzd/gotp"
	"gorm.io/gorm"
)

type UserService struct {
	settingService *SettingService
	userRepo       repository.UserRepository
}

// NewUserService 创建 UserService 实例，通过构造函数注入依赖
func NewUserService(userRepo repository.UserRepository, settingService *SettingService) *UserService {
	return &UserService{
		userRepo:       userRepo,
		settingService: settingService,
	}
}

// getUserRepo 返回 UserRepository，支持延迟初始化以保持向后兼容
func (s *UserService) getUserRepo() repository.UserRepository {
	if s.userRepo == nil {
		s.userRepo = repository.NewUserRepository()
	}
	return s.userRepo
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	return s.getUserRepo().FindFirst()
}

func (s *UserService) CheckUser(username string, password string, twoFactorCode string) *model.User {
	user, err := s.getUserRepo().FindByUsername(username)
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}

	if !crypto.CheckPasswordHash(user.Password, password) {
		return nil
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		logger.Warning("check two factor err:", err)
		return nil
	}

	if twoFactorEnable {
		twoFactorToken, err := s.settingService.GetTwoFactorToken()
		if err != nil {
			logger.Warning("check two factor token err:", err)
			return nil
		}

		if gotp.NewDefaultTOTP(twoFactorToken).Now() != twoFactorCode {
			return nil
		}
	}

	return user
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	hashedPassword, err := crypto.HashPasswordAsBcrypt(password)
	if err != nil {
		return err
	}

	twoFactorEnable, err := s.settingService.GetTwoFactorEnable()
	if err != nil {
		return err
	}

	if twoFactorEnable {
		_ = s.settingService.SetTwoFactorEnable(false)
		_ = s.settingService.SetTwoFactorToken("")
	}

	return s.getUserRepo().GetDB().Model(model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{"username": username, "password": hashedPassword}).
		Error
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return common.NewError("username can not be empty")
	} else if password == "" {
		return common.NewError("password can not be empty")
	}
	hashedPassword, er := crypto.HashPasswordAsBcrypt(password)

	if er != nil {
		return er
	}

	user, err := s.getUserRepo().FindFirst()
	if database.IsNotFound(err) {
		user = &model.User{
			Username: username,
			Password: hashedPassword,
		}
		return s.getUserRepo().Create(user)
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = hashedPassword
	return s.getUserRepo().Update(user)
}
