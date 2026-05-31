package db

type QuestionDO struct {
	BaseFields                 // 普通业务表公共字段。
	SoftDeleteFields           // 软删除字段。
	TenantID           uint64  `gorm:"column:tenant_id"`            // 所属租户 ID。
	SpaceID            *uint64 `gorm:"column:space_id"`             // 所属空间 ID，nil 表示租户公共题库。
	Type               string  `gorm:"column:type"`                 // 题型：single / multiple / judge / fill_blank / short_text。
	Difficulty         string  `gorm:"column:difficulty"`           // 难度：easy / medium / hard。
	Title              string  `gorm:"column:title"`                // 题干内容。
	Analysis           string  `gorm:"column:analysis"`             // 题目解析，出题人可选填。
	StandardAnswer     string  `gorm:"column:standard_answer"`      // 填空题标准答案或判断题标准答案。
	ReferenceAnswer    string  `gorm:"column:reference_answer"`     // 简答题参考答案。
	ScoreDefault       string  `gorm:"column:score_default"`        // 默认分值，使用字符串承载 DECIMAL/NUMERIC。
	ChoiceDisplayCount *int    `gorm:"column:choice_display_count"` // 选择题展示选项数量，nil 表示不限制。
	ShuffleOptions     bool    `gorm:"column:shuffle_options"`      // 题库默认选项随机设置。
	Status             string  `gorm:"column:status"`               // 题目状态：draft / enabled / disabled。
}

func (QuestionDO) TableName() string {
	return "questions"
}

// QuestionColumns 与 QuestionDO 同文件维护，题库可见性查询必须同时使用 tenant_id、space_id 和 deleted_at。
var QuestionColumns = struct {
	ID                 string
	TenantID           string
	SpaceID            string
	Type               string
	Difficulty         string
	Title              string
	Analysis           string
	StandardAnswer     string
	ReferenceAnswer    string
	ScoreDefault       string
	ChoiceDisplayCount string
	ShuffleOptions     string
	Status             string
	DeletedAt          string
}{
	ID:                 BaseColumns.ID,
	TenantID:           "tenant_id",
	SpaceID:            "space_id",
	Type:               "type",
	Difficulty:         "difficulty",
	Title:              "title",
	Analysis:           "analysis",
	StandardAnswer:     "standard_answer",
	ReferenceAnswer:    "reference_answer",
	ScoreDefault:       "score_default",
	ChoiceDisplayCount: "choice_display_count",
	ShuffleOptions:     "shuffle_options",
	Status:             "status",
	DeletedAt:          SoftDeleteColumns.DeletedAt,
}

type QuestionOptionDO struct {
	BaseFields          // 普通业务表公共字段。
	TenantID     uint64 `gorm:"column:tenant_id"`     // 所属租户 ID。
	QuestionID   uint64 `gorm:"column:question_id"`   // 题目 ID。
	OptionKey    string `gorm:"column:option_key"`    // 出题编辑时的原始展示标签，不参与判分。
	SortOrder    int    `gorm:"column:sort_order"`    // 选项原始排序。
	Content      string `gorm:"column:content"`       // 选项内容。
	IsCorrect    bool   `gorm:"column:is_correct"`    // 是否为正确答案。
	IsDistractor bool   `gorm:"column:is_distractor"` // 是否可作为随机补位干扰项。
}

func (QuestionOptionDO) TableName() string {
	return "question_options"
}

// QuestionOptionColumns 与 QuestionOptionDO 同文件维护，判分不得依赖 option_key，只能用于编辑展示。
var QuestionOptionColumns = struct {
	ID           string
	TenantID     string
	QuestionID   string
	OptionKey    string
	SortOrder    string
	Content      string
	IsCorrect    string
	IsDistractor string
}{
	ID:           BaseColumns.ID,
	TenantID:     "tenant_id",
	QuestionID:   "question_id",
	OptionKey:    "option_key",
	SortOrder:    "sort_order",
	Content:      "content",
	IsCorrect:    "is_correct",
	IsDistractor: "is_distractor",
}

type TagDO struct {
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	TenantID         uint64 `gorm:"column:tenant_id"` // 所属租户 ID。
	Name             string `gorm:"column:name"`      // 标签名称，例如知识点、章节、技能点。
}

func (TagDO) TableName() string {
	return "tags"
}

// TagColumns 与 TagDO 同文件维护，查询题目标签时必须过滤 tags.deleted_at = 0。
var TagColumns = struct {
	ID        string
	TenantID  string
	Name      string
	DeletedAt string
}{
	ID:        BaseColumns.ID,
	TenantID:  "tenant_id",
	Name:      "name",
	DeletedAt: SoftDeleteColumns.DeletedAt,
}

type QuestionTagDO struct {
	RelationFields        // 纯关系表公共字段。
	TenantID       uint64 `gorm:"column:tenant_id"`   // 所属租户 ID。
	QuestionID     uint64 `gorm:"column:question_id"` // 题目 ID。
	TagID          uint64 `gorm:"column:tag_id"`      // 标签 ID。
}

func (QuestionTagDO) TableName() string {
	return "question_tags"
}

// QuestionTagColumns 与 QuestionTagDO 同文件维护，题目和标签绑定通过唯一索引防止重复。
var QuestionTagColumns = struct {
	ID         string
	TenantID   string
	QuestionID string
	TagID      string
}{
	ID:         RelationColumns.ID,
	TenantID:   "tenant_id",
	QuestionID: "question_id",
	TagID:      "tag_id",
}
