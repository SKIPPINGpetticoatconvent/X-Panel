package service

import (
	"x-ui/database/model"
	"x-ui/database/repository"
	"x-ui/logger"
	"x-ui/xray"

	"gorm.io/gorm"
)

type OutboundService struct {
	outboundRepo repository.OutboundRepository
}

// getOutboundRepo 延迟初始化并返回 OutboundRepository
func (s *OutboundService) getOutboundRepo() repository.OutboundRepository {
	if s.outboundRepo == nil {
		s.outboundRepo = repository.NewOutboundRepository()
	}
	return s.outboundRepo
}

func (s *OutboundService) AddTraffic(traffics []*xray.Traffic, clientTraffics []*xray.ClientTraffic) (error, bool) {
	var err error
	db := s.getOutboundRepo().GetDB()
	tx := db.Begin()

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			tx.Commit()
		}
	}()

	err = s.addOutboundTraffic(tx, traffics)
	if err != nil {
		return err, false
	}

	return nil, false
}

func (s *OutboundService) addOutboundTraffic(tx *gorm.DB, traffics []*xray.Traffic) error {
	if len(traffics) == 0 {
		return nil
	}

	var err error

	for _, traffic := range traffics {
		if traffic.IsOutbound {

			var outbound model.OutboundTraffics

			err = tx.Model(&model.OutboundTraffics{}).Where("tag = ?", traffic.Tag).
				FirstOrCreate(&outbound).Error
			if err != nil {
				return err
			}

			outbound.Tag = traffic.Tag
			outbound.Up = outbound.Up + traffic.Up
			outbound.Down = outbound.Down + traffic.Down
			outbound.Total = outbound.Up + outbound.Down

			err = tx.Save(&outbound).Error
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *OutboundService) GetOutboundsTraffic() ([]*model.OutboundTraffics, error) {
	traffics, err := s.getOutboundRepo().FindAll()
	if err != nil {
		logger.Warning("Error retrieving OutboundTraffics: ", err)
		return nil, err
	}
	return traffics, nil
}

func (s *OutboundService) ResetOutboundTraffic(tag string) error {
	if tag == "-alltags-" {
		return s.getOutboundRepo().ResetAllTraffics()
	}
	return s.getOutboundRepo().ResetTraffic(tag)
}
