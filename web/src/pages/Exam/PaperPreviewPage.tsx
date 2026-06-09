import {
  ArrowLeft,
  Award,
  BookOpen,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Download,
  FileCheck2,
  FileText,
  ListChecks,
  NotebookText,
  Search,
  Send,
  Sigma,
  Upload,
} from "lucide-react";
import { useContext, useEffect, useMemo, useState } from "react";
import type { CSSProperties, ReactNode } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { ApiError, formatApiErrorMessage } from "../../api/client";
import type { ActorRole } from "../../api/grading";
import type {
  ExamAnswerSheetData,
  ExamCandidateListData,
  ExamCandidateRow,
  ExamCandidateStatus,
  ExamDetailAPI,
  ExamDetailExam,
  ExamDetailPermissions,
  ExamDetailStatus,
  ExamManagementDetail,
  ExamOperationLogListData,
  ExamOverviewData,
  ExamPaperPreviewData,
  ExamResultListData,
  ExamResultRow as ExamDetailResultRow,
  ExamResultStatus,
  ExamResultSummaryData,
} from "../../api/examDetail";
import { examDetailApi } from "../../api/examDetail";
import type { ManualQuestionRow, PaperAPI, PaperRow, PaperSectionRow } from "../../api/papers";
import { paperApi } from "../../api/papers";
import type { QuestionAPI, QuestionRow, QuestionType } from "../../api/questions";
import { questionApi } from "../../api/questions";
import { resultsApi } from "../../api/results";
import { useFeedback } from "../../app/feedback-context";
import type { AuthSession } from "../../auth/session-context";
import { SessionContext } from "../../auth/session-context";
import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";
import { Select } from "../../components/ui/Select";
import { StatusBadge } from "../../components/ui/StatusBadge";

type PaperPreviewPageProps = {
  examDetailApi?: ExamDetailAPI;
  examID?: number;
  paperApi?: PaperAPI;
  questionApi?: QuestionAPI;
  paperID?: number;
  tenantID?: number;
  spaceID?: number;
};

type PreviewQuestion = ManualQuestionRow & {
  section: PaperSectionRow;
  question: QuestionRow | null;
  globalIndex: number;
};

type PreviewFilter = "all" | QuestionType;
type PreviewLoadError = {
  description: string;
  title: string;
};
type OverviewDistributionRow = {
  count: number;
  label: string;
  percent: number;
  score: number;
};
type CandidateRow = {
  className: string;
  duration: string;
  name: string;
  score: string;
  resultAttemptID?: number;
  startTime: string;
  status: ExamCandidateStatus;
  studentNo: string;
  submitTime: string;
  userID?: number;
};
type ResultStatus = "published" | "pending_publish" | "pending_review";
type ResultRow = {
  attemptID?: number;
  className: string;
  name: string;
  objectiveScore: number;
  rank: number;
  status: ResultStatus;
  studentNo: string;
  subjectiveScore: number;
  totalScore: number;
};

const previewTabs = ["考试概览", "基本信息", "试卷预览", "考生管理", "考试监控", "成绩管理", "考试设置", "操作日志"] as const;
type PreviewTab = typeof previewTabs[number];

const supportedQuestionTypes: QuestionType[] = ["single", "multiple", "judge", "fill_blank", "short_text"];

// 题型筛选必须和系统真实支持的 QuestionType 保持一致，不展示未落库支持的虚拟题型。
const filterOptions: Array<{ value: PreviewFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "single", label: "单选题" },
  { value: "multiple", label: "多选题" },
  { value: "judge", label: "判断题" },
  { value: "fill_blank", label: "填空题" },
  { value: "short_text", label: "解答题" },
];

const questionTypeLabels: Record<QuestionType, string> = {
  single: "单选题",
  multiple: "多选题",
  judge: "判断题",
  fill_blank: "填空题",
  short_text: "解答题",
};

const pageSizeOptions = [
  { value: "1", label: "1" },
  { value: "5", label: "5" },
  { value: "10", label: "10" },
  { value: "20", label: "20" },
];

const examOverviewStats = {
  planned: 128,
  attended: 96,
  submitted: 84,
  ongoing: 12,
  durationMinutes: 120,
  questionDenominator: 35,
  rates: {
    completed: 65,
    notStarted: 25,
    ongoing: 10,
  },
  score: {
    pass: 72,
    total: 120,
  },
};

const examOverviewDistributionRows = [
  { count: 15, label: "单选题", score: 45 },
  { count: 5, label: "多选题", score: 15 },
  { count: 5, label: "填空题", score: 20 },
  { count: 5, label: "解答题", score: 30 },
  { count: 3, label: "判断题", score: 10 },
];

const examCandidateStats = {
  planned: 128,
  notStarted: 32,
  ongoing: 12,
  submitted: 84,
};

const candidateStatusOptions = [
  { value: "all", label: "全部状态" },
  { value: "not_started", label: "未开始" },
  { value: "in_progress", label: "进行中" },
  { value: "submitted", label: "已交卷" },
];

const examCandidateRows: CandidateRow[] = [
  { className: "高一1班", duration: "—", name: "李明", score: "—", startTime: "—", status: "not_started", studentNo: "2024100101", submitTime: "—" },
  { className: "高一1班", duration: "—", name: "王欣", score: "—", startTime: "—", status: "not_started", studentNo: "2024100102", submitTime: "—" },
  { className: "高一2班", duration: "00:28:36", name: "张宇", score: "—", startTime: "2024-09-28 09:15", status: "in_progress", studentNo: "2024100103", submitTime: "—" },
  { className: "高一2班", duration: "00:21:09", name: "陈晨", score: "—", startTime: "2024-09-28 09:22", status: "in_progress", studentNo: "2024100104", submitTime: "—" },
  { className: "高一1班", duration: "01:18:42", name: "赵雨", score: "98", startTime: "2024-09-28 09:00", status: "submitted", studentNo: "2024100105", submitTime: "2024-09-28 10:18" },
  { className: "高一2班", duration: "01:17:03", name: "刘航", score: "86", startTime: "2024-09-28 09:05", status: "submitted", studentNo: "2024100106", submitTime: "2024-09-28 10:22" },
  { className: "高一1班", duration: "01:04:27", name: "周琪", score: "110", startTime: "2024-09-28 09:02", status: "submitted", studentNo: "2024100107", submitTime: "2024-09-28 10:06" },
  { className: "高一2班", duration: "01:08:55", name: "孙浩", score: "92", startTime: "2024-09-28 09:03", status: "submitted", studentNo: "2024100108", submitTime: "2024-09-28 10:12" },
];

const examResultStats = {
  averageScore: "86.5",
  highestScore: 118,
  passRate: "82%",
  pendingReview: 16,
  submitted: 84,
};

const resultStatusOptions = [
  { value: "all", label: "全部成绩状态" },
  { value: "published", label: "已发布" },
  { value: "pending_publish", label: "待发布" },
  { value: "pending_review", label: "待阅卷" },
];

const resultScoreDistribution = [
  { count: 4, label: "0-59" },
  { count: 8, label: "60-69" },
  { count: 16, label: "70-79" },
  { count: 28, label: "80-89" },
  { count: 18, label: "90-99" },
  { count: 10, label: "100-120" },
];

const resultQuestionTypeRates = [
  { label: "单选题", rate: 92 },
  { label: "多选题", rate: 81 },
  { label: "填空题", rate: 85 },
  { label: "解答题", rate: 76 },
  { label: "判断题", rate: 83 },
];

const examResultRows: ResultRow[] = [
  { className: "高一（1）班", name: "张子涵", objectiveScore: 63, rank: 1, status: "published", studentNo: "2024010101", subjectiveScore: 48, totalScore: 111 },
  { className: "高一（1）班", name: "李昊然", objectiveScore: 62, rank: 2, status: "published", studentNo: "2024010102", subjectiveScore: 44, totalScore: 106 },
  { className: "高一（1）班", name: "王一诺", objectiveScore: 58, rank: 3, status: "published", studentNo: "2024010103", subjectiveScore: 45, totalScore: 103 },
  { className: "高一（1）班", name: "刘宇航", objectiveScore: 55, rank: 4, status: "pending_publish", studentNo: "2024010104", subjectiveScore: 40, totalScore: 95 },
  { className: "高一（1）班", name: "陈思雨", objectiveScore: 50, rank: 5, status: "pending_review", studentNo: "2024010105", subjectiveScore: 38, totalScore: 88 },
  { className: "高一（1）班", name: "赵子墨", objectiveScore: 46, rank: 6, status: "published", studentNo: "2024010106", subjectiveScore: 39, totalScore: 85 },
  { className: "高一（1）班", name: "孙雨桐", objectiveScore: 51, rank: 7, status: "pending_publish", studentNo: "2024010107", subjectiveScore: 33, totalScore: 84 },
  { className: "高一（1）班", name: "周景辰", objectiveScore: 47, rank: 8, status: "pending_review", studentNo: "2024010108", subjectiveScore: 36, totalScore: 83 },
];

