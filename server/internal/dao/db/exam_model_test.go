package db

import (
	"reflect"
	"strings"
	"testing"

	"gorm.io/datatypes"
)

func TestExamTablesUseExpectedNames(t *testing.T) {
	tests := []struct {
		name string
		do   interface{ TableName() string }
		want string
	}{
		{name: "exam", do: ExamDO{}, want: "exams"},
		{name: "exam target", do: ExamTargetDO{}, want: "exam_targets"},
		{name: "exam live question pool", do: ExamLiveQuestionPoolDO{}, want: "exam_live_question_pools"},
		{name: "exam attempt", do: ExamAttemptDO{}, want: "exam_attempts"},
		{name: "exam attempt question", do: ExamAttemptQuestionDO{}, want: "exam_attempt_questions"},
		{name: "exam answer", do: ExamAnswerDO{}, want: "exam_answers"},
		{name: "exam event", do: ExamEventDO{}, want: "exam_events"},
		{name: "exam operation log", do: ExamOperationLogDO{}, want: "exam_operation_logs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.do.TableName(); got != tt.want {
				t.Fatalf("TableName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExamBusinessFieldsUseColumnMappings(t *testing.T) {
	tests := []struct {
		model any
		names []string
	}{
		{model: ExamDO{}, names: []string{"TenantID", "PaperID", "Name", "StartTime", "EndTime", "DurationMinutes", "MaxAttempts", "ResultStrategy", "PublishMode", "ScorePublishTime", "InviteCode", "Status"}},
		{model: ExamTargetDO{}, names: []string{"TenantID", "ExamID", "TargetType", "TargetID"}},
		{model: ExamLiveQuestionPoolDO{}, names: []string{"TenantID", "ExamID", "SectionID", "RuleID", "QuestionID"}},
		{model: ExamAttemptDO{}, names: []string{"TenantID", "ExamID", "UserID", "AttemptNo", "Status", "StartedAt", "SubmittedAt", "ExamTokenHash", "ExamTokenExpiresAt", "ObjectiveScore", "SubjectiveScore", "TotalScore"}},
		{model: ExamAttemptQuestionDO{}, names: []string{"TenantID", "AttemptID", "SectionID", "QuestionID", "SectionSnapshot", "SortOrder", "Score", "QuestionSnapshot", "OptionSnapshot", "CorrectAnswerSnapshot"}},
		{model: ExamAnswerDO{}, names: []string{"TenantID", "AttemptID", "AttemptQuestionID", "AnswerContent", "Score", "GradingStatus", "GradedBy", "GradedAt", "GraderComment"}},
		{model: ExamEventDO{}, names: []string{"TenantID", "AttemptID", "EventType", "EventTime", "Payload"}},
		{model: ExamOperationLogDO{}, names: []string{"TenantID", "ExamID", "OperationType", "OperationTitle", "OperationDetail", "ActorID", "ActorType", "ActorRole", "SpaceID", "OperationGroupID"}},
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

func TestExamColumnMappings(t *testing.T) {
	if ExamColumns.MaxAttempts != "max_attempts" {
		t.Fatalf("ExamColumns.MaxAttempts = %q", ExamColumns.MaxAttempts)
	}
	if ExamAttemptColumns.ExamTokenHash != "exam_token_hash" {
		t.Fatalf("ExamAttemptColumns.ExamTokenHash = %q", ExamAttemptColumns.ExamTokenHash)
	}
	if ExamAttemptQuestionColumns.SectionSnapshot != "section_snapshot" {
		t.Fatalf("ExamAttemptQuestionColumns.SectionSnapshot = %q", ExamAttemptQuestionColumns.SectionSnapshot)
	}
	if ExamAnswerColumns.AttemptQuestionID != "attempt_question_id" {
		t.Fatalf("ExamAnswerColumns.AttemptQuestionID = %q", ExamAnswerColumns.AttemptQuestionID)
	}
	if ExamEventColumns.EventType != "event_type" {
		t.Fatalf("ExamEventColumns.EventType = %q", ExamEventColumns.EventType)
	}
	if ExamOperationLogColumns.OperationType != "operation_type" {
		t.Fatalf("ExamOperationLogColumns.OperationType = %q", ExamOperationLogColumns.OperationType)
	}
	if ExamOperationLogColumns.OperationGroupID != "operation_group_id" {
		t.Fatalf("ExamOperationLogColumns.OperationGroupID = %q", ExamOperationLogColumns.OperationGroupID)
	}
}

func TestExamFieldBoundaryTypes(t *testing.T) {
	assertHasEmbeddedField(t, reflect.TypeOf(ExamDO{}), "SoftDeleteFields")
	assertHasEmbeddedField(t, reflect.TypeOf(ExamTargetDO{}), "RelationFields")
	assertHasEmbeddedField(t, reflect.TypeOf(ExamLiveQuestionPoolDO{}), "RelationFields")
	assertHasEmbeddedField(t, reflect.TypeOf(ExamEventDO{}), "EventFields")
	assertHasEmbeddedField(t, reflect.TypeOf(ExamOperationLogDO{}), "EventFields")
	assertMissingField(t, reflect.TypeOf(ExamAttemptDO{}), "DeletedAt")

	assertPointerField(t, reflect.TypeOf(ExamDO{}), "ScorePublishTime")
	assertPointerField(t, reflect.TypeOf(ExamAttemptDO{}), "SubmittedAt")
	assertPointerField(t, reflect.TypeOf(ExamAnswerDO{}), "GradedAt")

	assertJSONField(t, reflect.TypeOf(ExamAttemptQuestionDO{}), "SectionSnapshot")
	assertJSONField(t, reflect.TypeOf(ExamAttemptQuestionDO{}), "QuestionSnapshot")
	assertJSONField(t, reflect.TypeOf(ExamEventDO{}), "Payload")
}

func assertJSONField(t *testing.T, modelType reflect.Type, fieldName string) {
	t.Helper()

	field, ok := modelType.FieldByName(fieldName)
	if !ok {
		t.Fatalf("%s missing %s", modelType.Name(), fieldName)
	}
	if field.Type != reflect.TypeOf(datatypes.JSON{}) {
		t.Fatalf("%s.%s type = %s, want datatypes.JSON", modelType.Name(), fieldName, field.Type)
	}
}
