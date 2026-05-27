package exam

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/permission"
)

type ScoreExportRow struct {
	StudentName     string // 考生姓名。
	SpaceID         uint64 // 考生所属空间 ID，用于权限范围判断。
	SpaceName       string // 空间名称。
	AttemptNo       int    // attempt 次数。
	ObjectiveScore  string // 客观题分。
	SubjectiveScore string // 主观题分。
	TotalScore      string // 总分。
	SubmittedAt     int64  // 提交时间，Unix 毫秒时间戳。
}

type ExportExamScoresInput struct {
	Permission permission.PermissionContext // 当前导出人权限上下文。
	TenantID   uint64                       // 所属租户 ID。
	ExamID     uint64                       // 考试 ID。
}

type ListExamScoresInput = ExportExamScoresInput

type ExportResult struct {
	FilePath string // 导出文件绝对或工作目录相对路径。
	RowCount int    // 导出数据行数。
}

type ExportRepository interface {
	ListScoreExportRows(ctx context.Context, tenantID uint64, examID uint64) ([]ScoreExportRow, error)
}

type ExportServiceOptions struct {
	Repo              ExportRepository
	PermissionChecker permission.PermissionChecker
	ExportDir         string
	Now               func() int64
}

type ExportService struct {
	repo              ExportRepository
	permissionChecker permission.PermissionChecker
	exportDir         string
	now               func() int64
}

func NewExportService(options ExportServiceOptions) *ExportService {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &ExportService{repo: options.Repo, permissionChecker: options.PermissionChecker, exportDir: options.ExportDir, now: now}
}

func (s *ExportService) ExportExamScores(ctx context.Context, input ExportExamScoresInput) (ExportResult, error) {
	rows, err := s.ListExamScores(ctx, ListExamScoresInput(input))
	if err != nil {
		return ExportResult{}, err
	}
	if err := os.MkdirAll(s.exportDir, 0o755); err != nil {
		return ExportResult{}, err
	}
	filePath := filepath.Join(s.exportDir, fmt.Sprintf("exam-%d-scores-%d.csv", input.ExamID, s.now()))
	file, err := os.Create(filePath)
	if err != nil {
		return ExportResult{}, err
	}
	defer file.Close()

	if err := writeScoreExportCSV(csv.NewWriter(file), rows); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{FilePath: filePath, RowCount: len(rows)}, nil
}

func (s *ExportService) ListExamScores(ctx context.Context, input ListExamScoresInput) ([]ScoreExportRow, error) {
	if !hasPossibleGradeRole(input.Permission) {
		return nil, permission.ErrForbidden
	}
	rows, err := s.repo.ListScoreExportRows(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return nil, err
	}
	allowed := make([]ScoreExportRow, 0, len(rows))
	for _, row := range rows {
		if err := s.permissionChecker.CanGradeExam(permissionWithExamScope(input.Permission, input.ExamID, row.SpaceID), input.ExamID); err == nil {
			allowed = append(allowed, row)
		}
	}
	return allowed, nil
}

func writeScoreExportCSV(writer *csv.Writer, rows []ScoreExportRow) error {
	if err := writer.Write([]string{"考生姓名", "空间名称", "attempt 次数", "客观题分", "主观题分", "总分", "提交时间"}); err != nil {
		return err
	}
	for _, row := range rows {
		// 导出文件直接面向教师下载，首版使用本地时区的可读时间展示提交时间。
		if err := writer.Write([]string{
			row.StudentName,
			row.SpaceName,
			strconv.Itoa(row.AttemptNo),
			row.ObjectiveScore,
			row.SubjectiveScore,
			row.TotalScore,
			formatExportTime(row.SubmittedAt),
		}); err != nil {
			return err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}
	return nil
}

func formatExportTime(unixMilli int64) string {
	if unixMilli == 0 {
		return ""
	}
	return time.UnixMilli(unixMilli).Format("2006-01-02 15:04:05")
}
