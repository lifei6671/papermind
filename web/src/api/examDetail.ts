import { createApiClient } from "./client";
import type { ApiClient } from "./client";

// ExamDetailStatus 是管理端考试详情页当前需要识别的考试状态。
export type ExamDetailStatus = "draft" | "published";

// ExamDetailExam 对应后端 examResponse，是详情页标题区和基础信息的数据来源。
export type ExamDetailExam = {
  id: number;
  tenantID: number;
  paperID: number;
  name: string;
  startTime: number;
  endTime: number;
  durationMinutes: number;
  maxAttempts: number;
  resultStrategy: string;
  publishMode: string;
  scorePublishTime: number | null;
  inviteCode: string;
  status: ExamDetailStatus;
  targetType?: "space" | "user";
  targetID?: number;
};

// ExamDetailTarget 表示考试发布时的一个投放目标，用于标题区和权限范围展示。
export type ExamDetailTarget = {
  targetType: "space" | "user";
  targetID: number;
};

// ExamDetailPermissions 是后端裁剪后的管理端按钮和 tab 可见权限。
export type ExamDetailPermissions = {
  canViewDetail: boolean;
  canViewOverview: boolean;
  canViewPaper: boolean;
  canViewCandidates: boolean;
  canManageCandidates: boolean;
  canViewResults: boolean;
  canExportResults: boolean;
  canPublishResults: boolean;
  canUpdateSettings: boolean;
  canViewLogs: boolean;
};

// ExamManagementDetail 是详情首屏接口返回的数据，包含考试、目标和权限。
export type ExamManagementDetail = {
  exam: ExamDetailExam;
  targets: ExamDetailTarget[];
  targetSpaceIDs: number[];
  allowedSpaceIDs: number[];
  permissions: ExamDetailPermissions;
};

// ExamOverviewQuestionType 是考试概览中的题型分布行。
export type ExamOverviewQuestionType = {
  questionType: string;
  questionCount: number;
  totalScore: string;
};

// ExamOverviewActivity 是考试概览中的近期动态行。
export type ExamOverviewActivity = {
  operationType: string;
  operationTitle: string;
  operationDetail: string;
  createdAt: number;
};

// ExamOverviewData 是考试概览 tab 的真实数据来源。
export type ExamOverviewData = {
  exam: ExamDetailExam;
  candidateStats: {
    planned: number;
    joined: number;
    submitted: number;
    inProgress: number;
  };
  questionTypes: ExamOverviewQuestionType[];
  recentActivities: ExamOverviewActivity[];
  permissions: ExamDetailPermissions;
};

// ExamPaperPreviewOption 是普通管理预览可展示的选项数据，不包含正确答案标记。
export type ExamPaperPreviewOption = {
  id: number;
  key: string;
  content: string;
};

// ExamPaperPreviewSection 是试卷预览左侧结构摘要。
export type ExamPaperPreviewSection = {
  sectionID: number;
  sectionName: string;
  questionType: string;
  questionCount: number;
  totalScore: string;
};

// ExamPaperPreviewQuestion 是试卷预览题目卡片数据，不包含标准答案。
export type ExamPaperPreviewQuestion = {
  sectionID: number;
  sectionName: string;
  questionID: number;
  questionType: string;
  title: string;
  score: string;
  blankCount: number;
  sortOrder: number;
  options: ExamPaperPreviewOption[];
};

// ExamPaperPreviewData 是试卷预览 tab 的结构、题目和分页数据。
export type ExamPaperPreviewData = {
  exam: ExamDetailExam;
  sections: ExamPaperPreviewSection[];
  items: ExamPaperPreviewQuestion[];
  page: number;
  pageSize: number;
  total: number;
  permissions: ExamDetailPermissions;
};

export type ExamCandidateStatus = "not_started" | "in_progress" | "submitted";

// ExamCandidateSourceTarget 表示考生名单来源，用于解释空间投放和用户直投去重后的来源。
export type ExamCandidateSourceTarget = {
  targetType: "space" | "user";
  targetID: number;
  spaceID: number;
  spaceName: string;
};

