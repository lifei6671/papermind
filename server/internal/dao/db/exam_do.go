package db

import "gorm.io/datatypes"

type ExamDO struct {
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	TenantID         uint64 `gorm:"column:tenant_id"`          // 所属租户 ID。
	PaperID          uint64 `gorm:"column:paper_id"`           // 关联试卷 ID。
	Name             string `gorm:"column:name"`               // 考试名称。
	StartTime        int64  `gorm:"column:start_time"`         // 考试开始时间，Unix 毫秒时间戳。
	EndTime          int64  `gorm:"column:end_time"`           // 考试结束时间，Unix 毫秒时间戳。
	DurationMinutes  int    `gorm:"column:duration_minutes"`   // 单次作答时长，单位分钟。
	MaxAttempts      int    `gorm:"column:max_attempts"`       // 每名考生最多作答次数。
	ResultStrategy   string `gorm:"column:result_strategy"`    // 多次作答成绩策略：latest / highest。
	PublishMode      string `gorm:"column:publish_mode"`       // 成绩发布模式：immediate_score / manual_publish。
	ScorePublishTime *int64 `gorm:"column:score_publish_time"` // 统一成绩公布时间，nil 表示未设置。
	InviteCode       string `gorm:"column:invite_code"`        // 考试邀请码。
	Status           string `gorm:"column:status"`             // 考试状态：draft / published / closed。
}

func (ExamDO) TableName() string {
	return "exams"
}

// ExamColumns 与 ExamDO 同文件维护，考试查询必须带 tenant_id 和 deleted_at 保证租户隔离。
var ExamColumns = struct {
	ID               string
	TenantID         string
	PaperID          string
	Name             string
	StartTime        string
	EndTime          string
	DurationMinutes  string
	MaxAttempts      string
	ResultStrategy   string
	PublishMode      string
	ScorePublishTime string
	InviteCode       string
	Status           string
	DeletedAt        string
}{
	ID:               BaseColumns.ID,
	TenantID:         "tenant_id",
	PaperID:          "paper_id",
	Name:             "name",
	StartTime:        "start_time",
	EndTime:          "end_time",
	DurationMinutes:  "duration_minutes",
	MaxAttempts:      "max_attempts",
	ResultStrategy:   "result_strategy",
	PublishMode:      "publish_mode",
	ScorePublishTime: "score_publish_time",
	InviteCode:       "invite_code",
	Status:           "status",
	DeletedAt:        SoftDeleteColumns.DeletedAt,
}

type ExamTargetDO struct {
	RelationFields        // 纯关系表公共字段。
	TenantID       uint64 `gorm:"column:tenant_id"`   // 所属租户 ID。
	ExamID         uint64 `gorm:"column:exam_id"`     // 考试 ID。
	TargetType     string `gorm:"column:target_type"` // 发布目标类型：space / user。
	TargetID       uint64 `gorm:"column:target_id"`   // 发布目标 ID。
}

func (ExamTargetDO) TableName() string {
	return "exam_targets"
}

// ExamTargetColumns 与 ExamTargetDO 同文件维护，发布范围通过唯一索引防止重复目标。
var ExamTargetColumns = struct {
	ID         string
	TenantID   string
	ExamID     string
	TargetType string
	TargetID   string
}{
	ID:         RelationColumns.ID,
	TenantID:   "tenant_id",
	ExamID:     "exam_id",
	TargetType: "target_type",
	TargetID:   "target_id",
}

type ExamLiveQuestionPoolDO struct {
	RelationFields        // 纯关系表公共字段。
	TenantID       uint64 `gorm:"column:tenant_id"`   // 所属租户 ID。
	ExamID         uint64 `gorm:"column:exam_id"`     // 考试 ID。
	SectionID      uint64 `gorm:"column:section_id"`  // 大题 ID。
	RuleID         uint64 `gorm:"column:rule_id"`     // 大题抽题规则 ID。
	QuestionID     uint64 `gorm:"column:question_id"` // 候选题目 ID。
}

func (ExamLiveQuestionPoolDO) TableName() string {
	return "exam_live_question_pools"
}

