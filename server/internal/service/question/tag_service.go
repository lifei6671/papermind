package question

import (
	"context"
	"errors"
)

var (
	ErrTagNameDuplicated     = errors.New("tag name duplicated in tenant")
	ErrQuestionTagDuplicated = errors.New("question tag duplicated")
)

type Tag struct {
	ID       uint64 // 标签主键 ID。
	TenantID uint64 // 所属租户 ID。
	Name     string // 标签名称，例如知识点、章节、技能点。
}

type CreateTagInput struct {
	TenantID uint64 // 所属租户 ID。
	Name     string // 标签名称。
}

type BindQuestionTagInput struct {
	TenantID   uint64 // 所属租户 ID。
	QuestionID uint64 // 题目 ID。
	TagID      uint64 // 标签 ID。
}

type TagRepository interface {
	ActiveTagNameExists(ctx context.Context, tenantID uint64, name string) (bool, error)
	CreateTag(ctx context.Context, tag Tag) (Tag, error)
	SoftDeleteTag(ctx context.Context, tenantID uint64, tagID uint64) error
	ListQuestionActiveTags(ctx context.Context, tenantID uint64, questionID uint64) ([]Tag, error)
	QuestionTagExists(ctx context.Context, tenantID uint64, questionID uint64, tagID uint64) (bool, error)
	BindQuestionTag(ctx context.Context, tenantID uint64, questionID uint64, tagID uint64) error
}

type TagServiceOptions struct {
	Repo TagRepository
}

type TagService struct {
	repo TagRepository
}

func NewTagService(options TagServiceOptions) *TagService {
	return &TagService{repo: options.Repo}
}

func (s *TagService) CreateTag(ctx context.Context, input CreateTagInput) (Tag, error) {
	exists, err := s.repo.ActiveTagNameExists(ctx, input.TenantID, input.Name)
	if err != nil {
		return Tag{}, err
	}
	if exists {
		return Tag{}, ErrTagNameDuplicated
	}
	return s.repo.CreateTag(ctx, Tag{
		TenantID: input.TenantID,
		Name:     input.Name,
	})
}

func (s *TagService) SoftDeleteTag(ctx context.Context, tenantID uint64, tagID uint64) error {
	return s.repo.SoftDeleteTag(ctx, tenantID, tagID)
}

func (s *TagService) ListQuestionTags(ctx context.Context, tenantID uint64, questionID uint64) ([]Tag, error) {
	return s.repo.ListQuestionActiveTags(ctx, tenantID, questionID)
}

func (s *TagService) BindQuestionTag(ctx context.Context, input BindQuestionTagInput) error {
	exists, err := s.repo.QuestionTagExists(ctx, input.TenantID, input.QuestionID, input.TagID)
	if err != nil {
		return err
	}
	if exists {
		return ErrQuestionTagDuplicated
	}
	return s.repo.BindQuestionTag(ctx, input.TenantID, input.QuestionID, input.TagID)
}
