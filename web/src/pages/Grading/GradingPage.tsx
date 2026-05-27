import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { PenLine, RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { formatApiErrorMessage } from "../../api/client";
import { gradingApi } from "../../api/grading";
import type { ActorRole, GradingAPI, PendingReviewRow } from "../../api/grading";

type GradingPageProps = {
  api?: GradingAPI;
  tenantID?: number;
  examID?: number;
  actorID?: number;
  actorRole?: ActorRole;
  spaceID?: number;
};

export function GradingPage({
  api = gradingApi,
  tenantID = 10,
  examID = 1,
  actorID = 1,
  actorRole = "tenant_admin",
  spaceID,
}: GradingPageProps) {
  const [attempts, setAttempts] = useState<PendingReviewRow[]>([]);
  const [selectedAttempt, setSelectedAttempt] = useState<PendingReviewRow | null>(null);
  const [score, setScore] = useState("7");
  const [comment, setComment] = useState("");
  const [saveMessage, setSaveMessage] = useState("");
  const [completeMessage, setCompleteMessage] = useState("");
  const [loadError, setLoadError] = useState("");
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

  async function loadPendingAttempts() {
    try {
      const result = await api.listPendingAttempts({ tenantID, examID, actorID, actorRole, spaceID });
      setAttempts(result.items);
      setLoadError("");
    } catch (err) {
      setLoadError(formatApiErrorMessage(err, "待阅卷列表加载失败"));
    }
  }

  useEffect(() => {
    let ignore = false;
    api.listPendingAttempts({ tenantID, examID, actorID, actorRole, spaceID })
      .then((result) => {
        if (!ignore) {
          setAttempts(result.items);
          setLoadError("");
        }
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setLoadError(formatApiErrorMessage(err, "待阅卷列表加载失败"));
        }
      });
    return () => {
      ignore = true;
    };
  }, [api, tenantID, examID, actorID, actorRole, spaceID]);

  function openGradingDialog(attempt: PendingReviewRow) {
    setSelectedAttempt(attempt);
    setScore(String(Math.min(7, Number(attempt.maxScore))));
    setComment("");
    setSaveMessage("");
    setCompleteMessage("");
  }

  async function handleSaveGrading(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedAttempt) {
      return;
    }

    try {
      // 阅卷保存必须携带答案 version，后端用乐观锁阻止并发覆盖评分。
      await api.gradeShortText({
        tenantID,
        examID,
        actorID,
        actorRole,
        spaceID,
        attemptID: selectedAttempt.attemptID,
        attemptQuestionID: selectedAttempt.attemptQuestionID,
        answerVersion: selectedAttempt.answerVersion,
        score,
        comment,
      });
      setAttempts((items) =>
        items.map((item) =>
          item.attemptID === selectedAttempt.attemptID && item.attemptQuestionID === selectedAttempt.attemptQuestionID
            ? { ...item, status: "completed" }
            : item,
        ),
      );
      setSelectedAttempt((attempt) => attempt ? { ...attempt, status: "completed" } : attempt);
      setSaveMessage(`${selectedAttempt.studentName}简答题已保存 ${score} 分`);
      setCompleteMessage(`${selectedAttempt.studentName}已完成阅卷，主观题和总分已重算。`);
    } catch (err) {
      setSaveMessage(formatApiErrorMessage(err, "保存阅卷结果失败"));
    }
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
            <Button variant="toolbarSecondary" onClick={() => { setAppliedSearchQuery(""); void loadPendingAttempts(); }} type="button">
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
        {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}

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
                <tr key={`${attempt.attemptID}-${attempt.attemptQuestionID}`}>
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
              <p>学生作答：{selectedAttempt.answerContent}</p>
            </div>
            <form className="platform-form" onSubmit={handleSaveGrading}>
              <label className="field">
                <span>评分</span>
                <input
                  max={selectedAttempt.maxScore}
                  min={0}
                  onChange={(event) => setScore(event.target.value)}
                  required
                  step="0.1"
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
