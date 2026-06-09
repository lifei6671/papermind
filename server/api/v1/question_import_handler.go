package v1

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type importQuestionsResponse struct {
	SuccessCount   int                           `json:"success_count"`
	DuplicateCount int                           `json:"duplicate_count"`
	Errors         []importQuestionErrorResponse `json:"errors"`
}

type importQuestionErrorResponse struct {
	RowNumber int    `json:"row_number"`
	Reason    string `json:"reason"`
}

type startQuestionImportJobResponse struct {
	JobID string `json:"job_id"`
}

type questionImportJobEventResponse struct {
	JobID          string                        `json:"job_id"`
	Status         string                        `json:"status"`
	FileName       string                        `json:"file_name"`
	TotalRows      int                           `json:"total_rows"`
	ProcessedRows  int                           `json:"processed_rows"`
	SuccessCount   int                           `json:"success_count"`
	ErrorCount     int                           `json:"error_count"`
	DuplicateCount int                           `json:"duplicate_count"`
	Errors         []importQuestionErrorResponse `json:"errors"`
	Message        string                        `json:"message,omitempty"`
}

type questionImportHeaderSchema struct {
	headers              []string
	typeIndex            int
	titleIndex           int
	optionsIndex         int
	correctAnswerIndex   int
	standardAnswerIndex  int
	referenceAnswerIndex int
	analysisIndex        int
	difficultyIndex      int
	tagsIndex            int
}

type questionImportJobEvent struct {
	jobID          string
	status         string
	fileName       string
	totalRows      int
	processedRows  int
	successCount   int
	errorCount     int
	duplicateCount int
	errors         []servicequestion.ImportError
	message        string
}

type questionImportJob struct {
	id          string
	tenantID    uint64
	spaceID     *uint64
	events      []questionImportJobEvent
	subscribers map[chan questionImportJobEvent]struct{}
	done        bool
	completedAt time.Time
}

const questionImportJobRetention = 30 * time.Minute

type questionImportJobStore struct {
	mu      sync.Mutex
	nextID  uint64
	jobs    map[string]*questionImportJob
	nowFunc func() time.Time
}

const defaultQuestionImportFileMaxBytes = 100 * 1024 * 1024
const questionImportMultipartMemoryBytes = 32 * 1024 * 1024
const questionImportMultipartOverheadBytes = 1024 * 1024
const defaultQuestionImportRequestMaxBytes = defaultQuestionImportFileMaxBytes + questionImportMultipartOverheadBytes

func newQuestionImportJobStore() *questionImportJobStore {
	return &questionImportJobStore{
		jobs:    map[string]*questionImportJob{},
		nowFunc: time.Now,
	}
}

var legacyQuestionImportHeaderSchema = questionImportHeaderSchema{
	headers:              []string{"type", "title", "options", "correct_answer", "analysis", "difficulty", "tags"},
	typeIndex:            0,
	titleIndex:           1,
	optionsIndex:         2,
	correctAnswerIndex:   3,
	standardAnswerIndex:  -1,
	referenceAnswerIndex: -1,
	analysisIndex:        4,
	difficultyIndex:      5,
	tagsIndex:            6,
}

func (h questionHandler) importQuestions(c *gin.Context) {
	limitQuestionImportRequestBody(c)
	if !parseQuestionImportMultipart(c) {
		return
	}
	tenantID, err := readUintForm(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, tenantID, h.members) {
		return
	}
	spaceID, err := readOptionalUintForm(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.permissionContextForQuestionTargetScope(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	targetStatus, err := readImportStatusForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "status 只能是 draft 或 enabled"))
		return
	}
	if err := h.validateQuestionImportWrite(c.Request.Context(), permissionContext, tenantID, spaceID, targetStatus); err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		if isRequestBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, response.Fail(code.InvalidParam, "导入文件不能超过 100MB"))
			return
		}
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "file 不能为空"))
		return
	}
	if fileHeader.Size > defaultQuestionImportFileMaxBytes {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "导入文件不能超过 100MB"))
		return
	}
	rows, parseErrors, err := parseQuestionImportCSV(fileHeader, h.service.ImportTemplateHeaders())
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}

	result, err := h.service.ImportQuestions(c.Request.Context(), servicequestion.ImportQuestionsInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		SpaceID:    spaceID,
		Status:     targetStatus,
		Rows:       rows,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	result.Errors = append(parseErrors, result.Errors...)
	c.JSON(http.StatusOK, response.OK(importQuestionsToResponse(result)))
}