// ExamLiveQuestionPoolColumns 与 ExamLiveQuestionPoolDO 同文件维护，rule_live 只能从冻结题池抽题。
var ExamLiveQuestionPoolColumns = struct {
	ID         string
	TenantID   string
	ExamID     string
	SectionID  string
	RuleID     string
	QuestionID string
}{
	ID:         RelationColumns.ID,
	TenantID:   "tenant_id",
	ExamID:     "exam_id",
	SectionID:  "section_id",
	RuleID:     "rule_id",
	QuestionID: "question_id",
}

type ExamAttemptDO struct {
	BaseFields                // 普通业务表公共字段。
	TenantID           uint64 `gorm:"column:tenant_id"`             // 所属租户 ID。
	ExamID             uint64 `gorm:"column:exam_id"`               // 考试 ID。
	UserID             uint64 `gorm:"column:user_id"`               // 考生用户 ID。
	AttemptNo          int    `gorm:"column:attempt_no"`            // 第几次作答，从 1 开始。
	Status             string `gorm:"column:status"`                // 作答状态：in_progress / submitted / graded。
	StartedAt          int64  `gorm:"column:started_at"`            // 开始作答时间，Unix 毫秒时间戳。
	SubmittedAt        *int64 `gorm:"column:submitted_at"`          // 提交时间，nil 表示尚未提交。
	ExamTokenHash      string `gorm:"column:exam_token_hash"`       // 考试过程 token 哈希。
	ExamTokenExpiresAt int64  `gorm:"column:exam_token_expires_at"` // 考试过程 token 过期时间。
	ObjectiveScore     string `gorm:"column:objective_score"`       // 客观题得分，使用字符串承载 DECIMAL/NUMERIC。
	SubjectiveScore    string `gorm:"column:subjective_score"`      // 主观题得分，使用字符串承载 DECIMAL/NUMERIC。
	TotalScore         string `gorm:"column:total_score"`           // 总分，使用字符串承载 DECIMAL/NUMERIC。
}

func (ExamAttemptDO) TableName() string {
	return "exam_attempts"
}

// ExamAttemptColumns 与 ExamAttemptDO 同文件维护，exam_token_hash 与 status 支撑考试过程鉴权。
var ExamAttemptColumns = struct {
	ID                 string
	TenantID           string
	ExamID             string
	UserID             string
	AttemptNo          string
	Status             string
	StartedAt          string
	SubmittedAt        string
	ExamTokenHash      string
	ExamTokenExpiresAt string
	ObjectiveScore     string
	SubjectiveScore    string
	TotalScore         string
}{
	ID:                 BaseColumns.ID,
	TenantID:           "tenant_id",
	ExamID:             "exam_id",
	UserID:             "user_id",
	AttemptNo:          "attempt_no",
	Status:             "status",
	StartedAt:          "started_at",
	SubmittedAt:        "submitted_at",
	ExamTokenHash:      "exam_token_hash",
	ExamTokenExpiresAt: "exam_token_expires_at",
	ObjectiveScore:     "objective_score",
	SubjectiveScore:    "subjective_score",
	TotalScore:         "total_score",
}

type ExamAttemptQuestionDO struct {
	BaseFields                           // 普通业务表公共字段。
	TenantID              uint64         `gorm:"column:tenant_id"`               // 所属租户 ID。
	AttemptID             uint64         `gorm:"column:attempt_id"`              // 作答 ID。
	SectionID             uint64         `gorm:"column:section_id"`              // 原始大题 ID，仅用于溯源。
	QuestionID            uint64         `gorm:"column:question_id"`             // 原始题目 ID。
	SectionSnapshot       datatypes.JSON `gorm:"column:section_snapshot"`        // 大题快照 JSON，包含大题名称和作答说明。
	SortOrder             int            `gorm:"column:sort_order"`              // 该考生看到的全局题号，从 1 连续递增。
	Score                 string         `gorm:"column:score"`                   // 该题在本次作答中的分值。
	QuestionSnapshot      datatypes.JSON `gorm:"column:question_snapshot"`       // 题干快照 JSON。
	OptionSnapshot        datatypes.JSON `gorm:"column:option_snapshot"`         // 选项快照 JSON。
	CorrectAnswerSnapshot datatypes.JSON `gorm:"column:correct_answer_snapshot"` // 正确答案快照 JSON。
}

