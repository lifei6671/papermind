package exam

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/permission"
)

type ScoreExportRow struct {
	StudentName     string   // 考生姓名。
	SpaceID         uint64   // 考生所属空间 ID，用于权限范围判断。
	SpaceName       string   // 空间名称。
	SpaceIDs        []uint64 // 本次考试实际命中的空间 ID 集合，用于多空间考生成绩权限判断。
	AttemptNo       int      // attempt 次数。
	ObjectiveScore  string   // 客观题分。
	SubjectiveScore string   // 主观题分。
	TotalScore      string   // 总分。
	SubmittedAt     int64    // 提交时间，Unix 毫秒时间戳。
}

type ExportExamScoresInput struct {
	Permission permission.PermissionContext // 当前导出人权限上下文。
	TenantID   uint64                       // 所属租户 ID。
	ExamID     uint64                       // 考试 ID。
}

type ListExamScoresInput = ExportExamScoresInput

var ErrInvalidExportFile = errors.New("invalid export file")

type ExportResult struct {
	FilePath string // 导出文件绝对或工作目录相对路径。
	RowCount int    // 导出数据行数。
}

type ExportRepository interface {
	ListScoreExportRows(ctx context.Context, tenantID uint64, examID uint64) ([]ScoreExportRow, error)
	ExamTargetSpaceIDs(ctx context.Context, tenantID uint64, examID uint64) ([]uint64, error)
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
	rows, err := s.listExamScoresByPermission(ctx, ListExamScoresInput(input), s.canExportExamInAnySpace)
	if err != nil {
		return ExportResult{}, err
	}
	if err := os.MkdirAll(s.exportDir, 0o755); err != nil {
		return ExportResult{}, err
	}
	filePath := filepath.Join(s.exportDir, fmt.Sprintf("tenant-%d-exam-%d-scores-%s-%d.csv", input.TenantID, input.ExamID, exportFileScope(input.Permission), s.now()))
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
	return s.listExamScoresByPermission(ctx, input, s.canViewExamResultsInAnySpace)
}

func (s *ExportService) ResolveExportFile(fileName string, examID uint64, ctx permission.PermissionContext) (string, error) {
	if fileName == "" || filepath.Base(fileName) != fileName || filepath.Ext(fileName) != ".csv" {
		return "", ErrInvalidExportFile
	}
	if !exportFileMatchesScope(fileName, examID, ctx) {
		return "", ErrInvalidExportFile
	}
	filePath := filepath.Join(s.exportDir, fileName)
	info, err := os.Stat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrInvalidExportFile
	}
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", ErrInvalidExportFile
	}
	return filePath, nil
}

func exportFileMatchesScope(fileName string, examID uint64, ctx permission.PermissionContext) bool {
	return strings.HasPrefix(fileName, fmt.Sprintf("tenant-%d-exam-%d-scores-%s-", ctx.TenantID, examID, exportFileScope(ctx)))
}

func exportFileScope(ctx permission.PermissionContext) string {
	if ctx.Role == permission.RoleTenantAdmin {
		return "tenant"
	}
	spaceIDs := make([]uint64, 0, len(ctx.SpaceMemberships))
	for spaceID, role := range ctx.SpaceMemberships {
		if role == permission.RoleSpaceAdmin || role == permission.RoleTeacher {
			spaceIDs = append(spaceIDs, spaceID)
		}
	}
	if len(spaceIDs) == 0 {
		return "space-0"
	}
	sort.Slice(spaceIDs, func(i int, j int) bool { return spaceIDs[i] < spaceIDs[j] })
	return fmt.Sprintf("space-%d", spaceIDs[0])
}

func (s *ExportService) listExamScoresByPermission(ctx context.Context, input ListExamScoresInput, allow func(permission.PermissionContext, uint64, []uint64) bool) ([]ScoreExportRow, error) {
	if !hasPossibleGradeRole(input.Permission) {
		return nil, permission.ErrForbidden
	}
	rows, err := s.repo.ListScoreExportRows(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		spaces, err := s.repo.ExamTargetSpaceIDs(ctx, input.TenantID, input.ExamID)
		if err != nil {
			return nil, err
		}
		if allow(input.Permission, input.ExamID, spaces) {
			return []ScoreExportRow{}, nil
		}
		return nil, permission.ErrForbidden
	}
	allowed := make([]ScoreExportRow, 0, len(rows))
	for _, row := range rows {
		if allow(input.Permission, input.ExamID, scoreExportSpaceIDs(row)) {
			allowed = append(allowed, row)
		}
	}
	if len(allowed) == 0 {
		return nil, permission.ErrForbidden
	}
	return allowed, nil
}

func (s *ExportService) canViewExamResultsInAnySpace(ctx permission.PermissionContext, examID uint64, spaces []uint64) bool {
	for _, spaceID := range spaces {
		if err := s.permissionChecker.CanViewExamResults(permissionWithExamScope(ctx, examID, spaceID), examID); err == nil {
			return true
		}
	}
	return false
}

func (s *ExportService) canExportExamInAnySpace(ctx permission.PermissionContext, examID uint64, spaces []uint64) bool {
	for _, spaceID := range spaces {
		if err := s.permissionChecker.CanExportExamResults(permissionWithExamScope(ctx, examID, spaceID), examID); err == nil {
			return true
		}
	}
	return false
}

func scoreExportSpaceIDs(row ScoreExportRow) []uint64 {
	if len(row.SpaceIDs) > 0 {
		return row.SpaceIDs
	}
	return []uint64{row.SpaceID}
}

func writeScoreExportCSV(writer *csv.Writer, rows []ScoreExportRow) error {
	if err := writer.Write([]string{"考生姓名", "空间名称", "attempt 次数", "客观题分", "主观题分", "总分", "提交时间"}); err != nil {
		return err
	}
	for _, row := range rows {
		// 导出文件直接面向教师下载，首版使用本地时区的可读时间展示提交时间。
		if err := writer.Write([]string{
			escapeCSVFormula(row.StudentName),
			escapeCSVFormula(row.SpaceName),
			strconv.Itoa(row.AttemptNo),
			escapeCSVFormula(row.ObjectiveScore),
			escapeCSVFormula(row.SubjectiveScore),
			escapeCSVFormula(row.TotalScore),
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

func escapeCSVFormula(value string) string {
	if value == "" {
		return value
	}
	switch value[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func formatExportTime(unixMilli int64) string {
	if unixMilli == 0 {
		return ""
	}
	return time.UnixMilli(unixMilli).Format("2006-01-02 15:04:05")
}
