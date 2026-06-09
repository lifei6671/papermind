import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Pagination } from "../../components/ui/Pagination";
import { Panel } from "../../components/ui/Panel";
import { PlatformDrawer } from "../../components/ui/PlatformDrawer";
import { PlatformDrawerHeader } from "../../components/ui/PlatformDrawerHeader";
import { PlatformModal } from "../../components/ui/PlatformModal";
import { StatusBadge } from "../../components/ui/StatusBadge";
import DatePicker from "antd/es/date-picker";
import datePickerZhCN from "antd/es/date-picker/locale/zh_CN";
import AntInput from "antd/es/input";
import AntInputNumber from "antd/es/input-number";
import AntSelect from "antd/es/select";
import TimePicker from "antd/es/time-picker";
import dayjs from "dayjs";
import "dayjs/locale/zh-cn";
import { X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { ApiError } from "../../api/client";
import { useFeedback } from "../../app/feedback-context";
import { examApi } from "../../api/exams";
import type { ExamCreationStatus, ExamManagementAPI, ExamRow, ExamStatus, ExamStatusUpdate } from "../../api/exams";
import { paperApi as defaultPaperApi } from "../../api/papers";
import type { PaperAPI, PaperRow } from "../../api/papers";
import { spaceApi as defaultSpaceApi } from "../../api/spaces";
import type { SpaceManagementAPI, SpaceMember, SpaceMemberAPI, SpaceRow } from "../../api/spaces";
import { userApi as defaultUserApi } from "../../api/users";
import type { TenantUserRow, UserManagementAPI } from "../../api/users";

type ExamManagementPageProps = {
  api?: ExamManagementAPI;
  paperApi?: Pick<PaperAPI, "listPublishPaperCandidates">;
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
type ExamTimeMode = "fixed" | "window";
type PendingStatusAction = {
  exam: ExamRow;
  status: ExamStatusUpdate;
};

const dateFormat = "YYYY-MM-DD";
const timeFormat = "HH:mm";
const dateTimeFormat = "YYYY-MM-DD HH:mm";
dayjs.locale("zh-cn");
const examPageSizeOptions = [5, 10, 20, 50];
const defaultExamPageSize = 5;
const publishPaperSearchDebounceMs = 300;
const userTargetCandidatePageSize = 10;
const managementSpacePageSize = 100;
const publishExamPickerPlacements = {
  bottomLeft: {
    points: ["tl", "bl"],
    offset: [0, 4],
    overflow: {
      adjustX: 1,
      adjustY: false,
    },
  },
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
  const { showError, showSuccess } = useFeedback();
  const [exams, setExams] = useState<ExamRow[]>([]);
  const [papers, setPapers] = useState<PaperRow[]>([]);
  const [targetOptions, setTargetOptions] = useState<TargetOption[]>([]);
  const [targetOptionsNotice, setTargetOptionsNotice] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isPublishDialogOpen, setIsPublishDialogOpen] = useState(false);
  const [paperID, setPaperID] = useState("");
  const [paperSearchQuery, setPaperSearchQuery] = useState("");
  const [debouncedPaperSearchQuery, setDebouncedPaperSearchQuery] = useState("");
  const [examName, setExamName] = useState("");
  const [scopeMode, setScopeMode] = useState<PublishScopeMode>("space");
  const [selectedSpaceTargetValues, setSelectedSpaceTargetValues] = useState<string[]>([]);
  const [selectedUserTargetValues, setSelectedUserTargetValues] = useState<string[]>([]);
  const [userTargetQuery, setUserTargetQuery] = useState("");
  const [examTimeMode, setExamTimeMode] = useState<ExamTimeMode>("fixed");
  const [examDate, setExamDate] = useState("");
  const [startTimeText, setStartTimeText] = useState("");
  const [endTimeText, setEndTimeText] = useState("");
  const [windowStartText, setWindowStartText] = useState("");
  const [windowEndText, setWindowEndText] = useState("");
  const [durationMinutes, setDurationMinutes] = useState("120");
  const [examStatus, setExamStatus] = useState<ExamCreationStatus | "">("");
  const [maxAttempts, setMaxAttempts] = useState("");
  const [resultStrategy, setResultStrategy] = useState<"latest" | "highest">("latest");
  const [publishMode, setPublishMode] = useState<"immediate_score" | "manual_publish">("manual_publish");
  const [scorePublishDateTime, setScorePublishDateTime] = useState("");
  const [pendingStatusAction, setPendingStatusAction] = useState<PendingStatusAction | null>(null);
  const [examPage, setExamPage] = useState(1);
  const [examPageSize, setExamPageSize] = useState(defaultExamPageSize);
  const [examTotal, setExamTotal] = useState(0);
  const [candidateUserTargetOptions, setCandidateUserTargetOptions] = useState<TargetOption[]>([]);

  useEffect(() => {
    let ignore = false;

    api.listExams({ tenantID, ...(spaceID === undefined ? {} : { spaceID }), page: examPage, pageSize: examPageSize })
      .then((data) => {
        if (!ignore) {
          setExams(data.items);
          setExamTotal(data.total ?? data.items.length);
        }
      })
      .catch(() => {
        if (!ignore) {
          setExams([]);
          setExamTotal(0);
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
  }, [api, showError, tenantID, spaceID, examPage, examPageSize]);

  useEffect(() => {
    const timeoutID = window.setTimeout(() => {
      setDebouncedPaperSearchQuery(paperSearchQuery.trim());
    }, publishPaperSearchDebounceMs);

    return () => window.clearTimeout(timeoutID);
  }, [paperSearchQuery]);

  useEffect(() => {
    let ignore = false;

    paperApi.listPublishPaperCandidates({
      tenantID,
      ...(spaceID === undefined ? {} : { spaceID }),
      search: debouncedPaperSearchQuery,
    })
      .then((paperData) => {
        if (ignore) {
          return;
        }
        setPapers(paperData.items);
        setPaperID((current) =>
          current && paperData.items.some((paper) => String(paper.id) === current)
            ? current
            : String(paperData.items[0]?.id ?? ""),
        );
      })
      .catch(() => {
        if (!ignore) {
          setPapers([]);
          setPaperID("");
          showError("发布试卷加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [paperApi, tenantID, spaceID, debouncedPaperSearchQuery, showError]);

  useEffect(() => {
    let ignore = false;
    queueMicrotask(() => {
      if (ignore) {
        return;
      }
      setUserTargetQuery((current) => current === "" ? current : "");
      setCandidateUserTargetOptions((current) => current.length === 0 ? current : []);
      setSelectedUserTargetValues((current) => current.length === 0 ? current : []);
    });
    return () => {
      ignore = true;
    };
  }, [tenantID, spaceID, canManageTenantTargets]);

  useEffect(() => {
    let ignore = false;

    const targetRequest = canManageTenantTargets
      ? listAllTargetSpaces(spaceApi, tenantID)
          .then((spaceData) => ({
            notice: null,
            options: buildTargetOptions(spaceData.items, []),
          }))
      : spaceID === undefined
        ? Promise.resolve({ notice: null, options: [] })
        : spaceApi.listSpaceMembers({ tenantID, spaceID, page: 1, pageSize: 1 })
            .then(() => ({
              notice: null,
              options: [currentSpaceTargetOption(spaceID)],
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

    targetRequest
      .then((targetData) => {
        if (ignore) {
          return;
        }
        const nextTargets = targetData.options;
        setTargetOptions((current) =>
          mergeTargetOptions(nextTargets, current.filter((target) => target.type === "user" && selectedUserTargetValues.includes(target.value))),
        );
        setTargetOptionsNotice(targetData.notice);
        setSelectedSpaceTargetValues((current) => {
          const availableSpaceTargets = nextTargets.filter((target) => target.type === "space");
          const currentAvailable = current.filter((value) => availableSpaceTargets.some((target) => target.value === value));
          return currentAvailable.length > 0
            ? currentAvailable
            : availableSpaceTargets.slice(0, 1).map((target) => target.value);
        });
        setScopeMode((current) => {
          if (current === "users" && selectedUserTargetValues.length > 0) {
            return current;
          }
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
          setSelectedSpaceTargetValues([]);
          setSelectedUserTargetValues((current) => current.length === 0 ? current : []);
          setCandidateUserTargetOptions((current) => current.length === 0 ? current : []);
          setScopeMode("space");
          setUserTargetQuery((current) => current === "" ? current : "");
          showError("发布选项加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [spaceApi, tenantID, spaceID, canManageTenantTargets, showError, selectedUserTargetValues]);

  useEffect(() => {
    const keyword = userTargetQuery.trim();
    if (!isPublishDialogOpen || scopeMode !== "users" || keyword === "") {
      queueMicrotask(() => setCandidateUserTargetOptions((current) => current.length === 0 ? current : []));
      return undefined;
    }

    let ignore = false;
    const request = canManageTenantTargets
      ? userApi.listUsers({
          tenantID,
          page: 1,
          pageSize: userTargetCandidatePageSize,
          search: keyword,
          filters: { role: "student", status: "enabled" },
        })
          .then((data) => buildUserTargetOptions(data.items))
      : spaceID === undefined
        ? Promise.resolve([])
        : spaceApi.listSpaceMembers({ tenantID, spaceID, page: 1, pageSize: userTargetCandidatePageSize, search: keyword, role: "student", status: "enabled" })
            .then((data) => buildSpaceMemberTargetOptions(data.items));

    request
      .then((options) => {
        if (!ignore) {
          setCandidateUserTargetOptions(options);
        }
      })
      .catch(() => {
        if (!ignore) {
          setCandidateUserTargetOptions([]);
          showError("指定同学候选加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [canManageTenantTargets, isPublishDialogOpen, scopeMode, showError, spaceApi, spaceID, tenantID, userApi, userTargetQuery]);

  const spaceTargetOptions = targetOptions.filter((option) => option.type === "space");
  const userTargetOptions = mergeTargetOptions(targetOptions.filter((option) => option.type === "user"), candidateUserTargetOptions);
  const selectedUserTargetSet = new Set(selectedUserTargetValues);
  const selectedUserTargets = userTargetOptions.filter((option) => selectedUserTargetSet.has(option.value));
  const visibleUserTargetCandidates = userTargetOptions
    .filter((option) => !selectedUserTargetSet.has(option.value));

  function openPublishDialog() {
    setExamStatus("");
    setPaperSearchQuery("");
    setDebouncedPaperSearchQuery("");
    setExamName("");
    setMaxAttempts("");
    setResultStrategy("latest");
    setPublishMode("manual_publish");
    setScorePublishDateTime("");
    setIsPublishDialogOpen(true);
  }

  function closePublishDialog() {
    setExamStatus("");
    setPaperSearchQuery("");
    setDebouncedPaperSearchQuery("");
    setExamName("");
    setMaxAttempts("");
    setResultStrategy("latest");
    setPublishMode("manual_publish");
    setScorePublishDateTime("");
    setUserTargetQuery("");
    setCandidateUserTargetOptions([]);
    setIsPublishDialogOpen(false);
  }

  async function handlePublishExam(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const selectedPaper = papers.find((option) => String(option.id) === paperID);
    const selectedTargets = scopeMode === "space"
      ? spaceTargetOptions.filter((option) => selectedSpaceTargetValues.includes(option.value))
      : userTargetOptions.filter((option) => selectedUserTargetSet.has(option.value));
    if (!selectedPaper || selectedTargets.length === 0) {
      showError("请先选择试卷和发布范围");
      return;
    }
    if (examStatus === "") {
      showError("请选择考试状态");
      return;
    }
    const maxAttemptsValue = maxAttempts.trim() === "" ? 0 : Number(maxAttempts);
    if (!Number.isInteger(maxAttemptsValue) || maxAttemptsValue < 0) {
      showError("最多作答次数必须为空或正整数");
      return;
    }

    const timeSettings = examTimeMode === "fixed"
      ? buildFixedExamTime(examDate, startTimeText, endTimeText)
      : buildWindowExamTime(windowStartText, windowEndText, durationMinutes);
    if (!timeSettings.isValid) {
      showError("请选择有效的考试日期和时间");
      return;
    }
    if (timeSettings.endTime <= timeSettings.startTime) {
      showError("考试结束时间必须晚于开始时间");
      return;
    }
    const windowMinutes = (timeSettings.endTime - timeSettings.startTime) / 60000;

    if (timeSettings.durationMinutes > windowMinutes) {
      showError("作答时长不能超过考试时间窗口");
      return;
    }
    const scorePublishTime = scorePublishDateTime === ""
      ? null
      : parseDateTime(scorePublishDateTime)?.valueOf();
    if (scorePublishDateTime !== "" && !Number.isFinite(scorePublishTime)) {
      showError("请选择有效的成绩公布时间");
      return;
    }

    try {
      const nextExam = await api.publishExam({
        tenantID,
        paperID: selectedPaper.id,
        name: examName.trim() || selectedPaper.name,
        targets: selectedTargets.map((target) => ({
          targetType: target.type,
          targetID: target.id,
        })),
        targetType: selectedTargets[0].type,
        targetID: selectedTargets[0].id,
        startTime: timeSettings.startTime,
        endTime: timeSettings.endTime,
        durationMinutes: timeSettings.durationMinutes,
        maxAttempts: maxAttemptsValue,
        resultStrategy,
        publishMode,
        scorePublishTime,
        status: examStatus,
      });

      setExams((items) => examPage === 1 ? [nextExam, ...items].slice(0, examPageSize) : items);
      setExamTotal((total) => total + 1);
      setExamPage(1);
      showSuccess(examStatus === "published"
        ? `${nextExam.name} 已发布，邀请码 ${nextExam.inviteCode}`
        : `${nextExam.name} 已保存为草稿`);
      closePublishDialog();
    } catch {
      showError("新建考试失败");
    }
  }

  function requestUpdateExamStatus(exam: ExamRow, status: ExamStatusUpdate) {
    setPendingStatusAction({ exam, status });
  }

  async function confirmUpdateExamStatus() {
    if (!pendingStatusAction) {
      return;
    }
    const action = pendingStatusAction;
    setPendingStatusAction(null);
    await updateExamStatus(action.exam, action.status);
  }

  async function updateExamStatus(exam: ExamRow, status: ExamStatusUpdate) {
    if (!api.updateExamStatus) {
      showError("考试状态更新失败");
      return;
    }
    try {
      const updated = await api.updateExamStatus({ tenantID, examID: exam.id, status });
      setExams((items) => items.map((item) => item.id === updated.id ? updated : item));
      showSuccess(statusSuccessMessage(status, updated));
    } catch {
      showError("考试状态更新失败");
    }
  }

  function toggleUserTarget(value: string) {
    setSelectedUserTargetValues((current) =>
      current.includes(value)
        ? current.filter((item) => item !== value)
        : [...current, value],
    );
  }

  function selectUserTarget(value: string) {
    const selectedOption = userTargetOptions.find((option) => option.value === value);
    if (selectedOption) {
      setTargetOptions((current) => mergeTargetOptions(current, [selectedOption]));
      toggleUserTarget(value);
      setUserTargetQuery("");
      setCandidateUserTargetOptions([]);
    }
  }

  function changeUserTargetQuery(value: string) {
    if (userTargetOptions.some((option) => option.value === value)) {
      return;
    }
    setUserTargetQuery(value);
  }

  const totalExamPages = Math.max(1, Math.ceil(examTotal / examPageSize));
  const currentExamPage = Math.min(examPage, totalExamPages);

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
            <Button variant="toolbarPrimary" onClick={openPublishDialog} type="button">
              新建考试
            </Button>
          </div>
        </div>

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
                <th scope="col">操作</th>
              </tr>
            </thead>
            <tbody>
              {exams.length === 0 && <EmptyTableRow colSpan={7} />}
              {exams.map((exam) => (
                <tr key={exam.id}>
                  <td>{exam.name}</td>
                  <td>{exam.paperName}</td>
                  <td>{exam.target}</td>
                  <td><code>{exam.inviteCode}</code></td>
                  <td>{exam.startAt} - {exam.endAt} / {exam.durationMinutes} 分钟</td>
                  <td>
                    <StatusBadge tone={examStatusTone(exam.status)}>
                      {examStatusLabel(exam.status)}
                    </StatusBadge>
                  </td>
                  <td>
                    {canManageTenantTargets
                      ? <ExamStatusActions exam={exam} onUpdate={requestUpdateExamStatus} />
                      : <span className="tenant-admin-muted">暂无操作</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <Pagination
          onPageChange={setExamPage}
          onPageSizeChange={(nextPageSize) => {
            setExamPage(1);
            setExamPageSize(nextPageSize);
          }}
          page={currentExamPage}
          pageSize={examPageSize}
          pageSizeOptions={examPageSizeOptions}
          total={examTotal}
        />
      </Panel>

        <PlatformDrawer ariaLabel="新建考试抽屉" onClose={closePublishDialog} open={isPublishDialogOpen}>
          <PlatformDrawerHeader onBack={closePublishDialog} title="新建考试" />
            <form className="platform-form exam-publish-form" onSubmit={handlePublishExam}>
              <label className="field">
                <RequiredLabel>发布试卷</RequiredLabel>
                <AntSelect
                  aria-label="发布试卷"
                  className="exam-publish-select exam-paper-select"
                  filterOption={false}
                  getPopupContainer={getDrawerPopupContainer}
                  onChange={(value) => setPaperID(String(value))}
                  onSearch={(value) => setPaperSearchQuery(value)}
                  optionFilterProp="label"
                  options={papers.map((option) => ({ label: option.name, value: String(option.id) }))}
                  placeholder="输入试卷名称搜索"
                  showSearch
                  value={paperID || undefined}
                  virtual={false}
                />
              </label>
              <label className="field">
                <RequiredLabel>考试名称</RequiredLabel>
                <AntInput
                  aria-label="考试名称"
                  className="exam-publish-input"
                  onChange={(event) => setExamName(event.target.value)}
                  placeholder="默认使用试卷名称"
                  value={examName}
                />
              </label>
              <label className="field">
                <RequiredLabel>考试状态</RequiredLabel>
                <AntSelect
                  aria-label="考试状态"
                  className="exam-publish-select"
                  getPopupContainer={getDrawerPopupContainer}
                  onChange={(value) => setExamStatus(value as ExamCreationStatus)}
                  options={[
                    { label: "草稿", value: "draft" },
                    { label: "已发布", value: "published" },
                  ]}
                  placeholder="请选择考试状态"
                  value={examStatus || undefined}
                  virtual={false}
                />
              </label>
              <div className="exam-time-range">
                <label className="field">
                  <span>最多作答次数</span>
                  <AntInputNumber
                    aria-label="最多作答次数"
                    className="exam-publish-number"
                    min={1}
                    onChange={(value) => setMaxAttempts(value === null ? "" : String(value))}
                    placeholder="不限制"
                    precision={0}
                    value={maxAttempts === "" ? null : Number(maxAttempts)}
                  />
                  <small className="field-note">
                    留空表示不限制作答次数；填写数字时必须为正整数。包含主观题的试卷必须填写 1 次。
                  </small>
                </label>
                <label className="field">
                  <RequiredLabel>多次作答成绩</RequiredLabel>
                  <AntSelect
                    aria-label="多次作答成绩"
                    className="exam-publish-select"
                    getPopupContainer={getDrawerPopupContainer}
                    onChange={(value) => setResultStrategy(value as "latest" | "highest")}
                    options={[
                      { label: "最后一次", value: "latest" },
                      { label: "最高分", value: "highest" },
                    ]}
                    value={resultStrategy}
                    virtual={false}
                  />
                  <small className="field-note">
                    多次作答时按所选策略计算最终成绩。
                  </small>
                </label>
              </div>
              <div className="exam-time-range">
                <label className="field">
                  <RequiredLabel>成绩发布方式</RequiredLabel>
                  <AntSelect
                    aria-label="成绩发布方式"
                    className="exam-publish-select"
                    getPopupContainer={getDrawerPopupContainer}
                    onChange={(value) => setPublishMode(value as "immediate_score" | "manual_publish")}
                    options={[
                      { label: "手动发布成绩", value: "manual_publish" },
                      { label: "提交后立即出分", value: "immediate_score" },
                    ]}
                    value={publishMode}
                    virtual={false}
                  />
                  <small className="field-note">
                    手动发布时成绩由教师确认后展示；立即出分会在提交后展示客观题和已完成评分的成绩。
                  </small>
                </label>
                <label className="field">
                  <span className="field-label">成绩公布时间</span>
                  <DatePicker
                    aria-label="成绩公布时间"
                    builtinPlacements={publishExamPickerPlacements}
                    className="exam-ant-picker"
                    format={dateTimeFormat}
                    getPopupContainer={getDrawerPopupContainer}
                    inputReadOnly={false}
                    locale={datePickerZhCN}
                    onBlur={(event) => {
                      const value = inputValue(event.currentTarget).trim();
                      if (value) {
                        setScorePublishDateTime(normalizeDateTimeInput(value));
                      }
                    }}
                    onChange={(value) => setScorePublishDateTime(value?.format(dateTimeFormat) ?? "")}
                    placeholder="选择成绩公布时间"
                    placement="bottomLeft"
                    showTime={{ format: timeFormat }}
                    value={parseDateTime(scorePublishDateTime)}
                  />
                  <small className="field-note">
                    可选；留空时按成绩发布方式和系统默认规则展示成绩。
                  </small>
                </label>
              </div>
              <fieldset className="exam-publish-scope">
                <legend>
                  <RequiredLabel>考试时间</RequiredLabel>
                </legend>
                <div aria-label="考试时间类型" className="exam-scope-mode" role="radiogroup">
                  <label className="exam-scope-mode__item">
                    <input
                      checked={examTimeMode === "fixed"}
                      onChange={() => setExamTimeMode("fixed")}
                      type="radio"
                    />
                    <span>固定场次</span>
                  </label>
                  <label className="exam-scope-mode__item">
                    <input
                      checked={examTimeMode === "window"}
                      onChange={() => setExamTimeMode("window")}
                      type="radio"
                    />
                    <span>开放时间窗</span>
                  </label>
                </div>
                {examTimeMode === "fixed" ? (
                  <>
                    <label className="field">
                      <RequiredLabel>考试日期</RequiredLabel>
                      <DatePicker
                        aria-label="考试日期"
                        builtinPlacements={publishExamPickerPlacements}
                        className="exam-ant-picker"
                        format={dateFormat}
                        getPopupContainer={getDrawerPopupContainer}
                        inputReadOnly={false}
                        locale={datePickerZhCN}
                        onBlur={(event) => {
                          const value = inputValue(event.currentTarget).trim();
                          if (value) {
                            setExamDate(normalizeDateInput(value));
                          }
                        }}
                        onChange={(value) => setExamDate(value?.format(dateFormat) ?? "")}
                        panelRender={renderPickerPanel("考试日期选择面板")}
                        placeholder="选择考试日期"
                        placement="bottomLeft"
                        value={parseDate(examDate)}
                      />
                    </label>
                    <div className="exam-time-range">
                      <label className="field">
                        <RequiredLabel>开始时间</RequiredLabel>
                        <TimePicker
                          aria-label="开始时间"
                          builtinPlacements={publishExamPickerPlacements}
                          className="exam-ant-picker"
                          format={timeFormat}
                          getPopupContainer={getDrawerPopupContainer}
                          inputReadOnly={false}
                          onBlur={(event) => {
                            const value = inputValue(event.currentTarget).trim();
                            if (value) {
                              setStartTimeText(normalizeTimeInput(value));
                            }
                          }}
                          onChange={(value) => setStartTimeText(value?.format(timeFormat) ?? "")}
                          panelRender={renderPickerPanel("开始时间选择面板")}
                          placeholder="选择开始时间"
                          placement="bottomLeft"
                          value={parseTime(startTimeText)}
                        />
                      </label>
                      <label className="field">
                        <RequiredLabel>结束时间</RequiredLabel>
                        <TimePicker
                          aria-label="结束时间"
                          builtinPlacements={publishExamPickerPlacements}
                          className="exam-ant-picker"
                          format={timeFormat}
                          getPopupContainer={getDrawerPopupContainer}
                          inputReadOnly={false}
                          onBlur={(event) => {
                            const value = inputValue(event.currentTarget).trim();
                            if (value) {
                              setEndTimeText(normalizeTimeInput(value));
                            }
                          }}
                          onChange={(value) => setEndTimeText(value?.format(timeFormat) ?? "")}
                          panelRender={renderPickerPanel("结束时间选择面板")}
                          placeholder="选择结束时间"
                          placement="bottomLeft"
                          value={parseTime(endTimeText)}
                        />
                      </label>
                    </div>
                  </>
                ) : (
                  <>
                    <div aria-label="考试开放范围" className="field" role="group">
                      <RequiredLabel>考试开放范围</RequiredLabel>
                      <DatePicker.RangePicker
                        builtinPlacements={publishExamPickerPlacements}
                        className="exam-ant-picker"
                        format={dateFormat}
                        getPopupContainer={getDrawerPopupContainer}
                        inputReadOnly={false}
                        locale={datePickerZhCN}
                        onBlur={(event, info) => {
                          const inputText = inputValue(event.currentTarget).trim();
                          if (!inputText) {
                            return;
                          }
                          const value = normalizeDateInput(inputText);
                          if (info.range === "start") {
                            setWindowStartText(value);
                          } else {
                            setWindowEndText(value);
                          }
                        }}
                        onChange={(value) => {
                          setWindowStartText(value?.[0]?.format(dateFormat) ?? "");
                          setWindowEndText(value?.[1]?.format(dateFormat) ?? "");
                        }}
                        order={false}
                        panelRender={(panel) => (
                          <div aria-label="考试开放范围日期面板" role="dialog">
                            {panel}
                          </div>
                        )}
                        placeholder={["开始日期", "结束日期"]}
                        placement="bottomLeft"
                        value={[parseDate(windowStartText), parseDate(windowEndText)]}
                      />
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
                  </>
                )}
              </fieldset>
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
                    <AntSelect
                      aria-label="选择班级范围"
                      className="exam-publish-select"
                      getPopupContainer={getDrawerPopupContainer}
                      mode="multiple"
                      onChange={(values) => setSelectedSpaceTargetValues(values)}
                      optionFilterProp="label"
                      options={spaceTargetOptions.map((option) => ({ label: option.label, value: option.value }))}
                      placeholder="请选择班级，可多选"
                      showSearch
                      value={selectedSpaceTargetValues}
                      virtual={false}
                    />
                  </label>
                ) : (
                  <div className="field exam-user-picker">
                    <span className="field-label">指定同学</span>
                    <div className="exam-user-combobox">
                      <input
                        aria-controls={userTargetQuery.trim() ? "exam-user-target-listbox" : undefined}
                        aria-expanded={userTargetQuery.trim() !== ""}
                        aria-label="指定同学"
                        autoComplete="off"
                        className="exam-user-target-input"
                        onChange={(event) => changeUserTargetQuery(event.target.value)}
                        placeholder="输入姓名筛选学生"
                        role="combobox"
                        value={userTargetQuery}
                      />
                      {userTargetQuery.trim() && (
                        <div
                          aria-label="指定同学列表"
                          className="exam-user-target-listbox"
                          id="exam-user-target-listbox"
                          role="listbox"
                        >
                          {visibleUserTargetCandidates.length === 0 ? (
                            <div aria-disabled="true" className="exam-user-target-empty" role="option">
                              暂无可选学生
                            </div>
                          ) : visibleUserTargetCandidates.map((option) => (
                            <button
                              aria-selected={selectedUserTargetSet.has(option.value)}
                              className="exam-user-target-option"
                              key={option.value}
                              onClick={() => selectUserTarget(option.value)}
                              role="option"
                              type="button"
                            >
                              {renderUserTargetOption(option)}
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                    <div aria-label="已选指定同学" className="exam-user-picker__selected" role="list">
                      {selectedUserTargets.length === 0 ? (
                        <span className="exam-user-picker__empty">暂未选择同学</span>
                      ) : selectedUserTargets.map((option) => {
                        return (
                          <button
                            className="exam-user-picker__tag"
                            key={option.value}
                            onClick={() => toggleUserTarget(option.value)}
                            type="button"
                          >
                            {option.label}
                            <X aria-hidden="true" size={13} />
                          </button>
                        );
                      })}
                    </div>
                  </div>
                )}
              </fieldset>
              {targetOptionsNotice && <div className="tenant-admin-warning" role="status">{targetOptionsNotice}</div>}
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={closePublishDialog} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认创建
                </Button>
              </div>
            </form>
        </PlatformDrawer>
        <ExamStatusConfirmModal
          action={pendingStatusAction}
          onCancel={() => setPendingStatusAction(null)}
          onConfirm={confirmUpdateExamStatus}
        />
    </section>
  );
}

type ExamStatusActionsProps = {
  exam: ExamRow;
  onUpdate(exam: ExamRow, status: ExamStatusUpdate): void;
};

function ExamStatusActions({ exam, onUpdate }: ExamStatusActionsProps) {
  const actions = examStatusActions(exam.status);
  if (actions.length === 0) {
    return <span className="tenant-admin-muted">暂无操作</span>;
  }
  return (
    <div className="tenant-actions">
      {actions.map((action) => (
        <Button
          key={action.status}
          onClick={() => onUpdate(exam, action.status)}
          type="button"
          variant={action.variant}
        >
          {action.label}
        </Button>
      ))}
    </div>
  );
}

function examStatusActions(status: ExamStatus): Array<{ label: string; status: ExamStatusUpdate; variant: "actionOpen" | "actionClose" | "actionReset" }> {
  if (status === "draft") {
    return [
      { label: "发布考试", status: "published", variant: "actionOpen" },
    ];
  }
  if (status === "published") {
    return [
      { label: "提前结束考试", status: "closed", variant: "actionReset" },
      { label: "禁用考试", status: "disabled", variant: "actionClose" },
    ];
  }
  return [];
}

function examStatusLabel(status: ExamStatus) {
  switch (status) {
    case "draft":
      return "草稿";
    case "published":
      return "已发布";
    case "closed":
      return "已结束";
    case "disabled":
      return "已禁用";
  }
}

function examStatusTone(status: ExamStatus) {
  switch (status) {
    case "published":
      return "success";
    case "closed":
      return "warning";
    case "disabled":
      return "danger";
    case "draft":
      return "info";
  }
}

function statusSuccessMessage(status: ExamStatusUpdate, exam: ExamRow) {
  if (status === "published") {
    return `${exam.name} 已发布，邀请码 ${exam.inviteCode}`;
  }
  if (status === "closed") {
    return `${exam.name} 已提前结束`;
  }
  return `${exam.name} 已禁用`;
}

type ExamStatusConfirmModalProps = {
  action: PendingStatusAction | null;
  onCancel: () => void;
  onConfirm: () => void;
};

function ExamStatusConfirmModal({ action, onCancel, onConfirm }: ExamStatusConfirmModalProps) {
  if (!action) {
    return null;
  }
  const content = examStatusConfirmContent(action.status);
  return (
    <PlatformModal onClose={onCancel} open title={content.title}>
      <p className="tenant-dialog-copy">
        {action.exam.name}
      </p>
      <p className="tenant-dialog-copy">
        {content.description}
      </p>
      <div className="platform-dialog__actions">
        <Button variant="secondary" onClick={onCancel} type="button">
          取消
        </Button>
        <Button variant={content.confirmVariant} onClick={onConfirm} type="button">
          {content.confirmLabel}
        </Button>
      </div>
    </PlatformModal>
  );
}

function examStatusConfirmContent(status: ExamStatusUpdate) {
  if (status === "published") {
    return {
      title: "确认发布考试",
      description: "发布后将生成邀请码，考生可在考试时间内通过入口开始作答。",
      confirmLabel: "确认发布",
      confirmVariant: "primary" as const,
    };
  }
  if (status === "closed") {
    return {
      title: "确认提前结束考试",
      description: "提前结束后，考生将不能继续开始或提交本场考试，考试结束时间会更新为当前时间。",
      confirmLabel: "确认结束",
      confirmVariant: "primary" as const,
    };
  }
  return {
    title: "确认禁用考试",
    description: "禁用后，邀请码入口和开考入口都会被关闭，考生不能再进入本场考试。",
    confirmLabel: "确认禁用",
    confirmVariant: "primary" as const,
  };
}

function buildFixedExamTime(dateValue: string, startTimeValue: string, endTimeValue: string) {
  const startTime = combineDateAndTime(dateValue, startTimeValue);
  const endTime = combineDateAndTime(dateValue, endTimeValue);
  const durationMinutes = (endTime - startTime) / 60000;
  return {
    durationMinutes,
    endTime,
    isValid: Number.isFinite(startTime) && Number.isFinite(endTime) && Number.isFinite(durationMinutes),
    startTime,
  };
}

function buildWindowExamTime(startValue: string, endValue: string, durationValue: string) {
  const startTime = combineDateAndTime(startValue, "00:00");
  const endTime = combineDateAndTime(endValue, "23:59");
  const durationMinutes = Number(durationValue);
  return {
    durationMinutes,
    endTime,
    isValid: Number.isFinite(startTime) && Number.isFinite(endTime) && Number.isFinite(durationMinutes) && durationMinutes > 0,
    startTime,
  };
}

function combineDateAndTime(dateValue: string, timeValue: string): number {
  if (!dateValue || !timeValue) {
    return Number.NaN;
  }
  return new Date(`${dateValue}T${timeValue}`).getTime();
}

function parseDate(value: string) {
  return value ? dayjs(value, dateFormat) : null;
}

function parseTime(value: string) {
  return value ? dayjs(`2000-01-01T${value}`) : null;
}

function parseDateTime(value: string) {
  return value ? dayjs(value, dateTimeFormat) : null;
}

function normalizeDateInput(value: string) {
  return parseDate(value)?.format(dateFormat) ?? "";
}

function normalizeTimeInput(value: string) {
  return parseTime(value)?.format(timeFormat) ?? "";
}

function normalizeDateTimeInput(value: string) {
  return parseDateTime(value)?.format(dateTimeFormat) ?? "";
}

function inputValue(target: EventTarget & HTMLElement) {
  return target instanceof HTMLInputElement ? target.value : "";
}

function getDrawerPopupContainer(trigger: HTMLElement) {
  const drawer = trigger.closest(".tenant-resource-drawer");
  return drawer instanceof HTMLElement ? drawer : document.body;
}

function renderPickerPanel(label: string) {
  return (panel: ReactNode) => (
    <div aria-label={label} role="dialog">
      {panel}
    </div>
  );
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
    ...spaces
      .filter((space) => space.status !== "disabled")
      .map((space) => ({
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

async function listAllTargetSpaces(api: Pick<SpaceManagementAPI, "listSpaces">, tenantID: number) {
  const spaces: SpaceRow[] = [];
  let page = 1;

  while (true) {
    const data = await api.listSpaces({
      tenantID,
      page,
      pageSize: managementSpacePageSize,
      filters: { status: "enabled" },
    });
    spaces.push(...data.items);
    if (data.items.length === 0 || data.total === undefined || spaces.length >= data.total) {
      break;
    }
    page += 1;
  }

  return { items: spaces, total: spaces.length };
}

function buildUserTargetOptions(users: TenantUserRow[]): TargetOption[] {
  return users
    .filter((user) => user.role === "student" && user.status === "enabled")
    .map((user) => ({
      id: user.id,
      label: `${user.name}（个人）`,
      type: "user" as const,
      value: `user:${user.id}`,
    }));
}

function mergeTargetOptions(base: TargetOption[], extra: TargetOption[]): TargetOption[] {
  const seen = new Set<string>();
  const merged: TargetOption[] = [];
  for (const option of [...base, ...extra]) {
    if (seen.has(option.value)) {
      continue;
    }
    seen.add(option.value);
    merged.push(option);
  }
  return merged;
}

function renderUserTargetOption(option: TargetOption) {
  return (
    <div className="exam-user-target-option__label">
      <span>{option.label}</span>
    </div>
  );
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
