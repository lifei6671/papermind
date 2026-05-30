import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { useEffect, useState } from "react";
import { examApi } from "../../api/exams";
import type { ExamManagementAPI, ExamRow } from "../../api/exams";
import { paperApi as defaultPaperApi } from "../../api/papers";
import type { PaperAPI, PaperRow } from "../../api/papers";
import { spaceApi as defaultSpaceApi } from "../../api/spaces";
import type { SpaceManagementAPI, SpaceRow } from "../../api/spaces";
import { userApi as defaultUserApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI } from "../../api/users";

type ExamManagementPageProps = {
  api?: ExamManagementAPI;
  paperApi?: Pick<PaperAPI, "listPapers">;
  spaceApi?: Pick<SpaceManagementAPI, "listSpaces">;
  userApi?: Pick<UserManagementAPI, "listUsers">;
  tenantID?: number;
  spaceID?: number;
  canManageTenantTargets?: boolean;
};

type TargetOption = {
  id: number;
  label: string;
  type: "space" | "user";
  value: string;
};

export function ExamManagementPage({
  api = examApi,
  paperApi = defaultPaperApi,
  spaceApi = defaultSpaceApi,
  userApi = defaultUserApi,
  tenantID = 10,
  spaceID,
  canManageTenantTargets = true,
}: ExamManagementPageProps) {
  const [exams, setExams] = useState<ExamRow[]>([]);
  const [papers, setPapers] = useState<PaperRow[]>([]);
  const [targetOptions, setTargetOptions] = useState<TargetOption[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [paperID, setPaperID] = useState("");
  const [targetValue, setTargetValue] = useState("");
  const [startAt, setStartAt] = useState("");
  const [endAt, setEndAt] = useState("");
  const [durationMinutes, setDurationMinutes] = useState("120");
  const [publishError, setPublishError] = useState("");
  const [publishMessage, setPublishMessage] = useState("");

  useEffect(() => {
    let ignore = false;

    api.listExams({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
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
  }, [api, tenantID, spaceID]);

  useEffect(() => {
    let ignore = false;

    const paperRequest = paperApi.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) });
    const targetRequest = canManageTenantTargets
      ? Promise.all([spaceApi.listSpaces(tenantID), userApi.listUsers(tenantID)])
          .then(([spaceData, userData]) => buildTargetOptions(spaceData.items, userData.items))
      : Promise.resolve(spaceID === undefined ? [] : [currentSpaceTargetOption(spaceID)]);

    Promise.all([paperRequest, targetRequest])
      .then(([paperData, nextTargets]) => {
        if (ignore) {
          return;
        }
        setPapers(paperData.items);
        setTargetOptions(nextTargets);
        setPaperID((current) => current || String(paperData.items[0]?.id ?? ""));
        setTargetValue((current) => current || (nextTargets[0]?.value ?? ""));
      })
      .catch(() => {
        if (!ignore) {
          setPublishError("发布选项加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [paperApi, spaceApi, tenantID, userApi, spaceID, canManageTenantTargets]);

  async function handlePublishExam(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const selectedPaper = papers.find((option) => String(option.id) === paperID);
    const selectedTarget = targetOptions.find((option) => option.value === targetValue);
    if (!selectedPaper || !selectedTarget) {
      setPublishError("请先选择试卷和发布范围");
      return;
    }

    const startTime = new Date(startAt).getTime();
    const endTime = new Date(endAt).getTime();
    const duration = Number(durationMinutes);
    const windowMinutes = (endTime - startTime) / 60000;

    if (duration > windowMinutes) {
      setPublishError("作答时长不能超过考试时间窗口");
      return;
    }

    const nextExam = await api.publishExam({
      tenantID,
      paperID: selectedPaper.id,
      name: selectedPaper.name,
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
              {exams.length === 0 && <EmptyTableRow colSpan={6} />}
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
                <select onChange={(event) => setPaperID(event.target.value)} required value={paperID}>
                  {papers.map((option) => (
                    <option key={option.id} value={option.id}>{option.name}</option>
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
                <select onChange={(event) => setTargetValue(event.target.value)} required value={targetValue}>
                  {targetOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
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

function currentSpaceTargetOption(spaceID: number): TargetOption {
  return {
    id: spaceID,
    label: `当前空间 ${spaceID}`,
    type: "space",
    value: `space:${spaceID}`,
  };
}

function buildTargetOptions(spaces: SpaceRow[], users: TenantUserRow[]): TargetOption[] {
  return [
    ...spaces.map((space) => ({
      id: space.id,
      label: space.name,
      type: "space" as const,
      value: `space:${space.id}`,
    })),
    ...users
      .filter((user) => user.status === "enabled")
      .map((user) => ({
        id: user.id,
        label: `${user.name}（个人）`,
        type: "user" as const,
        value: `user:${user.id}`,
      })),
  ];
}