export function PaperPreviewPage({
  examDetailApi: providedExamDetailApi = examDetailApi,
  examID: providedExamID,
  paperApi: providedPaperApi = paperApi,
  questionApi: providedQuestionApi = questionApi,
  paperID: providedPaperID,
  tenantID = 10,
  spaceID,
}: PaperPreviewPageProps) {
  const params = useParams();
  const location = useLocation();
  const navigate = useNavigate();
  const session = useContext(SessionContext)?.session ?? null;
  const { showError, showSuccess } = useFeedback();
  const examID = providedExamID ?? Number(params.examID);
  // 有 examID 时进入真实考试详情模式；没有 examID 时保留旧 paper 预览入口，避免影响现有试卷列表跳转。
  const isExamDetailMode = Number.isFinite(examID) && examID > 0;
  const paperID = providedPaperID ?? Number(params.paperID);
  const listSearch = scopedPaperSearch(location.search, spaceID);
  const listPath = isExamDetailMode ? `/exams${listSearch}` : `/papers${listSearch}`;
  const actorID = session?.user.userID;
  const actorRole = managementActorRole(session, tenantID, spaceID);
  const [paper, setPaper] = useState<PaperRow | null>(null);
  const [sections, setSections] = useState<PaperSectionRow[]>([]);
  const [sectionQuestions, setSectionQuestions] = useState<ManualQuestionRow[]>([]);
  const [questionPool, setQuestionPool] = useState<QuestionRow[]>([]);
  const [detailExam, setDetailExam] = useState<ExamDetailExam | null>(null);
  const [detailTargets, setDetailTargets] = useState<ExamManagementDetail["targets"]>([]);
  const [overview, setOverview] = useState<ExamOverviewData | null>(null);
  const [detailPermissions, setDetailPermissions] = useState<ExamDetailPermissions | null>(null);
  const [previewTotal, setPreviewTotal] = useState(0);
  const [activeFilter, setActiveFilter] = useState<PreviewFilter>("all");
  const [activeTab, setActiveTab] = useState<PreviewTab>("试卷预览");
  const [pageSize, setPageSize] = useState("1");
  const [candidateStatusFilter, setCandidateStatusFilter] = useState("all");
  const [candidateKeyword, setCandidateKeyword] = useState("");
  const [candidatePage, setCandidatePage] = useState(1);
  const [candidateData, setCandidateData] = useState<ExamCandidateListData | null>(null);
  const [isCandidatesLoading, setIsCandidatesLoading] = useState(false);
  const [candidateReloadKey, setCandidateReloadKey] = useState(0);
  const [candidateImportDraft, setCandidateImportDraft] = useState("");
  const [isCandidateImportOpen, setIsCandidateImportOpen] = useState(false);
  const [isCandidateActionPending, setIsCandidateActionPending] = useState(false);
  const [resultStatusFilter, setResultStatusFilter] = useState("all");
  const [resultKeyword, setResultKeyword] = useState("");
  const [resultPage, setResultPage] = useState(1);
  const [resultSummary, setResultSummary] = useState<ExamResultSummaryData | null>(null);
  const [resultData, setResultData] = useState<ExamResultListData | null>(null);
  const [isResultsLoading, setIsResultsLoading] = useState(false);
  const [isResultExporting, setIsResultExporting] = useState(false);
  const [selectedResult, setSelectedResult] = useState<ResultRow | ExamDetailResultRow | null>(null);
  const [answerSheet, setAnswerSheet] = useState<ExamAnswerSheetData | null>(null);
  const [isAnswerSheetLoading, setIsAnswerSheetLoading] = useState(false);
  const [operationLogs, setOperationLogs] = useState<ExamOperationLogListData | null>(null);
  const [isOperationLogsLoading, setIsOperationLogsLoading] = useState(false);
  const [operationLogPage, setOperationLogPage] = useState(1);
  const [isSettingsSaving, setIsSettingsSaving] = useState(false);
  const [loadError, setLoadError] = useState<PreviewLoadError | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const previewScopeKey = `${isExamDetailMode ? examID : paperID}:${activeFilter}:${pageSize}`;
  const availableTabs = useMemo(
    () => visiblePreviewTabs(isExamDetailMode, detailPermissions),
    [detailPermissions, isExamDetailMode],
  );
  const currentActiveTab = availableTabs.includes(activeTab) ? activeTab : availableTabs[0] ?? "试卷预览";
  const [previewPager, setPreviewPager] = useState({ scopeKey: "", startIndex: 0 });
  const previewStartIndexForRequest = previewPager.scopeKey === previewScopeKey ? previewPager.startIndex : 0;
  const previewPageSize = Math.max(Number(pageSize) || 1, 1);
  const previewRequestPage = Math.floor(previewStartIndexForRequest / previewPageSize) + 1;

  useEffect(() => {
    let ignore = false;

    async function loadPreview() {
      setIsLoading(true);
      setLoadError(null);
      try {
        // 考试详情 canonical 路由必须优先读取管理端考试详情接口，不再从试卷和题库接口临时拼数据。
        if (isExamDetailMode) {
          const detailData = await providedExamDetailApi.getDetail({ tenantID, examID, ...(spaceID === undefined ? {} : { spaceID }) });
          if (ignore) {
            return;
          }
          setPaper(mapExamToPaperRow(detailData.exam));
          setDetailExam(detailData.exam);
          setDetailTargets(detailData.targets);
          setDetailPermissions(detailData.permissions);
          setLoadError(null);

          if (detailData.permissions.canViewOverview) {
            try {
              const overviewData = await providedExamDetailApi.getOverview({ tenantID, examID, ...(spaceID === undefined ? {} : { spaceID }) });
              if (!ignore) {
                setOverview(overviewData);
              }
            } catch (error) {
              if (!ignore) {
                setOverview(null);
                showError(formatApiErrorMessage(error, "考试概览加载失败"));
              }
            }
          } else {
            setOverview(null);
          }

          if (detailData.permissions.canViewPaper) {
            try {
              const previewData = await providedExamDetailApi.getPaperPreview({
                tenantID,
                examID,
                ...(spaceID === undefined ? {} : { spaceID }),
                questionType: activeFilter === "all" ? undefined : activeFilter,
                page: previewRequestPage,
                pageSize: previewPageSize,
              });
              if (!ignore) {
                const previewRows = mapExamPreviewToPaperRows(previewData, tenantID);
                setSections(previewRows.sections);
                setSectionQuestions(previewRows.sectionQuestions);
                setQuestionPool(previewRows.questions);
                setPreviewTotal(previewData.total);
              }
            } catch (error) {
              if (!ignore) {
                setSections([]);
                setSectionQuestions([]);
                setQuestionPool([]);
                setPreviewTotal(0);
                showError(formatApiErrorMessage(error, "试卷预览加载失败"));
              }
            }
          } else {
            setSections([]);
            setSectionQuestions([]);
            setQuestionPool([]);
            setPreviewTotal(0);
          }
          setLoadError(null);
          return;
        }

        const [nextPaper, sectionData, sectionQuestionData, questionData] = await Promise.all([
          providedPaperApi.getPaper({ tenantID, paperID }),
          providedPaperApi.listSections({ tenantID, paperID }),
          providedPaperApi.listSectionQuestions({ tenantID, paperID }),
          loadQuestionPool(providedQuestionApi, tenantID, spaceID),
        ]);
        if (ignore) {
          return;
        }
        setPaper(nextPaper);
        setSections(sectionData.items);
        setSectionQuestions(sectionQuestionData.items);
        setQuestionPool(questionData);
        setDetailExam(null);
        setDetailTargets([]);
        setOverview(null);
        setDetailPermissions(null);
        setPreviewTotal(0);
        setResultSummary(null);
        setResultData(null);
        setLoadError(null);
      } catch (error) {
        if (!ignore) {
          const message = formatApiErrorMessage(error, "试卷预览加载失败");
          const previewError = previewLoadErrorFrom(error, message);
          showError(message);
          if (previewError) {
            // 403/404 属于确定不可展示的边界错误，必须清掉已加载数据，避免权限变化后残留旧内容。
            setPaper(null);
            setDetailExam(null);
            setDetailTargets([]);
            setOverview(null);
            setDetailPermissions(null);
            setSections([]);
            setSectionQuestions([]);
            setQuestionPool([]);
            setPreviewTotal(0);
            setResultSummary(null);
            setResultData(null);
            setLoadError(previewError);
          }
        }
      } finally {
        if (!ignore) {
          setIsLoading(false);
        }
      }
    }

    void loadPreview();

    return () => {
      ignore = true;
    };
  }, [activeFilter, examID, isExamDetailMode, paperID, previewPageSize, previewRequestPage, providedExamDetailApi, providedPaperApi, providedQuestionApi, showError, spaceID, tenantID]);

  useEffect(() => {
    if (!isExamDetailMode || currentActiveTab !== "考生管理" || loadError || !detailPermissions?.canViewCandidates) {
      return;
    }
    let ignore = false;

    async function loadCandidates() {
      setIsCandidatesLoading(true);
      setCandidateData(null);
      try {
        const data = await providedExamDetailApi.getCandidates({
          tenantID,
          examID,
          ...(spaceID === undefined ? {} : { spaceID }),
          keyword: candidateKeyword.trim() || undefined,
          status: candidateStatusFilter === "all" ? undefined : (candidateStatusFilter as ExamCandidateStatus),
          page: candidatePage,
          pageSize: 20,
        });
        if (!ignore) {
          setCandidateData(data);
        }
      } catch (error) {
        if (!ignore) {
          showError(formatApiErrorMessage(error, "考生列表加载失败"));
        }
      } finally {
        if (!ignore) {
          setIsCandidatesLoading(false);
        }
      }
    }

    void loadCandidates();

    return () => {
      ignore = true;
    };
  }, [currentActiveTab, candidateKeyword, candidatePage, candidateReloadKey, candidateStatusFilter, detailPermissions?.canViewCandidates, examID, isExamDetailMode, loadError, providedExamDetailApi, showError, spaceID, tenantID]);

  useEffect(() => {
    if (!isExamDetailMode || currentActiveTab !== "成绩管理" || loadError || !detailPermissions?.canViewResults) {
      return;
    }
    let ignore = false;

    async function loadResults() {
      setIsResultsLoading(true);
      setResultSummary(null);
      setResultData(null);
      try {
        const [summaryData, listData] = await Promise.all([
          providedExamDetailApi.getResultsSummary({
            tenantID,
            examID,
            ...(spaceID === undefined ? {} : { spaceID }),
          }),
          providedExamDetailApi.getResults({
            tenantID,
            examID,
            ...(spaceID === undefined ? {} : { spaceID }),
            keyword: resultKeyword.trim() || undefined,
            status: resultStatusFilter === "all" ? undefined : (resultStatusFilter as ExamResultStatus),
            page: resultPage,
            pageSize: 20,
          }),
        ]);
        if (!ignore) {
          setResultSummary(summaryData);
          setResultData(listData);
        }
      } catch (error) {
        if (!ignore) {
          showError(formatApiErrorMessage(error, "成绩列表加载失败"));
        }
      } finally {
        if (!ignore) {
          setIsResultsLoading(false);
        }
      }
    }

    void loadResults();

    return () => {
      ignore = true;
    };
  }, [currentActiveTab, detailPermissions?.canViewResults, examID, isExamDetailMode, loadError, providedExamDetailApi, resultKeyword, resultPage, resultStatusFilter, showError, spaceID, tenantID]);

  useEffect(() => {
    if (!isExamDetailMode || currentActiveTab !== "操作日志" || loadError || !detailPermissions?.canViewLogs) {
      return;
    }
    let ignore = false;

    async function loadOperationLogs() {
      setIsOperationLogsLoading(true);
      setOperationLogs(null);
      try {
        const data = await providedExamDetailApi.getOperationLogs({
          tenantID,
          examID,
          ...(spaceID === undefined ? {} : { spaceID }),
          page: operationLogPage,
          pageSize: 20,
        });
        if (!ignore) {
          setOperationLogs(data);
        }
      } catch (error) {
        if (!ignore) {
          setOperationLogs(null);
          showError(formatApiErrorMessage(error, "操作日志加载失败"));
        }
      } finally {
        if (!ignore) {
          setIsOperationLogsLoading(false);
        }
      }
    }

    void loadOperationLogs();

    return () => {
      ignore = true;
    };
  }, [currentActiveTab, detailPermissions?.canViewLogs, examID, isExamDetailMode, loadError, operationLogPage, providedExamDetailApi, showError, spaceID, tenantID]);

  const previewQuestions = useMemo(
    () => buildPreviewQuestions(sections, sectionQuestions, questionPool),
    [questionPool, sectionQuestions, sections],
  );
  const totalQuestionCount = questionCountForPreview(sections, Math.max(previewTotal, previewQuestions.length));
  const totalScore = scoreForPreview(paper, sections, sectionQuestions);
  const passScore = Math.round(totalScore * 0.6);
  const filteredQuestions = previewQuestions.filter((item) => {
    if (activeFilter === "all") {
      return true;
    }
    return previewQuestionType(item) === activeFilter;
  });
  const previewPagerTotal = isExamDetailMode
    ? Math.max(previewTotal, previewQuestions.length)
    : filteredQuestions.length;
  const previewDisplayTotal = isExamDetailMode ? previewPagerTotal : totalQuestionCount;
  const maxPreviewStartIndex = Math.max(previewPagerTotal - previewPageSize, 0);
  const previewStartIndex = Math.min(previewStartIndexForRequest, maxPreviewStartIndex);
  const displayQuestions = isExamDetailMode
    ? filteredQuestions.map((item, index) => ({ ...item, globalIndex: previewStartIndex + index + 1 }))
    : filteredQuestions;
  const visibleQuestions = isExamDetailMode ? displayQuestions : displayQuestions.slice(previewStartIndex, previewStartIndex + previewPageSize);
  const currentPreviewQuestionIndex = visibleQuestions[0]?.globalIndex ?? 1;
  const canManageCandidates = !isExamDetailMode || candidateData?.permissions.canManageCandidates === true;

  function updatePreviewStartIndex(nextStartIndex: number | ((current: number) => number)) {
    setPreviewPager((current) => {
      const currentStartIndex = current.scopeKey === previewScopeKey
        ? Math.min(current.startIndex, maxPreviewStartIndex)
        : 0;
      const startIndex = typeof nextStartIndex === "function"
        ? nextStartIndex(currentStartIndex)
        : nextStartIndex;
      return {
        scopeKey: previewScopeKey,
        startIndex: Math.min(Math.max(startIndex, 0), maxPreviewStartIndex),
      };
    });
  }

  async function handleImportCandidates() {
    const userIDs = parseCandidateUserIDs(candidateImportDraft);
    if (!isExamDetailMode) {
      showError("旧试卷预览入口不支持导入考生，请进入考试详情页操作。");
      return;
    }
    if (userIDs.length === 0) {
      showError("请输入要导入的考生用户 ID");
      return;
    }
    setIsCandidateActionPending(true);
    try {
      const result = await providedExamDetailApi.importCandidates({
        tenantID,
        examID,
        ...(spaceID === undefined ? {} : { spaceID }),
        userIDs,
      });
      showSuccess(`已导入 ${result.importedCount} 名考生，跳过 ${result.skippedCount} 名`);
      setCandidateImportDraft("");
      setIsCandidateImportOpen(false);
      setCandidateReloadKey((value) => value + 1);
    } catch (error) {
      showError(formatApiErrorMessage(error, "导入考生失败"));
    } finally {
      setIsCandidateActionPending(false);
    }
  }

  async function handleResendInvitations(userIDs: number[]) {
    if (!isExamDetailMode) {
      showError("旧试卷预览入口不支持发送邀请码，请进入考试详情页操作。");
      return;
    }
    if (userIDs.length === 0) {
      showError("当前页没有可发送邀请码的未开始考生");
      return;
    }
    setIsCandidateActionPending(true);
    try {
      const result = await providedExamDetailApi.resendInvitations({
        tenantID,
        examID,
        ...(spaceID === undefined ? {} : { spaceID }),
        userIDs,
      });
      const inviteCodeText = result.inviteCode ? `，邀请码 ${result.inviteCode}` : "";
      showSuccess(`已发送 ${result.sentCount} 名考生邀请码，跳过 ${result.skippedCount} 名${inviteCodeText}`);
      setCandidateReloadKey((value) => value + 1);
    } catch (error) {
      showError(formatApiErrorMessage(error, "发送邀请码失败"));
    } finally {
      setIsCandidateActionPending(false);
    }
  }

  async function handleUpdateSettings(publishMode: string) {
    if (!isExamDetailMode) {
      return;
    }
    setIsSettingsSaving(true);
    try {
      const result = await providedExamDetailApi.updateSettings({
        tenantID,
        examID,
        ...(spaceID === undefined ? {} : { spaceID }),
        publishMode,
        scorePublishTime: detailExam?.scorePublishTime ?? null,
      });
      setDetailExam(result.exam);
      setPaper(mapExamToPaperRow(result.exam));
      showSuccess("考试设置已保存");
    } catch (error) {
      showError(formatApiErrorMessage(error, "考试设置保存失败"));
    } finally {
      setIsSettingsSaving(false);
    }
  }

  async function handleExportResults() {
    if (!isExamDetailMode || !actorID) {
      showError("当前无法导出成绩，请确认登录身份和考试上下文。");
      return;
    }
    setIsResultExporting(true);
    try {
      const result = await resultsApi.exportResults({
        tenantID,
        examID,
        actorID,
        actorRole,
        ...(spaceID === undefined ? {} : { spaceID }),
      });
      triggerFileDownload(result.fileURL, result.filePath);
      showSuccess(`已导出 ${result.rowCount} 条成绩记录`);
    } catch (error) {
      showError(formatApiErrorMessage(error, "成绩导出失败"));
    } finally {
      setIsResultExporting(false);
    }
  }

  async function handleOpenAnswerSheet(attemptID?: number | null) {
    if (!isExamDetailMode || !attemptID) {
      showError("当前记录暂无可查看答卷。");
      return;
    }
    setIsAnswerSheetLoading(true);
    try {
      const data = await providedExamDetailApi.getAnswerSheet({
        tenantID,
        examID,
        attemptID,
        ...(spaceID === undefined ? {} : { spaceID }),
      });
      setAnswerSheet(data);
    } catch (error) {
      showError(formatApiErrorMessage(error, "答卷详情加载失败"));
    } finally {
      setIsAnswerSheetLoading(false);
    }
  }

  function openGradingPage(attemptID?: number) {
    if (!isExamDetailMode) {
      return;
    }
    navigate({
      pathname: "/grading",
      search: managementRouteSearch({ examID, spaceID, attemptID }),
    });
  }

  return (
    <section className="page platform-page paper-preview-page">
      <header className="paper-preview-page__hero">
        <div className="paper-preview-page__headline">
          <Link className="paper-preview-page__back" to={listPath}>
            <ArrowLeft aria-hidden="true" size={16} />
            <span>{isExamDetailMode ? "返回考试列表" : "返回试卷列表"}</span>
          </Link>
          <div className="paper-preview-page__title-row">
            <h1>{paper?.name ?? "试卷预览"}</h1>
            {paper && (
              <StatusBadge tone={paperStatusTone(paper.status)}>
                {paperStatusLabel(paper.status)}
              </StatusBadge>
            )}
          </div>
        </div>
      </header>

      <nav aria-label="考试管理菜单" className="paper-preview-tabs" role="tablist">
        {availableTabs.map((tab) => (
          <button
            aria-controls={tabPanelID(tab)}
            aria-selected={currentActiveTab === tab ? "true" : "false"}
            className={currentActiveTab === tab ? "paper-preview-tabs__item paper-preview-tabs__item--active" : "paper-preview-tabs__item"}
            id={tabID(tab)}
            key={tab}
            onClick={() => setActiveTab(tab)}
            role="tab"
            type="button"
          >
            {tab}
          </button>
        ))}
      </nav>

      {loadError ? (
        <PreviewErrorPanel error={loadError} />
      ) : currentActiveTab === "考试概览" ? (
        <ExamOverviewTabPanel
          exam={detailExam}
          isExamDetailMode={isExamDetailMode}
          overview={overview}
          paper={paper}
          passScore={passScore}
          sections={sections}
          targets={detailTargets}
          totalScore={totalScore}
        />
      ) : currentActiveTab === "基本信息" ? (
        <BasicInfoTabPanel
          exam={detailExam}
          paper={paper}
          passScore={passScore}
          targets={detailTargets}
          totalScore={totalScore}
        />
      ) : currentActiveTab === "试卷预览" ? (
        <div
          aria-labelledby={tabID("试卷预览")}
          className="paper-preview-tab-panel"
          id={tabPanelID("试卷预览")}
          role="tabpanel"
        >
          <Panel className="paper-preview-summary-card">
            <section aria-label="试卷统计" className="paper-preview-summary" role="region">
              <PreviewMetric icon={<FileText aria-hidden="true" size={22} />} label="总题数" value={String(totalQuestionCount)} unit="题" hint={`共 ${sections.length} 个大题`} tone="orange" />
              <PreviewMetric icon={<Sigma aria-hidden="true" size={22} />} label="总分值" value={String(totalScore)} unit="分" tone="blue" />
              <PreviewMetric icon={<Clock3 aria-hidden="true" size={22} />} label="考试时长" value={String(paper?.durationMinutes ?? 120)} unit="分钟" tone="green" />
              <PreviewMetric icon={<Award aria-hidden="true" size={22} />} label="合格分数" value={String(passScore)} unit="分" hint="60%" tone="violet" />
              <PreviewMetric icon={<ListChecks aria-hidden="true" size={22} />} label="试卷类型" value="正式考试" tone="gold" />
              <PreviewMetric icon={<FileCheck2 aria-hidden="true" size={22} />} label="计分方式" value="答题后公布" tone="teal" />
            </section>
          </Panel>

          <div className="paper-preview-layout">
            <Panel className="paper-preview-structure-card">
              <nav aria-label="试卷结构" className="paper-preview-structure">
                <h2>试卷结构</h2>
                {sections.length === 0 && !isLoading ? (
                  <p className="paper-preview-empty">暂无试卷结构</p>
                ) : (
                  <div className="paper-preview-structure__list">
                    {sections
                      .slice()
                      .sort((left, right) => left.sortOrder - right.sortOrder)
                      .map((section, sectionIndex) => (
                        <SectionStructure
                          isFirst={sectionIndex === 0}
                          key={section.id}
                          section={section}
                        />
                      ))}
                  </div>
                )}
              </nav>
            </Panel>

            <div className="paper-preview-content">
              <div className="paper-preview-content__toolbar">
                <div className="paper-preview-filters" aria-label="题型筛选">
                  {filterOptions.map((option) => (
                    <button
                      aria-pressed={activeFilter === option.value ? "true" : "false"}
                      className={activeFilter === option.value ? "paper-preview-filter paper-preview-filter--active" : "paper-preview-filter"}
                      key={option.value}
                      onClick={() => {
                        setActiveFilter(option.value);
                        setPreviewPager({ scopeKey: previewScopeKey, startIndex: 0 });
                      }}
                      type="button"
                    >
                      {option.label}
                    </button>
                  ))}
                </div>
                <div className="paper-preview-pager">
                  <span>每页题数：</span>
                  <div className="paper-preview-pager__select">
                    <Select
                      ariaLabel="每页题数"
                      onChange={(value) => {
                        setPageSize(value);
                        setPreviewPager({ scopeKey: previewScopeKey, startIndex: 0 });
                      }}
                      options={pageSizeOptions}
                      value={pageSize}
                    />
                  </div>
                  <div className="paper-preview-pager__controls">
                    <button
                      aria-label="上一题"
                      disabled={previewStartIndex === 0}
                      onClick={() => updatePreviewStartIndex((current) => current - previewPageSize)}
                      type="button"
                    >
                      <ChevronLeft aria-hidden="true" size={16} />
                    </button>
                    <strong>{currentPreviewQuestionIndex} / {Math.max(previewDisplayTotal, 1)} 题</strong>
                    <button
                      aria-label="下一题"
                      disabled={previewStartIndex >= maxPreviewStartIndex}
                      onClick={() => updatePreviewStartIndex((current) => current + previewPageSize)}
                      type="button"
                    >
                      <ChevronRight aria-hidden="true" size={16} />
                    </button>
                  </div>
                </div>
              </div>

              <div className="paper-preview-questions">
                {visibleQuestions.length === 0 ? (
                  <div className="paper-preview-empty">
                    {isLoading ? "正在加载试卷预览..." : "暂无可预览题目"}
                  </div>
                ) : visibleQuestions.map((item) => (
                  <QuestionCard
                    item={item}
                    key={`${item.sectionID}-${item.questionID}`}
                    totalQuestionCount={previewDisplayTotal}
                  />
                ))}
              </div>
            </div>
          </div>
        </div>
      ) : currentActiveTab === "考生管理" ? (
        <CandidateManagementTabPanel
          canManageCandidates={canManageCandidates}
          candidateData={candidateData}
          candidateImportDraft={candidateImportDraft}
          candidateKeyword={candidateKeyword}
          candidateStatusFilter={candidateStatusFilter}
          isExamDetailMode={isExamDetailMode}
          isCandidateActionPending={isCandidateActionPending}
          isCandidateImportOpen={isCandidateImportOpen}
          isLoading={isCandidatesLoading}
          onCandidateKeywordChange={(value) => {
            setCandidateKeyword(value);
            setCandidatePage(1);
          }}
          onCandidatePageChange={setCandidatePage}
          onCandidateImportDraftChange={setCandidateImportDraft}
          onCandidateStatusFilterChange={(value) => {
            setCandidateStatusFilter(value);
            setCandidatePage(1);
          }}
          onCloseCandidateImport={() => {
            setIsCandidateImportOpen(false);
            setCandidateImportDraft("");
          }}
          onImportCandidates={handleImportCandidates}
          onOpenCandidateImport={() => setIsCandidateImportOpen(true)}
          onResendCandidates={handleResendInvitations}
          onViewAnswerSheet={(attemptID) => void handleOpenAnswerSheet(attemptID)}
          overview={overview}
        />
      ) : currentActiveTab === "成绩管理" ? (
        <ResultManagementTabPanel
          isExporting={isResultExporting}
          isExamDetailMode={isExamDetailMode}
          isLoading={isResultsLoading}
          onConfigurePublish={() => setActiveTab("考试设置")}
          onExportResults={() => void handleExportResults()}
          onOpenAnswerSheet={(attemptID) => void handleOpenAnswerSheet(attemptID)}
          onOpenGrading={openGradingPage}
          onOpenScoreDetail={setSelectedResult}
          onResultKeywordChange={(value) => {
            setResultKeyword(value);
            setResultPage(1);
          }}
          onResultPageChange={setResultPage}
          onResultStatusFilterChange={(value) => {
            setResultStatusFilter(value);
            setResultPage(1);
          }}
          permissions={detailPermissions}
          resultData={resultData}
          resultKeyword={resultKeyword}
          resultStatusFilter={resultStatusFilter}
          resultSummary={resultSummary}
        />
      ) : currentActiveTab === "考试设置" ? (
        <ExamSettingsTabPanel
          canUpdateSettings={detailPermissions?.canUpdateSettings ?? false}
          exam={detailExam}
          isSaving={isSettingsSaving}
          onSave={handleUpdateSettings}
        />
      ) : currentActiveTab === "操作日志" ? (
        <OperationLogsTabPanel isLoading={isOperationLogsLoading} logs={operationLogs} onPageChange={setOperationLogPage} />
      ) : (
        <PlaceholderTabPanel tab={currentActiveTab} />
      )}
      <AnswerSheetDialog
        answerSheet={answerSheet}
        isLoading={isAnswerSheetLoading}
        onClose={() => {
          setAnswerSheet(null);
          setIsAnswerSheetLoading(false);
        }}
      />
      <ScoreDetailDialog onClose={() => setSelectedResult(null)} result={selectedResult} />
    </section>
  );
}

