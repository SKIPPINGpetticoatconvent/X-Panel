package service

import (
	"x-ui/database/model"
)

type MockTelegramService struct {
	SendMessageFunc        func(msg string) error
	IsRunningFunc          func() bool
	SendOneClickConfigFunc func(inbound *model.Inbound, inFromPanel bool, chatId int64) error
	GetDomainFunc          func() (string, error)
}

func (m *MockTelegramService) SendMessage(msg string) error {
	if m.SendMessageFunc != nil {
		return m.SendMessageFunc(msg)
	}
	return nil
}

func (m *MockTelegramService) IsRunning() bool {
	if m.IsRunningFunc != nil {
		return m.IsRunningFunc()
	}
	return true
}

func (m *MockTelegramService) SendOneClickConfig(inbound *model.Inbound, inFromPanel bool, chatId int64) error {
	if m.SendOneClickConfigFunc != nil {
		return m.SendOneClickConfigFunc(inbound, inFromPanel, chatId)
	}
	return nil
}

func (m *MockTelegramService) GetDomain() (string, error) {
	if m.GetDomainFunc != nil {
		return m.GetDomainFunc()
	}
	return "example.com", nil
}
