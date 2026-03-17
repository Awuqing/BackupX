package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"backupx/server/internal/apperror"
)

type providerFactoryWithNew interface {
	New(context.Context, map[string]any) (StorageProvider, error)
}

type providerFactoryWithCreate interface {
	Create(context.Context, json.RawMessage) (StorageProvider, error)
}

type providerFactoryWithSensitiveFields interface {
	SensitiveFields() []string
}

type providerFactoryWithSensitiveKeys interface {
	SensitiveKeys() []string
}

type providerFactoryWithValidate interface {
	Validate(json.RawMessage) error
}

type Registry struct {
	mu        sync.RWMutex
	factories map[ProviderType]ProviderFactory
}

func NewRegistry(factories ...ProviderFactory) *Registry {
	registry := &Registry{factories: make(map[ProviderType]ProviderFactory)}
	for _, factory := range factories {
		registry.Register(factory)
	}
	return registry
}

func (r *Registry) Register(factory ProviderFactory) {
	if factory == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.factories == nil {
		r.factories = make(map[ProviderType]ProviderFactory)
	}
	r.factories[factory.Type()] = factory
}

func (r *Registry) Factory(providerType string) (ProviderFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.factories[providerType]
	return factory, ok
}

func (r *Registry) Types() []ProviderType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]ProviderType, 0, len(r.factories))
	for providerType := range r.factories {
		items = append(items, providerType)
	}
	sort.Slice(items, func(i, j int) bool { return items[i] < items[j] })
	return items
}

func (r *Registry) SensitiveFields(providerType string) []string {
	factory, ok := r.Factory(providerType)
	if !ok {
		return nil
	}
	if typed, ok := factory.(providerFactoryWithSensitiveFields); ok {
		return typed.SensitiveFields()
	}
	if typed, ok := factory.(providerFactoryWithSensitiveKeys); ok {
		return typed.SensitiveKeys()
	}
	return nil
}

func (r *Registry) SensitiveKeys(providerType string) []string {
	return r.SensitiveFields(providerType)
}

func (r *Registry) Validate(providerType string, raw json.RawMessage) error {
	factory, ok := r.Factory(providerType)
	if !ok {
		return apperror.BadRequest("STORAGE_PROVIDER_UNSUPPORTED", "不支持的存储类型", fmt.Errorf("unsupported storage provider type: %s", providerType))
	}
	if typed, ok := factory.(providerFactoryWithValidate); ok {
		if err := typed.Validate(raw); err != nil {
			return apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "存储目标配置不合法", err)
		}
		return nil
	}
	configMap, err := decodeConfigMap(raw)
	if err != nil {
		return apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "存储目标配置不合法", err)
	}
	if typed, ok := factory.(providerFactoryWithNew); ok {
		if _, err := typed.New(context.Background(), configMap); err != nil {
			return apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "存储目标配置不合法", err)
		}
		return nil
	}
	return apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "存储目标配置不合法", fmt.Errorf("provider %s has no validator", providerType))
}

func (r *Registry) Create(ctx context.Context, providerType string, rawConfig any) (StorageProvider, error) {
	factory, ok := r.Factory(providerType)
	if !ok {
		return nil, apperror.BadRequest("STORAGE_PROVIDER_UNSUPPORTED", "不支持的存储类型", fmt.Errorf("unsupported storage provider type: %s", providerType))
	}
	raw, configMap, err := normalizeConfig(rawConfig)
	if err != nil {
		return nil, apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "存储目标配置不合法", err)
	}
	if typed, ok := factory.(providerFactoryWithNew); ok {
		provider, err := typed.New(ctx, configMap)
		if err != nil {
			return nil, apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "无法创建存储客户端", err)
		}
		return provider, nil
	}
	if typed, ok := factory.(providerFactoryWithCreate); ok {
		provider, err := typed.Create(ctx, raw)
		if err != nil {
			return nil, apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "无法创建存储客户端", err)
		}
		return provider, nil
	}
	return nil, apperror.BadRequest("STORAGE_TARGET_INVALID_CONFIG", "无法创建存储客户端", fmt.Errorf("provider %s has no constructor", providerType))
}

func normalizeConfig(rawConfig any) (json.RawMessage, map[string]any, error) {
	switch value := rawConfig.(type) {
	case nil:
		return json.RawMessage("{}"), map[string]any{}, nil
	case map[string]any:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal config: %w", err)
		}
		return raw, value, nil
	case json.RawMessage:
		configMap, err := decodeConfigMap(value)
		if err != nil {
			return nil, nil, err
		}
		return value, configMap, nil
	case []byte:
		raw := json.RawMessage(value)
		configMap, err := decodeConfigMap(raw)
		if err != nil {
			return nil, nil, err
		}
		return raw, configMap, nil
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal config: %w", err)
		}
		configMap, err := decodeConfigMap(raw)
		if err != nil {
			return nil, nil, err
		}
		return raw, configMap, nil
	}
}

func decodeConfigMap(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var configMap map[string]any
	if err := json.Unmarshal(raw, &configMap); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if configMap == nil {
		return map[string]any{}, nil
	}
	return configMap, nil
}