function BasicInfoTabPanel({
  exam,
  paper,
  passScore,
  targets,
  totalScore,
}: {
  exam: ExamDetailExam | null;
  paper: PaperRow | null;
  passScore: number;
  targets: ExamManagementDetail["targets"];
  totalScore: number;
}) {
  const durationMinutes = exam?.durationMinutes ?? paper?.durationMinutes ?? examOverviewStats.durationMinutes;
  const targetDescription = targets.length > 0 ? formatManagementTargets(targets) : "未配置";
  const totalScoreText = `${totalScore} 分`;
  const passScoreText = `${passScore} 分`;
  const resultStrategyText = exam ? resultStrategyLabel(exam.resultStrategy) : "按最新成绩";
  const publishModeText = exam ? publishModeLabel(exam.publishMode) : "手动发布成绩";
  const startTimeText = exam ? formatCandidateDateTime(exam.startTime) : "—";
  const endTimeText = exam ? formatCandidateDateTime(exam.endTime) : "—";
  const paperReference = exam ? `试卷 #${exam.paperID}` : (paper?.name ?? "未关联试卷");

  return (
    <div
      aria-labelledby={tabID("基本信息")}
      className="paper-preview-tab-panel paper-basic-info"
      id={tabPanelID("基本信息")}
      role="tabpanel"
    >
      <Panel className="paper-basic-info-card">
        <section aria-label="考试基础信息" className="paper-basic-info-section" role="region">
          <PaperInfoSectionTitle icon={<FileText aria-hidden="true" size={20} />} title="考试基础信息" />
          <div className="paper-basic-info-grid paper-basic-info-grid--base">
            <div className="paper-basic-info-column">
              <PaperInfoItem label="考试名称：" value={exam?.name ?? paper?.name ?? "未命名考试"} />
              <PaperInfoItem label="关联试卷：" value={paperReference} />
              <PaperInfoItem
                label="考试状态："
                value={<span className={`paper-basic-info-pill paper-basic-info-pill--${exam?.status === "published" ? "success" : "muted"}`}>{exam ? examStatusLabel(exam.status) : "草稿"}</span>}
              />
              <PaperInfoItem label="考试时长：" value={`${durationMinutes} 分钟`} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="最大作答次数：" value={`${exam?.maxAttempts ?? 1} 次`} />
              <PaperInfoItem label="总分：" value={totalScoreText} />
              <PaperInfoItem label="及格分：" value={passScoreText} />
              <PaperInfoItem label="成绩策略：" value={resultStrategyText} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="邀请码：" value={exam?.inviteCode ?? "—"} />
              <PaperInfoItem label="发布范围：" value={targetDescription} />
              <PaperInfoItem label="成绩公布方式：" value={publishModeText} />
            </div>
          </div>
        </section>
      </Panel>

      <Panel className="paper-basic-info-card">
        <section aria-label="时间与发布安排" className="paper-basic-info-section" role="region">
          <PaperInfoSectionTitle icon={<Clock3 aria-hidden="true" size={20} />} title="时间与发布安排" />
          <div className="paper-basic-info-grid paper-basic-info-grid--schedule">
            <div className="paper-basic-info-column">
              <PaperInfoItem label="考试开始时间：" value={startTimeText} />
              <PaperInfoItem label="考试结束时间：" value={endTimeText} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="发布范围：" value={targetDescription} />
              <PaperInfoItem label="成绩公布方式：" value={publishModeText} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="总分：" value={totalScoreText} />
              <PaperInfoItem label="及格分：" value={passScoreText} />
            </div>
          </div>
        </section>
      </Panel>
    </div>
  );
}

