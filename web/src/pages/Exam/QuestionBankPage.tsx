import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { ArrowLeft, Search, X } from "lucide-react";
import { useEffect, useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { questionApi } from "../../api/questions";
import type { QuestionAPI, QuestionRow } from "../../api/questions";

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
      item.status === "ready" ? "可用" : "草稿",
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
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">题目</th>
                <th scope="col">标签</th>
                <th scope="col">选项</th>
                <th scope="col">解析</th>
                <th scope="col">状态</th>
              </tr>
            </thead>
            <tbody>
              {filteredQuestions.length === 0 && <EmptyTableRow colSpan={5} />}
              {filteredQuestions.map((item) => (
                <tr key={item.id}>
                  <td>
                    <strong>{item.title}</strong>
                    <p className="tenant-admin-muted">{item.stem}</p>
                  </td>
                  <td>{(item.tags.length > 0 ? item.tags : [item.tag]).join("、")}</td>
                  <td>{item.options.join(" / ")}</td>
                  <td>{`解析：${item.analysis}`}</td>
                  <td>
                    <StatusBadge tone={item.status === "ready" ? "success" : "info"}>
                      {item.status === "ready" ? "可用" : "草稿"}
                    </StatusBadge>
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