func (h questionHandler) startImportQuestionsJob(c *gin.Context) {
	limitQuestionImportRequestBody(c)
	if !parseQuestionImportMultipart(c) {
		return
	}
	tenantID, err := readUintForm(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, tenantID, h.members) {
		return
	}
	spaceID, err := readOptionalUintForm(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.permissionContextForQuestionTargetScope(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	targetStatus, err := readImportStatusForm(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "status 只能是 draft 或 enabled"))
		return
	}
	if err := h.validateQuestionImportWrite(c.Request.Context(), permissionContext, tenantID, spaceID, targetStatus); err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		if isRequestBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, response.Fail(code.InvalidParam, "导入文件不能超过 100MB"))
			return
		}
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "file 不能为空"))
		return
	}
	fileContent, err := readQuestionImportFile(fileHeader)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	store := h.importJobs
	if store == nil {
		store = newQuestionImportJobStore()
	}
	jobID := store.Create(fileHeader.Filename, tenantID, spaceID)
	go h.runImportQuestionsJob(questionImportJobInput{
		JobID:             jobID,
		FileName:          fileHeader.Filename,
		FileContent:       fileContent,
		TenantID:          tenantID,
		SpaceID:           spaceID,
		TargetStatus:      targetStatus,
		PermissionContext: permissionContext,
		Store:             store,
	})
	c.JSON(http.StatusOK, response.OK(startQuestionImportJobResponse{JobID: jobID}))
}

func (h questionHandler) streamImportQuestionJobEvents(c *gin.Context) {
	store := h.importJobs
	if store == nil {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "导入任务不存在"))
		return
	}
	jobID := c.Param("job_id")
	tenantID, spaceID, ok := store.Scope(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "导入任务不存在"))
		return
	}
	if !authorizeExamBusiness(c, tenantID, h.members) {
		return
	}
	permissionContext, err := h.permissionContextForQuestionTargetScope(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	if err := h.validateQuestionImportWrite(c.Request.Context(), permissionContext, tenantID, spaceID, servicequestion.QuestionStatusDraft); err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	events, updates, unsubscribe, ok := store.Subscribe(jobID)
	if !ok {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "导入任务不存在"))
		return
	}
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	for _, event := range events {
		writeQuestionImportSSE(c, event)
	}
	c.Writer.Flush()
	if len(events) > 0 {
		last := events[len(events)-1]
		if last.status == "completed" || last.status == "failed" {
			return
		}
	}

	notify := c.Request.Context().Done()
	for {
		select {
		case <-notify:
			return
		case event, ok := <-updates:
			if !ok {
				return
			}
			writeQuestionImportSSE(c, event)
			c.Writer.Flush()
			if event.status == "completed" || event.status == "failed" {
				return
			}
		}
	}
}

type questionImportJobInput struct {
	JobID             string
	FileName          string
	FileContent       []byte
	TenantID          uint64
	SpaceID           *uint64
	TargetStatus      string
	PermissionContext permission.PermissionContext
	Store             *questionImportJobStore
}