function ExamSettingsTabPanel({
  canUpdateSettings,
  exam,
  isSaving,
  onSave,
}: {
  canUpdateSettings: boolean;
  exam: ExamDetailExam | null;
  isSaving: boolean;
  onSave: (publishMode: string) => Promise<void>;
}) {
  const sourcePublishMode = exam?.publishMode ?? "manual_publish";
  const [publishModeDraft, setPublishModeDraft] = useState({
    source: sourcePublishMode,
    value: sourcePublishMode,
  });
  const publishMode = publishModeDraft.source === sourcePublishMode ? publishModeDraft.value : sourcePublishMode;
  const durationMinutes = exam?.durationMinutes ?? 120;
  return (
    <div
      aria-labelledby={tabID("考试设置")}
      className="paper-preview-tab-panel"
      id={tabPanelID("考试设置")}
      role="tabpanel"
    >
      <Panel className="paper-basic-info-card">
        <section aria-label="考试发布范围" className="paper-basic-info-section" role="region">
          <PaperInfoSectionTitle icon={<ListChecks aria-hidden="true" size={20} />} title="考试发布范围" />
          <div className="paper-basic-info-grid paper-basic-info-grid--schedule">
            <div className="paper-basic-info-column">
              <PaperInfoItem label="发布范围：" value="按已发布目标范围生效" />
              <PaperInfoItem label="范围修改：" value={<span className="paper-basic-info-pill paper-basic-info-pill--muted">发布后不可在此修改</span>} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="考试时长：" value={`${durationMinutes} 分钟`} />
              <PaperInfoItem label="时长修改：" value={<span className="paper-basic-info-pill paper-basic-info-pill--muted">发布后不可在此修改</span>} />
            </div>
          </div>
        </section>
      </Panel>

      <Panel className="paper-basic-info-card">
        <section aria-label="成绩发布配置" className="paper-basic-info-section paper-settings-section" role="region">
          <PaperInfoSectionTitle icon={<Award aria-hidden="true" size={20} />} title="成绩发布配置" />
          <label className="paper-settings-field">
            <span>成绩发布方式</span>
            <select
	              aria-label="成绩发布方式"
	              disabled={!canUpdateSettings || isSaving}
	              onChange={(event) => setPublishModeDraft({ source: sourcePublishMode, value: event.currentTarget.value })}
	              value={publishMode}
            >
              <option value="manual_publish">手动发布成绩</option>
              <option value="immediate_score">答题后立即公布</option>
            </select>
          </label>
          <p className="paper-settings-hint">
            当前版本只开放成绩发布方式；考试范围、考试时长和公平性配置发布后保持只读。
          </p>
          <Button
            className="paper-settings-save-button"
            disabled={!canUpdateSettings || isSaving}
            onClick={() => void onSave(publishMode)}
            variant="primary"
          >
            {isSaving ? "保存中..." : "保存设置"}
          </Button>
        </section>
      </Panel>

    </div>
  );
}

function CandidateManagementTabPanel({
  canManageCandidates,
  candidateData,
  candidateImportDraft,
  candidateKeyword,
  candidateStatusFilter,
  isExamDetailMode,
  isCandidateActionPending,
  isCandidateImportOpen,
  isLoading,
  onCandidateImportDraftChange,
  onCandidateKeywordChange,
  onCandidatePageChange,
  onCandidateStatusFilterChange,
  onCloseCandidateImport,
  onImportCandidates,
  onOpenCandidateImport,
  onResendCandidates,
  onViewAnswerSheet,
  overview,
}: {
  canManageCandidates: boolean;
  candidateData: ExamCandidateListData | null;
  candidateImportDraft: string;
  candidateKeyword: string;
  candidateStatusFilter: string;
  isExamDetailMode: boolean;
  isCandidateActionPending: boolean;
  isCandidateImportOpen: boolean;
  isLoading: boolean;
  onCandidateImportDraftChange: (value: string) => void;
  onCandidateKeywordChange: (value: string) => void;
  onCandidatePageChange: (page: number) => void;
  onCandidateStatusFilterChange: (value: string) => void;
  onCloseCandidateImport: () => void;
  onImportCandidates: () => void;
  onOpenCandidateImport: () => void;
  onResendCandidates: (userIDs: number[]) => void;
  onViewAnswerSheet: (attemptID?: number | null) => void;
  overview: ExamOverviewData | null;
}) {
  const [classFilter, setClassFilter] = useState("all");
  const [pageFilter, setPageFilter] = useState("1");
  const rows = useMemo(
    () => isExamDetailMode
      ? (candidateData?.items.map(candidateRowFromAPI) ?? [])
      : examCandidateRows,
    [candidateData?.items, isExamDetailMode],
  );
  const classOptions = useMemo(() => {
    const labels = Array.from(new Set(rows.map((candidate) => candidate.className).filter((value) => value)));
    return [
      { value: "all", label: isExamDetailMode ? "全部班级" : "高一全年级" },
      ...labels.map((label) => ({ value: label, label })),
    ];
  }, [isExamDetailMode, rows]);
  const effectiveClassFilter = classOptions.some((option) => option.value === classFilter) ? classFilter : "all";
  const visibleRows = effectiveClassFilter === "all"
    ? rows
    : rows.filter((candidate) => candidate.className === effectiveClassFilter);
  const planned = isExamDetailMode ? (overview?.candidateStats.planned ?? 0) : examCandidateStats.planned;
  const submitted = isExamDetailMode ? (overview?.candidateStats.submitted ?? 0) : examCandidateStats.submitted;
  const ongoing = isExamDetailMode ? (overview?.candidateStats.inProgress ?? 0) : examCandidateStats.ongoing;
  const notStarted = Math.max(planned - submitted - ongoing, 0);
  const total = isExamDetailMode ? candidateData?.total ?? 0 : 128;
  const currentPage = isExamDetailMode ? candidateData?.page ?? 1 : 1;
  // 旧试卷预览入口仍沿用静态 UI 图中的 4 页展示；真实考试详情才使用后端分页总数。
  const pageCount = isExamDetailMode ? Math.max(1, Math.ceil(total / 20)) : 4;
  const pageOptions = Array.from({ length: pageCount }, (_, index) => ({
    value: String(index + 1),
    label: `${index + 1} / ${pageCount} 页`,
  }));
  const pageSelectValue = isExamDetailMode ? String(currentPage) : pageFilter;
  const visibleNotStartedUserIDs = visibleRows
    .filter((candidate) => candidate.status === "not_started" && candidate.userID !== undefined)
    .map((candidate) => candidate.userID as number);

  return (
    <div
      aria-labelledby={tabID("考生管理")}
      className="paper-preview-tab-panel paper-candidates"
      id={tabPanelID("考生管理")}
      role="tabpanel"
    >
      <section aria-label="考生统计" className="paper-candidates-metrics">
        <CandidateMetricCard icon={<ListChecks aria-hidden="true" size={24} />} label="应考人数" tone="blue" value={String(planned)} />
        <CandidateMetricCard icon={<Clock3 aria-hidden="true" size={24} />} label="未开始" tone="orange" value={String(notStarted)} />
        <CandidateMetricCard icon={<FileCheck2 aria-hidden="true" size={24} />} label="进行中" tone="green" value={String(ongoing)} />
        <CandidateMetricCard icon={<Award aria-hidden="true" size={24} />} label="已交卷" tone="violet" value={String(submitted)} />
      </section>

      <Panel className="paper-candidates-card">
        <section aria-label="考生管理列表" className="paper-candidates-panel">
          <div className="paper-candidates-toolbar">
            <div className="paper-candidates-toolbar__actions">
              <Button
                className="paper-candidates-import-button"
                disabled={!canManageCandidates || isCandidateActionPending}
                onClick={onOpenCandidateImport}
                variant="primary"
              >
                <Upload aria-hidden="true" size={15} />
                <span>批量导入考生</span>
              </Button>
              <Button
                className="paper-candidates-send-button"
                disabled={!canManageCandidates || isCandidateActionPending || (isExamDetailMode && visibleNotStartedUserIDs.length === 0)}
                onClick={() => onResendCandidates(visibleNotStartedUserIDs)}
                variant="secondary"
              >
                <Send aria-hidden="true" size={15} />
                <span>发送邀请码</span>
              </Button>
            </div>
            <div className="paper-candidates-toolbar__filters">
              <label className="paper-candidates-search">
                <Search aria-hidden="true" size={15} />
                <input
                  onChange={(event) => onCandidateKeywordChange(event.target.value)}
                  placeholder="搜索姓名、学号或班级"
                  type="search"
                  value={candidateKeyword}
                />
              </label>
              <div className="paper-candidates-select">
                <Select ariaLabel="考试状态筛选" onChange={onCandidateStatusFilterChange} options={candidateStatusOptions} value={candidateStatusFilter} />
              </div>
              <div className="paper-candidates-select">
                <Select ariaLabel="班级筛选" onChange={setClassFilter} options={classOptions} value={effectiveClassFilter} />
              </div>
            </div>
          </div>
          {isCandidateImportOpen && (
            <div className="paper-candidates-import-panel">
              <label>
                <span>导入考生用户 ID</span>
                <textarea
                  aria-label="导入考生用户 ID"
                  onChange={(event) => onCandidateImportDraftChange(event.target.value)}
                  placeholder="请输入考生用户 ID，多个 ID 可用逗号、空格或换行分隔"
                  value={candidateImportDraft}
                />
              </label>
              <p>导入会追加用户直投目标，后端会校验租户、空间权限和考试状态。</p>
              <div className="paper-candidates-import-panel__actions">
                <Button disabled={isCandidateActionPending} onClick={onImportCandidates} variant="primary">
                  确认导入
                </Button>
                <Button disabled={isCandidateActionPending} onClick={onCloseCandidateImport} variant="secondary">
                  取消
                </Button>
              </div>
            </div>
          )}

          <div className="paper-candidates-table-wrap">
            <table aria-label="考生列表" className="paper-candidates-table">
              <thead>
                <tr>
                  <th scope="col">姓名</th>
                  <th scope="col">学号</th>
                  <th scope="col">班级/空间</th>
                  <th scope="col">考试状态</th>
                  <th scope="col">开始时间</th>
                  <th scope="col">提交时间</th>
                  <th scope="col">用时</th>
                  <th scope="col">成绩</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {visibleRows.map((candidate) => (
                  <CandidateTableRow
                    canManageCandidates={canManageCandidates}
                    candidate={candidate}
                    isActionPending={isCandidateActionPending}
                    key={candidate.studentNo}
                    onResendCandidate={onResendCandidates}
                    onViewAnswerSheet={onViewAnswerSheet}
                  />
                ))}
              </tbody>
            </table>
            {isLoading && <p className="paper-preview-empty">正在加载考生列表</p>}
            {!isLoading && visibleRows.length === 0 && <p className="paper-preview-empty">暂无考生数据</p>}
          </div>

          <div className="paper-candidates-pagination">
            <span>{`共 ${total} 条`}</span>
            <div className="paper-candidates-pagination__controls">
              <button
                aria-label="上一页"
                disabled={currentPage <= 1}
                onClick={() => onCandidatePageChange(Math.max(1, currentPage - 1))}
                type="button"
              >
                <ChevronLeft aria-hidden="true" size={15} />
              </button>
              {Array.from({ length: Math.min(pageCount, 4) }, (_, index) => index + 1).map((page) => (
                <button
                  aria-current={page === currentPage ? "page" : undefined}
                  aria-label={`第 ${page} 页`}
                  className={page === currentPage ? "paper-candidates-pagination__page paper-candidates-pagination__page--active" : "paper-candidates-pagination__page"}
                  key={page}
                  onClick={() => onCandidatePageChange(page)}
                  type="button"
                >
                  {page}
                </button>
              ))}
              <button
                aria-label="下一页"
                disabled={currentPage >= pageCount}
                onClick={() => onCandidatePageChange(Math.min(pageCount, currentPage + 1))}
                type="button"
              >
                <ChevronRight aria-hidden="true" size={15} />
              </button>
              <div className="paper-candidates-pagination__select">
                <Select
                  ariaLabel="考生分页"
                  onChange={(value) => {
                    if (isExamDetailMode) {
                      onCandidatePageChange(Number(value));
                      return;
                    }
                    setPageFilter(value);
                  }}
                  options={pageOptions}
                  value={pageSelectValue}
                />
              </div>
            </div>
          </div>
        </section>
      </Panel>
    </div>
  );
}

