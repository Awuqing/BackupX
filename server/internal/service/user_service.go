package service

import (
	"context"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
)

// UserService 管理账号（admin 专属）。
// 初始化阶段（无用户）由 AuthService.Setup 负责创建首个管理员，本服务从第二个用户开始。
type UserService struct {
	users repository.UserRepository
}

func NewUserService(users repository.UserRepository) *UserService {
	return &UserService{users: users}
}

// UserSummary 用户列表项（不含密码哈希）。
type UserSummary struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
	CreatedAt   string `json:"createdAt"`
}

// UserUpsertInput 创建/更新用户的输入。
type UserUpsertInput struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Password    string `json:"password" binding:"omitempty,min=8,max=128"`
	DisplayName string `json:"displayName" binding:"required,min=1,max=128"`
	Email       string `json:"email" binding:"omitempty,max=255"`
	Role        string `json:"role" binding:"required,oneof=admin operator viewer"`
	Disabled    bool   `json:"disabled"`
}

func (s *UserService) List(ctx context.Context) ([]UserSummary, error) {
	items, err := s.users.List(ctx)
	if err != nil {
		return nil, apperror.Internal("USER_LIST_FAILED", "无法获取用户列表", err)
	}
	result := make([]UserSummary, 0, len(items))
	for i := range items {
		result = append(result, toUserSummary(&items[i]))
	}
	return result, nil
}

func (s *UserService) Create(ctx context.Context, input UserUpsertInput) (*UserSummary, error) {
	if !model.IsValidRole(input.Role) {
		return nil, apperror.BadRequest("USER_INVALID", "非法的角色", nil)
	}
	if strings.TrimSpace(input.Password) == "" {
		return nil, apperror.BadRequest("USER_INVALID", "创建用户必须指定密码", nil)
	}
	existing, err := s.users.FindByUsername(ctx, strings.TrimSpace(input.Username))
	if err != nil {
		return nil, apperror.Internal("USER_LOOKUP_FAILED", "无法校验用户名", err)
	}
	if existing != nil {
		return nil, apperror.Conflict("USER_USERNAME_EXISTS", "用户名已存在", nil)
	}
	hash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, apperror.Internal("USER_HASH_FAILED", "无法处理密码", err)
	}
	user := &model.User{
		Username:     strings.TrimSpace(input.Username),
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		Email:        strings.TrimSpace(input.Email),
		Role:         input.Role,
		Disabled:     input.Disabled,
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, apperror.Internal("USER_CREATE_FAILED", "无法创建用户", err)
	}
	summary := toUserSummary(user)
	return &summary, nil
}

func (s *UserService) Update(ctx context.Context, id uint, input UserUpsertInput) (*UserSummary, error) {
	existing, err := s.users.FindByID(ctx, id)
	if err != nil {
		return nil, apperror.Internal("USER_GET_FAILED", "无法获取用户", err)
	}
	if existing == nil {
		return nil, apperror.New(404, "USER_NOT_FOUND", "用户不存在", nil)
	}
	if !model.IsValidRole(input.Role) {
		return nil, apperror.BadRequest("USER_INVALID", "非法的角色", nil)
	}
	// 校验用户名冲突
	if strings.TrimSpace(input.Username) != existing.Username {
		dup, err := s.users.FindByUsername(ctx, strings.TrimSpace(input.Username))
		if err != nil {
			return nil, apperror.Internal("USER_LOOKUP_FAILED", "无法校验用户名", err)
		}
		if dup != nil {
			return nil, apperror.Conflict("USER_USERNAME_EXISTS", "用户名已存在", nil)
		}
	}
	existing.Username = strings.TrimSpace(input.Username)
	existing.DisplayName = strings.TrimSpace(input.DisplayName)
	existing.Email = strings.TrimSpace(input.Email)
	existing.Role = input.Role
	existing.Disabled = input.Disabled
	if strings.TrimSpace(input.Password) != "" {
		hash, err := security.HashPassword(input.Password)
		if err != nil {
			return nil, apperror.Internal("USER_HASH_FAILED", "无法处理密码", err)
		}
		existing.PasswordHash = hash
	}
	if err := s.users.Update(ctx, existing); err != nil {
		return nil, apperror.Internal("USER_UPDATE_FAILED", "无法更新用户", err)
	}
	summary := toUserSummary(existing)
	return &summary, nil
}

func (s *UserService) Delete(ctx context.Context, id uint) error {
	existing, err := s.users.FindByID(ctx, id)
	if err != nil {
		return apperror.Internal("USER_GET_FAILED", "无法获取用户", err)
	}
	if existing == nil {
		return apperror.New(404, "USER_NOT_FOUND", "用户不存在", nil)
	}
	// 禁止删除系统中最后一个 admin（防止系统失权）
	if existing.Role == model.UserRoleAdmin {
		count, err := s.users.CountByRole(ctx, model.UserRoleAdmin)
		if err != nil {
			return apperror.Internal("USER_COUNT_FAILED", "无法统计管理员数量", err)
		}
		if count <= 1 {
			return apperror.BadRequest("USER_LAST_ADMIN", "不能删除系统最后一个管理员", nil)
		}
	}
	return s.users.Delete(ctx, id)
}

func toUserSummary(u *model.User) UserSummary {
	return UserSummary{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Role:        u.Role,
		Disabled:    u.Disabled,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
