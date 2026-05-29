package v1

import (
	"encoding/csv"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type importQuestionsResponse struct {
	SuccessCount int                           `json:"success_count"`
	Errors       []importQuestionErrorResponse `json:"errors"`
}

type importQuestionErrorResponse struct {
	RowNumber int    `json:"row_number"`
	Reason    string `json:"reason"`
}

func (h questionHandler) importQuestions(c *gin.Context) {
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
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "file 不能为空"))
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
		Rows:       rows,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	result.Errors = append(parseErrors, result.Errors...)
	c.JSON(http.StatusOK, response.OK(importQuestionsToResponse(result)))
}

func parseQuestionImportCSV(fileHeader *multipart.FileHeader, expectedHeaders []string) ([]servicequestion.ImportRow, []servicequestion.ImportError, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, nil, errors.New("读取导入文件失败")
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil, errors.New("导入文件不能为空")
		}
		return nil, nil, errors.New("读取导入表头失败")
	}
	if !sameImportHeaders(header, expectedHeaders) {
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
		if len(record) != len(expectedHeaders) {
			rowErrors = append(rowErrors, servicequestion.ImportError{RowNumber: rowNumber, Reason: "列数量与模板不一致"})
			continue
		}
		// CSV 模板列顺序与 QuestionService.ImportTemplateHeaders 保持一致，行号用于前端准确提示错误位置。
		rows = append(rows, servicequestion.ImportRow{
			RowNumber:     rowNumber,
			Type:          record[0],
			Title:         record[1],
			Options:       record[2],
			CorrectAnswer: record[3],
			Analysis:      record[4],
			Difficulty:    record[5],
			Tags:          record[6],
		})
	}
	return rows, rowErrors, nil
}

func importQuestionsToResponse(result servicequestion.ImportResult) importQuestionsResponse {
	errors := make([]importQuestionErrorResponse, 0, len(result.Errors))
	for _, item := range result.Errors {
		errors = append(errors, importQuestionErrorResponse{
			RowNumber: item.RowNumber,
			Reason:    item.Reason,
		})
	}
	return importQuestionsResponse{SuccessCount: result.SuccessCount, Errors: errors}
}

func sameImportHeaders(actual []string, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
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
