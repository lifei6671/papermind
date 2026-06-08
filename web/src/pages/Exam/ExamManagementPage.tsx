import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { useEffect, useState } from "react";
import { ApiError } from "../../api/client";
import { useFeedback } from "../../app/feedback-context";
import { examApi } from "../../api/exams";
import type { ExamManagementAPI, ExamRow } from "../../api/exams";
import { paperApi as defaultPaperApi } from "../../api/papers";
import type { PaperAPI, PaperRow } from "../../api/papers";
import { spaceApi as defaultSpaceApi } from "../../api/spaces";
import type { SpaceManagementAPI, SpaceMember, SpaceMemberAPI, SpaceRow } from "../../api/spaces";
import { userApi as defaultUserApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI } from "../../api/users";

type ExamManagementPageProps = {
  api?: ExamManagementAPI;
  paperApi?: Pick<PaperAPI, "listPapers">;
  spaceApi?: Pick<SpaceManagementAPI & SpaceMemberAPI, "listSpaces" | "listSpaceMembers">;
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

type PublishScopeMode = "space" | "users";

export function ExamManagementPage({
  api = examApi,
  paperApi = defaultPaperApi,
  spaceApi = defaultSpaceApi,
  userApi = defaultUserApi,
  tenantID = 10,
  spaceID,
  canManageTenantTargets = true,
}: ExamManagementPageProps) {
  const { showError } = useFeedback();
  const [exams, setExams] = useState<ExamRow[]>([]);
  const [papers, setPapers] = useState<PaperRow[]>([]);
  const [targetOptions, setTargetOptions] = useState<TargetOption[]>([]);
  const [targetOptionsNotice, setTargetOptionsNotice] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [paperID, setPaperID] = useState("");
  const [scopeMode, setScopeMode] = useState<PublishScopeMode>("space");
  const [selectedSpaceTargetValue, setSelectedSpaceTargetValue] = useState("");
  const [selectedUserTargetValues, setSelectedUserTargetValues] = useState<string[]>([]);
  const [isUserPickerOpen, setIsUserPickerOpen] = useState(false);
  const [examDate, setExamDate] = useState("");
  const [startTimeText, setStartTimeText] = useState("");
  const [endTimeText, setEndTimeText] = useState("");
  const [durationMinutes, setDurationMinutes] = useState("120");
  const [publishMessage, setPublishMessage] = useState("");

  useEffect(() => {
    let ignore = false;

    api.listExams({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then((data) => {
        if (!ignore) {
          setExams(data.items);
        }
      })
      .catch(() => {
        if (!ignore) {
          showError("考试列表加载失败");
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
  }, [api, showError, tenantID, spaceID]);

  useEffect(() => {
    let ignore = false;

    const paperRequest = paperApi.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) });
    const targetRequest = canManageTenantTargets
      ? Promise.all([spaceApi.listSpaces(tenantID), userApi.listUsers(tenantID)])
          .then(([spaceData, userData]) => ({
            notice: null,
            options: buildTargetOptions(spaceData.items, userData.items),
          }))
      : spaceID === undefined
        ? Promise.resolve({ notice: null, options: [] })
        : spaceApi.listSpaceMembers({ tenantID, spaceID })
            .then((memberData) => ({
              notice: null,
              options: [
                currentSpaceTargetOption(spaceID),
                ...buildSpaceMemberTargetOptions(memberData.items),
              ],
            }))
            .catch((error: unknown) => {
              if (error instanceof ApiError && error.status === 403) {
                return {
                  // 当前身份无法读取成员列表时，只允许继续向当前空间发布，并显式提示范围已收窄。
                  notice: "当前身份无法读取空间成员列表，仅支持向当前空间发布考试",
                  options: [currentSpaceTargetOption(spaceID)],
                };
              }
              throw error;
            });

    Promise.all([paperRequest, targetRequest])
      .then(([paperData, targetData]) => {
        if (ignore) {
          return;
        }
        const availablePapers = paperData.items.filter((paper) => paper.status === "enabled");
        const nextTargets = targetData.options;
        setPapers(availablePapers);
        setTargetOptions(nextTargets);
        setTargetOptionsNotice(targetData.notice);
        setPaperID((current) =>
          current && availablePapers.some((paper) => String(paper.id) === current)
            ? current
            : String(availablePapers[0]?.id ?? ""),
        );
        setSelectedSpaceTargetValue((current) => {
          const availableSpaceTargets = nextTargets.filter((target) => target.type === "space");
          return current && availableSpaceTargets.some((target) => target.value === current)
            ? current
            : availableSpaceTargets[0]?.value ?? "";
        });
        setSelectedUserTargetValues((current) => {
          const availableUserValues = new Set(
            nextTargets.filter((target) => target.type === "user").map((target) => target.value),
          );
          return current.filter((value) => availableUserValues.has(value));
        });
        setScopeMode((current) => {
          const hasCurrentModeOptions = nextTargets.some((target) =>
            current === "space" ? target.type === "space" : target.type === "user",
          );
          if (hasCurrentModeOptions) {
            return current;
          }
          return nextTargets.some((target) => target.type === "space") ? "space" : "users";
        });
      })
      .catch(() => {
        if (!ignore) {
          setPapers([]);
          setTargetOptions([]);
          setTargetOptionsNotice(null);
          setPaperID("");
          setSelectedSpaceTargetValue("");
          setSelectedUserTargetValues([]);
          setScopeMode("space");
          setIsUserPickerOpen(false);
          showError("发布选项加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [paperApi, spaceApi, tenantID, userApi, spaceID, canManageTenantTargets, showError]);

  const spaceTargetOptions = targetOptions.filter((option) => option.type === "space");
  const userTargetOptions = targetOptions.filter((option) => option.type === "user");
  const selectedUserTargetSet = new Set(selectedUserTargetValues);
  const selectedUserLabels = userTargetOptions
    .filter((option) => selectedUserTargetSet.has(option.value))
    .map((option) => option.label);

  async function handlePublishExam(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const selectedPaper = papers.find((option) => String(option.id) === paperID);
    const selectedTargets = scopeMode === "space"
      ? spaceTargetOptions.filter((option) => option.value === selectedSpaceTargetValue)
      : userTargetOptions.filter((option) => selectedUserTargetSet.has(option.value));
    if (!selectedPaper || selectedTargets.length === 0) {
      showError("请先选择试卷和发布范围");
      return;
    }

    const startTime = combineDateAndTime(examDate, startTimeText);
    const endTime = combineDateAndTime(examDate, endTimeText);
    const duration = Number(durationMinutes);
    if (!Number.isFinite(startTime) || !Number.isFinite(endTime) || !Number.isFinite(duration)) {
      showError("请选择有效的考试日期和时间");
      return;
    }
    if (endTime <= startTime) {
      showError("考试结束时间必须晚于开始时间");
      return;
    }
    const windowMinutes = (endTime - startTime) / 60000;

    if (duration > windowMinutes) {
      showError("作答时长不能超过考试时间窗口");
      return;
    }

    try {
      const nextExam = await api.publishExam({
        tenantID,
        paperID: selectedPaper.id,
        name: selectedPaper.name,
        targets: selectedTargets.map((target) => ({
          targetType: target.type,
          targetID: target.id,
        })),
        targetType: selectedTargets[0].type,
        targetID: selectedTargets[0].id,
        startTime,
        endTime,
        durationMinutes: duration,
        maxAttempts: 1,
        resultStrategy: "latest",
        publishMode: "manual_publish",
      });

      setExams((items) => [...items, nextExam]);
      setPublishMessage(`${nextExam.name} 已发布，邀请码 ${nextExam.inviteCode}`);
      setIsPublishDialogOpen(false);
    } catch {
      showError("发布考试失败");
    }
  }

  function toggleUserTarget(value: string) {
    setSelectedUserTargetValues((current) =>
      current.includes(value)
        ? current.filter((item) => item !== value)
        : [...current, value],
    );
  }

  return (
    <section className="page platform-page">
      <nav aria-label="考试菜单" className="platform-tabbar" role="tablist">
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          考试列表
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
                <RequiredLabel>发布试卷</RequiredLabel>
                <select aria-label="发布试卷" onChange={(event) => setPaperID(event.target.value)} required value={paperID}>
                  {papers.map((option) => (
                    <option key={option.id} value={option.id}>{option.name}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <RequiredLabel>考试日期</RequiredLabel>
                <input aria-label="考试日期" onChange={(event) => setExamDate(event.target.value)} required type="date" value={examDate} />
              </label>
              <div className="exam-time-range">
                <label className="field">
                  <RequiredLabel>开始时间</RequiredLabel>
                  <input aria-label="开始时间" onChange={(event) => setStartTimeText(event.target.value)} required type="time" value={startTimeText} />
                </label>
                <label className="field">
                  <RequiredLabel>结束时间</RequiredLabel>
                  <input aria-label="结束时间" onChange={(event) => setEndTimeText(event.target.value)} required type="time" value={endTimeText} />
                </label>
              </div>
              <label className="field">
                <RequiredLabel>单次作答时长</RequiredLabel>
                <input
                  aria-label="单次作答时长"
                  min={1}
                  onChange={(event) => setDurationMinutes(event.target.value)}
                  required
                  type="number"
                  value={durationMinutes}
                />
              </label>
              <fieldset className="exam-publish-scope">
                <legend>
                  <RequiredLabel>发布范围</RequiredLabel>
                </legend>
                <div aria-label="发布范围类型" className="exam-scope-mode" role="radiogroup">
                  <label className="exam-scope-mode__item">
                    <input
                      checked={scopeMode === "space"}
                      onChange={() => setScopeMode("space")}
                      type="radio"
                    />
                    <span>班级范围</span>
                  </label>
                  <label className="exam-scope-mode__item">
                    <input
                      checked={scopeMode === "users"}
                      onChange={() => setScopeMode("users")}
                      type="radio"
                    />
                    <span>指定人群</span>
                  </label>
                </div>
                {scopeMode === "space" ? (
                  <label className="field">
                    <span className="field-label">选择班级范围</span>
                    <select
                      aria-label="选择班级范围"
                      onChange={(event) => setSelectedSpaceTargetValue(event.target.value)}
                      required
                      value={selectedSpaceTargetValue}
                    >
                      {spaceTargetOptions.map((option) => (
                        <option key={option.value} value={option.value}>{option.label}</option>
                      ))}
                    </select>
                  </label>
                ) : (
                  <div className="field exam-user-picker">
                    <span className="field-label">指定同学</span>
                    <button
                      aria-expanded={isUserPickerOpen}
                      aria-label="指定同学"
                      className="exam-user-picker__trigger"
                      onClick={() => setIsUserPickerOpen((current) => !current)}
                      type="button"
                    >
                      {selectedUserLabels.length > 0 ? selectedUserLabels.join("、") : "请选择指定同学"}
                    </button>
                    {isUserPickerOpen && (
                      <div aria-label="指定同学列表" className="exam-user-picker__menu" role="group">
                        {userTargetOptions.length === 0 && (
                          <div className="exam-user-picker__empty">暂无可选学生</div>
                        )}
                        {userTargetOptions.map((option) => (
                          <label className="exam-user-picker__option" key={option.value}>
                            <input
                              checked={selectedUserTargetSet.has(option.value)}
                              onChange={() => toggleUserTarget(option.value)}
                              type="checkbox"
                            />
                            <span>{option.label}</span>
                          </label>
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </fieldset>
              {targetOptionsNotice && <div className="tenant-admin-warning" role="status">{targetOptionsNotice}</div>}
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

function combineDateAndTime(dateValue: string, timeValue: string): number {
  if (!dateValue || !timeValue) {
    return Number.NaN;
  }
  return new Date(`${dateValue}T${timeValue}`).getTime();
}

function RequiredLabel({ children }: { children: string }) {
  return (
    <span className="field-label">
      {children}
      <span aria-hidden="true" className="required-marker">*</span>
    </span>
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
      .filter((user) => user.role === "student" && user.status === "enabled")
      .map((user) => ({
        id: user.id,
        label: `${user.name}（个人）`,
        type: "user" as const,
        value: `user:${user.id}`,
      })),
  ];
}

// buildSpaceMemberTargetOptions 将当前空间内启用学生转换成个人发布目标。
//
// 空间管理员和教师只能在当前空间范围内发起考试，因此候选个人目标必须来自
// space_members，而不能直接复用租户全量用户列表。
function buildSpaceMemberTargetOptions(members: SpaceMember[]): TargetOption[] {
  return members
    .filter((member) => member.role === "student" && member.status === "enabled")
    .map((member) => ({
      id: member.userID,
      label: `${member.name}（个人）`,
      type: "user" as const,
      value: `user:${member.userID}`,
    }));
}
