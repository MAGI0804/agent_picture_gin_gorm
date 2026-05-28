package tools

import (
	"context"

	agentsecurity "gin-biz-web-api/internal/service/agent_v2/security"
)

// StaticSafetyProvider adapts the local safety policy to the V2 tool interface.
type StaticSafetyProvider struct {
	policy agentsecurity.StaticSafetyPolicy
}

func NewStaticSafetyProvider(policy agentsecurity.StaticSafetyPolicy) *StaticSafetyProvider {
	return &StaticSafetyProvider{policy: policy}
}

func (provider *StaticSafetyProvider) CheckContent(ctx context.Context, request SafetyRequest) (SafetyResult, error) {
	result, err := provider.policy.CheckContent(ctx, agentsecurity.SafetyRequest{
		Text:     request.Text,
		ImageRef: request.ImageRef,
	})
	if err != nil {
		return SafetyResult{}, err
	}
	return SafetyResult{Allowed: result.Allowed, Reason: result.Reason}, nil
}