func (ExamAttemptQuestionDO) TableName() string {
	return "exam_attempt_questions"
}

// ExamAttemptQuestionColumns 与 ExamAttemptQuestionDO 同文件维护，快照字段保证考试后题库变更不影响作答。
var ExamAttemptQuestionColumns = struct {
	ID                    string
	TenantID              string
	AttemptID             string
	SectionID             string
	QuestionID            string
	SectionSnapshot       string
	SortOrder             string
	Score                 string
	QuestionSnapshot      string
	OptionSnapshot        string
	CorrectAnswerSnapshot string
}{
	ID:                    BaseColumns.ID,
	TenantID:              "tenant_id",
	AttemptID:             "attempt_id",
	SectionID:             "section_id",
	QuestionID:            "question_id",
	SectionSnapshot:       "section_snapshot",
	SortOrder:             "sort_order",
	Score:                 "score",
	QuestionSnapshot:      "question_snapshot",
	OptionSnapshot:        "option_snapshot",
	CorrectAnswerSnapshot: "correct_answer_snapshot",
}

type ExamAnswerDO struct {
	BaseFields               // 普通业务表公共字段。
	TenantID          uint64 `gorm:"column:tenant_id"`           // 所属租户 ID。
	AttemptID         uint64 `gorm:"column:attempt_id"`          // 作答 ID。
	AttemptQuestionID uint64 `gorm:"column:attempt_question_id"` // 考生题目快照 ID。
	AnswerContent     string `gorm:"column:answer_content"`      // 考生答案内容。
	Score             string `gorm:"column:score"`               // 该题得分，使用字符串承载 DECIMAL/NUMERIC。
	GradingStatus     string `gorm:"column:grading_status"`      // 阅卷状态：auto / pending / graded。
	GradedBy          uint64 `gorm:"column:graded_by"`           // 阅卷人用户 ID，0 表示未阅卷。
	GradedAt          *int64 `gorm:"column:graded_at"`           // 阅卷时间，nil 表示未阅卷。
	GraderComment     string `gorm:"column:grader_comment"`      // 阅卷评语。
}

func (ExamAnswerDO) TableName() string {
	return "exam_answers"
}

// ExamAnswerColumns 与 ExamAnswerDO 同文件维护，答案 upsert 必须以 attempt_question_id 为边界。
var ExamAnswerColumns = struct {
	ID                string
	TenantID          string
	AttemptID         string
	AttemptQuestionID string
	AnswerContent     string
	Score             string
	GradingStatus     string
	GradedBy          string
	GradedAt          string
	GraderComment     string
}{
	ID:                BaseColumns.ID,
	TenantID:          "tenant_id",
	AttemptID:         "attempt_id",
	AttemptQuestionID: "attempt_question_id",
	AnswerContent:     "answer_content",
	Score:             "score",
	GradingStatus:     "grading_status",
	GradedBy:          "graded_by",
	GradedAt:          "graded_at",
	GraderComment:     "grader_comment",
}

type ExamEventDO struct {
	EventFields                // 追加写事件表公共字段。
	TenantID    uint64         `gorm:"column:tenant_id"`  // 所属租户 ID。
	AttemptID   uint64         `gorm:"column:attempt_id"` // 作答 ID。
	EventType   string         `gorm:"column:event_type"` // 事件类型：blur / focus / auto_save / submit / auto_submit。
	EventTime   int64          `gorm:"column:event_time"` // 事件发生时间，Unix 毫秒时间戳。
	Payload     datatypes.JSON `gorm:"column:payload"`    // 事件负载 JSON。
}

func (ExamEventDO) TableName() string {
	return "exam_events"
}

// ExamEventColumns 与 ExamEventDO 同文件维护，事件表只追加写入，不允许业务更新。
var ExamEventColumns = struct {
	ID        string
	TenantID  string
	AttemptID string
	EventType string
	EventTime string
	Payload   string
}{
	ID:        EventColumns.ID,
	TenantID:  "tenant_id",
	AttemptID: "attempt_id",
	EventType: "event_type",
	EventTime: "event_time",
	Payload:   "payload",
}
