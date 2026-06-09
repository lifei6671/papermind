import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type ExamStatus = "draft" | "published" | "closed" | "disabled";
export type ExamCreationStatus = "draft" | "published";
export type ExamStatusUpdate = "published" | "closed" | "disabled";

export type ExamRow = {
  id: number;
  tenantID: number;
  paperID: number;
  name: string;
  paperName: string;
  inviteCode: string;
  target: string;
  status: ExamStatus;
  startAt: string;
  endAt: string;
  durationMinutes: number;
};

// PublishExamTargetInput 表示一次考试发布中的单个投放目标。
//
// 后端会逐个目标做权限校验，因此前端只负责把用户选择的空间/个人目标
// 原样传递，不在浏览器里推导额外权限。
export type PublishExamTargetInput = {
  targetType: "space" | "user";
  targetID: number;
};

export type PublishExamInput = {
  tenantID: number;
  paperID: number;
  name: string;
  targetType?: "space" | "user";
  targetID?: number;
  targets?: PublishExamTargetInput[];
  startTime: number;
  endTime: number;
  durationMinutes: number;
  maxAttempts: number;
  resultStrategy: "latest" | "highest";
  publishMode: "immediate_score" | "manual_publish";
  scorePublishTime?: number | null;
  status: ExamCreationStatus;
};

export type UpdateExamStatusInput = {
  tenantID: number;
  examID: number;
  status: ExamStatusUpdate;
};

export type ResolveExamInviteInput = {
  inviteCode: string;
};

export type ExamListResult = {
  items: ExamRow[];
  page: number;
  pageSize: number;
  total: number;
};

export type ListExamsInput = {
  tenantID: number;
  spaceID?: number;
  paperID?: number;
  page?: number;
  pageSize?: number;
};

export type ExamManagementAPI = {
  listExams(input: ListExamsInput): Promise<ExamListResult>;
  publishExam(input: PublishExamInput): Promise<ExamRow>;
  updateExamStatus?(input: UpdateExamStatusInput): Promise<ExamRow>;
};

export type ExamEntryAPI = {
  resolveInvite(input: ResolveExamInviteInput): Promise<ExamRow>;
};

export type StudentExamQuestionType = "single" | "multiple" | "judge" | "fill_blank" | "short_text";

export type StudentExamOption = {
  id: number;
  key: string;
  content: string;
};

export type StudentExamQuestion = {
  id: number;
  number: number;
  sectionTitle: string;
  sectionSubtitle: string;
  type: StudentExamQuestionType;
  stem: string;
  score: number;
  options: StudentExamOption[];
  blankCount?: number;
};

export type StartAttemptInput = {
  tenantID: number;
  examID: number;
};

export type StartAttemptResult = {
  attemptID: number;
  examToken: string;
  answerDeadline: number;
  questions: StudentExamQuestion[];
};

export type SaveStudentAnswerInput = {
  tenantID: number;
  attemptID: number;
  attemptQuestionID: number;
  examToken: string;
  questionType: StudentExamQuestionType;
  optionIDs?: number[];
  text?: string;
};

export type SubmitAttemptEventType = "submit" | "auto_submit";

export type SubmitAttemptInput = {
  tenantID: number;
  attemptID: number;
  examToken: string;
  eventType?: SubmitAttemptEventType;
};

export type StudentVisibleResult = {
  attemptID: number;
  examID: number;
  attemptNo: number;
  objectiveScore: string;
  subjectiveScore: string;
  totalScore: string;
  analysisVisible: boolean;
};

export type RecordExamEventInput = {
  tenantID: number;
  attemptID: number;
  examToken: string;
  eventType: string;
  payload?: string;
};

export type StudentExamAPI = {
  startAttempt(input: StartAttemptInput): Promise<StartAttemptResult>;
  saveAnswer(input: SaveStudentAnswerInput): Promise<void>;
  submitAttempt(input: SubmitAttemptInput): Promise<void>;
  getVisibleResult(attemptID: number): Promise<StudentVisibleResult>;
  recordEvent(input: RecordExamEventInput): Promise<void>;
};

type ExamAPIResponse = {
  id: number;
  tenant_id: number;
  paper_id: number;
  name: string;
  start_time: number;
  end_time: number;
  duration_minutes: number;
  invite_code: string;
  status: ExamStatus;
  target_type?: "space" | "user";
  target_id?: number;
  targets?: Array<{
    target_type: "space" | "user";
    target_id: number;
  }>;
};

type StartAttemptAPIResponse = {
  attempt: {
    id: number;
    answer_deadline: number;
  };
  exam_token: string;
  questions: AttemptQuestionAPIResponse[];
};

type StudentVisibleResultAPIResponse = {
  attempt_id: number;
  exam_id: number;
  attempt_no: number;
  objective_score: string;
  subjective_score: string;
  total_score: string;
  analysis_visible: boolean;
};

