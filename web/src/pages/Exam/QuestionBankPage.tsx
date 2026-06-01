import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { ArrowLeft, Search, X } from "lucide-react";
import { useEffect, useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { formatApiErrorMessage } from "../../api/client";
import { questionApi } from "../../api/questions";
import type { QuestionAPI, QuestionRow, QuestionType } from "../../api/questions";

type QuestionBankPageProps = {
  api?: QuestionAPI;
  tenantID?: number;
  spaceID?: number;
};

type ImportRecord = {
  id: number;
  fileName: string;
  successCount: number;
  errorCount: number;
};

const questionDifficultyLabels = {
  easy: "简单",
  medium: "中等",
  hard: "困难",
};

const questionTypeLabels: Record<QuestionType, string> = {
  single: "单选题",
  multiple: "多选题",
  judge: "判断题",
  fill_blank: "填空题",
  short_text: "简答题",
};

const questionAuthorRoleLabels: Record<string, string> = {
  tenant_admin: "租户管理员",
  space_admin: "空间管理员",
  teacher: "教师",
  student: "学生",
};

export function QuestionBankPage({ api = questionApi, tenantID = 10, spaceID }: QuestionBankPageProps) {
  const [questions, setQuestions] = useState<QuestionRow[]>([]);
  const [isImportDrawerOpen, setIsImportDrawerOpen] = useState(false);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importMessage, setImportMessage] = useState("");
  const [importRecords, setImportRecords] = useState<ImportRecord[]>([]);
  const [isImporting, setIsImporting] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [loadError, setLoadError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [pendingQuestionID, setPendingQuestionID] = useState<number | null>(null);
  const [isQuestionListRefreshing, setIsQuestionListRefreshing] = useState(false);
  const questionCreateHref = `/questions/new${window.location.search}`;

  useEffect(() => {
    let ignore = false;

    api.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then((data) => {
        if (!ignore) {
          setQuestions(data.items);
          setLoadError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("题目列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, spaceID]);

  const filteredQuestions = questions.filter((item) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 题库搜索只匹配题目列表可见字段，便于按题干、标签、选项或解析快速定位。
    return [
      item.title,
      item.stem,
      item.tags.join(" "),
      item.options.join(" "),
      item.analysis,
      questionDifficultyLabels[item.difficulty],
      questionTypeLabels[item.type],
      item.authorName ?? "",
      questionAuthorRoleLabels[item.authorRole ?? ""] ?? item.authorRole ?? "",
      formatQuestionCreatedAt(item.createdAt ?? 0),
      item.status === "ready" ? "可用" : item.status === "disabled" ? "禁用" : "草稿",
    ].some(
      (value) => value.toLowerCase().includes(keyword),
    );
  });

  function handleSearchQuestions() {
    setAppliedSearchQuery(searchQuery);
  }

  async function handleRefreshQuestions() {
    setSearchQuery("");
    setAppliedSearchQuery("");
    setIsQuestionListRefreshing(true);
    try {
      const data = await withRefreshFeedback(api.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) }));
      setQuestions(data.items);
      setLoadError("");
    } catch {
      setLoadError("题目列表加载失败");
    } finally {
      setIsQuestionListRefreshing(false);
    }
  }

  async function handleImportQuestions() {
    if (!importFile) {
      setImportMessage("请选择题目导入文件");
      return;
    }

    setIsImporting(true);
    try {
      const result = await api.importQuestions({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        file: importFile,
      });
      setImportRecords((items) => [
        {
          id: Date.now(),
          fileName: importFile.name,
          successCount: result.successCount,
          errorCount: result.errors.length,
        },
        ...items,
      ]);
      setImportMessage(`${importFile.name} 导入成功 ${result.successCount} 条，失败 ${result.errors.length} 条`);
      const data = await api.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) });
      setQuestions(data.items);
      setImportFile(null);
    } catch {
      setImportMessage("题目导入失败");
    } finally {
      setIsImporting(false);
    }
  }

  async function runQuestionAction(question: QuestionRow, action: "delete" | "disable" | "enable") {
    setPendingQuestionID(question.id);
    setLoadError("");
    setActionMessage("");
    try {
      if (action === "delete") {
        await api.deleteQuestion({ tenantID, questionID: question.id });
        setQuestions((items) => items.filter((item) => item.id !== question.id));
        setActionMessage("题目已删除");
        return;
      }
      const updated = action === "disable"
        ? await api.disableQuestion({ tenantID, questionID: question.id })
        : await api.enableQuestion({ tenantID, questionID: question.id });
      setQuestions((items) => items.map((item) => item.id === updated.id ? updated : item));
      setActionMessage(action === "disable" ? "题目已禁用" : "题目已启用");
    } catch (error) {
      setLoadError(formatApiErrorMessage(error, action === "delete" ? "删除题目失败" : "更新题目状态失败"));
    } finally {
      setPendingQuestionID(null);
    }
  }

  return (
    <section className="page platform-page exam-builder-page">
      <nav aria-label="题库菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/questions" role="tab" aria-selected="true">
          题库
        </a>
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="题库操作区">
            <a className="primary-button tenant-create-button" href={questionCreateHref}>
              新增题目
            </a>
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
        {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
        {actionMessage && <div className="tenant-admin-success" role="status">{actionMessage}</div>}
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">题干</th>
                <th scope="col">难度</th>
                <th scope="col">题目类型</th>
                <th scope="col">出题人</th>
                <th scope="col">出题人身份角色</th>
                <th scope="col">出题时间</th>
                <th scope="col">操作区</th>
              </tr>
            </thead>
            <tbody>
              {filteredQuestions.length === 0 && <EmptyTableRow colSpan={7} />}
              {filteredQuestions.map((item) => (
                <tr key={item.id}>
                  <td>
                    <strong title={item.title}>{truncateQuestionTitle(item.title)}</strong>
                    {item.status === "disabled" && (
                      <p className="tenant-admin-muted">
                        <StatusBadge tone="warning">已禁用</StatusBadge>
                      </p>
                    )}
                  </td>
                  <td>
                    {questionDifficultyLabels[item.difficulty]}
                  </td>
                  <td>{questionTypeLabels[item.type]}</td>
                  <td>{item.authorName || "-"}</td>
                  <td>{questionAuthorRoleLabels[item.authorRole ?? ""] ?? (item.authorRole || "-")}</td>
                  <td>{formatQuestionCreatedAt(item.createdAt ?? 0)}</td>
                  <td>
                    <div className="tenant-table-actions">
                      <a
                        aria-label={`编辑 ${item.title}`}
                        className="tenant-action-button tenant-action--edit"
                        href={`/questions/${item.id}/edit${window.location.search}`}
                      >
                        编辑
                      </a>
                      {item.status === "disabled" ? (
                        <Button
                          aria-label={`启用 ${item.title}`}
                          disabled={pendingQuestionID === item.id}
                          onClick={() => void runQuestionAction(item, "enable")}
                          variant="actionOpen"
                        >
                          启用
                        </Button>
                      ) : (
                        <Button
                          aria-label={`禁用 ${item.title}`}
                          disabled={pendingQuestionID === item.id}
                          onClick={() => void runQuestionAction(item, "disable")}
                          variant="actionClose"
                        >
                          禁用
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
                accept={["text/csv", "application/vnd.ms-excel", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"]}
                emptyPreviewText="暂未选择文件"
                label="题目导入文件"
                maxSizeBytes={2 * 1024 * 1024}
                onFileAccepted={(file) => {
                  setImportFile(file);
                  setImportMessage(file.name);
                }}
                uploadPrompt="选择 CSV/Excel 文件或拖动文件到此处"
              />
              <div className="platform-dialog__actions">
                <Button disabled={isImporting} variant="primary" onClick={() => void handleImportQuestions()} type="button">
                  确认导入
                </Button>
              </div>
              {importMessage && <div role="status" className="tenant-admin-success">{importMessage}</div>}
              <div className="table-wrap tenant-resource-drawer__table">
                <table className="data-table tenant-resource-table">
                  <thead>
                    <tr>
                      <th scope="col">文件</th>
                      <th scope="col">成功</th>
                      <th scope="col">失败</th>
                    </tr>
                  </thead>
                  <tbody>
                    {importRecords.length === 0 && <EmptyTableRow colSpan={3} />}
                    {importRecords.map((record) => (
                      <tr key={record.id}>
                        <td>{record.fileName}</td>
                        <td>{record.successCount}</td>
                        <td>{record.errorCount}</td>
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

function formatQuestionCreatedAt(value: number) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  const pad = (item: number) => String(item).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}
