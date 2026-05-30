import { createApiClient } from "./client";
import type { ApiClient } from "./client";

export type ActorRole = "tenant_admin" | "space_admin" | "teacher" | "student";

export type PendingReviewRow = {
  attemptID: number;
  attemptQuestionID: number;
  studentName: string;
  spaceName: string;
  examName: string;
  questionTitle: string;
  answerContent: string;
  submittedAt: string;
  maxScore: string;
  answerVersion: number;
  pendingShortTextCount: number;
  status: "pending" | "completed";
};

export type ListPendingReviewsInput = {
  tenantID: number;
  examID: number;
  actorID: number;
  actorRole: ActorRole;
  spaceID?: number;
};

export type GradeShortTextInput = ListPendingReviewsInput & {
  attemptID: number;
  attemptQuestionID: number;
  answerVersion: number;
  score: string;
  comment: string;
};

export type GradingAPI = {
  listPendingAttempts(input: ListPendingReviewsInput): Promise<{ items: PendingReviewRow[] }>;
  gradeShortText(input: GradeShortTextInput): Promise<void>;
};

type PendingReviewAPIResponse = {
  attempt_id: number;
  attempt_question_id: number;
  student_name: string;
  space_name: string;
  exam_name: string;
  question_title: string;
  answer_content: string;
  submitted_at: number;
  max_score: string;
  answer_version: number;
  pending_short_text_count: number;
  status: "pending" | "completed";
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const gradingApi = createGradingAPI(defaultApiClient);

export function createGradingAPI(apiClient: ApiClient): GradingAPI {
  return {
    async listPendingAttempts(input) {
      const data = await apiClient.get<{ items: PendingReviewAPIResponse[] }>(
        `/api/v1/grading/pending?${actorQuery(input)}`,
      );
      return { items: data.items.map(mapPendingReviewResponse) };
    },
    async gradeShortText(input) {
      await apiClient.post(`/api/v1/exam-attempts/${input.attemptID}/questions/${input.attemptQuestionID}/grade`, {
        tenant_id: input.tenantID,
        exam_id: input.examID,
        actor_id: input.actorID,
        actor_role: input.actorRole,
        space_id: input.spaceID,
        answer_version: input.answerVersion,
        score: input.score,
        comment: input.comment,
      });
    },
  };
}

function actorQuery(input: ListPendingReviewsInput) {
  const params = new URLSearchParams({
    tenant_id: String(input.tenantID),
    exam_id: String(input.examID),
    actor_id: String(input.actorID),
    actor_role: input.actorRole,
  });
  if (input.spaceID) {
    params.set("space_id", String(input.spaceID));
  }
  return params.toString();
}

function mapPendingReviewResponse(row: PendingReviewAPIResponse): PendingReviewRow {
  return {
    attemptID: row.attempt_id,
    attemptQuestionID: row.attempt_question_id,
    studentName: row.student_name,
    spaceName: row.space_name,
    examName: row.exam_name,
    questionTitle: row.question_title,
    answerContent: row.answer_content,
    submittedAt: formatDateTime(row.submitted_at),
    maxScore: row.max_score,
    answerVersion: row.answer_version,
    pendingShortTextCount: row.pending_short_text_count,
    status: row.status,
  };
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