type AttemptQuestionAPIResponse = {
  id: number;
  sort_order: number;
  section: {
    name: string;
    instructions: string;
  };
  question: {
    title: string;
    type: StudentExamQuestionType;
    blank_count?: number;
  };
  options: Array<{
    id: number;
    key: string;
    content: string;
  }>;
  score: string;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const examApi = createExamAPI(defaultApiClient);

export function createExamAPI(apiClient: ApiClient): ExamManagementAPI & ExamEntryAPI & StudentExamAPI {
  return {
    async listExams(input) {
      const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
      if (input.spaceID !== undefined) {
        params.set("space_id", String(input.spaceID));
      }
      if (input.paperID !== undefined) {
        params.set("paper_id", String(input.paperID));
      }
      if (input.page !== undefined) {
        params.set("page", String(input.page));
      }
      if (input.pageSize !== undefined) {
        params.set("page_size", String(input.pageSize));
      }
      const data = await apiClient.get<PageData<ExamAPIResponse>>(`/api/v1/exams?${params.toString()}`);
      return {
        items: data.items.map(mapExamResponse),
        page: data.page,
        pageSize: data.page_size,
        total: data.total,
      };
    },
    async publishExam(input) {
      const targets = normalizePublishTargets(input);
      const data = await apiClient.post<ExamAPIResponse>("/api/v1/exams", {
        tenant_id: input.tenantID,
        paper_id: input.paperID,
        name: input.name,
        target_type: targets[0]?.targetType,
        target_id: targets[0]?.targetID,
        targets: targets.map((target) => ({
          target_type: target.targetType,
          target_id: target.targetID,
        })),
        start_time: input.startTime,
        end_time: input.endTime,
        duration_minutes: input.durationMinutes,
        max_attempts: input.maxAttempts,
        result_strategy: input.resultStrategy,
        publish_mode: input.publishMode,
        score_publish_time: input.scorePublishTime ?? null,
        status: input.status,
      });
      return mapExamResponse(data);
    },
    async updateExamStatus(input) {
      const data = await apiClient.post<ExamAPIResponse>(`/api/v1/exams/${input.examID}/status`, {
        tenant_id: input.tenantID,
        status: input.status,
      });
      return mapExamResponse(data);
    },
    async resolveInvite(input) {
      const data = await apiClient.post<ExamAPIResponse>("/api/v1/exam-entry/invite/resolve", {
        invite_code: input.inviteCode,
      });
      return mapExamResponse(data);
    },
    async startAttempt(input) {
      const data = await apiClient.post<StartAttemptAPIResponse>(`/api/v1/exam-entry/exams/${input.examID}/attempts/start`, {
        tenant_id: input.tenantID,
      });
      return {
        attemptID: data.attempt.id,
        examToken: data.exam_token,
        answerDeadline: data.attempt.answer_deadline,
        questions: data.questions.map(mapAttemptQuestionResponse),
      };
    },
    async saveAnswer(input) {
      await apiClient.post(`/api/v1/exam-entry/attempts/${input.attemptID}/answers/${input.attemptQuestionID}`, {
        tenant_id: input.tenantID,
        exam_token: input.examToken,
        question_type: input.questionType,
        option_ids: input.optionIDs ?? [],
        text: input.text ?? "",
      });
    },
    async submitAttempt(input) {
      await apiClient.post(`/api/v1/exam-entry/attempts/${input.attemptID}/submit`, {
        tenant_id: input.tenantID,
        exam_token: input.examToken,
        event_type: input.eventType ?? "submit",
      });
    },
    async getVisibleResult(attemptID) {
      const data = await apiClient.get<StudentVisibleResultAPIResponse>(`/api/v1/exam-entry/results/${attemptID}`);
      return {
        attemptID: data.attempt_id,
        examID: data.exam_id,
        attemptNo: data.attempt_no,
        objectiveScore: data.objective_score,
        subjectiveScore: data.subjective_score,
        totalScore: data.total_score,
        analysisVisible: data.analysis_visible,
      };
    },
    async recordEvent(input) {
      await apiClient.post(`/api/v1/exam-entry/attempts/${input.attemptID}/events`, {
        tenant_id: input.tenantID,
        exam_token: input.examToken,
        event_type: input.eventType,
        payload: input.payload ?? "{}",
      });
    },
  };
}

function mapAttemptQuestionResponse(row: AttemptQuestionAPIResponse): StudentExamQuestion {
  return {
    id: row.id,
    number: row.sort_order,
    sectionTitle: row.section.name,
    sectionSubtitle: row.section.instructions,
    type: row.question.type,
    stem: row.question.title,
    score: Number(row.score),
    options: row.options.map((option) => ({
      id: option.id,
      key: option.key,
      content: option.content,
    })),
    blankCount: row.question.blank_count,
  };
}

// normalizePublishTargets 兼容旧版单目标调用和新版多目标调用。
//
// 页面会优先传 targets；保留 targetType/targetID 是为了让当前测试和其他潜在
// 调用点在后端升级期间不必一次性全部迁移。
function normalizePublishTargets(input: PublishExamInput): PublishExamTargetInput[] {
  if (input.targets !== undefined && input.targets.length > 0) {
    return input.targets;
  }
  if (input.targetType !== undefined && input.targetID !== undefined) {
    return [{ targetType: input.targetType, targetID: input.targetID }];
  }
  return [];
}

function mapExamResponse(row: ExamAPIResponse): ExamRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    paperID: row.paper_id,
    name: row.name,
    paperName: "试卷 " + row.paper_id,
    inviteCode: row.invite_code,
    target: formatTargets(row.targets, row.target_type, row.target_id),
    status: row.status,
    startAt: formatDateTime(row.start_time),
    endAt: formatDateTime(row.end_time),
    durationMinutes: row.duration_minutes,
  };
}

function formatTarget(targetType: "space" | "user" | undefined, targetID: number | undefined) {
  if (!targetType || !targetID) {
    return "未配置";
  }
  if (targetType === "space") {
    return "空间 " + targetID;
  }
  return "用户 " + targetID;
}

function formatTargets(
  targets: ExamAPIResponse["targets"],
  targetType: "space" | "user" | undefined,
  targetID: number | undefined,
) {
  if (targets !== undefined && targets.length > 0) {
    return targets.map((target) => formatTarget(target.target_type, target.target_id)).join("、");
  }
  return formatTarget(targetType, targetID);
}

function formatDateTime(value: number) {
  const date = new Date(value);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day} ${hour}:${minute}`;
}
