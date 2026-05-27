import { Button } from "../../components/ui/Button";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { questionApi } from "../../api/questions";
import type { QuestionImportAPI } from "../../api/questions";
import { formatApiErrorMessage } from "../../api/client";

type ImportRecord = {
  id: number;
  fileName: string;
  result: string;
};

type QuestionImportPageProps = {
  api?: QuestionImportAPI;
  tenantID?: number;
  spaceID?: number;
};

export function QuestionImportPage({ api = questionApi, tenantID = 10, spaceID }: QuestionImportPageProps) {
  const [importRecords, setImportRecords] = useState<ImportRecord[]>([]);
  const [importFile, setImportFile] = useState<File | null>(null);
  const [importMessage, setImportMessage] = useState("");
  const [isImporting, setIsImporting] = useState(false);
  const [isImportDialogOpen, setIsImportDialogOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");

  const filteredImportRecords = importRecords.filter((record) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 导入记录搜索只匹配当前可见字段，便于按文件名或解析结果快速定位。
    return [record.fileName, record.result].some((value) => value.toLowerCase().includes(keyword));
  });

  async function handleImportQuestions() {
    if (!importFile) {
      setImportMessage("请选择题目导入文件");
      return;
    }

    setIsImporting(true);
    try {
      // 题目文件由后端按统一 CSV 模板解析，前端只负责提交原始文件并展示成功数与行级错误。
      const result = await api.importQuestions({
        tenantID,
        spaceID,
        file: importFile,
      });
      const summary = formatImportSummary(result.successCount, result.errors.length);
      const detail = formatImportErrors(result.errors);
      setImportMessage(`${importFile.name} ${summary}${detail ? `：${detail}` : ""}`);
      setImportRecords((items) => [
        ...items,
        { id: Date.now(), fileName: importFile.name, result: summary },
      ]);
      setIsImportDialogOpen(false);
    } catch (err) {
      setImportMessage(formatApiErrorMessage(err, "导入题目失败"));
    } finally {
      setIsImporting(false);
    }
  }

  function handleSearchImportRecords() {
    setAppliedSearchQuery(searchQuery);
  }

  function handleRefreshImportRecords() {
    setSearchQuery("");
    setAppliedSearchQuery("");
  }

  function openImportDialog() {
    setImportFile(null);
    setIsImportDialogOpen(true);
  }

  function closeImportDialog() {
    setImportFile(null);
    setIsImportDialogOpen(false);
  }

  return (
    <section className="page platform-page exam-builder-page">
      <nav aria-label="题目导入菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/imports" role="tab" aria-selected="true">
          题目导入
        </a>
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="题目导入操作区">
            <Button
              variant="toolbarPrimary"
              onClick={openImportDialog}
              type="button"
            >
              导入题目
            </Button>
          </div>
          <div className="tenant-search-actions">
            <label className="tenant-search-field">
              <span className="sr-only">搜索导入记录</span>
              <input
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="输入文件名或解析结果"
                value={searchQuery}
              />
            </label>
            <Button aria-label="搜索" variant="icon" onClick={handleSearchImportRecords} type="button">
              <Search aria-hidden="true" size={16} />
            </Button>
            <Button
              aria-label="刷新导入记录"
              variant="icon"
              onClick={handleRefreshImportRecords}
              type="button"
            >
              <RefreshCw aria-hidden="true" size={16} />
            </Button>
          </div>
        </div>
        {importMessage && <div className="tenant-admin-status" role="status">{importMessage}</div>}
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">文件</th>
                <th scope="col">解析结果</th>
              </tr>
            </thead>
            <tbody>
              {filteredImportRecords.map((record) => (
                <tr key={record.id}>
                  <td>{record.fileName}</td>
                  <td>{record.result}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>

      {isImportDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="导入题目弹窗">
          <div className="platform-dialog__card">
            <h2>导入题目</h2>
            <div className="platform-form">
              <FileUploadField
                accept={["text/csv", "application/vnd.ms-excel"]}
                label="题目导入文件"
                maxSizeBytes={2 * 1024 * 1024}
                onFileAccepted={(file) => {
                  setImportFile(file);
                }}
              />
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={closeImportDialog} type="button">
                  取消
                </Button>
                <Button variant="primary" disabled={isImporting} onClick={handleImportQuestions} type="button">
                  {isImporting ? "导入中" : "确认导入"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}

function formatImportSummary(successCount: number, errorCount: number) {
  if (errorCount === 0) {
    return `已导入 ${successCount} 道题`;
  }
  return `已导入 ${successCount} 道题，${errorCount} 行失败`;
}

function formatImportErrors(errors: Array<{ rowNumber: number; reason: string }>) {
  return errors.map((error) => `第 ${error.rowNumber} 行：${error.reason}`).join("；");
}