function ExamOverviewTabPanel({
  exam,
  isExamDetailMode,
  overview,
  paper,
  passScore,
  sections,
  targets,
  totalScore,
}: {
  exam: ExamDetailExam | null;
  isExamDetailMode: boolean;
  overview?: ExamOverviewData | null;
  paper: PaperRow | null;
  passScore: number;
  sections: PaperSectionRow[];
  targets: ExamManagementDetail["targets"];
  totalScore: number;
}) {
  const overviewQuestionTypes = overview?.questionTypes.length ? overview.questionTypes : null;
  const overviewQuestionTotal = overviewQuestionTypes?.reduce((sum, item) => sum + item.questionCount, 0) ?? 0;
  const distributionRows = overviewQuestionTypes
    ? overviewQuestionTypes.map((item) => ({
        count: item.questionCount,
        label: questionTypeLabel(item.questionType) ?? item.questionType,
        percent: overviewQuestionTotal > 0 ? (item.questionCount / overviewQuestionTotal) * 100 : 0,
        score: Number(item.totalScore),
      }))
    : overviewDistributionRows(sections, !isExamDetailMode);
  const distributionQuestionTotal = distributionRows.reduce((sum, item) => sum + item.count, 0);
  const distributionScoreTotal = distributionRows.reduce((sum, item) => sum + item.score, 0);
  const planned = overview?.candidateStats.planned ?? (isExamDetailMode ? 0 : examOverviewStats.planned);
  const joined = overview?.candidateStats.joined ?? (isExamDetailMode ? 0 : examOverviewStats.attended);
  const submitted = overview?.candidateStats.submitted ?? (isExamDetailMode ? 0 : examOverviewStats.submitted);
  const inProgress = overview?.candidateStats.inProgress ?? (isExamDetailMode ? 0 : examOverviewStats.ongoing);
  const notStarted = Math.max(planned - joined, 0);
  const completedRate = overview && planned > 0 ? Math.round((submitted / planned) * 100) : isExamDetailMode ? 0 : examOverviewStats.rates.completed;
  const ongoingRate = overview && planned > 0 ? Math.round((inProgress / planned) * 100) : isExamDetailMode ? 0 : examOverviewStats.rates.ongoing;
  const notStartedRate = overview && planned > 0 ? Math.max(100 - completedRate - ongoingRate, 0) : isExamDetailMode ? 0 : examOverviewStats.rates.notStarted;
  const durationMinutes = exam?.durationMinutes ?? paper?.durationMinutes ?? (isExamDetailMode ? 0 : examOverviewStats.durationMinutes);
  const overviewPassScore = passScore > 0 ? passScore : isExamDetailMode ? 0 : examOverviewStats.score.pass;
  const overviewTotalScore = totalScore > 0 ? totalScore : isExamDetailMode ? 0 : examOverviewStats.score.total;
  const scheduleTime = exam
    ? `${formatCandidateDateTime(exam.startTime)} - ${formatCandidateDateTime(exam.endTime).slice(-5)}`
    : isExamDetailMode ? "未配置" : "2024-09-28 09:00 - 11:00";
  const targetDescription = targets.length > 0 ? formatManagementTargets(targets) : isExamDetailMode ? "未配置" : "高一全年级";
  // 真实考试详情返回空数组时必须展示空态，而不是回退到旧试卷预览的示例动态。
  const recentActivities = overview?.recentActivities ?? (isExamDetailMode ? [] : null);
  const riskRows = overview ? buildOverviewRiskRows({ inProgress, notStarted, submitted }) : isExamDetailMode ? [] : null;
  const fallbackActivities = [
    {
      description: "张老师 发布了考试",
      time: "2024-09-20 14:30",
      title: "发布考试",
      tone: "orange" as const,
    },
    {
      description: "张老师 向高一全年级发送了考试邀请码",
      time: "2024-09-20 14:35",
      title: "发送邀请码",
      tone: "blue" as const,
    },
    {
      description: "已有 96 名考生进入考试并开始作答",
      time: "2024-09-28 09:01",
      title: "考生开始作答",
      tone: "green" as const,
    },
    {
      description: "系统自动保存考生答题数据",
      time: "2024-09-28 09:15",
      title: "自动保存成功",
      tone: "violet" as const,
    },
  ];

  return (
    <div
      aria-labelledby={tabID("考试概览")}
      className="paper-preview-tab-panel paper-overview"
      id={tabPanelID("考试概览")}
      role="tabpanel"
    >
      <section aria-label="考试关键指标" className="paper-overview-metrics">
        <OverviewMetricCard icon={<ListChecks aria-hidden="true" size={22} />} label="计划考生" tone="blue" unit="人" value={String(planned)} />
        <OverviewMetricCard icon={<FileCheck2 aria-hidden="true" size={22} />} label="已参加" tone="green" unit="人" value={String(joined)} />
        <OverviewMetricCard icon={<NotebookText aria-hidden="true" size={22} />} label="已交卷" tone="blue" unit="人" value={String(submitted)} />
        <OverviewMetricCard icon={<Clock3 aria-hidden="true" size={22} />} label="进行中" tone="orange" unit="人" value={String(inProgress)} />
        <OverviewMetricCard icon={<Clock3 aria-hidden="true" size={22} />} label="考试时长" tone="green" unit="分钟" value={String(durationMinutes)} />
        <OverviewMetricCard icon={<Award aria-hidden="true" size={22} />} label="满分 / 及格" tone="violet" unit="分" value={`${overviewTotalScore} / ${overviewPassScore}`} />
      </section>

      <div className="paper-overview-grid">
        <Panel className="paper-overview-card paper-overview-card--progress">
          <section aria-label="考试进度" className="paper-overview-section">
            <h2>考试进度</h2>
            <div className="paper-overview-progress">
              <div
                aria-label={`已完成率 ${completedRate}%`}
                className="paper-overview-donut"
                role="img"
                style={{
                  background: `conic-gradient(#42c779 0 ${completedRate}%, #ff6b2c ${completedRate}% ${completedRate + ongoingRate}%, #d9d9d9 ${completedRate + ongoingRate}% 100%)`,
                }}
              >
                <span>已完成率</span>
                <strong>{completedRate}</strong>
                <em>%</em>
              </div>
              <dl className="paper-overview-progress__legend">
                <OverviewProgressRow count={notStarted} label="未开始" percent={notStartedRate} tone="muted" />
                <OverviewProgressRow count={inProgress} label="进行中" percent={ongoingRate} tone="orange" />
                <OverviewProgressRow count={submitted} label="已完成" percent={completedRate} tone="green" />
              </dl>
            </div>
          </section>
        </Panel>

        <Panel className="paper-overview-card paper-overview-card--distribution">
          <section aria-label="题型分布" className="paper-overview-section">
            <h2>题型分布</h2>
            <table aria-label="题型分布" className="paper-overview-distribution">
              <thead>
                <tr>
                  <th scope="col">题型</th>
                  <th scope="col">题量（题）</th>
                  <th scope="col">占比</th>
                  <th scope="col">分值（分）</th>
                </tr>
              </thead>
              <tbody>
                {distributionRows.map((row) => (
                  <tr key={row.label}>
                    <td>{row.label}</td>
                    <td>{row.count}</td>
                    <td>
                      <span className="paper-overview-distribution__bar" style={{ "--bar-width": `${row.percent}%` } as CSSProperties} />
                      {row.percent.toFixed(1)}%
                    </td>
                    <td>{formatScore(row.score)}</td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr>
                  <th scope="row">合计</th>
                  <td>{distributionQuestionTotal}</td>
                  <td />
                  <td>{formatScore(distributionScoreTotal)}</td>
                </tr>
              </tfoot>
            </table>
          </section>
        </Panel>

        <Panel className="paper-overview-card paper-overview-card--schedule">
          <section aria-label="考试安排" className="paper-overview-section">
            <h2>考试安排</h2>
            <dl className="paper-overview-detail-list">
              <OverviewDetailItem label="考试时间" value={scheduleTime} />
              <OverviewDetailItem label="适用对象" value={targetDescription} />
              <OverviewDetailItem label="考试时长" value={`${durationMinutes} 分钟`} />
              <OverviewDetailItem label="计分方式" value={exam ? publishModeLabel(exam.publishMode) : "客观题自动判分 + 主观题人工阅卷"} />
            </dl>
          </section>
        </Panel>

        <Panel className="paper-overview-card paper-overview-card--risk">
          <section aria-label="风险提醒 / 监考提示" className="paper-overview-section">
            <h2>风险提醒 / 监考提示</h2>
            <div className="paper-overview-risk-list">
              {(riskRows ?? [
                { text: "12 名考生尚未开考", tone: "warning" as const },
                { text: "2 名考生网络波动", tone: "blue" as const },
                { text: "1 道主观题需人工复核模板", tone: "violet" as const },
                { text: "考试结束后自动进入阅卷流程", tone: "green" as const },
              ]).map((row) => (
                <OverviewRiskRow key={row.text} text={row.text} tone={row.tone} />
              ))}
            </div>
          </section>
        </Panel>
      </div>

      <Panel className="paper-overview-card paper-overview-card--activity">
        <section aria-label="近期动态" className="paper-overview-section">
          <h2>近期动态</h2>
          <div className="paper-overview-activity">
            {recentActivities === null ? (
              fallbackActivities.map((item) => (
                <OverviewActivityItem
                  description={item.description}
                  key={`${item.title}-${item.time}`}
                  time={item.time}
                  title={item.title}
                  tone={item.tone}
                />
              ))
            ) : recentActivities.length === 0 ? (
              <p className="paper-preview-empty">暂无近期动态</p>
            ) : (
              recentActivities.map((item) => (
                <OverviewActivityItem
                  description={item.operationDetail || item.operationTitle}
                  key={`${item.operationType}-${item.createdAt}`}
                  time={formatCandidateDateTime(item.createdAt)}
                  title={item.operationTitle}
                  tone={activityTone(item.operationType)}
                />
              ))
            )}
          </div>
        </section>
      </Panel>
    </div>
  );
}

type ResultManagementTabPanelProps = {
  isExporting: boolean;
  isExamDetailMode: boolean;
  isLoading: boolean;
  onConfigurePublish: () => void;
  onExportResults: () => void;
  onOpenAnswerSheet: (attemptID?: number) => void;
  onOpenGrading: (attemptID?: number) => void;
  onOpenScoreDetail: (result: ResultRow | ExamDetailResultRow) => void;
  onResultKeywordChange: (value: string) => void;
  onResultPageChange: (page: number) => void;
  onResultStatusFilterChange: (value: string) => void;
  permissions: ExamDetailPermissions | null;
  resultData: ExamResultListData | null;
  resultKeyword: string;
  resultStatusFilter: string;
  resultSummary: ExamResultSummaryData | null;
};

function ResultManagementTabPanel({
  isExporting,
  isExamDetailMode,
  isLoading,
  onConfigurePublish,
  onExportResults,
  onOpenAnswerSheet,
  onOpenGrading,
  onOpenScoreDetail,
  onResultKeywordChange,
  onResultPageChange,
  onResultStatusFilterChange,
  permissions,
  resultData,
  resultKeyword,
  resultStatusFilter,
  resultSummary,
}: ResultManagementTabPanelProps) {
  const effectivePermissions = permissions ?? resultData?.permissions ?? resultSummary?.permissions;
  const summarySubmitted = isExamDetailMode ? (resultSummary?.stats.submitted ?? 0) : examResultStats.submitted;
  const summaryAverageScore = isExamDetailMode ? (resultSummary?.stats.averageScore ?? "0") : examResultStats.averageScore;
  const summaryHighestScore = isExamDetailMode ? (resultSummary?.stats.highestScore ?? "0") : String(examResultStats.highestScore);
  const summaryPassRate = isExamDetailMode ? (resultSummary?.stats.passRate ?? "0%") : examResultStats.passRate;
  const summaryPendingSubjective = isExamDetailMode ? (resultSummary?.stats.pendingSubjective ?? 0) : examResultStats.pendingReview;
  const scoreDistribution = isExamDetailMode ? (resultSummary?.scoreDistribution ?? []) : resultScoreDistribution;
  const typeRates = isExamDetailMode && resultSummary
    ? resultSummary.questionTypeRates.map((item) => ({
        label: item.questionTypeLabel || questionTypeLabel(item.questionType) || item.questionType,
        rate: item.averageRate,
      }))
    : isExamDetailMode ? [] : resultQuestionTypeRates;
  const rows = isExamDetailMode ? (resultData?.items ?? []) : examResultRows;
  const currentPage = isExamDetailMode ? resultData?.page ?? 1 : 1;
  const pageSize = isExamDetailMode ? resultData?.pageSize ?? 20 : 20;
  const total = isExamDetailMode ? resultData?.total ?? 0 : examResultRows.length;
  const pageCount = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)));
  const maxDistributionCount = Math.max(1, ...scoreDistribution.map((item) => item.count));
  const canPublishResults = effectivePermissions?.canPublishResults ?? true;
  const canExportResults = effectivePermissions?.canExportResults ?? true;
  const pageOptions = Array.from({ length: pageCount }, (_, index) => ({
    value: String(index + 1),
    label: `${index + 1} / ${pageCount} 页`,
  }));

  return (
    <div
      aria-labelledby={tabID("成绩管理")}
      className="paper-preview-tab-panel paper-results"
      id={tabPanelID("成绩管理")}
      role="tabpanel"
    >
      <section aria-label="成绩统计" className="paper-results-metrics">
        <ResultMetricCard icon={<FileCheck2 aria-hidden="true" size={24} />} label="已交卷" tone="blue" value={String(summarySubmitted)} />
        <ResultMetricCard icon={<Sigma aria-hidden="true" size={24} />} label="平均分" tone="green" value={String(summaryAverageScore)} />
        <ResultMetricCard icon={<Award aria-hidden="true" size={24} />} label="最高分" tone="orange" value={String(summaryHighestScore)} />
        <ResultMetricCard icon={<NotebookText aria-hidden="true" size={24} />} label="及格率" tone="violet" value={String(summaryPassRate)} />
        <ResultMetricCard icon={<FileText aria-hidden="true" size={24} />} label="待阅主观题" tone="gold" unit="份" value={String(summaryPendingSubjective)} />
      </section>

      <div className="paper-results-analysis-grid">
        <Panel className="paper-results-card">
          <section aria-label="成绩分布" className="paper-results-section" role="region">
            <h2>成绩分布</h2>
            <div className="paper-results-histogram">
              <div className="paper-results-histogram__axis">
                <span>人数</span>
                {[40, 30, 20, 10, 0].map((tick) => <em key={tick}>{tick}</em>)}
              </div>
              <div className="paper-results-histogram__plot">
                <div className="paper-results-histogram__grid" />
                {scoreDistribution.map((item) => (
                  <div className="paper-results-histogram__bar-group" key={item.label}>
                    <span>{item.count}</span>
                    <div
                      className="paper-results-histogram__bar"
                      style={{ "--bar-height": `${(item.count / maxDistributionCount) * 100}%` } as CSSProperties}
                    />
                    <strong>{item.label}</strong>
                  </div>
                ))}
                <small>分数段</small>
              </div>
            </div>
          </section>
        </Panel>

        <Panel className="paper-results-card">
          <section aria-label="题型得分分析" className="paper-results-section" role="region">
            <h2>题型得分分析</h2>
            <p>平均得分率</p>
            <div className="paper-results-rate-list">
              {typeRates.map((item) => (
                <div className="paper-results-rate-row" key={item.label}>
                  <span>{item.label}</span>
                  <div className="paper-results-rate-row__track">
                    <i style={{ "--rate-width": `${item.rate}%` } as CSSProperties} />
                  </div>
                  <strong>{item.rate}%</strong>
                </div>
              ))}
            </div>
            <div className="paper-results-rate-axis" aria-hidden="true">
              <span>0%</span>
              <span>25%</span>
              <span>50%</span>
              <span>75%</span>
              <span>100%</span>
            </div>
          </section>
        </Panel>
      </div>

      <Panel className="paper-results-card">
        <section aria-label="成绩管理列表" className="paper-results-panel">
          <div className="paper-results-toolbar">
            <div className="paper-results-toolbar__actions">
              {canPublishResults ? (
                <Button className="paper-results-publish-button" onClick={onConfigurePublish} variant="primary">
                  <Send aria-hidden="true" size={15} />
                  <span>配置成绩发布</span>
                </Button>
              ) : null}
              {canExportResults ? (
                <Button className="paper-results-export-button" disabled={isExporting} onClick={onExportResults} variant="secondary">
                  <Download aria-hidden="true" size={15} />
                  <span>{isExporting ? "导出中..." : "导出成绩"}</span>
                </Button>
              ) : null}
            </div>
            <div className="paper-results-toolbar__filters">
              <label className="paper-candidates-search paper-results-search">
                <Search aria-hidden="true" size={15} />
                <input onChange={(event) => onResultKeywordChange(event.target.value)} placeholder="搜索姓名、学号或班级" type="search" value={resultKeyword} />
              </label>
              <div className="paper-results-select">
                <Select ariaLabel="成绩状态筛选" onChange={onResultStatusFilterChange} options={resultStatusOptions} value={resultStatusFilter} />
              </div>
            </div>
          </div>

          <div className="paper-results-table-wrap">
            <table aria-label="成绩列表" className="paper-results-table">
              <thead>
                <tr>
                  <th scope="col">排名</th>
                  <th scope="col">姓名</th>
                  <th scope="col">学号</th>
                  <th scope="col">班级/空间</th>
                  <th scope="col">客观题</th>
                  <th scope="col">主观题</th>
                  <th scope="col">总分</th>
                  <th scope="col">成绩状态</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((result) => (
                  <ResultTableRow
                    key={resultTableKey(result)}
                    onOpenAnswerSheet={onOpenAnswerSheet}
                    onOpenGrading={onOpenGrading}
                    onOpenScoreDetail={onOpenScoreDetail}
                    result={result}
                  />
                ))}
              </tbody>
            </table>
            {isLoading ? <p className="paper-preview-empty">正在加载成绩列表</p> : null}
            {!isLoading && rows.length === 0 ? <p className="paper-preview-empty">暂无成绩数据</p> : null}
          </div>

          <div className="paper-results-pagination">
            <button
              aria-label="上一页"
              disabled={currentPage <= 1}
              onClick={() => onResultPageChange(Math.max(1, currentPage - 1))}
              type="button"
            >
              <ChevronLeft aria-hidden="true" size={15} />
            </button>
            <div className="paper-results-pagination__select">
              <Select ariaLabel="成绩分页" onChange={(value) => onResultPageChange(Number(value))} options={pageOptions} value={String(currentPage)} />
            </div>
            <button
              aria-label="下一页"
              disabled={currentPage >= pageCount}
              onClick={() => onResultPageChange(Math.min(pageCount, currentPage + 1))}
              type="button"
            >
              <ChevronRight aria-hidden="true" size={15} />
            </button>
          </div>
        </section>
      </Panel>
    </div>
  );
}