func (h questionHandler) runImportQuestionsJob(input questionImportJobInput) {
	input.Store.Append(input.JobID, questionImportJobEvent{jobID: input.JobID, status: "running", fileName: input.FileName})
	rows, parseErrors, err := parseQuestionImportCSVReader(bytes.NewReader(input.FileContent), h.service.ImportTemplateHeaders())
	if err != nil {
		input.Store.Append(input.JobID, questionImportJobEvent{jobID: input.JobID, status: "failed", fileName: input.FileName, message: err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	result, err := h.service.ImportQuestions(ctx, servicequestion.ImportQuestionsInput{
		Permission: input.PermissionContext,
		TenantID:   input.TenantID,
		SpaceID:    input.SpaceID,
		Status:     input.TargetStatus,
		Rows:       rows,
		OnProgress: func(progress servicequestion.ImportProgress) {
			errorCount := len(parseErrors) + len(progress.Errors)
			input.Store.Append(input.JobID, questionImportJobEvent{
				jobID:          input.JobID,
				status:         "running",
				fileName:       input.FileName,
				totalRows:      progress.TotalRows,
				processedRows:  progress.ProcessedRows,
				successCount:   progress.SuccessCount,
				errorCount:     errorCount,
				duplicateCount: progress.DuplicateCount,
			})
		},
	})
	if err != nil {
		input.Store.Append(input.JobID, questionImportJobEvent{jobID: input.JobID, status: "failed", fileName: input.FileName, message: err.Error()})
		return
	}
	result.Errors = append(parseErrors, result.Errors...)
	input.Store.Append(input.JobID, questionImportJobEvent{
		jobID:          input.JobID,
		status:         "completed",
		fileName:       input.FileName,
		totalRows:      len(rows),
		processedRows:  len(rows),
		successCount:   result.SuccessCount,
		errorCount:     len(result.Errors),
		duplicateCount: result.DuplicateCount,
		errors:         result.Errors,
	})
}

func (h questionHandler) validateQuestionImportWrite(ctx context.Context, permissionContext permission.PermissionContext, tenantID uint64, spaceID *uint64, targetStatus string) error {
	_, err := h.service.ImportQuestions(ctx, servicequestion.ImportQuestionsInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		SpaceID:    spaceID,
		Status:     targetStatus,
		Rows:       nil,
	})
	return err
}

func writeQuestionImportSSE(c *gin.Context, event questionImportJobEvent) {
	payload, err := json.Marshal(questionImportJobEventToResponse(event))
	if err != nil {
		return
	}
	c.Writer.WriteString("event: import_progress\n")
	c.Writer.WriteString("data: " + string(payload) + "\n\n")
}

func questionImportJobEventToResponse(event questionImportJobEvent) questionImportJobEventResponse {
	errors := make([]importQuestionErrorResponse, 0, len(event.errors))
	for _, item := range event.errors {
		errors = append(errors, importQuestionErrorResponse{RowNumber: item.RowNumber, Reason: item.Reason})
	}
	return questionImportJobEventResponse{
		JobID:          event.jobID,
		Status:         event.status,
		FileName:       event.fileName,
		TotalRows:      event.totalRows,
		ProcessedRows:  event.processedRows,
		SuccessCount:   event.successCount,
		ErrorCount:     event.errorCount,
		DuplicateCount: event.duplicateCount,
		Errors:         errors,
		Message:        event.message,
	}
}

func (s *questionImportJobStore) Create(fileName string, tenantID uint64, spaceID *uint64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFunc()
	s.cleanupExpiredLocked(now)
	s.nextID++
	jobID := "qimp-" + strconv.FormatInt(now.UnixNano(), 36) + "-" + strconv.FormatUint(s.nextID, 36)
	s.jobs[jobID] = &questionImportJob{
		id:          jobID,
		tenantID:    tenantID,
		spaceID:     spaceID,
		events:      []questionImportJobEvent{{jobID: jobID, status: "queued", fileName: fileName}},
		subscribers: map[chan questionImportJobEvent]struct{}{},
	}
	return jobID
}

func (s *questionImportJobStore) Scope(jobID string) (uint64, *uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(s.nowFunc())
	job := s.jobs[jobID]
	if job == nil {
		return 0, nil, false
	}
	return job.tenantID, job.spaceID, true
}

func (s *questionImportJobStore) Append(jobID string, event questionImportJobEvent) {
	s.mu.Lock()
	now := s.nowFunc()
	s.cleanupExpiredLocked(now)
	job := s.jobs[jobID]
	if job == nil || job.done {
		s.mu.Unlock()
		return
	}
	if event.jobID == "" {
		event.jobID = jobID
	}
	job.events = append(job.events, event)
	if event.status == "completed" || event.status == "failed" {
		job.done = true
		job.completedAt = now
	}
	subscribers := make([]chan questionImportJobEvent, 0, len(job.subscribers))
	for subscriber := range job.subscribers {
		subscribers = append(subscribers, subscriber)
	}
	s.mu.Unlock()

	for _, subscriber := range subscribers {
		sendQuestionImportJobEvent(subscriber, event)
	}
}

func sendQuestionImportJobEvent(subscriber chan questionImportJobEvent, event questionImportJobEvent) {
	select {
	case subscriber <- event:
		return
	default:
	}
	if event.status != "completed" && event.status != "failed" {
		return
	}
	select {
	case <-subscriber:
	default:
	}
	select {
	case subscriber <- event:
	default:
	}
}

func (s *questionImportJobStore) Subscribe(jobID string) ([]questionImportJobEvent, <-chan questionImportJobEvent, func(), bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(s.nowFunc())
	job := s.jobs[jobID]
	if job == nil {
		return nil, nil, nil, false
	}
	events := append([]questionImportJobEvent(nil), job.events...)
	updates := make(chan questionImportJobEvent, 8)
	if !job.done {
		job.subscribers[updates] = struct{}{}
	}
	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		current := s.jobs[jobID]
		if current == nil {
			return
		}
		delete(current.subscribers, updates)
	}
	return events, updates, unsubscribe, true
}

func (s *questionImportJobStore) cleanupExpiredLocked(now time.Time) {
	for jobID, job := range s.jobs {
		if !job.done || job.completedAt.IsZero() {
			continue
		}
		if now.Sub(job.completedAt) >= questionImportJobRetention {
			delete(s.jobs, jobID)
		}
	}
}

func parseQuestionImportCSV(fileHeader *multipart.FileHeader, expectedHeaders []string) ([]servicequestion.ImportRow, []servicequestion.ImportError, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, errors.New("读取导入文件失败")
	}
	defer file.Close()

	return parseQuestionImportCSVReader(file, expectedHeaders)
}

