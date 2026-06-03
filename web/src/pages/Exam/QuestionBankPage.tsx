import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { ArrowLeft, Search, X } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { Pagination } from "../../components/ui/Pagination";
import { RadioGroup, RadioGroupItem } from "../../components/ui/RadioGroup";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { Select } from "../../components/ui/Select";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { Tooltip } from "../../components/ui/Tooltip";
import { formatApiErrorMessage } from "../../api/client";
import { useFeedback } from "../../app/feedback-context";
import { questionApi } from "../../api/questions";
import type { QuestionAPI, QuestionImportJobEvent, QuestionListResult, QuestionRow, QuestionType } from "../../api/questions";
import { questionImportTemplateFileName, questionImportTemplateHref } from "./questionImportTemplate";

type QuestionBankPageProps = {
  api?: QuestionAPI;
  tenantID?: number;
  spaceID?: number;
};

type ImportRecord = {
  id: number;
  file: File;
  fileName: string;
  status: "pending" | "uploading" | "processing" | "completed" | "failed";
  progress: number;
  successCount: number;
  errorCount: number;
  duplicateCount: number;
  message: string;
};

const questionDifficultyLabels = {
  easy: "简单",
  medium: "中等",
  hard: "困难",
};

const questionImportFileMaxBytes = 100 * 1024 * 1024;

const questionTypeLabels: Record<QuestionType, string> = {
  single: "单选题",
  multiple: "多选题",
  judge: "判断题",
  fill_blank: "填空题",
  short_text: "简答题",
};

const questionStatusLabels: Record<QuestionRow["status"], string> = {
  draft: "草稿",
  ready: "可用",
  disabled: "已禁用",
};

const questionAuthorRoleLabels: Record<string, string> = {
  tenant_admin: "租户管理员",
  space_admin: "空间管理员",
  teacher: "教师",
  student: "学生",
};

const questionPageSizeOptions = [10, 20, 30, 40, 50];

