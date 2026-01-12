package service

import (
	"context"
	"x-ui/xray"

	"github.com/xtls/xray-core/app/proxyman/command"
	"google.golang.org/grpc"
)

type MockHandlerServiceClient struct {
	AddInboundFunc    func(ctx context.Context, in *command.AddInboundRequest, opts ...grpc.CallOption) (*command.AddInboundResponse, error)
	RemoveInboundFunc func(ctx context.Context, in *command.RemoveInboundRequest, opts ...grpc.CallOption) (*command.RemoveInboundResponse, error)
	AlterInboundFunc  func(ctx context.Context, in *command.AlterInboundRequest, opts ...grpc.CallOption) (*command.AlterInboundResponse, error)
}

func (m *MockHandlerServiceClient) AddInbound(ctx context.Context, in *command.AddInboundRequest, opts ...grpc.CallOption) (*command.AddInboundResponse, error) {
	if m.AddInboundFunc != nil {
		return m.AddInboundFunc(ctx, in, opts...)
	}
	return &command.AddInboundResponse{}, nil
}

func (m *MockHandlerServiceClient) RemoveInbound(ctx context.Context, in *command.RemoveInboundRequest, opts ...grpc.CallOption) (*command.RemoveInboundResponse, error) {
	if m.RemoveInboundFunc != nil {
		return m.RemoveInboundFunc(ctx, in, opts...)
	}
	return &command.RemoveInboundResponse{}, nil
}

func (m *MockHandlerServiceClient) AlterInbound(ctx context.Context, in *command.AlterInboundRequest, opts ...grpc.CallOption) (*command.AlterInboundResponse, error) {
	if m.AlterInboundFunc != nil {
		return m.AlterInboundFunc(ctx, in, opts...)
	}
	return &command.AlterInboundResponse{}, nil
}

func (m *MockHandlerServiceClient) ListInbounds(ctx context.Context, in *command.ListInboundsRequest, opts ...grpc.CallOption) (*command.ListInboundsResponse, error) {
	return &command.ListInboundsResponse{}, nil
}

func (m *MockHandlerServiceClient) GetInboundUsers(ctx context.Context, in *command.GetInboundUserRequest, opts ...grpc.CallOption) (*command.GetInboundUserResponse, error) {
	return &command.GetInboundUserResponse{}, nil
}

func (m *MockHandlerServiceClient) GetInboundUsersCount(ctx context.Context, in *command.GetInboundUserRequest, opts ...grpc.CallOption) (*command.GetInboundUsersCountResponse, error) {
	return &command.GetInboundUsersCountResponse{}, nil
}

func (m *MockHandlerServiceClient) AddOutbound(ctx context.Context, in *command.AddOutboundRequest, opts ...grpc.CallOption) (*command.AddOutboundResponse, error) {
	return &command.AddOutboundResponse{}, nil
}

func (m *MockHandlerServiceClient) RemoveOutbound(ctx context.Context, in *command.RemoveOutboundRequest, opts ...grpc.CallOption) (*command.RemoveOutboundResponse, error) {
	return &command.RemoveOutboundResponse{}, nil
}

func (m *MockHandlerServiceClient) AlterOutbound(ctx context.Context, in *command.AlterOutboundRequest, opts ...grpc.CallOption) (*command.AlterOutboundResponse, error) {
	return &command.AlterOutboundResponse{}, nil
}

func (m *MockHandlerServiceClient) ListOutbounds(ctx context.Context, in *command.ListOutboundsRequest, opts ...grpc.CallOption) (*command.ListOutboundsResponse, error) {
	return &command.ListOutboundsResponse{}, nil
}

// MockXrayWrapper wraps xray.XrayAPI to satisfy xray.API interface
// and override Init/Close to preserve mock state.
type MockXrayWrapper struct {
	// Embed the real struct (or pointer to it) to inherit AddInbound etc.
	// Note: We need to use type embedding to promote methods.
	// But xray.XrayAPI methods are defined on *XrayAPI.
	// So we embed *xray.XrayAPI.
	// However, importing "x-ui/xray" creates import cycle if mock is in "x-ui/xray"?
	// This file is in "service" package, so importing "x-ui/xray" is fine.
	*xray.XrayAPI
}

// Override Init to do nothing (or succeed)
func (m *MockXrayWrapper) Init(apiPort int) error {
	// We assume HandlerServiceClient is already set up manually.
	return nil
}

// Override Close to do nothing
func (m *MockXrayWrapper) Close() {
	// Do NOT clear the clients
}

// Add these if they are part of the interface but not used yet, to satisfy the interface.
// Assuming the interface matches what's used in api.go.
// If `HandlerServiceClient` has other methods, I might need to implement them too.
// I will start with these and let the compiler tell me what's missing.