function PaperInfoSectionTitle({ icon, title }: { icon: ReactNode; title: string }) {
  return (
    <h2 className="paper-basic-info-title">
      <span>{icon}</span>
      {title}
    </h2>
  );
}

function PaperInfoItem({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="paper-basic-info-item">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function ResultMetricCard({
  icon,
  label,
  tone,
  unit,
  value,
}: {
  icon: ReactNode;
  label: string;
  tone: "orange" | "blue" | "green" | "violet" | "gold";
  unit?: string;
  value: string;
}) {
  return (
    <Panel className="paper-results-metric-card">
      <div className="paper-results-metric">
        <span className={`paper-preview-metric__icon paper-preview-metric__icon--${tone}`}>
          {icon}
        </span>
        <div className="paper-results-metric__body">
          <span>{label}</span>
          <strong>{value}{unit ? <em>{unit}</em> : null}</strong>
        </div>
      </div>
    </Panel>
  );
}

function ResultTableRow({
  onOpenAnswerSheet,
  onOpenGrading,
  onOpenScoreDetail,
  result,
}: {
  onOpenAnswerSheet: (attemptID?: number) => void;
  onOpenGrading: (attemptID?: number) => void;
  onOpenScoreDetail: (result: ResultRow | ExamDetailResultRow) => void;
  result: ResultRow | ExamDetailResultRow;
}) {
  const display = resultTableDisplay(result);
  const isPendingReview = display.status === "pending_review";
  const attemptID = "attemptID" in result ? result.attemptID : undefined;

  return (
    <tr>
      <td>{display.rank}</td>
      <td>{display.name}</td>
      <td>{display.studentNo}</td>
      <td>{display.className}</td>
      <td>{display.objectiveScore}</td>
      <td>{display.subjectiveScore}</td>
      <td className="paper-results-table__total">{display.totalScore}</td>
      <td>
        <span className={`paper-results-status paper-results-status--${display.status}`}>
          {resultStatusLabel(display.status)}
        </span>
      </td>
      <td>
        <div className="paper-results-actions">
          <button disabled={isPendingReview} onClick={() => onOpenScoreDetail(result)} type="button">查看成绩</button>
          <button disabled={!attemptID} onClick={() => onOpenAnswerSheet(attemptID)} type="button">查看答卷</button>
          <button
            className={isPendingReview ? "paper-results-actions__review" : undefined}
            disabled={!isPendingReview}
            onClick={() => onOpenGrading(attemptID)}
            type="button"
          >
            去阅卷
          </button>
        </div>
      </td>
    </tr>
  );
}

function CandidateMetricCard({
  icon,
  label,
  tone,
  value,
}: {
  icon: ReactNode;
  label: string;
  tone: "orange" | "blue" | "green" | "violet";
  value: string;
}) {
  return (
    <Panel className="paper-candidates-metric-card">
      <div className="paper-candidates-metric">
        <span className={`paper-preview-metric__icon paper-preview-metric__icon--${tone}`}>
          {icon}
        </span>
        <div className="paper-candidates-metric__body">
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      </div>
    </Panel>
  );
}

function CandidateTableRow({
  canManageCandidates,
  candidate,
  isActionPending,
  onResendCandidate,
  onViewAnswerSheet,
}: {
  canManageCandidates: boolean;
  candidate: CandidateRow;
  isActionPending: boolean;
  onResendCandidate: (userIDs: number[]) => void;
  onViewAnswerSheet: (attemptID?: number | null) => void;
}) {
  return (
    <tr>
      <td>{candidate.name}</td>
      <td>{candidate.studentNo}</td>
      <td>{candidate.className}</td>
      <td>
        <span className={`paper-candidates-status paper-candidates-status--${candidate.status}`}>
          {candidateStatusLabel(candidate.status)}
        </span>
      </td>
      <td>{candidate.startTime}</td>
      <td>{candidate.submitTime}</td>
      <td>{candidate.duration}</td>
      <td>{candidate.score}</td>
      <td>
        <div className="paper-candidates-actions">
          {candidate.status === "submitted" ? (
            <button
              disabled={!candidate.resultAttemptID}
              onClick={() => onViewAnswerSheet(candidate.resultAttemptID)}
              type="button"
            >
              查看答卷
            </button>
          ) : null}
          {candidate.status === "not_started" ? (
            <button
              disabled={!canManageCandidates || isActionPending || candidate.userID === undefined}
              onClick={() => {
                if (candidate.userID !== undefined) {
                  onResendCandidate([candidate.userID]);
                }
              }}
              type="button"
            >
              重发邀请码
            </button>
          ) : null}
        </div>
      </td>
    </tr>
  );
}

function OperationLogsTabPanel({
  isLoading,
  logs,
  onPageChange,
}: {
  isLoading: boolean;
  logs: ExamOperationLogListData | null;
  onPageChange: (page: number) => void;
}) {
  const items = logs?.items ?? [];
  const currentPage = logs?.page ?? 1;
  const pageSize = logs?.pageSize ?? 20;
  const total = logs?.total ?? 0;
  const pageCount = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)));
  const pageOptions = Array.from({ length: pageCount }, (_, index) => ({
    value: String(index + 1),
    label: `${index + 1} / ${pageCount} 页`,
  }));
  return (
    <Panel className="paper-results-card">
      <section
        aria-labelledby={tabID("操作日志")}
        className="paper-results-section"
        id={tabPanelID("操作日志")}
        role="tabpanel"
      >
        <div className="paper-results-toolbar">
          <div className="paper-results-toolbar__actions">
            <h2>操作日志</h2>
            <span>共 {logs?.total ?? 0} 条</span>
          </div>
        </div>
        {isLoading ? (
          <p className="paper-preview-empty">操作日志加载中...</p>
        ) : items.length === 0 ? (
          <p className="paper-preview-empty">暂无操作日志</p>
        ) : (
          <div className="paper-results-table-wrap">
            <table aria-label="操作日志列表" className="paper-results-table">
              <thead>
                <tr>
                  <th>操作时间</th>
                  <th>操作类型</th>
                  <th>操作内容</th>
                  <th>操作人</th>
                  <th>关联空间</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id}>
                    <td>{formatCandidateDateTime(item.createdAt)}</td>
                    <td>{item.operationTitle}</td>
                    <td>{item.operationDetail || "—"}</td>
                    <td>{actorRoleLabel(item.actorRole)}</td>
                    <td>{item.spaceID ?? "全局"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="paper-results-pagination">
          <button
            aria-label="上一页"
            disabled={currentPage <= 1}
            onClick={() => onPageChange(Math.max(1, currentPage - 1))}
            type="button"
          >
            <ChevronLeft aria-hidden="true" size={15} />
          </button>
          <div className="paper-results-pagination__select">
            <Select ariaLabel="操作日志分页" onChange={(value) => onPageChange(Number(value))} options={pageOptions} value={String(currentPage)} />
          </div>
          <button
            aria-label="下一页"
            disabled={currentPage >= pageCount}
            onClick={() => onPageChange(Math.min(pageCount, currentPage + 1))}
            type="button"
          >
            <ChevronRight aria-hidden="true" size={15} />
          </button>
        </div>
      </section>
    </Panel>
  );
}

function PlaceholderTabPanel({ tab }: { tab: PreviewTab }) {
  return (
    <Panel className="paper-preview-placeholder-card">
      <section
        aria-labelledby={tabID(tab)}
        className="paper-preview-placeholder"
        id={tabPanelID(tab)}
        role="tabpanel"
      >
        <span>占位模块</span>
        <h2>{tab}</h2>
        <p>该模块内容正在接入，当前先保留入口位置。</p>
      </section>
    </Panel>
  );
}

// PreviewErrorPanel 只承载确定不可继续展示的错误，避免 403/404 后继续渲染空统计或残留数据。
function PreviewErrorPanel({ error }: { error: PreviewLoadError }) {
  return (
    <Panel className="paper-preview-error-card">
      <section className="paper-preview-error" role="alert">
        <span>访问受限</span>
        <h2>{error.title}</h2>
        <p>{error.description}</p>
      </section>
    </Panel>
  );
}

function candidateStatusLabel(status: ExamCandidateStatus) {
  switch (status) {
    case "not_started":
      return "未开始";
    case "in_progress":
      return "进行中";
    case "submitted":
      return "已交卷";
  }
}

// parseCandidateUserIDs 将输入框里的用户 ID 列表转换为后端需要的正整数数组，并在前端先去重。
function parseCandidateUserIDs(value: string) {
  const ids = value
    .split(/[\s,，;；]+/)
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isInteger(item) && item > 0);
  return Array.from(new Set(ids));
}

