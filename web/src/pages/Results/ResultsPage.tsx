import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { Download, Save } from "lucide-react";
import { useState } from "react";

type ResultRow = {
  id: number;
  studentName: string;
  spaceName: string;
  attemptNo: number;
  objectiveScore: number;
  subjectiveScore: number;
  totalScore: number;
  submittedAt: string;
};

const resultRows: ResultRow[] = [
  {
    id: 1,
    studentName: "张三",
    spaceName: "高一 1 班",
    attemptNo: 1,
    objectiveScore: 72,
    subjectiveScore: 16,
    totalScore: 88,
    submittedAt: "2026-05-30 10:48",
  },
  {
    id: 2,
    studentName: "李四",
    spaceName: "高一 2 班",
    attemptNo: 1,
    objectiveScore: 68,
    subjectiveScore: 15,
    totalScore: 83,
    submittedAt: "2026-05-30 10:52",
  },
];

function formatPublishTime(value: string) {
  return value.replace("T", " ");
}

export function ResultsPage() {
  const [publishMode, setPublishMode] = useState("manual_publish");
  const [publishTime, setPublishTime] = useState("");
  const [publishMessage, setPublishMessage] = useState("");
  const [publishError, setPublishError] = useState("");
  const [exportMessage, setExportMessage] = useState("");
  const [exportHref, setExportHref] = useState("");

  function handleSavePublishConfig(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (publishMode === "scheduled_publish" && !publishTime.trim()) {
      setPublishError("统一公布时间不能为空。");
      setPublishMessage("");
      return;
    }

    // 成绩发布时间最终由服务端判断可见性，前端只负责提交配置并展示反馈。
    const visibleTime = publishTime ? formatPublishTime(publishTime) : "待教师手动发布";
    setPublishError("");
    setPublishMessage(`成绩发布配置已保存：${publishMode === "scheduled_publish" ? "统一公布" : "手动发布"}，${visibleTime}`);
  }

  function handleExportResults() {
    const header = "考生姓名,空间名称,attempt次数,客观题分,主观题分,总分,提交时间";
    const rows = resultRows.map((row) =>
      [row.studentName, row.spaceName, row.attemptNo, row.objectiveScore, row.subjectiveScore, row.totalScore, row.submittedAt].join(","),
    );
    const csv = [header, ...rows].join("\n");

    setExportHref(`data:text/csv;charset=utf-8,${encodeURIComponent(csv)}`);
    setExportMessage("成绩导出任务已创建，文件将写入 server/data/exports/papermind-results.csv");
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
        <Button variant="toolbarPrimary" onClick={handleExportResults} type="button">
          <Download aria-hidden="true" size={16} />
          导出成绩
        </Button>
      </div>

      <Panel title="高一语文期中考试" subtitle="result_strategy: highest，解析展示：成绩可见后展示。">
        <form className="result-publish-form" onSubmit={handleSavePublishConfig}>
          <label className="field">
            <span>成绩发布模式</span>
            <select onChange={(event) => setPublishMode(event.target.value)} value={publishMode}>
              <option value="manual_publish">教师手动发布</option>
              <option value="scheduled_publish">统一公布时间</option>
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
