package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"backupx/server/internal/apperror"
	"backupx/server/internal/model"
	"backupx/server/internal/repository"
	"backupx/server/internal/security"
)

type SetupInput struct {
	Username    string `json:"username" binding:"required,min=3,max=64"`
	Password    string `json:"password" binding:"required,min=8,max=128"`
	DisplayName string `json:"displayName" binding:"required,min=1,max=128"`
}

type LoginInput struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

type AuthPayload struct {
	Token string      `json:"token"`
	User  *UserOutput `json:"user"`
}

type UserOutput struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
}

type AuthService struct {
	users       repository.UserRepository
	configs     repository.SystemConfigRepository
	jwtManager  *security.JWTManager
	rateLimiter *security.LoginRateLimiter
}

func NewAuthService(
	users repository.UserRepository,
	configs repository.SystemConfigRepository,
	jwtManager *security.JWTManager,
	rateLimiter *security.LoginRateLimiter,
) *AuthService {
	return &AuthService{users: users, configs: configs, jwtManager: jwtManager, rateLimiter: rateLimiter}
}

func (s *AuthService) SetupStatus(ctx context.Context) (bool, error) {
	count, err := s.users.Count(ctx)
	if err != nil {
		return false, apperror.Internal("AUTH_STATUS_FAILED", "无法检查初始化状态", err)
	}
	return count > 0, nil
}

func (s *AuthService) Setup(ctx context.Context, input SetupInput) (*AuthPayload, error) {
	initialized, err := s.SetupStatus(ctx)
	if err != nil {
		return nil, err
	}
	if initialized {
		return nil, apperror.Conflict("AUTH_SETUP_DISABLED", "系统已初始化，请直接登录", nil)
	}

	existing, err := s.users.FindByUsername(ctx, strings.TrimSpace(input.Username))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法检查账户状态", err)
	}
	if existing != nil {
		return nil, apperror.Conflict("AUTH_USERNAME_EXISTS", "用户名已存在", nil)
	}

	hash, err := security.HashPassword(input.Password)
	if err != nil {
		return nil, apperror.Internal("AUTH_HASH_FAILED", "无法处理密码", err)
	}

	user := &model.User{
		Username:     strings.TrimSpace(input.Username),
		PasswordHash: hash,
		DisplayName:  strings.TrimSpace(input.DisplayName),
		Role:         "admin",
	}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, apperror.Internal("AUTH_CREATE_USER_FAILED", "无法创建管理员账户", err)
	}

	token, err := s.jwtManager.Generate(user)
	if err != nil {
		return nil, apperror.Internal("AUTH_TOKEN_FAILED", "无法生成访问令牌", err)
	}

	return &AuthPayload{Token: token, User: ToUserOutput(user)}, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput, clientKey string) (*AuthPayload, error) {
	if clientKey == "" {
		clientKey = "unknown"
	}
	if !s.rateLimiter.Allow(clientKey) {
		return nil, apperror.TooManyRequests("AUTH_RATE_LIMITED", "登录尝试过于频繁，请稍后再试", nil)
	}

	user, err := s.users.FindByUsername(ctx, strings.TrimSpace(input.Username))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法执行登录校验", err)
	}
	if user == nil {
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误", nil)
	}
	if err := security.ComparePassword(user.PasswordHash, input.Password); err != nil {
		return nil, apperror.Unauthorized("AUTH_INVALID_CREDENTIALS", "用户名或密码错误", err)
	}

	s.rateLimiter.Reset(clientKey)
	token, err := s.jwtManager.Generate(user)
	if err != nil {
		return nil, apperror.Internal("AUTH_TOKEN_FAILED", "无法生成访问令牌", err)
	}
	return &AuthPayload{Token: token, User: ToUserOutput(user)}, nil
}

func (s *AuthService) GetCurrentUser(ctx context.Context, subject string) (*UserOutput, error) {
	userID, err := strconv.ParseUint(subject, 10, 64)
	if err != nil {
		return nil, apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效用户身份", err)
	}
	user, err := s.users.FindByID(ctx, uint(userID))
	if err != nil {
		return nil, apperror.Internal("AUTH_LOOKUP_FAILED", "无法获取当前用户", err)
	}
	if user == nil {
		return nil, apperror.Unauthorized("AUTH_USER_NOT_FOUND", "当前用户不存在", errors.New("user not found"))
	}
	return ToUserOutput(user), nil
}

type ChangePasswordInput struct {
	OldPassword string `json:"oldPassword" binding:"required,min=8,max=128"`
	NewPassword string `json:"newPassword" binding:"required,min=8,max=128"`
}

func (s *AuthService) ChangePassword(ctx context.Context, subject string, input ChangePasswordInput) error {
	userID, err := strconv.ParseUint(subject, 10, 64)
	if err != nil {
		return apperror.Unauthorized("AUTH_INVALID_SUBJECT", "无效用户身份", err)
	}
	user, err := s.users.FindByID(ctx, uint(userID))
	if err != nil {
		return apperror.Internal("AUTH_LOOKUP_FAILED", "无法获取当前用户", err)
	}
	if user == nil {
		return apperror.Unauthorized("AUTH_USER_NOT_FOUND", "当前用户不存在", errors.New("user not found"))
	}
	if err := security.ComparePassword(user.PasswordHash, input.OldPassword); err != nil {
		return apperror.BadRequest("AUTH_WRONG_PASSWORD", "旧密码不正确", err)
	}
	hash, err := security.HashPassword(input.NewPassword)
	if err != nil {
		return apperror.Internal("AUTH_HASH_FAILED", "无法处理密码", err)
	}
	user.PasswordHash = hash
	if err := s.users.Update(ctx, user); err != nil {
		return apperror.Internal("AUTH_UPDATE_FAILED", "密码修改失败", err)
	}
	return nil
}

func ToUserOutput(user *model.User) *UserOutput {
	if user == nil {
		return nil
	}
	return &UserOutput{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
	}
}

func SubjectFromContextValue(value any) (string, error) {
	subject, ok := value.(string)
	if !ok || strings.TrimSpace(subject) == "" {
		return "", fmt.Errorf("invalid subject context")
	}
	return subject, nil
}