// candidateRowFromAPI 将后端考生分页行转换为当前表格展示结构。
// 后端只返回时间戳和空间信息，页面层负责做轻量格式化，避免把 UI 展示文案扩散到接口层。
function candidateRowFromAPI(candidate: ExamCandidateRow): CandidateRow {
  return {
    className: candidate.spaceName || "未分配空间",
    duration: formatCandidateDuration(candidate.startedAt, candidate.submittedAt),
    name: candidate.realName || candidate.username,
    resultAttemptID: candidate.resultAttemptID ?? undefined,
    score: candidate.totalScore || "—",
    startTime: formatCandidateDateTime(candidate.startedAt),
    status: candidate.status,
    studentNo: candidate.username || String(candidate.userID),
    submitTime: formatCandidateDateTime(candidate.submittedAt),
    userID: candidate.userID,
  };
}

// formatCandidateDateTime 只处理接口返回的毫秒时间戳；空值展示为破折号，和现有表格风格一致。
function formatCandidateDateTime(value: number | null | undefined) {
  if (!value) {
    return "—";
  }
  const date = new Date(value);
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

// formatCandidateDuration 用开始和提交时间计算作答时长；进行中或未开始时继续展示破折号。
function formatCandidateDuration(startedAt: number | null | undefined, submittedAt: number | null | undefined) {
  if (!startedAt || !submittedAt || submittedAt < startedAt) {
    return "—";
  }
  const totalSeconds = Math.floor((submittedAt - startedAt) / 1000);
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
}

// mapExamToPaperRow 将考试详情接口的标题区数据映射到当前页面已有的 PaperRow 结构。
// 这样可以复用现有标题、状态 badge、统计和基础信息组件，避免阶段 5 引入大规模 UI 重写。
function mapExamToPaperRow(exam: ExamOverviewData["exam"]): PaperRow {
  return {
    id: exam.paperID,
    tenantID: exam.tenantID,
    name: exam.name,
    durationMinutes: exam.durationMinutes,
    buildMode: "manual",
    status: exam.status === "published" ? "enabled" : "draft",
    totalScore: "0",
    createdAt: exam.startTime,
    creatorName: "",
  };
}

// mapExamPreviewToPaperRows 把考试试卷预览响应转换成当前预览组件所需的三段数据。
// 后端普通管理预览不会返回正确答案，这里也只保留题干和选项，避免在页面层扩散答案信息。
function mapExamPreviewToPaperRows(preview: ExamPaperPreviewData, tenantID: number) {
  const sections: PaperSectionRow[] = preview.sections.map((section, index) => ({
    id: section.sectionID,
    tenantID,
    paperID: preview.exam.paperID,
    sortOrder: index + 1,
    name: section.sectionName,
    questionType: section.questionType,
    instructions: "",
    totalScore: section.totalScore,
    questionCount: section.questionCount,
  }));
  const sectionQuestions: ManualQuestionRow[] = preview.items.map((item) => ({
    tenantID,
    paperID: preview.exam.paperID,
    sectionID: item.sectionID,
    questionID: item.questionID,
    sortOrder: item.sortOrder,
    score: item.score,
  }));
  const questions: QuestionRow[] = preview.items.map((item) => {
    const questionType = isSupportedQuestionType(item.questionType) ? item.questionType : "short_text";
    return {
      id: item.questionID,
      tenantID,
      type: questionType,
      title: item.title,
      stem: item.title,
      options: item.options.map((option) => option.content),
      analysis: "",
      difficulty: "medium",
      tag: "",
      tags: [],
      scoreDefault: item.score,
      blankCount: item.blankCount,
      status: "ready",
    } as QuestionRow;
  });

  return { sections, sectionQuestions, questions };
}

function actorRoleLabel(role: string) {
  switch (role) {
    case "tenant_admin":
      return "租户管理员";
    case "space_admin":
      return "空间管理员";
    case "teacher":
      return "教师";
    default:
      return role || "—";
  }
}

function resultStatusLabel(status: ResultStatus) {
  switch (status) {
    case "published":
      return "已发布";
    case "pending_publish":
      return "待发布";
    case "pending_review":
      return "待阅卷";
  }
}

function resultTableDisplay(result: ResultRow | ExamDetailResultRow) {
  if ("name" in result) {
    return result;
  }
  return {
    className: result.spaceName || "未分配空间",
    name: result.realName || result.username,
    objectiveScore: result.objectiveScore,
    rank: result.rank,
    status: result.status,
    studentNo: result.username || String(result.userID),
    subjectiveScore: result.subjectiveScore,
    totalScore: result.totalScore,
  };
}

function resultTableKey(result: ResultRow | ExamDetailResultRow) {
  if ("studentNo" in result) {
    return result.studentNo;
  }
  return `${result.attemptID}-${result.userID}`;
}

function ScoreDetailDialog({
  onClose,
  result,
}: {
  onClose: () => void;
  result: ResultRow | ExamDetailResultRow | null;
}) {
  if (!result) {
    return null;
  }
  const display = resultTableDisplay(result);
  return (
    <div aria-label="成绩详情" className="paper-answer-sheet-dialog" role="dialog">
      <Panel className="paper-basic-info-card">
        <div className="paper-results-toolbar">
          <div>
            <h2>成绩详情</h2>
            <p>{display.name} · {display.className}</p>
          </div>
          <Button onClick={onClose} variant="secondary">关闭成绩详情</Button>
        </div>
        <section aria-label="成绩详情摘要" className="paper-basic-info-section" role="region">
          <div className="paper-basic-info-grid paper-basic-info-grid--schedule">
            <div className="paper-basic-info-column">
              <PaperInfoItem label="姓名：" value={display.name} />
              <PaperInfoItem label="学号：" value={display.studentNo} />
              <PaperInfoItem label="班级/空间：" value={display.className} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="客观题：" value={String(display.objectiveScore)} />
              <PaperInfoItem label="主观题：" value={String(display.subjectiveScore)} />
              <PaperInfoItem label="总分：" value={String(display.totalScore)} />
            </div>
            <div className="paper-basic-info-column">
              <PaperInfoItem label="成绩状态：" value={resultStatusLabel(display.status)} />
            </div>
          </div>
        </section>
      </Panel>
    </div>
  );
}

function AnswerSheetDialog({
  answerSheet,
  isLoading,
  onClose,
}: {
  answerSheet: ExamAnswerSheetData | null;
  isLoading: boolean;
  onClose: () => void;
}) {
  if (!answerSheet && !isLoading) {
    return null;
  }

  return (
    <div aria-label="答卷详情" className="paper-answer-sheet-dialog" role="dialog">
      <Panel className="paper-basic-info-card">
        <div className="paper-results-toolbar">
          <div>
            <h2>答卷详情</h2>
            {answerSheet ? (
              <p>
                {answerSheet.attempt.realName || answerSheet.attempt.username}
                {" · "}
                总分 {answerSheet.attempt.totalScore}
              </p>
            ) : (
              <p>正在加载答卷详情</p>
            )}
          </div>
          <Button onClick={onClose} variant="secondary">关闭答卷详情</Button>
        </div>
        {isLoading && !answerSheet ? <p className="paper-preview-empty">正在加载答卷详情</p> : null}
        {answerSheet ? (
          <div className="paper-preview-content">
            {answerSheet.items.map((item) => (
              <Panel className="paper-basic-info-card" key={item.attemptQuestionID}>
                <section aria-label={`答卷题目 ${item.sortOrder}`} className="paper-basic-info-section" role="region">
                  <PaperInfoSectionTitle icon={<BookOpen aria-hidden="true" size={20} />} title={`第 ${item.sortOrder} 题`} />
                  <div className="paper-basic-info-grid paper-basic-info-grid--schedule">
                    <div className="paper-basic-info-column">
                      <PaperInfoItem label="题目：" value={item.questionTitle} />
                      <PaperInfoItem label="考生答案：" value={item.answerContent || "—"} />
                      <PaperInfoItem label="标准答案：" value={item.correctAnswerSnapshot || "—"} />
                    </div>
                    <div className="paper-basic-info-column">
                      <PaperInfoItem label="题目分值：" value={item.score} />
                      <PaperInfoItem label="得分：" value={item.answerScore || "—"} />
                      <PaperInfoItem label="判分状态：" value={item.gradingStatus || "—"} />
                    </div>
                    <div className="paper-basic-info-column">
                      <PaperInfoItem label="评语：" value={item.graderComment || "—"} />
                    </div>
                  </div>
                </section>
              </Panel>
            ))}
          </div>
        ) : null}
      </Panel>
    </div>
  );
}

function managementActorRole(session: AuthSession | null, tenantID?: number, spaceID?: number): ActorRole {
  const scopedRole = session?.profileSpaces?.find((space) =>
    space.status === "enabled" &&
    (tenantID === undefined || space.tenantID === tenantID) &&
    (spaceID === undefined || space.spaceID === spaceID)
  )?.role;
  switch (scopedRole ?? session?.user.role) {
    case "space_admin":
    case "teacher":
    case "student":
      return (scopedRole ?? session?.user.role) as ActorRole;
    default:
      return "tenant_admin";
  }
}

function managementRouteSearch({
  attemptID,
  examID,
  spaceID,
}: {
  attemptID?: number;
  examID: number;
  spaceID?: number;
}) {
  const params = new URLSearchParams({ exam_id: String(examID) });
  if (spaceID !== undefined) {
    params.set("space_id", String(spaceID));
  }
  if (attemptID) {
    params.set("attempt_id", String(attemptID));
  }
  return `?${params.toString()}`;
}

function triggerFileDownload(fileURL: string, filePath: string) {
  const link = document.createElement("a");
  link.href = fileURL;
  link.download = filePath.split("/").pop() || "papermind-results.csv";
  link.rel = "noopener";
  link.style.display = "none";
  document.body.append(link);
  link.click();
  link.remove();
}

function OverviewMetricCard({
  icon,
  label,
  tone,
  unit,
  value,
}: {
  icon: ReactNode;
  label: string;
  tone: "orange" | "blue" | "green" | "violet";
  unit: string;
  value: string;
}) {
  return (
    <Panel className="paper-overview-metric-card">
      <div className="paper-overview-metric">
        <span className={`paper-preview-metric__icon paper-preview-metric__icon--${tone}`}>
          {icon}
        </span>
        <div className="paper-overview-metric__body">
          <span>{label}</span>
          <strong>{value}<em>{unit}</em></strong>
        </div>
      </div>
    </Panel>
  );
}

function OverviewProgressRow({
  count,
  label,
  percent,
  tone,
}: {
  count: number;
  label: string;
  percent: number;
  tone: "muted" | "orange" | "green";
}) {
  return (
    <div className="paper-overview-progress__row">
      <dt>
        <span className={`paper-overview-dot paper-overview-dot--${tone}`} />
        {label}
      </dt>
      <dd><strong>{count} 人</strong><span>{percent}%</span></dd>
    </div>
  );
}

function OverviewDetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="paper-overview-detail-list__item">
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

function OverviewRiskRow({ text, tone }: { text: string; tone: "warning" | "blue" | "violet" | "green" }) {
  return (
    <div className={`paper-overview-risk paper-overview-risk--${tone}`}>
      <span className="paper-overview-risk__icon" />
      <strong>{text}</strong>
    </div>
  );
}

function OverviewActivityItem({
  description,
  time,
  title,
  tone,
}: {
  description: string;
  time: string;
  title: string;
  tone: "orange" | "blue" | "green" | "violet";
}) {
  return (
    <article className="paper-overview-activity__item">
      <span className={`paper-overview-activity__icon paper-overview-activity__icon--${tone}`} />
      <div>
        <h3>{title}</h3>
        <p>{description}</p>
      </div>
      <time>{time}</time>
    </article>
  );
}

function PreviewMetric({
  hint,
  icon,
  label,
  tone,
  unit,
  value,
}: {
  hint?: string;
  icon: ReactNode;
  label: string;
  tone: "orange" | "blue" | "green" | "violet" | "gold" | "teal";
  unit?: string;
  value: string;
}) {
  return (
    <div className="paper-preview-metric">
      <span className={`paper-preview-metric__icon paper-preview-metric__icon--${tone}`}>
        {icon}
      </span>
      <div className="paper-preview-metric__body">
        <span>{label}</span>
        <strong>{value}{unit ? <em>{unit}</em> : null}</strong>
        {hint ? <small>{hint}</small> : null}
      </div>
    </div>
  );
}

function SectionStructure({ isFirst, section }: { isFirst: boolean; section: PaperSectionRow }) {
  const children = sectionStructureChildren(section);

  return (
    <section className="paper-preview-structure__section">
      <div className="paper-preview-structure__section-title">
        <span>{sectionStructureLabel(section)}</span>
      </div>
      {children.length > 0 && (
        <div className="paper-preview-structure__children">
          {children.map((child, index) => (
            <button
              aria-current={isFirst && index === 0 ? "true" : undefined}
              className={isFirst && index === 0 ? "paper-preview-structure__child paper-preview-structure__child--active" : "paper-preview-structure__child"}
              key={child}
              type="button"
            >
              {child}
            </button>
          ))}
        </div>
      )}
    </section>
  );
}

function QuestionCard({ item, totalQuestionCount }: { item: PreviewQuestion; totalQuestionCount: number }) {
  const question = item.question;
  const type = previewQuestionType(item);
  const title = question?.stem || question?.title || `题目 ${item.questionID}`;

  return (
    <article aria-label={`第 ${item.globalIndex} 题`} className="paper-preview-question-card">
      <div className="paper-preview-question-card__meta">
        <span className="paper-preview-question-card__type">{questionTypeLabel(type) ?? questionTypeLabel(item.section.questionType) ?? "题目"}</span>
        <strong>{item.globalIndex} / {Math.max(totalQuestionCount, item.globalIndex)}</strong>
        <span>[分值 {formatScore(item.score)}分]</span>
      </div>
      <p className="paper-preview-question-card__stem">{title}</p>
      {question?.options && question.options.length > 0 ? (
        <ol className="paper-preview-question-options">
          {question.options.map((option, index) => (
            <li key={`${item.questionID}-${index}`}>
              <span>{String.fromCharCode(65 + index)}</span>
              <p className="paper-preview-question-card__option-text">{option}</p>
            </li>
          ))}
        </ol>
      ) : (
        <div className="paper-preview-answer-box">
          <NotebookText aria-hidden="true" size={16} />
          <span>{question?.referenceAnswer || question?.standardAnswer || "主观题作答区"}</span>
        </div>
      )}
    </article>
  );
}

async function loadQuestionPool(api: QuestionAPI, tenantID: number, spaceID: number | undefined) {
  const pageSize = 100;
  const allItems: QuestionRow[] = [];
  let page = 1;
  let loadedCount = 0;
  let total = 0;

  while (page === 1 || loadedCount < total) {
    const data = await api.listQuestions({
      tenantID,
      ...(spaceID === undefined ? {} : { spaceID }),
      page,
      pageSize,
    });
    total = data.total;
    loadedCount += data.items.length;
    allItems.push(...data.items);
    if (data.items.length === 0) {
      break;
    }
    page += 1;
  }

  return allItems;
}

function buildPreviewQuestions(
  sections: PaperSectionRow[],
  sectionQuestions: ManualQuestionRow[],
  questionPool: QuestionRow[],
): PreviewQuestion[] {
  const questionMap = new Map(questionPool.map((item) => [item.id, item]));
  let globalIndex = 0;

  return sections
    .slice()
    .sort((left, right) => left.sortOrder - right.sortOrder)
    .flatMap((section) =>
      sectionQuestions
        .filter((item) => item.sectionID === section.id)
        .sort((left, right) => left.sortOrder - right.sortOrder)
        .map((item) => {
          globalIndex += 1;
          return {
            ...item,
            globalIndex,
            question: questionMap.get(item.questionID) ?? null,
            section,
          };
        }),
    );
}

function previewQuestionType(item: PreviewQuestion): PreviewFilter {
  const questionType = item.question?.type;
  if (isSupportedQuestionType(questionType)) {
    return questionType;
  }
  return isSupportedQuestionType(item.section.questionType) ? item.section.questionType : "short_text";
}

// isSupportedQuestionType 统一判断系统当前真实支持的题型，历史未知题型不参与筛选项展示。
function isSupportedQuestionType(value: string | undefined): value is QuestionType {
  return supportedQuestionTypes.includes(value as QuestionType);
}

function questionTypeLabel(value: string | undefined) {
  return isSupportedQuestionType(value) ? questionTypeLabels[value] : undefined;
}

function questionCountForPreview(sections: PaperSectionRow[], selectedQuestionCount: number) {
  const sectionCount = sections.reduce((sum, section) => sum + Number(section.questionCount || 0), 0);
  return sectionCount > 0 ? sectionCount : selectedQuestionCount;
}

function scoreForPreview(paper: PaperRow | null, sections: PaperSectionRow[], sectionQuestions: ManualQuestionRow[]) {
  const paperScore = Number(paper?.totalScore ?? 0);
  if (paperScore > 0) {
    return paperScore;
  }
  const sectionScore = sections.reduce((sum, section) => sum + Number(section.totalScore || 0), 0);
  if (sectionScore > 0) {
    return sectionScore;
  }
  return sectionQuestions.reduce((sum, item) => sum + Number(item.score || 0), 0);
}

function sectionStructureLabel(section: PaperSectionRow) {
  return `${section.name}（共${section.questionCount}题，${formatScore(section.totalScore)}分）`;
}

function sectionStructureChildren(section: PaperSectionRow) {
  return section.instructions
    ?.split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item) => item.includes("共") && item.includes("分")) ?? [];
}

