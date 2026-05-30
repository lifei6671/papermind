import { useNavigate } from "react-router-dom";
import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { DoorOpen, ShieldCheck } from "lucide-react";
import { useState } from "react";
import { examApi } from "../../api/exams";
import type { ExamEntryAPI } from "../../api/exams";
import { formatApiErrorMessage } from "../../api/client";
import { type AuthSession, useSession } from "../../auth/session-context";

const examNotices = [
  "开考后系统自动开始计时，到达截止时间将自动交卷。",
  "答案会在作答过程中自动保存，请保持网络稳定。",
  "考试过程中请勿频繁切换页面，异常事件会被记录。",
  "交卷后无法继续修改答案，成绩公布后按试卷设置展示解析。",
];

type ExamEntryPageProps = {
  api?: ExamEntryAPI;
};

function canEnterExam(session: AuthSession | null) {
  if (!session?.user.tenantID) {
    return false;
  }
  if (session.user.role === "student") {
    return true;
  }
  if (!session.selectedSpaceID) {
    return false;
  }
  return session.profileSpaces?.some((space) =>
    space.status === "enabled" &&
    space.tenantID === session.user.tenantID &&
    space.spaceID === session.selectedSpaceID &&
    space.role === "student",
  ) ?? false;
}

export function ExamEntryPage({ api = examApi }: ExamEntryPageProps) {
  const navigate = useNavigate();
  const { session } = useSession();
  const [inviteCode, setInviteCode] = useState("");
  const [resolveMessage, setResolveMessage] = useState("");
  const [entryError, setEntryError] = useState("");

  async function handleEnterExam(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!canEnterExam(session)) {
      setResolveMessage("");
      setEntryError("请先使用租户学生账号登录后再进入考试");
      return;
    }

    try {
      // 邀请码进入考试前必须先由服务端解析，避免前端仅凭本地表单直接打开考试端。
      const exam = await api.resolveInvite({ inviteCode });
      setResolveMessage(`${exam.name} 已校验，正在进入考试端`);
      setEntryError("");
      navigate(`/student/exam?tenant_id=${exam.tenantID}&exam_id=${exam.id}`);
    } catch (err) {
      setResolveMessage("");
      setEntryError(formatApiErrorMessage(err, "邀请码校验失败"));
    }
  }

  return (
    <section className="page platform-page exam-entry-page">
      <nav aria-label="考试入口菜单" className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          考试入口
        </span>
      </nav>

      <div className="toolbar">
        <div>
          <h1>考试入口</h1>
          <p>考生通过邀请码进入考试，确认说明后打开独立考试端。</p>
        </div>
        <StatusBadge tone="info">P9.5</StatusBadge>
      </div>

      <div className="exam-entry-grid">
        <Panel title="邀请码进入" subtitle="用于临时参加已发布考试，仍需要登录或注册后作答。">
          <form className="platform-form" onSubmit={handleEnterExam}>
            <label className="field">
              <span>邀请码</span>
              <input
                onChange={(event) => setInviteCode(event.target.value.toUpperCase())}
                placeholder="例如 PM2026"
                required
                value={inviteCode}
              />
            </label>
            <div className="tenant-list-actions">
              <Button variant="primary" type="submit">
                <DoorOpen aria-hidden="true" size={16} />
                进入考试
              </Button>
            </div>
            {resolveMessage && (
              <div aria-label="entry-resolve-result" className="tenant-admin-status" role="status">
                {resolveMessage}
              </div>
            )}
            {entryError && <div className="tenant-admin-warning" role="alert">{entryError}</div>}
          </form>
        </Panel>

        <Panel title="考试说明" subtitle="首版考试端需要考生在进入前确认以下规则。">
          <ol className="exam-entry-notice-list">
            {examNotices.map((item, index) => (
              <li key={item}>
                <span>{index + 1}</span>
                {item}
              </li>
            ))}
          </ol>
          <div className="exam-entry-assurance">
            <ShieldCheck aria-hidden="true" size={22} />
            <span>服务端会以 exam token 和业务截止时间作为最终校验。</span>
          </div>
        </Panel>
      </div>
    </section>
  );
}
