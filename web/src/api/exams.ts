import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";
import { readStoredAccessToken } from "./session-token";

export type ExamStatus = "draft" | "published";

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

export type PublishExamInput = {
  tenantID: number;
  paperID: number;
  name: string;
  targetType: "space" | "user";
  targetID: number;
  startTime: number;
  endTime: number;
  durationMinutes: number;
  maxAttempts: number;
  resultStrategy: "latest" | "highest";
  publishMode: "immediate_score" | "manual_publish";
};

export type ResolveExamInviteInput = {
  inviteCode: string;
};

export type ExamListResult = {
  items: ExamRow[];
};

export type ExamManagementAPI = {
  listExams(tenantID: number): Promise<ExamListResult>;
  publishExam(input: PublishExamInput): Promise<ExamRow>;
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

export type SubmitAttemptInput = {
  tenantID: number;
  attemptID: number;
  examToken: string;
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
  getAccessToken: readStoredAccessToken,
});

export const examApi = createExamAPI(defaultApiClient);

export function createExamAPI(apiClient: ApiClient): ExamManagementAPI & ExamEntryAPI & StudentExamAPI {
  return {
    async listExams(tenantID) {
      const data = await apiClient.get<PageData<ExamAPIResponse>>(`/api/v1/exams?tenant_id=${tenantID}`);
      return {
        items: data.items.map(mapExamResponse),
      };
    },
    async publishExam(input) {
      const data = await apiClient.post<ExamAPIResponse>("/api/v1/exams", {
        tenant_id: input.tenantID,
        paper_id: input.paperID,
        name: input.name,
        target_type: input.targetType,
        target_id: input.targetID,
        start_time: input.startTime,
        end_time: input.endTime,
        duration_minutes: input.durationMinutes,
        max_attempts: input.maxAttempts,
        result_strategy: input.resultStrategy,
        publish_mode: input.publishMode,
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
        event_type: "submit",
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
  };
}

function mapExamResponse(row: ExamAPIResponse): ExamRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    paperID: row.paper_id,
    name: row.name,
    paperName: paperNameByID(row.paper_id),
    inviteCode: row.invite_code,
    target: formatTarget(row.target_type, row.target_id),
    status: row.status,
    startAt: formatDateTime(row.start_time),
    endAt: formatDateTime(row.end_time),
    durationMinutes: row.duration_minutes,
  };
}

function paperNameByID(paperID: number) {
  if (paperID === 100) {
    return "高一语文月考试卷";
  }
  if (paperID === 101) {
    return "高二数学阶段测评";
  }
  return `试卷 ${paperID}`;
}

function formatTarget(targetType: "space" | "user" | undefined, targetID: number | undefined) {
  if (!targetType || !targetID) {
    return "未配置";
  }
  if (targetType === "space") {
    return targetID === 100 ? "高一全年级" : `空间 ${targetID}`;
  }
  return `用户 ${targetID}`;
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
