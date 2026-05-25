package db

import (
	"reflect"
	"strings"
	"testing"
)

func TestQuestionTablesUseExpectedNames(t *testing.T) {
	tests := []struct {
		name string
		do   interface{ TableName() string }
		want string
	}{
		{name: "question", do: QuestionDO{}, want: "questions"},
		{name: "question option", do: QuestionOptionDO{}, want: "question_options"},
		{name: "tag", do: TagDO{}, want: "tags"},
		{name: "question tag", do: QuestionTagDO{}, want: "question_tags"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.do.TableName(); got != tt.want {
				t.Fatalf("TableName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQuestionBusinessFieldsUseColumnMappings(t *testing.T) {
	tests := []struct {
		model any
		names []string
	}{
		{model: QuestionDO{}, names: []string{"TenantID", "SpaceID", "Type", "Difficulty", "Title", "Analysis", "ScoreDefault", "ChoiceDisplayCount", "ShuffleOptions", "Status"}},
		{model: QuestionOptionDO{}, names: []string{"TenantID", "QuestionID", "OptionKey", "SortOrder", "Content", "IsCorrect", "IsDistractor"}},
		{model: TagDO{}, names: []string{"TenantID", "Name"}},
		{model: QuestionTagDO{}, names: []string{"TenantID", "QuestionID", "TagID"}},
	}

	for _, tt := range tests {
		modelType := reflect.TypeOf(tt.model)
		for _, name := range tt.names {
			field, ok := modelType.FieldByName(name)
			if !ok {
				t.Fatalf("%s missing %s", modelType.Name(), name)
			}
			if !strings.Contains(field.Tag.Get("gorm"), "column:") {
				t.Fatalf("%s.%s gorm tag = %q", modelType.Name(), name, field.Tag.Get("gorm"))
			}
		}
	}
}

func TestQuestionColumnMappings(t *testing.T) {
	if QuestionColumns.Type != "type" {
		t.Fatalf("QuestionColumns.Type = %q", QuestionColumns.Type)
	}
	if QuestionColumns.Analysis != "analysis" {
		t.Fatalf("QuestionColumns.Analysis = %q", QuestionColumns.Analysis)
	}
	if QuestionOptionColumns.OptionKey != "option_key" {
		t.Fatalf("QuestionOptionColumns.OptionKey = %q", QuestionOptionColumns.OptionKey)
	}
	if QuestionOptionColumns.SortOrder != "sort_order" {
		t.Fatalf("QuestionOptionColumns.SortOrder = %q", QuestionOptionColumns.SortOrder)
	}
	if TagColumns.Name != "name" {
		t.Fatalf("TagColumns.Name = %q", TagColumns.Name)
	}
	if QuestionTagColumns.TagID != "tag_id" {
		t.Fatalf("QuestionTagColumns.TagID = %q", QuestionTagColumns.TagID)
	}
}

func TestQuestionSoftDeleteAndRelationBoundaries(t *testing.T) {
	assertHasEmbeddedField(t, reflect.TypeOf(QuestionDO{}), "SoftDeleteFields")
	assertHasEmbeddedField(t, reflect.TypeOf(TagDO{}), "SoftDeleteFields")
	assertHasEmbeddedField(t, reflect.TypeOf(QuestionOptionDO{}), "BaseFields")
	assertHasEmbeddedField(t, reflect.TypeOf(QuestionTagDO{}), "RelationFields")
	assertMissingField(t, reflect.TypeOf(QuestionTagDO{}), "UpdatedAt")
	assertMissingField(t, reflect.TypeOf(QuestionTagDO{}), "DeletedAt")
}

func TestQuestionNullableFieldsUsePointers(t *testing.T) {
	questionType := reflect.TypeOf(QuestionDO{})

	spaceID, ok := questionType.FieldByName("SpaceID")
	if !ok {
		t.Fatalf("QuestionDO missing SpaceID")
	}
	if spaceID.Type.Kind() != reflect.Pointer {
		t.Fatalf("QuestionDO.SpaceID type = %s, want pointer for nullable public-question marker", spaceID.Type)
	}

	choiceDisplayCount, ok := questionType.FieldByName("ChoiceDisplayCount")
	if !ok {
		t.Fatalf("QuestionDO missing ChoiceDisplayCount")
	}
	if choiceDisplayCount.Type.Kind() != reflect.Pointer {
		t.Fatalf("QuestionDO.ChoiceDisplayCount type = %s, want pointer for optional display count", choiceDisplayCount.Type)
	}
}