// ExamCandidateRow 是考生管理 tab 的真实列表行。
export type ExamCandidateRow = {
  userID: number;
  username: string;
  realName: string;
  spaceID: number;
  spaceName: string;
  status: ExamCandidateStatus;
  startedAt: number | null;
  submittedAt: number | null;
  totalScore: string;
  attemptCount: number;
  currentAttemptID: number | null;
  resultAttemptID: number | null;
  sourceTargets: ExamCandidateSourceTarget[];
};

// ExamCandidateListData 是考生管理 tab 的真实分页数据。
export type ExamCandidateListData = {
  exam: ExamDetailExam;
  items: ExamCandidateRow[];
  page: number;
  pageSize: number;
  total: number;
  permissions: ExamDetailPermissions;
};

export type ExamResultStatus = "published" | "pending_publish" | "pending_review";

// ExamResultSummaryData 是成绩管理 tab 顶部统计和图表的数据来源。
export type ExamResultSummaryData = {
  exam: ExamDetailExam;
  stats: {
    submitted: number;
    averageScore: string;
    highestScore: string;
    passRate: string;
    pendingSubjective: number;
  };
  scoreDistribution: Array<{
    label: string;
    count: number;
  }>;
  questionTypeRates: Array<{
    questionType: string;
    questionTypeLabel: string;
    averageRate: number;
  }>;
  permissions: ExamDetailPermissions;
};

// ExamResultRow 是成绩管理表格行，rank 必须来自后端全量排序，前端不能用当前页下标生成。
export type ExamResultRow = {
  rank: number;
  attemptID: number;
  userID: number;
  username: string;
  realName: string;
  spaceID: number;
  spaceName: string;
  objectiveScore: string;
  subjectiveScore: string;
  totalScore: string;
  status: ExamResultStatus;
  submittedAt: number | null;
};

// ExamResultListData 是成绩管理列表的分页响应。
export type ExamResultListData = {
  exam: ExamDetailExam;
  items: ExamResultRow[];
  page: number;
  pageSize: number;
  total: number;
  permissions: ExamDetailPermissions;
};

// ExamAnswerSheetAttempt 是管理端答卷详情的作答摘要。
export type ExamAnswerSheetAttempt = {
  attemptID: number;
  examID: number;
  userID: number;
  username: string;
  realName: string;
  objectiveScore: string;
  subjectiveScore: string;
  totalScore: string;
  submittedAt: number;
};

// ExamAnswerSheetItem 是答卷详情中的单题记录，包含标准答案与考生答案快照。
export type ExamAnswerSheetItem = {
  attemptQuestionID: number;
  sectionID: number;
  questionID: number;
  sortOrder: number;
  sectionSnapshot: string;
  questionSnapshot: string;
  questionType: string;
  questionTitle: string;
  optionSnapshot: string;
  correctAnswerSnapshot: string;
  score: string;
  answerContent: string;
  answerScore: string;
  gradingStatus: string;
  graderComment: string;
  gradedBy: number;
  gradedAt: number | null;
};

// ExamAnswerSheetData 是管理端答卷详情弹层的数据源。
export type ExamAnswerSheetData = {
  exam: ExamDetailExam;
  attempt: ExamAnswerSheetAttempt;
  items: ExamAnswerSheetItem[];
  permissions: ExamDetailPermissions;
};

// ExamOperationLogRow 是操作日志 tab 的审计记录行。
export type ExamOperationLogRow = {
  id: number;
  operationType: string;
  operationTitle: string;
  operationDetail: string;
  actorID: number;
  actorType: string;
  actorRole: string;
  operationGroupID: string;
  spaceID: number | null;
  createdAt: number;
};

// ExamOperationLogListData 是操作日志 tab 的分页响应。
export type ExamOperationLogListData = {
  exam: ExamDetailExam;
  items: ExamOperationLogRow[];
  page: number;
  pageSize: number;
  total: number;
  permissions: ExamDetailPermissions;
};

// ExamSettingsUpdateResult 是考试设置保存后的返回结果。
// 后端当前只返回最新考试基础信息，页面继续沿用已加载的权限结果。
export type ExamSettingsUpdateResult = {
  exam: ExamDetailExam;
  saved: boolean;
};

// ExamDetailRequest 是详情页各读接口共用的考试定位和授权范围参数。
export type ExamDetailRequest = {
  tenantID: number;
  examID: number;
  spaceID?: number;
};

