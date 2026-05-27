package constant

const (
	// QuestionTypeSingle 表示单选题。
	QuestionTypeSingle = "single"
	// QuestionTypeMultiple 表示多选题。
	QuestionTypeMultiple = "multiple"
	// QuestionTypeJudge 表示判断题。
	QuestionTypeJudge = "judge"
	// QuestionTypeFillBlank 表示填空题。
	QuestionTypeFillBlank = "fill_blank"
	// QuestionTypeShortText 表示简答题。
	QuestionTypeShortText = "short_text"
)

const (
	// BuildModeManual 表示手动固化组卷。
	BuildModeManual = "manual"
	// BuildModeRuleFixed 表示规则生成后固化组卷。
	BuildModeRuleFixed = "rule_fixed"
	// BuildModeRuleLive 表示考试开始时实时抽题。
	BuildModeRuleLive = "rule_live"
)

const (
	// ExamStatusDraft 表示考试草稿状态。
	ExamStatusDraft = "draft"
	// ExamStatusPublished 表示考试已发布。
	ExamStatusPublished = "published"
	// PaperStatusDraft 表示试卷草稿状态。
	PaperStatusDraft = "draft"
	// QuestionStatusEnabled 表示题目可用。
	QuestionStatusEnabled = "enabled"
)

const (
	// AttemptStatusInProgress 表示作答进行中。
	AttemptStatusInProgress = "in_progress"
	// AttemptStatusSubmitted 表示作答已提交。
	AttemptStatusSubmitted = "submitted"
)

const (
	// ExamEventTypeBlur 表示考生窗口失焦事件。
	ExamEventTypeBlur = "blur"
	// ExamEventTypeFocus 表示考生窗口聚焦事件。
	ExamEventTypeFocus = "focus"
	// ExamEventTypeSubmit 表示手动提交事件。
	ExamEventTypeSubmit = "submit"
	// ExamEventTypeAutoSubmit 表示自动交卷事件。
	ExamEventTypeAutoSubmit = "auto_submit"
)

const (
	// GradingStatusAuto 表示客观题已经由系统自动判分。
	GradingStatusAuto = "auto"
	// GradingStatusPending 表示主观题等待教师人工阅卷。
	GradingStatusPending = "pending"
	// GradingStatusGraded 表示主观题已人工阅卷。
	GradingStatusGraded = "graded"
	// GradingModeManual 表示人工阅卷。
	GradingModeManual = "manual"
)