function overviewDistributionRows(
  sections: PaperSectionRow[],
  allowDemoFallback = true,
): OverviewDistributionRow[] {
  const rawRows = sections
    .slice()
    .sort((left, right) => left.sortOrder - right.sortOrder)
    .flatMap((section) => {
      const childRows = parseSectionInstructionDistribution(section.instructions);
      if (childRows.length > 0) {
        return childRows;
      }
      return [{
        count: Number(section.questionCount || 0),
        label: questionTypeLabel(section.questionType) ?? sectionNameWithoutOrder(section.name),
        score: Number(section.totalScore || 0),
      }];
    });
  const rawQuestionTotal = rawRows.reduce((sum, row) => sum + row.count, 0);
  if (rawQuestionTotal === 0) {
    return allowDemoFallback ? overviewDistributionRowsWithPercent(examOverviewDistributionRows) : [];
  }
  if (allowDemoFallback && rawQuestionTotal < examOverviewStats.questionDenominator) {
    return overviewDistributionRowsWithPercent(examOverviewDistributionRows);
  }

  return overviewDistributionRowsWithPercent(rawRows, allowDemoFallback ? undefined : rawQuestionTotal);
}

function overviewDistributionRowsWithPercent(
  rawRows: Array<{ count: number; label: string; score: number }>,
  denominator?: number,
): OverviewDistributionRow[] {
  const totalCount = denominator ?? Math.max(
    rawRows.reduce((sum, row) => sum + row.count, 0),
    examOverviewStats.questionDenominator,
  );

  return rawRows.map((row) => ({
    ...row,
    percent: totalCount > 0 ? (row.count / totalCount) * 100 : 0,
  }));
}

function parseSectionInstructionDistribution(instructions: string | undefined) {
  return instructions
    ?.split(/\r?\n/)
    .map((line) => line.trim())
    .map((line) => {
      const match = /^(.+?)（共(\d+)题，(\d+(?:\.\d+)?)分）$/.exec(line);
      if (!match) {
        return null;
      }
      return {
        count: Number(match[2]),
        label: match[1],
        score: Number(match[3]),
      };
    })
    .filter((item): item is { count: number; label: string; score: number } => item !== null) ?? [];
}

function sectionNameWithoutOrder(name: string) {
  return name.replace(/^[一二三四五六七八九十]+、\s*/, "");
}

function visiblePreviewTabs(isExamDetailMode: boolean, permissions: ExamDetailPermissions | null): PreviewTab[] {
  if (!isExamDetailMode) {
    return [...previewTabs];
  }
  if (!permissions) {
    return ["试卷预览"];
  }
  const tabs: PreviewTab[] = [];
  if (permissions.canViewOverview) {
    tabs.push("考试概览");
  }
  if (permissions.canViewDetail) {
    tabs.push("基本信息");
  }
  if (permissions.canViewPaper) {
    tabs.push("试卷预览");
  }
  if (permissions.canViewCandidates) {
    tabs.push("考生管理");
  }
  if (permissions.canViewResults) {
    tabs.push("成绩管理");
  }
  if (permissions.canUpdateSettings) {
    tabs.push("考试设置");
  }
  if (permissions.canViewLogs) {
    tabs.push("操作日志");
  }
  return tabs.length > 0 ? tabs : ["试卷预览"];
}

function formatManagementTargets(targets: ExamManagementDetail["targets"]) {
  return targets
    .map((target) => target.targetType === "space" ? `空间 ${target.targetID}` : `用户 ${target.targetID}`)
    .join("、");
}

function resultStrategyLabel(strategy: string) {
  switch (strategy) {
    case "latest":
      return "按最新成绩";
    case "highest":
      return "按最高成绩";
    default:
      return strategy || "—";
  }
}

function publishModeLabel(publishMode: string) {
  switch (publishMode) {
    case "manual_publish":
      return "手动发布成绩";
    case "immediate_score":
      return "答题后立即公布";
    default:
      return publishMode || "—";
  }
}

function examStatusLabel(status: ExamDetailStatus) {
  switch (status) {
    case "published":
      return "已发布";
    case "draft":
      return "草稿";
  }
}

function buildOverviewRiskRows({
  inProgress,
  notStarted,
  submitted,
}: {
  inProgress: number;
  notStarted: number;
  submitted: number;
}) {
  const rows: Array<{ text: string; tone: "warning" | "blue" | "violet" | "green" }> = [];
  if (notStarted > 0) {
    rows.push({ text: `${notStarted} 名考生尚未开考`, tone: "warning" });
  }
  if (inProgress > 0) {
    rows.push({ text: `${inProgress} 名考生正在作答`, tone: "blue" });
  }
  if (submitted > 0) {
    rows.push({ text: `${submitted} 名考生已交卷`, tone: "green" });
  }
  rows.push({ text: "考试结束后自动进入阅卷流程", tone: "violet" });
  return rows;
}

function activityTone(operationType: string): "orange" | "blue" | "green" | "violet" {
  switch (operationType) {
    case "publish_exam":
      return "orange";
    case "send_invite":
      return "blue";
    case "update_settings":
      return "violet";
    default:
      return "green";
  }
}

function paperStatusTone(status: PaperRow["status"]) {
  switch (status) {
    case "enabled":
      return "success";
    case "disabled":
      return "danger";
    default:
      return "info";
  }
}

function paperStatusLabel(status: PaperRow["status"]) {
  switch (status) {
    case "enabled":
      return "已发布";
    case "disabled":
      return "已禁用";
    default:
      return "草稿";
  }
}

// previewLoadErrorFrom 只把 403/404 转成页面级错误，其它失败继续用 toast，保留用户重试空间。
function previewLoadErrorFrom(error: unknown, message: string): PreviewLoadError | null {
  if (!(error instanceof ApiError)) {
    return null;
  }
  if (error.status === 403) {
    return {
      title: message || "没有权限查看该考试",
      description: "请确认当前账号是否拥有该考试所在空间的管理权限。",
    };
  }
  if (error.status === 404) {
    return {
      title: message || "考试不存在或已删除",
      description: "该考试可能已被删除，或当前空间下不存在这场考试。",
    };
  }
  return null;
}

function formatScore(value: string | number) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) {
    return String(value);
  }
  return Number.isInteger(numeric) ? String(numeric) : numeric.toFixed(1);
}

function tabID(tab: PreviewTab) {
  return `paper-preview-tab-${previewTabs.indexOf(tab)}`;
}

function tabPanelID(tab: PreviewTab) {
  return `paper-preview-panel-${previewTabs.indexOf(tab)}`;
}

function scopedPaperSearch(currentSearch: string, spaceID: number | undefined) {
  if (spaceID !== undefined) {
    return currentSearch;
  }
  const params = new URLSearchParams(currentSearch);
  params.delete("space_id");
  const query = params.toString();
  return query ? `?${query}` : "";
}
