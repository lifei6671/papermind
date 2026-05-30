import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { Download, Save } from "lucide-react";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { formatApiErrorMessage } from "../../api/client";
import type { ActorRole } from "../../api/grading";
import { resultsApi } from "../../api/results";
import type { ResultRow, ResultsAPI } from "../../api/results";

type ResultsPageProps = {
  api?: ResultsAPI;
  tenantID?: number;
  examID?: number;
  actorID?: number;
  actorRole?: ActorRole;
  spaceID?: number;
};

function formatPublishTime(value: string) {
  return value.replace("T", " ");
}

export function ResultsPage({
  api = resultsApi,
  tenantID = 10,
  examID,
  actorID = 1,
  actorRole = "tenant_admin",
  spaceID,
}: ResultsPageProps) {
  const [searchParams] = useSearchParams();
  const effectiveSpaceID = spaceID ?? positiveID(searchParams.get("space_id"));
  const effectiveExamID = examID ?? positiveID(searchParams.get("exam_id"));
  const [resultRows, setResultRows] = useState<ResultRow[]>([]);
  const [publishMode, setPublishMode] = useState("manual_publish");
  const [publishTime, setPublishTime] = useState("");
  const [publishMessage, setPublishMessage] = useState("");
  const [publishError, setPublishError] = useState("");
  const [exportMessage, setExportMessage] = useState("");
  const [exportHref, setExportHref] = useState("");
  const [loadError, setLoadError] = useState("");
  const teacherNeedsSpace = actorRole === "teacher" && !effectiveSpaceID;
  const needsExam = !effectiveExamID;
  const canExportResults = actorRole !== "teacher";

  useEffect(() => {
    if (teacherNeedsSpace || !effectiveExamID) {
      return undefined;
    }
    let ignore = false;
    api.listResults({ tenantID, examID: effectiveExamID, actorID, actorRole, spaceID: effectiveSpaceID })
      .then((result) => {
        if (!ignore) {
          setResultRows(result.items);
          setLoadError("");
        }
      })
      .catch((err) => {
        if (!ignore) {
          setLoadError(formatApiErrorMessage(err, "成绩列表加载失败"));
        }
      });
    return () => {
      ignore = true;
    };
  }, [api, tenantID, effectiveExamID, actorID, actorRole, effectiveSpaceID, teacherNeedsSpace]);

  async function handleSavePublishConfig(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!effectiveExamID) {
      return;
    }

    if (publishMode === "manual_publish" && !publishTime.trim()) {
      setPublishError("统一公布时间不能为空。");
      setPublishMessage("");
      return;
    }

    try {
      // 成绩发布时间最终由服务端判断可见性，前端只负责提交配置并展示反馈。
      const scorePublishTime = publishTime ? new Date(publishTime).getTime() : undefined;
      await api.savePublishConfig({
        tenantID,
        examID: effectiveExamID,
        actorID,
        actorRole,
        spaceID: effectiveSpaceID,
        publishMode: publishMode as "immediate_score" | "manual_publish",
        scorePublishTime,
      });
      const visibleTime = publishTime ? formatPublishTime(publishTime) : "提交后立即出分";
      setPublishError("");
      setPublishMessage(`成绩发布配置已保存：${publishMode === "manual_publish" ? "统一公布" : "立即出分"}，${visibleTime}`);
    } catch (err) {
      setPublishError(formatApiErrorMessage(err, "保存成绩发布配置失败"));
      setPublishMessage("");
    }
  }

  async function handleExportResults() {
    try {
      if (!effectiveExamID) {
        return;
      }
      const result = await api.exportResults({ tenantID, examID: effectiveExamID, actorID, actorRole, spaceID: effectiveSpaceID });
      setExportHref(result.fileURL);
      setExportMessage(`成绩导出完成：已导出 ${result.rowCount} 行，文件 ${result.filePath}`);
    } catch (err) {
      setExportHref("");
      setExportMessage(formatApiErrorMessage(err, "成绩导出失败"));
    }
  }

  if (needsExam) {
    return (
      <section className="page platform-page results-page">
        <nav aria-label="成绩菜单" className="platform-tabbar" role="tablist">
          <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
            成绩
          </span>
        </nav>
        <Panel>
          <div className="empty-state">
            <strong>请先选择考试后再查看成绩。</strong>
          </div>
        </Panel>
      </section>
    );
  }

  if (teacherNeedsSpace) {
    return (
      <section className="page platform-page results-page">
        <nav aria-label="成绩菜单" className="platform-tabbar" role="tablist">
          <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
            成绩
          </span>
        </nav>
        <Panel>
          <div className="empty-state">
            <strong>该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷</strong>
          </div>
        </Panel>
      </section>
    );
  }

  return (
    <section className="page platform-page results-page">
      <nav aria-label="成绩菜单" className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          成绩
        </span>
      </nav>

      <div className="toolbar">
        <div>
          <h1>成绩</h1>
          <p>查看考试成绩、配置发布策略，并导出成绩文件。</p>
        </div>
        {canExportResults && (
          <Button variant="toolbarPrimary" onClick={handleExportResults} type="button">
            <Download aria-hidden="true" size={16} />
            导出成绩
          </Button>
        )}
      </div>

      <Panel title="成绩发布配置" subtitle="按所选考试配置成绩可见性和解析展示。">
        <form className="result-publish-form" onSubmit={handleSavePublishConfig}>
          <label className="field">
            <span>成绩发布模式</span>
            <select onChange={(event) => setPublishMode(event.target.value)} value={publishMode}>
              <option value="manual_publish">统一公布时间</option>
              <option value="immediate_score">提交后立即出分</option>
            </select>
          </label>
          <label className="field">
            <span>统一公布时间</span>
            <input
              onChange={(event) => setPublishTime(event.target.value)}
              placeholder="2026-05-30T10:00"
              type="text"
              value={publishTime}
            />
          </label>
          <Button variant="primary" type="submit">
            <Save aria-hidden="true" size={16} />
            保存发布配置
          </Button>
        </form>

        {publishError && <div className="tenant-admin-warning" role="alert">{publishError}</div>}
        {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
        {publishMessage && (
          <div aria-label="result-publish-config" className="tenant-admin-status" role="status">
            {publishMessage}
          </div>
        )}
        {exportMessage && (
          <div aria-label="result-export" className="tenant-admin-status" role="status">
            {exportMessage}
            {exportHref && (
              <a className="result-export-link" download="papermind-results.csv" href={exportHref}>
                下载导出文件
              </a>
            )}
          </div>
        )}
      </Panel>

      <Panel>
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">考生姓名</th>
                <th scope="col">空间名称</th>
                <th scope="col">attempt 次数</th>
                <th scope="col">客观题分</th>
                <th scope="col">主观题分</th>
                <th scope="col">总分</th>
                <th scope="col">提交时间</th>
                <th scope="col">状态</th>
              </tr>
            </thead>
            <tbody>
              {resultRows.length === 0 && <EmptyTableRow colSpan={8} />}
              {resultRows.map((row) => (
                <tr key={row.id}>
                  <td>{row.studentName}</td>
                  <td>{row.spaceName}</td>
                  <td>{row.attemptNo}</td>
                  <td>{row.objectiveScore}</td>
                  <td>{row.subjectiveScore}</td>
                  <td><strong>{row.totalScore}</strong></td>
                  <td>{row.submittedAt}</td>
                  <td><StatusBadge tone="success">可发布</StatusBadge></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>
    </section>
  );
}

function positiveID(value?: string | number | null) {
  const numberValue = Number(value);
  return Number.isInteger(numberValue) && numberValue > 0 ? numberValue : undefined;
}