func parseQuestionImportCSVReader(source io.Reader, expectedHeaders []string) ([]servicequestion.ImportRow, []servicequestion.ImportError, error) {
	reader := csv.NewReader(source)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil, errors.New("导入文件不能为空")
		}
		return nil, nil, errors.New("读取导入表头失败")
	}
	schema, ok := matchImportHeaderSchema(header, expectedHeaders)
	if !ok {
		return nil, nil, errors.New("导入文件表头不符合模板")
	}

	rows := []servicequestion.ImportRow{}
	rowErrors := []servicequestion.ImportError{}
	rowNumber := 1
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowNumber++
		if err != nil {
			rowErrors = append(rowErrors, servicequestion.ImportError{RowNumber: rowNumber, Reason: "CSV 行解析失败"})
			continue
		}
		if len(record) != len(schema.headers) {
			rowErrors = append(rowErrors, servicequestion.ImportError{RowNumber: rowNumber, Reason: "列数量与模板不一致"})
			continue
		}
		// 新模板走中文表头，历史英文表头继续兼容；解析后统一映射到同一 ImportRow。
		rows = append(rows, servicequestion.ImportRow{
			RowNumber:       rowNumber,
			Type:            readImportRecordCell(record, schema.typeIndex),
			Title:           readImportRecordCell(record, schema.titleIndex),
			Options:         readImportRecordCell(record, schema.optionsIndex),
			CorrectAnswer:   readImportRecordCell(record, schema.correctAnswerIndex),
			StandardAnswer:  readImportRecordCell(record, schema.standardAnswerIndex),
			ReferenceAnswer: readImportRecordCell(record, schema.referenceAnswerIndex),
			Analysis:        readImportRecordCell(record, schema.analysisIndex),
			Difficulty:      readImportRecordCell(record, schema.difficultyIndex),
			Tags:            readImportRecordCell(record, schema.tagsIndex),
		})
	}
	return rows, rowErrors, nil
}

