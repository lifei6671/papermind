import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { useEffect, useState } from "react";
import { examApi } from "../../api/exams";
import type { ExamManagementAPI, ExamRow } from "../../api/exams";

type ExamManagementPageProps = {
  api?: ExamManagementAPI;
  tenantID?: number;
};

const paperOptions = [
  { id: 100, name: "高一语文月考试卷" },
  { id: 101, name: "高二数学阶段测评" },
];

const targetOptions = [
  { id: 100, label: "高一全年级", type: "space" },
  { id: 200, label: "高一 1 班", type: "space" },
  { id: 201, label: "高一 2 班", type: "space" },
  { id: 300, label: "指定学生", type: "user" },
] as const;

export function ExamManagementPage({ api = examApi, tenantID = 10 }: ExamManagementPageProps) {
  const [exams, setExams] = useState<ExamRow[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [paperName, setPaperName] = useState<string>(paperOptions[0].name);
  const [target, setTarget] = useState<string>(targetOptions[0].label);
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const [durationMinutes, setDurationMinutes] = useState("120");
  const [publishError, setPublishError] = useState("");
  const [publishMessage, setPublishMessage] = useState("");

  useEffect(() => {
    let ignore = false;

    api.listExams(tenantID)
      .then((data) => {
        if (!ignore) {
          setExams(data.items);
          setPublishError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setPublishError("考试列表加载失败");
        }
      })
      .finally(() => {
        if (!ignore) {
          setIsLoading(false);
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID]);

  async function handlePublishExam(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const startTime = new Date(startAt).getTime();
    const endTime = new Date(endAt).getTime();
    const duration = Number(durationMinutes);
    const windowMinutes = (endTime - startTime) / 60000;

    if (duration > windowMinutes) {
      setPublishError("作答时长不能超过考试时间窗口");
      return;
    }

    const selectedPaper = paperOptions.find((option) => option.name === paperName) ?? paperOptions[0];
    const selectedTarget = targetOptions.find((option) => option.label === target) ?? targetOptions[0];
    const nextExam = await api.publishExam({
      tenantID,
      paperID: selectedPaper.id,
      name: paperName,
      targetType: selectedTarget.type,
      targetID: selectedTarget.id,
      startTime,
      endTime,
      durationMinutes: duration,
      maxAttempts: 1,
      resultStrategy: "latest",
      publishMode: "manual_publish",
    });

    setExams((items) => [...items, nextExam]);
    setPublishError("");
    setPublishMessage(`${nextExam.name} 已发布，邀请码 ${nextExam.inviteCode}`);
    setIsPublishDialogOpen(false);
  }

  return (
    <section className="page platform-page">
      <nav aria-label="考试菜单" className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          考试发布
        </span>
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="考试操作区">
            <Button variant="toolbarPrimary" onClick={() => setIsPublishDialogOpen(true)} type="button">
              发布考试
            </Button>
          </div>
        </div>

        {publishMessage && (
          <div aria-label="exam-publish-result" className="tenant-admin-status" role="status">
            {publishMessage}
          </div>
        )}
        {isLoading && <div className="tenant-admin-status" role="status">正在加载考试列表</div>}
        {publishError && !isPublishDialogOpen && <div className="tenant-admin-warning" role="alert">{publishError}</div>}

        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">考试</th>
                <th scope="col">试卷</th>
                <th scope="col">范围</th>
                <th scope="col">邀请码</th>
                <th scope="col">时间</th>
                <th scope="col">状态</th>
              </tr>
            </thead>
            <tbody>
              {exams.map((exam) => (
                <tr key={exam.id}>
                  <td>{exam.name}</td>
                  <td>{exam.paperName}</td>
                  <td>{exam.target}</td>
                  <td><code>{exam.inviteCode}</code></td>
                  <td>{exam.startAt} - {exam.endAt} / {exam.durationMinutes} 分钟</td>
                  <td>
                    <StatusBadge tone={exam.status === "published" ? "success" : "info"}>
                      {exam.status === "published" ? "已发布" : "草稿"}
                    </StatusBadge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>

      {isPublishDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="发布考试弹窗">
          <div className="platform-dialog__card">
            <h2>发布考试</h2>
            <form className="platform-form" onSubmit={handlePublishExam}>
              <label className="field">
                <span>发布试卷</span>
                <select onChange={(event) => setPaperName(event.target.value)} value={paperName}>
                  {paperOptions.map((option) => (
                    <option key={option.id} value={option.name}>{option.name}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span>考试开始时间</span>
                <input onChange={(event) => setStartAt(event.target.value)} required type="datetime-local" value={startAt} />
              </label>
              <label className="field">
                <span>考试结束时间</span>
                <input onChange={(event) => setEndAt(event.target.value)} required type="datetime-local" value={endAt} />
              </label>
              <label className="field">
                <span>单次作答时长</span>
                <input
                  min={1}
                  onChange={(event) => setDurationMinutes(event.target.value)}
                  required
                  type="number"
                  value={durationMinutes}
                />
              </label>
              <label className="field">
                <span>发布范围</span>
                <select onChange={(event) => setTarget(event.target.value)} value={target}>
                  {targetOptions.map((option) => (
                    <option key={option.id} value={option.label}>{option.label}</option>
                  ))}
                </select>
              </label>
              {publishError && <div className="tenant-admin-warning" role="alert">{publishError}</div>}
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsPublishDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认发布
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </section>
  );
}
