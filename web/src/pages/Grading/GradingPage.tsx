import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { CheckCircle2, PenLine, RefreshCw, Search } from "lucide-react";
import { useState } from "react";

type PendingAttempt = {
  id: number;
  studentName: string;
  spaceName: string;
  examName: string;
  questionTitle: string;
  submittedAt: string;
  maxScore: number;
  version: number;
  status: "pending" | "completed";
};

const pendingAttempts: PendingAttempt[] = [
  {
    id: 1,
    studentName: "张三",
    spaceName: "高一 1 班",
    examName: "高一语文期中考试",
    questionTitle: "岳阳楼记思想内涵",
    submittedAt: "2026-05-30 10:48",
    maxScore: 10,
    version: 3,
    status: "pending",
  },
  {
    id: 2,
    studentName: "李四",
    spaceName: "高一 2 班",
    examName: "高一语文期中考试",
    questionTitle: "现代文阅读观点概括",
    submittedAt: "2026-05-30 10:52",
    maxScore: 8,
    version: 2,
    status: "pending",
  },
];

export function GradingPage() {
  const [attempts, setAttempts] = useState<PendingAttempt[]>(pendingAttempts);
  const [selectedAttempt, setSelectedAttempt] = useState<PendingAttempt | null>(null);
  const [score, setScore] = useState("7");
  const [comment, setComment] = useState("");
  const [saveMessage, setSaveMessage] = useState("");
  const [completeMessage, setCompleteMessage] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");

  const pendingCount = attempts.filter((attempt) => attempt.status === "pending").length;

  const filteredAttempts = attempts.filter((attempt) => {
    const keyword = appliedSearchQuery.trim();
    if (!keyword) {
      return true;
    }

    return [attempt.studentName, attempt.spaceName, attempt.examName, attempt.questionTitle].some((value) =>
      value.includes(keyword),
    );
  });

  function openGradingDialog(attempt: PendingAttempt) {
    setSelectedAttempt(attempt);
    setScore(String(Math.min(7, attempt.maxScore)));
    setComment("");
    setSaveMessage("");
    setCompleteMessage("");
  }

  function handleSaveGrading(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 阅卷保存需要携带 version，后端乐观锁冲突时应由 API 返回明确错误。
    setSaveMessage(`${selectedAttempt?.studentName ?? "当前考生"}简答题已保存 ${score} 分，version=${selectedAttempt?.version ?? 0}`);
  }

  function handleCompleteGrading() {
    if (!selectedAttempt) {
      return;
    }

    setAttempts((items) =>
      items.map((item) => item.id === selectedAttempt.id ? { ...item, status: "completed" } : item),
    );
    setSelectedAttempt((attempt) => attempt ? { ...attempt, status: "completed" } : attempt);
    setCompleteMessage(`${selectedAttempt?.studentName ?? "当前考生"}已完成阅卷，主观题和总分已重算。`);
  }

  return (
    <section className="page platform-page grading-page">
      <nav aria-label="阅卷中心菜单" className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          阅卷中心
        </span>
      </nav>

      <div className="toolbar">
        <div>
          <h1>阅卷中心</h1>
          <p>按 attempt 粒度处理简答题评分、评语和完成状态。</p>
        </div>
        <StatusBadge tone={pendingCount > 0 ? "warning" : "success"}>待阅卷 {pendingCount}</StatusBadge>
      </div>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="阅卷操作区">
            <Button variant="toolbarSecondary" onClick={() => setAppliedSearchQuery("")} type="button">
              <RefreshCw aria-hidden="true" size={16} />
              刷新待阅卷
            </Button>
          </div>
          <div className="tenant-search-actions">
            <label className="tenant-search-field">
              <span className="sr-only">搜索待阅卷</span>
              <input
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="考生、空间、考试或题目"
                value={searchQuery}
              />
            </label>
            <Button aria-label="搜索" variant="icon" onClick={() => setAppliedSearchQuery(searchQuery)} type="button">
              <Search aria-hidden="true" size={16} />
            </Button>
          </div>
        </div>

        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">考生</th>
                <th scope="col">空间</th>
                <th scope="col">考试</th>
                <th scope="col">题目</th>
                <th scope="col">提交时间</th>
                <th scope="col">状态</th>
                <th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredAttempts.map((attempt) => (
                <tr key={attempt.id}>
                  <td>{attempt.studentName}</td>
                  <td>{attempt.spaceName}</td>
                  <td>{attempt.examName}</td>
                  <td>{attempt.questionTitle}</td>
                  <td>{attempt.submittedAt}</td>
                  <td>
                    <StatusBadge tone={attempt.status === "pending" ? "warning" : "success"}>
                      {attempt.status === "pending" ? "待阅卷" : "已完成"}
                    </StatusBadge>
                  </td>
                  <td>
                    <Button
                      disabled={attempt.status === "completed"}
                      variant="actionEdit"
                      onClick={() => openGradingDialog(attempt)}
                      type="button"
                    >
                      开始阅卷
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>

      {selectedAttempt && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="简答题阅卷弹窗">
          <div className="platform-dialog__card grading-dialog">
            <h2>简答题阅卷</h2>
            <div className="grading-answer-block">
              <strong>{selectedAttempt.studentName} · {selectedAttempt.questionTitle}</strong>
              <p>学生作答：先忧后乐体现了士人以天下为己任的责任意识，也强调个人得失应服从公共价值。</p>
            </div>
            <form className="platform-form" onSubmit={handleSaveGrading}>
              <label className="field">
                <span>评分</span>
                <input
                  max={selectedAttempt.maxScore}
                  min={0}
                  onChange={(event) => setScore(event.target.value)}
                  required
                  type="number"
                  value={score}
                />
              </label>
              <label className="field">
                <span>阅卷评语</span>
                <textarea onChange={(event) => setComment(event.target.value)} value={comment} />
              </label>
              {saveMessage && (
                <div aria-label="grading-save-result" className="tenant-admin-status" role="status">
                  {saveMessage}
                </div>
              )}
              {completeMessage && (
                <div aria-label="grading-complete-result" className="tenant-admin-status" role="status">
                  {completeMessage}
                </div>
              )}
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setSelectedAttempt(null)} type="button">
                  关闭
                </Button>
                <Button variant="secondary" onClick={handleCompleteGrading} type="button">
                  <CheckCircle2 aria-hidden="true" size={16} />
                  完成阅卷
                </Button>
                <Button variant="primary" type="submit">
                  <PenLine aria-hidden="true" size={16} />
                  保存阅卷
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </section>
  );
}