func readQuestionImportFile(fileHeader *multipart.FileHeader) ([]byte, error) {
	if fileHeader.Size > defaultQuestionImportFileMaxBytes {
		return nil, errors.New("导入文件不能超过 100MB")
	}
	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("读取导入文件失败")
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.New("读取导入文件失败")
	}
	return content, nil
}

func limitQuestionImportRequestBody(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, defaultQuestionImportRequestMaxBytes)
}

func parseQuestionImportMultipart(c *gin.Context) bool {
	if err := c.Request.ParseMultipartForm(questionImportMultipartMemoryBytes); err != nil {
		if isRequestBodyTooLarge(err) {
			c.JSON(http.StatusRequestEntityTooLarge, response.Fail(code.InvalidParam, "导入文件不能超过 100MB"))
			return false
		}
	}
	return true
}

func isRequestBodyTooLarge(err error) bool {
	return strings.Contains(err.Error(), "request body too large")
}

func readImportStatusForm(c *gin.Context) (string, error) {
	raw := strings.TrimSpace(c.PostForm("status"))
	if raw == "" || raw == servicequestion.QuestionStatusDraft {
		return servicequestion.QuestionStatusDraft, nil
	}
	if raw == servicequestion.QuestionStatusEnabled {
		return servicequestion.QuestionStatusEnabled, nil
	}
	return "", errors.New("invalid import status")
}

func importQuestionsToResponse(result servicequestion.ImportResult) importQuestionsResponse {
	errors := make([]importQuestionErrorResponse, 0, len(result.Errors))
	for _, item := range result.Errors {
		errors = append(errors, importQuestionErrorResponse{
			RowNumber: item.RowNumber,
			Reason:    item.Reason,
		})
	}
	return importQuestionsResponse{SuccessCount: result.SuccessCount, DuplicateCount: result.DuplicateCount, Errors: errors}
}

func sameImportHeaders(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if normalizeImportHeaderCell(actual[index]) != expected[index] {
			return false
		}
	}
	return true
}

func matchImportHeaderSchema(actual []string, expectedHeaders []string) (questionImportHeaderSchema, bool) {
	current := questionImportHeaderSchema{
		headers:              expectedHeaders,
		typeIndex:            0,
		titleIndex:           1,
		optionsIndex:         2,
		correctAnswerIndex:   3,
		standardAnswerIndex:  4,
		referenceAnswerIndex: 5,
		analysisIndex:        6,
		difficultyIndex:      7,
		tagsIndex:            8,
	}
	if sameImportHeaders(actual, current.headers) {
		return current, true
	}
	if sameImportHeaders(actual, legacyQuestionImportHeaderSchema.headers) {
		return legacyQuestionImportHeaderSchema, true
	}
	return questionImportHeaderSchema{}, false
}

func normalizeImportHeaderCell(value string) string {
	return strings.TrimPrefix(strings.TrimSpace(value), "\uFEFF")
}

func readImportRecordCell(record []string, index int) string {
	if index < 0 || index >= len(record) {
		return ""
	}
	return record[index]
}

func readUintForm(c *gin.Context, key string) (uint64, error) {
	value, err := strconv.ParseUint(c.PostForm(key), 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid form value")
	}
	return value, nil
}

func readOptionalUintForm(c *gin.Context, key string) (*uint64, error) {
	raw := c.PostForm(key)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return nil, errors.New("invalid form value")
	}
	return &value, nil
}
