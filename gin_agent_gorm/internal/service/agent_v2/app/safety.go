package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	agentsecurity "gin-biz-web-api/internal/service/agent_v2/security"
	"gin-biz-web-api/internal/service/agent_v2/tools"
	"gin-biz-web-api/pkg/config"
)

func registerSafetyTool(registry *tools.Registry, store tools.InvocationStore) error {
	if registry == nil {
		return nil
	}
	if isNilInvocationStore(store) {
		store = nil
	}
	policy := agentsecurity.StaticSafetyPolicy{
		Enabled:      configBool("cfg.ai_agent.safety.enabled", true),
		FailClosed:   configBool("cfg.ai_agent.safety.fail_closed", true),
		BlockedTerms: configStringSlice("cfg.ai_agent.safety.blocked_terms"),
	}
	return registry.Register(tools.InstrumentTool(tools.Tool{
		Name:     "static_safety_policy",
		Kind:     tools.KindSafety,
		Provider: "local",
		Model:    "static_safety_policy_v1",
		Capability: tools.Capability{
			CostPolicy: "local_safety",
		},
		SafetyProvider: tools.NewStaticSafetyProvider(policy),
	}, store))
}

func isNilInvocationStore(store tools.InvocationStore) bool {
	if store == nil {
		return true
	}
	value := reflect.ValueOf(store)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func configBool(path string, defaultValue bool) (value bool) {
	value = defaultValue
	defer func() {
		if recover() != nil {
			value = defaultValue
		}
	}()
	return config.GetBool(path, defaultValue)
}

func configStringSlice(path string) (value []string) {
	defer func() {
		if recover() != nil {
			value = []string{}
		}
	}()
	return config.GetStringSlice(path)
}

func (svc *Service) checkTextSafety(ctx context.Context, userID uint, prompt string) error {
	registry := tools.NewRegistry()
	if err := registerSafetyTool(registry, svc.dao); err != nil {
		return err
	}
	tool, err := registry.FindTool(tools.FindToolRequest{Kind: tools.KindSafety, UserID: userID})
	if err != nil {
		return err
	}
	result, err := tool.SafetyProvider.CheckContent(ctx, tools.SafetyRequest{
		UserID: userID,
		Text:   strings.TrimSpace(prompt),
	})
	if err != nil {
		return err
	}
	if !result.Allowed {
		return fmt.Errorf("文本安全检查拒绝内容: %s", result.Reason)
	}
	return nil
}

func (svc *Service) checkImageSafety(ctx context.Context, userID uint, imageRef string) error {
	registry := tools.NewRegistry()
	if err := registerSafetyTool(registry, svc.dao); err != nil {
		return err
	}
	tool, err := registry.FindTool(tools.FindToolRequest{Kind: tools.KindSafety, UserID: userID})
	if err != nil {
		return err
	}
	result, err := tool.SafetyProvider.CheckContent(ctx, tools.SafetyRequest{
		UserID:   userID,
		ImageRef: strings.TrimSpace(imageRef),
	})
	if err != nil {
		return err
	}
	if !result.Allowed {
		return fmt.Errorf("图片安全检查拒绝内容: %s", result.Reason)
	}
	return nil
}
