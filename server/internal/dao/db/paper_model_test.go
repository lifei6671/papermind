package db

import (
	"reflect"
	"strings"
	"testing"
)

func TestPaperTablesUseExpectedNames(t *testing.T) {
	tests := []struct {
		name string
		do   interface{ TableName() string }
		want string
	}{
		{name: "paper", do: PaperDO{}, want: "papers"},
		{name: "paper section", do: PaperSectionDO{}, want: "paper_sections"},
		{name: "paper section question", do: PaperSectionQuestionDO{}, want: "paper_section_questions"},
		{name: "paper section rule", do: PaperSectionRuleDO{}, want: "paper_section_rules"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.do.TableName(); got != tt.want {
				t.Fatalf("TableName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPaperBusinessFieldsUseColumnMappings(t *testing.T) {
	tests := []struct {
		model any
		names []string
	}{
		{model: PaperDO{}, names: []string{"TenantID", "SpaceID", "Name", "Description", "DurationMinutes", "GradeLevel", "TotalScore", "BuildMode", "ShuffleQuestions", "ShowAnalysis", "Status"}},
		{model: PaperSectionDO{}, names: []string{"TenantID", "PaperID", "SortOrder", "Name", "QuestionType", "Instructions", "TotalScore", "QuestionCount"}},
		{model: PaperSectionQuestionDO{}, names: []string{"TenantID", "SectionID", "PaperID", "QuestionID", "SortOrder", "Score", "ShuffleOptions"}},
		{model: PaperSectionRuleDO{}, names: []string{"TenantID", "SectionID", "PaperID", "SortOrder", "Difficulty", "TagFilter", "QuestionCount", "ScorePerQuestion", "ShuffleOptions"}},
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

func TestPaperColumnMappings(t *testing.T) {
	if PaperColumns.BuildMode != "build_mode" {
		t.Fatalf("PaperColumns.BuildMode = %q", PaperColumns.BuildMode)
	}
	if PaperColumns.ShowAnalysis != "show_analysis" {
		t.Fatalf("PaperColumns.ShowAnalysis = %q", PaperColumns.ShowAnalysis)
	}
	if PaperColumns.DurationMinutes != "duration_minutes" {
		t.Fatalf("PaperColumns.DurationMinutes = %q", PaperColumns.DurationMinutes)
	}
	if PaperColumns.GradeLevel != "grade_level" {
		t.Fatalf("PaperColumns.GradeLevel = %q", PaperColumns.GradeLevel)
	}
	if PaperSectionColumns.QuestionType != "question_type" {
		t.Fatalf("PaperSectionColumns.QuestionType = %q", PaperSectionColumns.QuestionType)
	}
	if PaperSectionQuestionColumns.ShuffleOptions != "shuffle_options" {
		t.Fatalf("PaperSectionQuestionColumns.ShuffleOptions = %q", PaperSectionQuestionColumns.ShuffleOptions)
	}
	if PaperSectionRuleColumns.ScorePerQuestion != "score_per_question" {
		t.Fatalf("PaperSectionRuleColumns.ScorePerQuestion = %q", PaperSectionRuleColumns.ScorePerQuestion)
	}
}

func TestPaperSoftDeleteBoundaries(t *testing.T) {
	assertHasEmbeddedField(t, reflect.TypeOf(PaperDO{}), "SoftDeleteFields")
	assertHasEmbeddedField(t, reflect.TypeOf(PaperSectionDO{}), "SoftDeleteFields")
	assertMissingField(t, reflect.TypeOf(PaperSectionQuestionDO{}), "DeletedAt")
	assertMissingField(t, reflect.TypeOf(PaperSectionRuleDO{}), "DeletedAt")
}

func TestPaperNullableFieldsUsePointers(t *testing.T) {
	assertPointerField(t, reflect.TypeOf(PaperDO{}), "SpaceID")
	assertPointerField(t, reflect.TypeOf(PaperSectionQuestionDO{}), "ShuffleOptions")
	assertPointerField(t, reflect.TypeOf(PaperSectionRuleDO{}), "Difficulty")
	assertPointerField(t, reflect.TypeOf(PaperSectionRuleDO{}), "ShuffleOptions")
}

func assertPointerField(t *testing.T, modelType reflect.Type, fieldName string) {
	t.Helper()

	field, ok := modelType.FieldByName(fieldName)
	if !ok {
		t.Fatalf("%s missing %s", modelType.Name(), fieldName)
	}
	if field.Type.Kind() != reflect.Pointer {
		t.Fatalf("%s.%s type = %s, want pointer for nullable field", modelType.Name(), fieldName, field.Type)
	}
}
