package db

type PaperDO struct {
	BaseFields               // 普通业务表公共字段。
	SoftDeleteFields         // 软删除字段。
	TenantID         uint64  `gorm:"column:tenant_id"`         // 所属租户 ID。
	SpaceID          *uint64 `gorm:"column:space_id"`          // 所属空间 ID，nil 表示租户公共试卷。
	Name             string  `gorm:"column:name"`              // 试卷名称。
	Description      string  `gorm:"column:description"`       // 试卷说明。
	TotalScore       string  `gorm:"column:total_score"`       // 试卷总分，使用字符串承载 DECIMAL/NUMERIC。
	BuildMode        string  `gorm:"column:build_mode"`        // 组卷方式：manual / rule_fixed / rule_live。
	ShuffleQuestions bool    `gorm:"column:shuffle_questions"` // 是否对每个考生随机题目顺序。
	ShowAnalysis     bool    `gorm:"column:show_analysis"`     // 成绩可见后是否向考生展示题目解析。
	Status           string  `gorm:"column:status"`            // 试卷状态：draft / enabled / disabled。
}

func (PaperDO) TableName() string {
	return "papers"
}

// PaperColumns 与 PaperDO 同文件维护，试卷查询必须带 tenant_id 和 deleted_at 保证租户隔离。
var PaperColumns = struct {
	ID               string
	TenantID         string
	SpaceID          string
	Name             string
	Description      string
	TotalScore       string
	BuildMode        string
	ShuffleQuestions string
	ShowAnalysis     string
	Status           string
	DeletedAt        string
}{
	ID:               BaseColumns.ID,
	TenantID:         "tenant_id",
	SpaceID:          "space_id",
	Name:             "name",
	Description:      "description",
	TotalScore:       "total_score",
	BuildMode:        "build_mode",
	ShuffleQuestions: "shuffle_questions",
	ShowAnalysis:     "show_analysis",
	Status:           "status",
	DeletedAt:        SoftDeleteColumns.DeletedAt,
}

type PaperSectionDO struct {
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	TenantID         uint64 `gorm:"column:tenant_id"`      // 所属租户 ID。
	PaperID          uint64 `gorm:"column:paper_id"`       // 试卷 ID。
	SortOrder        int    `gorm:"column:sort_order"`     // 大题排序。
	Name             string `gorm:"column:name"`           // 大题名称，例如一、单选题。
	QuestionType     string `gorm:"column:question_type"`  // 大题题型。
	Instructions     string `gorm:"column:instructions"`   // 大题作答说明。
	TotalScore       string `gorm:"column:total_score"`    // 大题小计分，使用字符串承载 DECIMAL/NUMERIC。
	QuestionCount    int    `gorm:"column:question_count"` // 大题题目数量，由系统聚合计算。
}

func (PaperSectionDO) TableName() string {
	return "paper_sections"
}

// PaperSectionColumns 与 PaperSectionDO 同文件维护，大题查询必须过滤 deleted_at = 0。
var PaperSectionColumns = struct {
	ID            string
	TenantID      string
	PaperID       string
	SortOrder     string
	Name          string
	QuestionType  string
	Instructions  string
	TotalScore    string
	QuestionCount string
	DeletedAt     string
}{
	ID:            BaseColumns.ID,
	TenantID:      "tenant_id",
	PaperID:       "paper_id",
	SortOrder:     "sort_order",
	Name:          "name",
	QuestionType:  "question_type",
	Instructions:  "instructions",
	TotalScore:    "total_score",
	QuestionCount: "question_count",
	DeletedAt:     SoftDeleteColumns.DeletedAt,
}

type PaperSectionQuestionDO struct {
	BaseFields            // 普通业务表公共字段。
	TenantID       uint64 `gorm:"column:tenant_id"`       // 所属租户 ID。
	SectionID      uint64 `gorm:"column:section_id"`      // 大题 ID。
	PaperID        uint64 `gorm:"column:paper_id"`        // 试卷 ID，冗余保存用于减少查询 JOIN。
	QuestionID     uint64 `gorm:"column:question_id"`     // 题目 ID。
	SortOrder      int    `gorm:"column:sort_order"`      // 题目在大题中的排序。
	Score          string `gorm:"column:score"`           // 该题在本试卷中的分值，使用字符串承载 DECIMAL/NUMERIC。
	ShuffleOptions *bool  `gorm:"column:shuffle_options"` // 是否随机选项，nil 表示回退题库默认值。
}

func (PaperSectionQuestionDO) TableName() string {
	return "paper_section_questions"
}

// PaperSectionQuestionColumns 与 PaperSectionQuestionDO 同文件维护，section_id 与 paper_id 必须保持一致。
var PaperSectionQuestionColumns = struct {
	ID             string
	TenantID       string
	SectionID      string
	PaperID        string
	QuestionID     string
	SortOrder      string
	Score          string
	ShuffleOptions string
}{
	ID:             BaseColumns.ID,
	TenantID:       "tenant_id",
	SectionID:      "section_id",
	PaperID:        "paper_id",
	QuestionID:     "question_id",
	SortOrder:      "sort_order",
	Score:          "score",
	ShuffleOptions: "shuffle_options",
}

type PaperSectionRuleDO struct {
	BaseFields               // 普通业务表公共字段。
	TenantID         uint64  `gorm:"column:tenant_id"`          // 所属租户 ID。
	SectionID        uint64  `gorm:"column:section_id"`         // 大题 ID。
	PaperID          uint64  `gorm:"column:paper_id"`           // 试卷 ID，冗余保存用于减少查询 JOIN。
	SortOrder        int     `gorm:"column:sort_order"`         // 规则在大题内的排序。
	Difficulty       *string `gorm:"column:difficulty"`         // 抽题难度条件，nil 表示不限难度。
	TagFilter        string  `gorm:"column:tag_filter"`         // 标签过滤条件，JSON 数组字符串。
	QuestionCount    int     `gorm:"column:question_count"`     // 该规则抽题数量。
	ScorePerQuestion string  `gorm:"column:score_per_question"` // 该规则下每题分值，使用字符串承载 DECIMAL/NUMERIC。
	ShuffleOptions   *bool   `gorm:"column:shuffle_options"`    // 是否随机选项，nil 表示回退题库默认值。
}

func (PaperSectionRuleDO) TableName() string {
	return "paper_section_rules"
}

// PaperSectionRuleColumns 与 PaperSectionRuleDO 同文件维护，difficulty 为空表示不限难度。
var PaperSectionRuleColumns = struct {
	ID               string
	TenantID         string
	SectionID        string
	PaperID          string
	SortOrder        string
	Difficulty       string
	TagFilter        string
	QuestionCount    string
	ScorePerQuestion string
	ShuffleOptions   string
}{
	ID:               BaseColumns.ID,
	TenantID:         "tenant_id",
	SectionID:        "section_id",
	PaperID:          "paper_id",
	SortOrder:        "sort_order",
	Difficulty:       "difficulty",
	TagFilter:        "tag_filter",
	QuestionCount:    "question_count",
	ScorePerQuestion: "score_per_question",
	ShuffleOptions:   "shuffle_options",
}