export function QuestionBankPage({ api = questionApi, tenantID = 10, spaceID }: QuestionBankPageProps) {
  const location = useLocation();
  const { showError, showSuccess } = useFeedback();
  const [questions, setQuestions] = useState<QuestionRow[]>([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [isImportDrawerOpen, setIsImportDrawerOpen] = useState(false);
  const [importRecords, setImportRecords] = useState<ImportRecord[]>([]);
  const [importTargetStatus, setImportTargetStatus] = useState<"draft" | "enabled">("draft");
  const [isImporting, setIsImporting] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [pendingQuestionID, setPendingQuestionID] = useState<number | null>(null);
  const [isQuestionListRefreshing, setIsQuestionListRefreshing] = useState(false);
  const questionCreateHref = `/questions/new${location.search}`;

  function applyQuestionPageData(data: QuestionListResult) {
    setQuestions(data.items);
    setPage(data.page);
    setPageSize(data.pageSize);
    setTotal(data.total);
  }

  useEffect(() => {
    let ignore = false;

    api.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }), page, pageSize, search: appliedSearchQuery })
      .then((data) => {
        if (!ignore) {
          applyQuestionPageData(data);
        }
      })
      .catch(() => {
        if (!ignore) {
          showError("题目列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, spaceID, page, pageSize, appliedSearchQuery, showError]);

  const visibleQuestions = questions;

  function handleSearchQuestions() {
    setPage(1);
    setAppliedSearchQuery(searchQuery.trim());
  }

  async function handleRefreshQuestions() {
    const nextPage = 1;
    setSearchQuery("");
    setAppliedSearchQuery("");
    setPage(nextPage);
    setIsQuestionListRefreshing(true);
    try {
      const data = await withRefreshFeedback(api.listQuestions({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        page: nextPage,
        pageSize,
        search: "",
      }));
      applyQuestionPageData(data);
    } catch {
      showError("题目列表加载失败");
    } finally {
      setIsQuestionListRefreshing(false);
    }
  }

  async function handleImportQuestions() {
    const pendingRecords = importRecords.filter((record) => record.status === "pending" || record.status === "failed");
    if (pendingRecords.length === 0) {
      showError("请选择题目导入文件");
      return;
    }

    setIsImporting(true);
    try {
      for (const record of pendingRecords) {
        await runImportRecord(record);
      }
      const data = await api.listQuestions({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        page,
        pageSize,
        search: appliedSearchQuery,
      });
      applyQuestionPageData(data);
    } catch (error) {
      showError(formatApiErrorMessage(error, "题目导入失败"));
    } finally {
      setIsImporting(false);
    }
  }

  async function runImportRecord(record: ImportRecord) {
    try {
      updateImportRecord(record.id, { status: "uploading", progress: Math.max(record.progress, 8), message: "上传中" });
      const job = await api.startQuestionImportJob({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        file: record.file,
        status: importTargetStatus,
      });
      updateImportRecord(record.id, { status: "processing", progress: Math.max(record.progress, 15), message: "解析中" });
      await waitForImportJob(record.id, job.jobID);
    } catch (error) {
      updateImportRecord(record.id, {
        status: "failed",
        progress: 100,
        message: formatApiErrorMessage(error, "题目导入失败"),
      });
      throw error;
    }
  }

  function waitForImportJob(recordID: number, jobID: string) {
    return new Promise<void>((resolve, reject) => {
      let unsubscribe: () => void = () => undefined;
      unsubscribe = api.subscribeQuestionImportJob(
        { jobID },
        (event) => {
          const progress = importEventProgress(event);
          updateImportRecord(recordID, {
            status: event.status === "completed" ? "completed" : event.status === "failed" ? "failed" : "processing",
            progress,
            successCount: event.successCount,
            errorCount: event.errorCount,
            duplicateCount: event.duplicateCount,
            message: event.message || importEventMessage(event),
          });
          if (event.status === "completed") {
            showSuccess(`${event.fileName} 导入完成 ${event.successCount} 条，失败 ${event.errorCount} 条，重复 ${event.duplicateCount} 条`);
            unsubscribe();
            resolve();
          }
          if (event.status === "failed") {
            unsubscribe();
            reject(new Error(event.message || "题目导入失败"));
          }
        },
        (error) => {
          updateImportRecord(recordID, { status: "failed", message: error.message });
          reject(error);
        },
      );
    });
  }

  function updateImportRecord(recordID: number, patch: Partial<ImportRecord>) {
    setImportRecords((items) => items.map((item) => item.id === recordID ? { ...item, ...patch } : item));
  }

  function addImportFiles(files: File[]) {
    const now = Date.now();
    setImportRecords((items) => [
      ...items,
      ...files.map((file, index) => ({
        id: now + index,
        file,
        fileName: file.name,
        status: "pending" as const,
        progress: 0,
        successCount: 0,
        errorCount: 0,
        duplicateCount: 0,
        message: "等待导入",
      })),
    ]);
  }

  async function runQuestionAction(question: QuestionRow, action: "delete" | "disable" | "enable") {
    setPendingQuestionID(question.id);
    try {
      if (action === "delete") {
        await api.deleteQuestion({ tenantID, questionID: question.id });
        setQuestions((items) => items.filter((item) => item.id !== question.id));
        setTotal((value) => Math.max(0, value - 1));
        showSuccess("题目已删除");
        return;
      }
      const updated = action === "disable"
        ? await api.disableQuestion({ tenantID, questionID: question.id })
        : await api.enableQuestion({ tenantID, questionID: question.id });
      setQuestions((items) => items.map((item) => item.id === updated.id ? updated : item));
      showSuccess(action === "disable" ? "题目已禁用" : "题目已启用");
    } catch (error) {
      showError(formatApiErrorMessage(error, action === "delete" ? "删除题目失败" : "更新题目状态失败"));
    } finally {
      setPendingQuestionID(null);
    }
  }

  return (
    <section className="page platform-page exam-builder-page">
      <nav aria-label="题库菜单" className="platform-tabbar" role="tablist">
        <Link className="platform-tab platform-tab--active" to={`/questions${location.search}`} role="tab" aria-selected="true">
          题库
        </Link>
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="题库操作区">
            <Link className="primary-button tenant-create-button" to={questionCreateHref}>
              新增题目
            </Link>
            <Button variant="toolbarSecondary" onClick={() => setIsImportDrawerOpen(true)} type="button">
              导入题目
            </Button>
          </div>
          <div className="tenant-search-actions">
            <label className="tenant-search-field">
              <span className="sr-only">搜索题目</span>
              <input
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="输入题目、标签、选项或解析"
                value={searchQuery}
              />
            </label>
            <Button aria-label="搜索" variant="icon" onClick={handleSearchQuestions} type="button">
              <Search aria-hidden="true" size={16} />
            </Button>
            <Button
              aria-label="刷新题目列表"
              variant="icon"
              disabled={isQuestionListRefreshing}
              onClick={() => void handleRefreshQuestions()}
              type="button"
            >
              <RefreshIcon active={isQuestionListRefreshing} />
            </Button>
          </div>
        </div>
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">ID</th>
                <th scope="col">题干</th>
                <th scope="col">难度</th>
                <th scope="col">题目类型</th>
                <th scope="col">题目状态</th>
                <th scope="col">出题人</th>
                <th scope="col">角色</th>
                <th scope="col">出题时间</th>
                <th scope="col">操作区</th>
              </tr>
            </thead>
            <tbody>
              {visibleQuestions.length === 0 && <EmptyTableRow colSpan={9} />}
              {visibleQuestions.map((item) => (
                <tr key={item.id}>
                  <td>{item.id}</td>
                  <td>
                    {renderQuestionTitleCell(item.title)}
                  </td>
                  <td>
                    {questionDifficultyLabels[item.difficulty]}
                  </td>
                  <td>{questionTypeLabels[item.type]}</td>
                  <td><StatusBadge tone={questionStatusTone(item.status)}>{questionStatusLabels[item.status]}</StatusBadge></td>
                  <td>{item.authorName || "-"}</td>
                  <td>{questionAuthorRoleLabels[item.authorRole ?? ""] ?? (item.authorRole || "-")}</td>
                  <td>{formatQuestionCreatedAt(item.createdAt ?? 0)}</td>
                  <td>
                    <div className="tenant-table-actions">
                      <Link
                        aria-label={`编辑 ${item.title}`}
                        className="tenant-action-button tenant-action--edit"
                        to={`/questions/${item.id}/edit${location.search}`}
                      >
                        编辑
                      </Link>
                      {item.status === "ready" ? (
                        <Button
                          aria-label={`禁用 ${item.title}`}
                          disabled={pendingQuestionID === item.id}
                          onClick={() => void runQuestionAction(item, "disable")}
                          variant="actionClose"
                        >
                          禁用
                        </Button>
                      ) : (
                        <Button
                          aria-label={`启用 ${item.title}`}
                          disabled={pendingQuestionID === item.id}
                          onClick={() => void runQuestionAction(item, "enable")}
                          variant="actionOpen"
                        >
                          启用
                        </Button>
                      )}
                      <Button
                        aria-label={`删除 ${item.title}`}
                        disabled={pendingQuestionID === item.id}
                        onClick={() => void runQuestionAction(item, "delete")}
                        variant="actionReset"
                      >
                        删除
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <div className="question-bank-pagination">
          <div className="question-bank-pagination__page-size">
            <span>每页条数</span>
            <div className="question-bank-pagination__page-size-select">
              <Select
                ariaLabel="每页条数"
                onChange={(value) => {
                  const nextPageSize = Number(value);
                  setPage(1);
                  setPageSize(nextPageSize);
                }}
                options={questionPageSizeOptions.map((option) => ({
                  value: String(option),
                  label: `${option} 条 / 页`,
                }))}
                value={String(pageSize)}
              />
            </div>
          </div>
          <Pagination page={page} pageSize={pageSize} total={total} onPageChange={setPage} />
        </div>
      </Panel>

      {isImportDrawerOpen && (
        <div className="tenant-resource-drawer-layer tenant-resource-drawer-layer--overlay tenant-resource-drawer-layer--visible">
          <div aria-hidden="true" className="tenant-resource-drawer-backdrop" />
          <aside
            aria-label="题目导入抽屉"
            aria-modal="true"
            className="tenant-resource-drawer tenant-resource-drawer--half tenant-resource-drawer--open"
            role="dialog"
          >
            <header className="tenant-resource-drawer__head tenant-resource-drawer__head--inline">
              <div className="tenant-resource-drawer__return-line">
                <button
                  aria-label="返回题库"
                  className="tenant-resource-drawer__back"
                  onClick={() => setIsImportDrawerOpen(false)}
                  type="button"
                >
                  <ArrowLeft aria-hidden="true" size={19} />
                  <span>返回</span>
                </button>
                <span aria-hidden="true" className="tenant-resource-drawer__separator">
                  |
                </span>
                <span className="tenant-resource-drawer__space-name">题目导入</span>
              </div>
              <div className="tenant-resource-drawer__tools">
                <button
                  aria-label="关闭题目导入"
                  className="tenant-resource-drawer__icon"
                  onClick={() => setIsImportDrawerOpen(false)}
                  type="button"
                >
                  <X aria-hidden="true" size={19} />
                </button>
              </div>
            </header>
            <div className="tenant-resource-drawer__body">
              <FileUploadField
                accept={["text/csv"]}
                emptyPreviewText="暂未选择文件"
                helperAction={
                  <a className="file-upload-field__template-link" download={questionImportTemplateFileName} href={questionImportTemplateHref}>
                    下载 CSV 模板
                  </a>
                }
                label="题目导入文件"
                maxSizeBytes={questionImportFileMaxBytes}
                onFileAccepted={() => undefined}
                onFilesAccepted={(files) => {
                  addImportFiles(files);
                }}
                multiple
                showPreview={false}
                uploadPrompt="选择 CSV 文件或拖动文件到此处"
              />
              <div className="question-import-status-field">
                <span>导入后题目状态</span>
                <RadioGroup
                  ariaLabel="导入后题目状态"
                  className="question-import-status-options"
                  onValueChange={(value) => setImportTargetStatus(value as "draft" | "enabled")}
                  value={importTargetStatus}
                >
                  <label className="question-import-status-option" htmlFor="question-import-status-draft">
                    <RadioGroupItem id="question-import-status-draft" value="draft" />
                    <span>草稿</span>
                  </label>
                  <label className="question-import-status-option" htmlFor="question-import-status-enabled">
                    <RadioGroupItem id="question-import-status-enabled" value="enabled" />
                    <span>已启用</span>
                  </label>
                </RadioGroup>
              </div>
              <div className="platform-dialog__actions">
                <Button disabled={isImporting} variant="primary" onClick={() => void handleImportQuestions()} type="button">
                  确认导入
                </Button>
              </div>
              <div className="table-wrap tenant-resource-drawer__table">
                <table className="data-table tenant-resource-table">
                  <thead>
                    <tr>
                      <th scope="col">文件</th>
                      <th scope="col">进度</th>
                      <th scope="col">结果</th>
                    </tr>
                  </thead>
                  <tbody>
                    {importRecords.length === 0 && <EmptyTableRow colSpan={3} />}
                    {importRecords.map((record) => (
                      <tr key={record.id}>
                        <td>{record.fileName}</td>
                        <td>
                          <div className="question-import-progress">
                            <progress aria-label={`${record.fileName} 导入进度`} max={100} value={record.progress} />
                            <span>{record.progress}%</span>
                          </div>
                        </td>
                        <td>{formatImportRecordResult(record)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </aside>
        </div>
      )}

    </section>
  );
}

function truncateQuestionTitle(title: string) {
  const normalized = title.replace(/\s+/g, " ").trim();
  if (normalized.length <= 32) {
    return normalized;
  }
  return `${normalized.slice(0, 32)}...`;
}

function importEventProgress(event: QuestionImportJobEvent) {
  if (event.status === "completed") {
    return 100;
  }
  if (event.status === "failed") {
    return 100;
  }
  if (event.totalRows <= 0) {
    return 15;
  }
  return Math.max(15, Math.min(99, Math.round((event.processedRows / event.totalRows) * 100)));
}

function importEventMessage(event: QuestionImportJobEvent) {
  if (event.status === "completed") {
    return "导入完成";
  }
  if (event.status === "failed") {
    return "导入失败";
  }
  return "导入中";
}

function formatImportRecordResult(record: ImportRecord) {
  if (record.status === "pending" || record.status === "uploading" || record.status === "processing") {
    return record.message;
  }
  if (record.status === "failed") {
    return record.message || "导入失败";
  }
  return `成功 ${record.successCount} / 失败 ${record.errorCount} / 重复 ${record.duplicateCount}`;
}

function renderQuestionTitleCell(title: string) {
  const truncated = truncateQuestionTitle(title);
  if (truncated === title.replace(/\s+/g, " ").trim()) {
    return <strong>{truncated}</strong>;
  }
  return (
    <Tooltip content={title}>
      <strong className="question-bank-title-tooltip-trigger" tabIndex={0}>
        {truncated}
      </strong>
    </Tooltip>
  );
}

function questionStatusTone(status: QuestionRow["status"]) {
  if (status === "ready") {
    return "success";
  }
  if (status === "disabled") {
    return "warning";
  }
  return "info";
}

function formatQuestionCreatedAt(value: number) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  const pad = (item: number) => String(item).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