// ExamPaperPreviewRequest 是试卷预览接口的筛选和分页参数。
export type ExamPaperPreviewRequest = ExamDetailRequest & {
  questionType?: string;
  page: number;
  pageSize: number;
};

// ExamCandidateListRequest 是考生管理接口的筛选和分页参数。
export type ExamCandidateListRequest = ExamDetailRequest & {
  keyword?: string;
  status?: ExamCandidateStatus;
  page: number;
  pageSize: number;
};

// ExamResultListRequest 是成绩管理接口的筛选和分页参数。
export type ExamResultListRequest = ExamDetailRequest & {
  keyword?: string;
  status?: ExamResultStatus;
  page: number;
  pageSize: number;
};

// ExamOperationLogListRequest 是操作日志 tab 的筛选和分页参数。
export type ExamOperationLogListRequest = ExamDetailRequest & {
  operationType?: string;
  page: number;
  pageSize: number;
};

// ExamSettingsUpdateRequest 是考试设置页首版写入参数。
// 首版只允许修改成绩发布配置，不携带考试时长、发布范围或公平性配置，避免误改已开考关键规则。
export type ExamSettingsUpdateRequest = ExamDetailRequest & {
  publishMode: string;
  scorePublishTime: number | null;
};

// ExamCandidateWriteRequest 是考生导入和邀请码重发共用的写请求，后端用 examID 定位考试，用 userIDs 限定目标考生。
export type ExamCandidateWriteRequest = ExamDetailRequest & {
  userIDs: number[];
};

// ExamCandidateImportResult 是导入考生接口返回的写入结果和最新权限。
export type ExamCandidateImportResult = {
  importedCount: number;
  skippedCount: number;
  permissions: ExamDetailPermissions;
};

// ExamInvitationResendResult 是重发邀请码接口返回的发送结果和当前考试邀请码。
export type ExamInvitationResendResult = {
  sentCount: number;
  skippedCount: number;
  inviteCode: string;
  permissions: ExamDetailPermissions;
};

// ExamDetailAPI 收敛考试详情页已接入的管理端读写接口；查询只用 GET，写操作只用 POST。
export type ExamDetailAPI = {
  getDetail(input: ExamDetailRequest): Promise<ExamManagementDetail>;
  getOverview(input: ExamDetailRequest): Promise<ExamOverviewData>;
  getPaperPreview(input: ExamPaperPreviewRequest): Promise<ExamPaperPreviewData>;
  getCandidates(input: ExamCandidateListRequest): Promise<ExamCandidateListData>;
  importCandidates(input: ExamCandidateWriteRequest): Promise<ExamCandidateImportResult>;
  resendInvitations(input: ExamCandidateWriteRequest): Promise<ExamInvitationResendResult>;
  getResultsSummary(input: ExamDetailRequest): Promise<ExamResultSummaryData>;
  getResults(input: ExamResultListRequest): Promise<ExamResultListData>;
  getAnswerSheet(input: ExamDetailRequest & { attemptID: number }): Promise<ExamAnswerSheetData>;
  getOperationLogs(input: ExamOperationLogListRequest): Promise<ExamOperationLogListData>;
  updateSettings(input: ExamSettingsUpdateRequest): Promise<ExamSettingsUpdateResult>;
};

type ExamAPIResponse = {
  id: number;
  tenant_id: number;
  paper_id: number;
  name: string;
  start_time: number;
  end_time: number;
  duration_minutes: number;
  max_attempts: number;
  result_strategy: string;
  publish_mode: string;
  score_publish_time: number | null;
  invite_code: string;
  status: ExamDetailStatus;
  target_type?: "space" | "user";
  target_id?: number;
};

type ExamManagementPermissionsAPIResponse = {
  can_view_detail: boolean;
  can_view_overview: boolean;
  can_view_paper: boolean;
  can_view_candidates: boolean;
  can_manage_candidates: boolean;
  can_view_results: boolean;
  can_export_results: boolean;
  can_publish_results: boolean;
  can_update_settings: boolean;
  can_view_logs: boolean;
};

