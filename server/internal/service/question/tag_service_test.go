package question

import (
	"context"
	"errors"
	"testing"
)

func TestCreateTagRejectsDuplicateNameInSameTenant(t *testing.T) {
	repo := &fakeTagRepository{
		tagNames: map[tagNameKey]bool{
			{tenantID: 10, name: "数学"}: true,
		},
	}
	svc := NewTagService(TagServiceOptions{Repo: repo})

	_, err := svc.CreateTag(context.Background(), CreateTagInput{
		TenantID: 10,
		Name:     "数学",
	})
	if !errors.Is(err, ErrTagNameDuplicated) {
		t.Fatalf("expected ErrTagNameDuplicated, got %v", err)
	}
	if repo.created.Name != "" {
		t.Fatalf("expected duplicate tag not created, got %#v", repo.created)
	}
}

func TestCreateTagAllowsSameNameInDifferentTenant(t *testing.T) {
	repo := &fakeTagRepository{
		tagNames: map[tagNameKey]bool{
			{tenantID: 10, name: "数学"}: true,
		},
	}
	svc := NewTagService(TagServiceOptions{Repo: repo})

	created, err := svc.CreateTag(context.Background(), CreateTagInput{
		TenantID: 20,
		Name:     "数学",
	})
	if err != nil {
		t.Fatalf("CreateTag returned error: %v", err)
	}
	if created.ID != 1 {
		t.Fatalf("expected tag ID 1, got %d", created.ID)
	}
	if repo.created.TenantID != 20 || repo.created.Name != "数学" {
		t.Fatalf("expected tenant 20 tag 数学, got %#v", repo.created)
	}
}

func TestSoftDeleteTagMarksTagDeleted(t *testing.T) {
	repo := &fakeTagRepository{}
	svc := NewTagService(TagServiceOptions{Repo: repo})

	if err := svc.SoftDeleteTag(context.Background(), 10, 99); err != nil {
		t.Fatalf("SoftDeleteTag returned error: %v", err)
	}
	if repo.deletedTenantID != 10 || repo.deletedTagID != 99 {
		t.Fatalf("expected soft delete tenant=10 tag=99, got tenant=%d tag=%d", repo.deletedTenantID, repo.deletedTagID)
	}
}

func TestListQuestionTagsUsesActiveTagJoin(t *testing.T) {
	repo := &fakeTagRepository{
		activeQuestionTags: []Tag{
			{ID: 1, TenantID: 10, Name: "数学"},
		},
	}
	svc := NewTagService(TagServiceOptions{Repo: repo})

	tags, err := svc.ListQuestionTags(context.Background(), 10, 100)
	if err != nil {
		t.Fatalf("ListQuestionTags returned error: %v", err)
	}
	if !repo.listActiveQuestionTagsCalled {
		t.Fatalf("expected repository to query active tags through tags join")
	}
	if len(tags) != 1 || tags[0].Name != "数学" {
		t.Fatalf("expected active tag 数学, got %#v", tags)
	}
}

func TestBindQuestionTagRejectsDuplicateBinding(t *testing.T) {
	repo := &fakeTagRepository{
		questionTagBindings: map[questionTagKey]bool{
			{tenantID: 10, questionID: 100, tagID: 1}: true,
		},
	}
	svc := NewTagService(TagServiceOptions{Repo: repo})

	err := svc.BindQuestionTag(context.Background(), BindQuestionTagInput{
		TenantID:   10,
		QuestionID: 100,
		TagID:      1,
	})
	if !errors.Is(err, ErrQuestionTagDuplicated) {
		t.Fatalf("expected ErrQuestionTagDuplicated, got %v", err)
	}
	if repo.boundTagID != 0 {
		t.Fatalf("expected duplicate binding not inserted, got tag ID %d", repo.boundTagID)
	}
}

func TestBindQuestionTagCreatesRelation(t *testing.T) {
	repo := &fakeTagRepository{}
	svc := NewTagService(TagServiceOptions{Repo: repo})

	err := svc.BindQuestionTag(context.Background(), BindQuestionTagInput{
		TenantID:   10,
		QuestionID: 100,
		TagID:      1,
	})
	if err != nil {
		t.Fatalf("BindQuestionTag returned error: %v", err)
	}
	if repo.boundTenantID != 10 || repo.boundQuestionID != 100 || repo.boundTagID != 1 {
		t.Fatalf("expected binding tenant=10 question=100 tag=1, got tenant=%d question=%d tag=%d", repo.boundTenantID, repo.boundQuestionID, repo.boundTagID)
	}
}

type fakeTagRepository struct {
	tagNames map[tagNameKey]bool
	created  Tag

	deletedTenantID uint64
	deletedTagID    uint64

	activeQuestionTags           []Tag
	listActiveQuestionTagsCalled bool

	questionTagBindings map[questionTagKey]bool
	boundTenantID       uint64
	boundQuestionID     uint64
	boundTagID          uint64
}

func (r *fakeTagRepository) ActiveTagNameExists(ctx context.Context, tenantID uint64, name string) (bool, error) {
	return r.tagNames[tagNameKey{tenantID: tenantID, name: name}], nil
}

func (r *fakeTagRepository) CreateTag(ctx context.Context, tag Tag) (Tag, error) {
	tag.ID = 1
	r.created = tag
	return tag, nil
}

func (r *fakeTagRepository) SoftDeleteTag(ctx context.Context, tenantID uint64, tagID uint64) error {
	r.deletedTenantID = tenantID
	r.deletedTagID = tagID
	return nil
}

func (r *fakeTagRepository) ListQuestionActiveTags(ctx context.Context, tenantID uint64, questionID uint64) ([]Tag, error) {
	r.listActiveQuestionTagsCalled = true
	return r.activeQuestionTags, nil
}

func (r *fakeTagRepository) QuestionTagExists(ctx context.Context, tenantID uint64, questionID uint64, tagID uint64) (bool, error) {
	return r.questionTagBindings[questionTagKey{tenantID: tenantID, questionID: questionID, tagID: tagID}], nil
}

func (r *fakeTagRepository) BindQuestionTag(ctx context.Context, tenantID uint64, questionID uint64, tagID uint64) error {
	r.boundTenantID = tenantID
	r.boundQuestionID = questionID
	r.boundTagID = tagID
	return nil
}

type tagNameKey struct {
	tenantID uint64
	name     string
}

type questionTagKey struct {
	tenantID   uint64
	questionID uint64
	tagID      uint64
}
