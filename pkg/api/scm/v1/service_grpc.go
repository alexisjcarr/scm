package scmv1

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	ApplyService_ServiceName = "scm.v1.ApplyService"
	AgentService_ServiceName = "scm.v1.AgentService"
)

// ApplyServiceClient is the client contract for apply-oriented RPCs.
type ApplyServiceClient interface {
	SubmitApply(ctx context.Context, in *SubmitApplyRequest, opts ...grpc.CallOption) (*SubmitApplyResponse, error)
	GetApply(ctx context.Context, in *GetApplyRequest, opts ...grpc.CallOption) (*ApplySummary, error)
	StreamApplyEvents(ctx context.Context, in *StreamApplyEventsRequest, opts ...grpc.CallOption) (ApplyService_StreamApplyEventsClient, error)
}

type applyServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewApplyServiceClient(cc grpc.ClientConnInterface) ApplyServiceClient {
	return &applyServiceClient{cc: cc}
}

func (c *applyServiceClient) SubmitApply(ctx context.Context, in *SubmitApplyRequest, opts ...grpc.CallOption) (*SubmitApplyResponse, error) {
	out := new(SubmitApplyResponse)
	err := c.cc.Invoke(ctx, "/"+ApplyService_ServiceName+"/SubmitApply", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applyServiceClient) GetApply(ctx context.Context, in *GetApplyRequest, opts ...grpc.CallOption) (*ApplySummary, error) {
	out := new(ApplySummary)
	err := c.cc.Invoke(ctx, "/"+ApplyService_ServiceName+"/GetApply", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applyServiceClient) StreamApplyEvents(ctx context.Context, in *StreamApplyEventsRequest, opts ...grpc.CallOption) (ApplyService_StreamApplyEventsClient, error) {
	stream, err := c.cc.NewStream(ctx, &ApplyService_ServiceDesc.Streams[0], "/"+ApplyService_ServiceName+"/StreamApplyEvents", opts...)
	if err != nil {
		return nil, err
	}
	x := &applyServiceStreamApplyEventsClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// ApplyServiceServer is the server contract for apply-oriented RPCs.
type ApplyServiceServer interface {
	SubmitApply(context.Context, *SubmitApplyRequest) (*SubmitApplyResponse, error)
	GetApply(context.Context, *GetApplyRequest) (*ApplySummary, error)
	StreamApplyEvents(*StreamApplyEventsRequest, ApplyService_StreamApplyEventsServer) error
}

type UnimplementedApplyServiceServer struct{}

func (UnimplementedApplyServiceServer) SubmitApply(context.Context, *SubmitApplyRequest) (*SubmitApplyResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method SubmitApply not implemented")
}

func (UnimplementedApplyServiceServer) GetApply(context.Context, *GetApplyRequest) (*ApplySummary, error) {
	return nil, status.Error(codes.Unimplemented, "method GetApply not implemented")
}

func (UnimplementedApplyServiceServer) StreamApplyEvents(*StreamApplyEventsRequest, ApplyService_StreamApplyEventsServer) error {
	return status.Error(codes.Unimplemented, "method StreamApplyEvents not implemented")
}

func RegisterApplyServiceServer(s grpc.ServiceRegistrar, srv ApplyServiceServer) {
	s.RegisterService(&ApplyService_ServiceDesc, srv)
}

type ApplyService_StreamApplyEventsClient interface {
	Recv() (*ApplyEvent, error)
	grpc.ClientStream
}

type applyServiceStreamApplyEventsClient struct {
	grpc.ClientStream
}

func (x *applyServiceStreamApplyEventsClient) Recv() (*ApplyEvent, error) {
	m := new(ApplyEvent)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type ApplyService_StreamApplyEventsServer interface {
	Send(*ApplyEvent) error
	grpc.ServerStream
}

type applyServiceStreamApplyEventsServer struct {
	grpc.ServerStream
}

func (x *applyServiceStreamApplyEventsServer) Send(m *ApplyEvent) error {
	return x.ServerStream.SendMsg(m)
}

func _ApplyService_SubmitApply_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SubmitApplyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplyServiceServer).SubmitApply(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ApplyService_ServiceName + "/SubmitApply"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplyServiceServer).SubmitApply(ctx, req.(*SubmitApplyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplyService_GetApply_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetApplyRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplyServiceServer).GetApply(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ApplyService_ServiceName + "/GetApply"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplyServiceServer).GetApply(ctx, req.(*GetApplyRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplyService_StreamApplyEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StreamApplyEventsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ApplyServiceServer).StreamApplyEvents(m, &applyServiceStreamApplyEventsServer{ServerStream: stream})
}

var ApplyService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: ApplyService_ServiceName,
	HandlerType: (*ApplyServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "SubmitApply", Handler: _ApplyService_SubmitApply_Handler},
		{MethodName: "GetApply", Handler: _ApplyService_GetApply_Handler},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamApplyEvents",
			Handler:       _ApplyService_StreamApplyEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "proto/scm/v1/scm.proto",
}

type AgentServiceClient interface {
	RegisterAgent(ctx context.Context, in *RegisterAgentRequest, opts ...grpc.CallOption) (*RegisterAgentResponse, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
	FetchWork(ctx context.Context, in *FetchWorkRequest, opts ...grpc.CallOption) (*FetchWorkResponse, error)
	ReportWorkStatus(ctx context.Context, in *ReportWorkStatusRequest, opts ...grpc.CallOption) (*ReportWorkStatusResponse, error)
	ListAgents(ctx context.Context, in *ListAgentsRequest, opts ...grpc.CallOption) (*ListAgentsResponse, error)
}

type agentServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewAgentServiceClient(cc grpc.ClientConnInterface) AgentServiceClient {
	return &agentServiceClient{cc: cc}
}

func (c *agentServiceClient) RegisterAgent(ctx context.Context, in *RegisterAgentRequest, opts ...grpc.CallOption) (*RegisterAgentResponse, error) {
	out := new(RegisterAgentResponse)
	err := c.cc.Invoke(ctx, "/"+AgentService_ServiceName+"/RegisterAgent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentServiceClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
	out := new(HeartbeatResponse)
	err := c.cc.Invoke(ctx, "/"+AgentService_ServiceName+"/Heartbeat", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentServiceClient) FetchWork(ctx context.Context, in *FetchWorkRequest, opts ...grpc.CallOption) (*FetchWorkResponse, error) {
	out := new(FetchWorkResponse)
	err := c.cc.Invoke(ctx, "/"+AgentService_ServiceName+"/FetchWork", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentServiceClient) ReportWorkStatus(ctx context.Context, in *ReportWorkStatusRequest, opts ...grpc.CallOption) (*ReportWorkStatusResponse, error) {
	out := new(ReportWorkStatusResponse)
	err := c.cc.Invoke(ctx, "/"+AgentService_ServiceName+"/ReportWorkStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *agentServiceClient) ListAgents(ctx context.Context, in *ListAgentsRequest, opts ...grpc.CallOption) (*ListAgentsResponse, error) {
	out := new(ListAgentsResponse)
	err := c.cc.Invoke(ctx, "/"+AgentService_ServiceName+"/ListAgents", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type AgentServiceServer interface {
	RegisterAgent(context.Context, *RegisterAgentRequest) (*RegisterAgentResponse, error)
	Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
	FetchWork(context.Context, *FetchWorkRequest) (*FetchWorkResponse, error)
	ReportWorkStatus(context.Context, *ReportWorkStatusRequest) (*ReportWorkStatusResponse, error)
	ListAgents(context.Context, *ListAgentsRequest) (*ListAgentsResponse, error)
}

type UnimplementedAgentServiceServer struct{}

func (UnimplementedAgentServiceServer) RegisterAgent(context.Context, *RegisterAgentRequest) (*RegisterAgentResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method RegisterAgent not implemented")
}

func (UnimplementedAgentServiceServer) Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method Heartbeat not implemented")
}

func (UnimplementedAgentServiceServer) FetchWork(context.Context, *FetchWorkRequest) (*FetchWorkResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method FetchWork not implemented")
}

func (UnimplementedAgentServiceServer) ReportWorkStatus(context.Context, *ReportWorkStatusRequest) (*ReportWorkStatusResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ReportWorkStatus not implemented")
}

func (UnimplementedAgentServiceServer) ListAgents(context.Context, *ListAgentsRequest) (*ListAgentsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method ListAgents not implemented")
}

func RegisterAgentServiceServer(s grpc.ServiceRegistrar, srv AgentServiceServer) {
	s.RegisterService(&AgentService_ServiceDesc, srv)
}

func _AgentService_RegisterAgent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterAgentRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).RegisterAgent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + AgentService_ServiceName + "/RegisterAgent"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).RegisterAgent(ctx, req.(*RegisterAgentRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentService_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + AgentService_ServiceName + "/Heartbeat"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).Heartbeat(ctx, req.(*HeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentService_FetchWork_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FetchWorkRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).FetchWork(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + AgentService_ServiceName + "/FetchWork"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).FetchWork(ctx, req.(*FetchWorkRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentService_ReportWorkStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReportWorkStatusRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).ReportWorkStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + AgentService_ServiceName + "/ReportWorkStatus"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).ReportWorkStatus(ctx, req.(*ReportWorkStatusRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _AgentService_ListAgents_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListAgentsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(AgentServiceServer).ListAgents(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + AgentService_ServiceName + "/ListAgents"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(AgentServiceServer).ListAgents(ctx, req.(*ListAgentsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var AgentService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: AgentService_ServiceName,
	HandlerType: (*AgentServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "RegisterAgent", Handler: _AgentService_RegisterAgent_Handler},
		{MethodName: "Heartbeat", Handler: _AgentService_Heartbeat_Handler},
		{MethodName: "FetchWork", Handler: _AgentService_FetchWork_Handler},
		{MethodName: "ReportWorkStatus", Handler: _AgentService_ReportWorkStatus_Handler},
		{MethodName: "ListAgents", Handler: _AgentService_ListAgents_Handler},
	},
	Metadata: "proto/scm/v1/scm.proto",
}