type ExamManagementDetailAPIResponse = {
  exam: ExamAPIResponse;
  targets: Array<{ target_type: "space" | "user"; target_id: number }>;
  target_space_ids: number[];
  allowed_space_ids: number[];
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamOverviewAPIResponse = {
  exam: ExamAPIResponse;
  candidate_stats: {
    planned: number;
    joined: number;
    submitted: number;
    in_progress: number;
  };
  question_types: Array<{
    question_type: string;
    question_count: number;
    total_score: string;
  }>;
  recent_activities: Array<{
    operation_type: string;
    operation_title: string;
    operation_detail: string;
    created_at: number;
  }>;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamPaperPreviewAPIResponse = {
  exam: ExamAPIResponse;
  sections: Array<{
    section_id: number;
    section_name: string;
    question_type: string;
    question_count: number;
    total_score: string;
  }>;
  items: Array<{
    section_id: number;
    section_name: string;
    question_id: number;
    question_type: string;
    title: string;
    score: string;
    blank_count: number;
    sort_order: number;
    options: Array<{
      id: number;
      key: string;
      content: string;
    }>;
  }>;
  page: number;
  page_size: number;
  total: number;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamCandidateListAPIResponse = {
  exam: ExamAPIResponse;
  items: Array<{
    user_id: number;
    username: string;
    real_name: string;
    space_id: number;
    space_name: string;
    status: ExamCandidateStatus;
    started_at: number | null;
    submitted_at: number | null;
    total_score: string;
    attempt_count: number;
    current_attempt_id: number | null;
    result_attempt_id: number | null;
    source_targets: Array<{
      target_type: "space" | "user";
      target_id: number;
      space_id: number;
      space_name: string;
    }>;
  }>;
  page: number;
  page_size: number;
  total: number;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamCandidateImportAPIResponse = {
  imported_count: number;
  skipped_count: number;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamInvitationResendAPIResponse = {
  sent_count: number;
  skipped_count: number;
  invite_code: string;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamResultSummaryAPIResponse = {
  exam: ExamAPIResponse;
  stats: {
    submitted: number;
    average_score: string;
    highest_score: string;
    pass_rate: string;
    pending_subjective: number;
  };
  score_distribution: Array<{
    label: string;
    count: number;
  }>;
  question_type_rates: Array<{
    question_type: string;
    question_type_label: string;
    average_rate: number;
  }>;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamResultListAPIResponse = {
  exam: ExamAPIResponse;
  items: Array<{
    rank: number;
    attempt_id: number;
    user_id: number;
    username: string;
    real_name: string;
    space_id: number;
    space_name: string;
    objective_score: string;
    subjective_score: string;
    total_score: string;
    status: ExamResultStatus;
    submitted_at: number | null;
  }>;
  page: number;
  page_size: number;
  total: number;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamAnswerSheetAPIResponse = {
  exam: ExamAPIResponse;
  attempt: {
    attempt_id: number;
    exam_id: number;
    user_id: number;
    username: string;
    real_name: string;
    objective_score: string;
    subjective_score: string;
    total_score: string;
    submitted_at: number;
  };
  items: Array<{
    attempt_question_id: number;
    section_id: number;
    question_id: number;
    sort_order: number;
    section_snapshot: string;
    question_snapshot: string;
    question_type: string;
    question_title: string;
    option_snapshot: string;
    correct_answer_snapshot: string;
    score: string;
    answer_content: string;
    answer_score: string;
    grading_status: string;
    grader_comment: string;
    graded_by: number;
    graded_at: number | null;
  }>;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamOperationLogListAPIResponse = {
  exam: ExamAPIResponse;
  items: Array<{
    id: number;
    operation_type: string;
    operation_title: string;
    operation_detail: string;
    actor_id: number;
    actor_type: string;
    actor_role: string;
    operation_group_id: string;
    space_id: number | null;
    created_at: number;
  }>;
  page: number;
  page_size: number;
  total: number;
  permissions: ExamManagementPermissionsAPIResponse;
};

type ExamSettingsUpdateAPIResponse = {
  saved: boolean;
  exam: ExamAPIResponse;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const examDetailApi = createExamDetailAPI(defaultApiClient);

export function createExamDetailAPI(apiClient: ApiClient): ExamDetailAPI {
  return {
    async getDetail(input) {
      const data = await apiClient.get<ExamManagementDetailAPIResponse>(
        `/api/v1/exams/${input.examID}/detail?${managementQuery(input).toString()}`,
      );
      return mapManagementDetail(data);
    },
    async getOverview(input) {
      const data = await apiClient.get<ExamOverviewAPIResponse>(
        `/api/v1/exams/${input.examID}/overview?${managementQuery(input).toString()}`,
      );
      return mapOverview(data);
    },
    async getPaperPreview(input) {
      const params = managementQuery(input);
      if (input.questionType) {
        params.set("type", input.questionType);
      }
      params.set("page", String(input.page));
      params.set("page_size", String(input.pageSize));
      const data = await apiClient.get<ExamPaperPreviewAPIResponse>(
        `/api/v1/exams/${input.examID}/paper-preview?${params.toString()}`,
      );
      return mapPaperPreview(data);
    },
    async getCandidates(input) {
      const params = managementQuery(input);
      if (input.keyword) {
        params.set("keyword", input.keyword);
      }
      if (input.status) {
        params.set("status", input.status);
      }
      params.set("page", String(input.page));
      params.set("page_size", String(input.pageSize));
      const data = await apiClient.get<ExamCandidateListAPIResponse>(
        `/api/v1/exams/${input.examID}/candidates?${params.toString()}`,
      );
      return mapCandidates(data);
    },
    async importCandidates(input) {
      const data = await apiClient.post<ExamCandidateImportAPIResponse>(
        `/api/v1/exams/${input.examID}/candidates/import`,
        // 详情页已按 space_id 收窄读取范围，写接口也必须显式透传当前空间，避免多空间管理员跨空间误写。
        { tenant_id: input.tenantID, ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }), user_ids: input.userIDs },
      );
      return mapCandidateImport(data);
    },
    async resendInvitations(input) {
      const data = await apiClient.post<ExamInvitationResendAPIResponse>(
        `/api/v1/exams/${input.examID}/invitations/resend`,
        // 邀请码重发必须显式传目标考生与当前空间范围，避免 scoped 页面误操作到其它授权空间。
        { tenant_id: input.tenantID, ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }), user_ids: input.userIDs },
      );
      return mapInvitationResend(data);
    },
    async getResultsSummary(input) {
      const data = await apiClient.get<ExamResultSummaryAPIResponse>(
        `/api/v1/exams/${input.examID}/results/summary?${managementQuery(input).toString()}`,
      );
      return mapResultSummary(data);
    },
    async getResults(input) {
      const params = managementQuery(input);
      if (input.keyword) {
        params.set("keyword", input.keyword);
      }
      if (input.status) {
        params.set("status", input.status);
      }
      params.set("page", String(input.page));
      params.set("page_size", String(input.pageSize));
      const data = await apiClient.get<ExamResultListAPIResponse>(
        `/api/v1/exams/${input.examID}/results?${params.toString()}`,
      );
      return mapResults(data);
    },
    async getAnswerSheet(input) {
      const data = await apiClient.get<ExamAnswerSheetAPIResponse>(
        `/api/v1/exams/${input.examID}/attempts/${input.attemptID}/answer-sheet?${managementQuery(input).toString()}`,
      );
      return mapAnswerSheet(data);
    },
    async getOperationLogs(input) {
      const params = managementQuery(input);
      if (input.operationType) {
        params.set("operation_type", input.operationType);
      }
      params.set("page", String(input.page));
      params.set("page_size", String(input.pageSize));
      const data = await apiClient.get<ExamOperationLogListAPIResponse>(
        `/api/v1/exams/${input.examID}/logs?${params.toString()}`,
      );
      return mapOperationLogs(data);
    },
    async updateSettings(input) {
      const data = await apiClient.post<ExamSettingsUpdateAPIResponse>(
        `/api/v1/exams/${input.examID}/settings`,
        // 后端使用严格 JSON 解析；这里刻意只发送成绩发布配置，不能夹带考试时长或发布范围字段。
        {
          tenant_id: input.tenantID,
          ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }),
          publish_mode: input.publishMode,
          score_publish_time: input.scorePublishTime,
        },
      );
      return { exam: mapExam(data.exam), saved: data.saved };
    },
  };
}

// managementQuery 统一拼接管理端详情接口的租户和空间查询参数，避免页面分散处理鉴权范围。
function managementQuery(input: ExamDetailRequest) {
  const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
  if (input.spaceID !== undefined) {
    params.set("space_id", String(input.spaceID));
  }
  return params;
}

function mapManagementDetail(row: ExamManagementDetailAPIResponse): ExamManagementDetail {
  return {
    exam: mapExam(row.exam),
    targets: row.targets.map((target) => ({
      targetType: target.target_type,
      targetID: target.target_id,
    })),
    targetSpaceIDs: row.target_space_ids,
    allowedSpaceIDs: row.allowed_space_ids,
    permissions: mapPermissions(row.permissions),
  };
}

function mapOverview(row: ExamOverviewAPIResponse): ExamOverviewData {
  return {
    exam: mapExam(row.exam),
    candidateStats: {
      planned: row.candidate_stats.planned,
      joined: row.candidate_stats.joined,
      submitted: row.candidate_stats.submitted,
      inProgress: row.candidate_stats.in_progress,
    },
    questionTypes: row.question_types.map((item) => ({
      questionType: item.question_type,
      questionCount: item.question_count,
      totalScore: item.total_score,
    })),
    recentActivities: row.recent_activities.map((item) => ({
      operationType: item.operation_type,
      operationTitle: item.operation_title,
      operationDetail: item.operation_detail,
      createdAt: item.created_at,
    })),
    permissions: mapPermissions(row.permissions),
  };
}

function mapPaperPreview(row: ExamPaperPreviewAPIResponse): ExamPaperPreviewData {
  return {
    exam: mapExam(row.exam),
    sections: row.sections.map((section) => ({
      sectionID: section.section_id,
      sectionName: section.section_name,
      questionType: section.question_type,
      questionCount: section.question_count,
      totalScore: section.total_score,
    })),
    items: row.items.map((item) => ({
      sectionID: item.section_id,
      sectionName: item.section_name,
      questionID: item.question_id,
      questionType: item.question_type,
      title: item.title,
      score: item.score,
      blankCount: item.blank_count,
      sortOrder: item.sort_order,
      options: item.options.map((option) => ({
        id: option.id,
        key: option.key,
        content: option.content,
      })),
    })),
    page: row.page,
    pageSize: row.page_size,
    total: row.total,
    permissions: mapPermissions(row.permissions),
  };
}

function mapCandidates(row: ExamCandidateListAPIResponse): ExamCandidateListData {
  return {
    exam: mapExam(row.exam),
    items: row.items.map((item) => ({
      userID: item.user_id,
      username: item.username,
      realName: item.real_name,
      spaceID: item.space_id,
      spaceName: item.space_name,
      status: item.status,
      startedAt: item.started_at,
      submittedAt: item.submitted_at,
      totalScore: item.total_score,
      attemptCount: item.attempt_count,
      currentAttemptID: item.current_attempt_id,
      resultAttemptID: item.result_attempt_id,
      sourceTargets: item.source_targets.map((source) => ({
        targetType: source.target_type,
        targetID: source.target_id,
        spaceID: source.space_id,
        spaceName: source.space_name,
      })),
    })),
    page: row.page,
    pageSize: row.page_size,
    total: row.total,
    permissions: mapPermissions(row.permissions),
  };
}

function mapCandidateImport(row: ExamCandidateImportAPIResponse): ExamCandidateImportResult {
  return {
    importedCount: row.imported_count,
    skippedCount: row.skipped_count,
    permissions: mapPermissions(row.permissions),
  };
}

function mapInvitationResend(row: ExamInvitationResendAPIResponse): ExamInvitationResendResult {
  return {
    sentCount: row.sent_count,
    skippedCount: row.skipped_count,
    inviteCode: row.invite_code,
    permissions: mapPermissions(row.permissions),
  };
}

function mapResultSummary(row: ExamResultSummaryAPIResponse): ExamResultSummaryData {
  return {
    exam: mapExam(row.exam),
    stats: {
      submitted: row.stats.submitted,
      averageScore: row.stats.average_score,
      highestScore: row.stats.highest_score,
      passRate: row.stats.pass_rate,
      pendingSubjective: row.stats.pending_subjective,
    },
    scoreDistribution: row.score_distribution.map((item) => ({
      label: item.label,
      count: item.count,
    })),
    questionTypeRates: row.question_type_rates.map((item) => ({
      questionType: item.question_type,
      questionTypeLabel: item.question_type_label,
      averageRate: item.average_rate,
    })),
    permissions: mapPermissions(row.permissions),
  };
}

function mapResults(row: ExamResultListAPIResponse): ExamResultListData {
  return {
    exam: mapExam(row.exam),
    items: row.items.map((item) => ({
      rank: item.rank,
      attemptID: item.attempt_id,
      userID: item.user_id,
      username: item.username,
      realName: item.real_name,
      spaceID: item.space_id,
      spaceName: item.space_name,
      objectiveScore: item.objective_score,
      subjectiveScore: item.subjective_score,
      totalScore: item.total_score,
      status: item.status,
      submittedAt: item.submitted_at,
    })),
    page: row.page,
    pageSize: row.page_size,
    total: row.total,
    permissions: mapPermissions(row.permissions),
  };
}

function mapAnswerSheet(row: ExamAnswerSheetAPIResponse): ExamAnswerSheetData {
  return {
    exam: mapExam(row.exam),
    attempt: {
      attemptID: row.attempt.attempt_id,
      examID: row.attempt.exam_id,
      userID: row.attempt.user_id,
      username: row.attempt.username,
      realName: row.attempt.real_name,
      objectiveScore: row.attempt.objective_score,
      subjectiveScore: row.attempt.subjective_score,
      totalScore: row.attempt.total_score,
      submittedAt: row.attempt.submitted_at,
    },
    items: row.items.map((item) => ({
      attemptQuestionID: item.attempt_question_id,
      sectionID: item.section_id,
      questionID: item.question_id,
      sortOrder: item.sort_order,
      sectionSnapshot: item.section_snapshot,
      questionSnapshot: item.question_snapshot,
      questionType: item.question_type,
      questionTitle: item.question_title,
      optionSnapshot: item.option_snapshot,
      correctAnswerSnapshot: item.correct_answer_snapshot,
      score: item.score,
      answerContent: item.answer_content,
      answerScore: item.answer_score,
      gradingStatus: item.grading_status,
      graderComment: item.grader_comment,
      gradedBy: item.graded_by,
      gradedAt: item.graded_at,
    })),
    permissions: mapPermissions(row.permissions),
  };
}

function mapOperationLogs(row: ExamOperationLogListAPIResponse): ExamOperationLogListData {
  return {
    exam: mapExam(row.exam),
    items: row.items.map((item) => ({
      id: item.id,
      operationType: item.operation_type,
      operationTitle: item.operation_title,
      operationDetail: item.operation_detail,
      actorID: item.actor_id,
      actorType: item.actor_type,
      actorRole: item.actor_role,
      operationGroupID: item.operation_group_id,
      spaceID: item.space_id,
      createdAt: item.created_at,
    })),
    page: row.page,
    pageSize: row.page_size,
    total: row.total,
    permissions: mapPermissions(row.permissions),
  };
}

function mapExam(row: ExamAPIResponse): ExamDetailExam {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    paperID: row.paper_id,
    name: row.name,
    startTime: row.start_time,
    endTime: row.end_time,
    durationMinutes: row.duration_minutes,
    maxAttempts: row.max_attempts,
    resultStrategy: row.result_strategy,
    publishMode: row.publish_mode,
    scorePublishTime: row.score_publish_time,
    inviteCode: row.invite_code,
    status: row.status,
    targetType: row.target_type,
    targetID: row.target_id,
  };
}

function mapPermissions(row: ExamManagementPermissionsAPIResponse): ExamDetailPermissions {
  return {
    canViewDetail: row.can_view_detail,
    canViewOverview: row.can_view_overview,
    canViewPaper: row.can_view_paper,
    canViewCandidates: row.can_view_candidates,
    canManageCandidates: row.can_manage_candidates,
    canViewResults: row.can_view_results,
    canExportResults: row.can_export_results,
    canPublishResults: row.can_publish_results,
    canUpdateSettings: row.can_update_settings,
    canViewLogs: row.can_view_logs,
  };
}
