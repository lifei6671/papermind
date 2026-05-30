import { createApiClient } from "./client";
import type { ApiClient } from "./client";
import type { ActorRole } from "./grading";

export type ResultRow = {
  id: number;
  studentName: string;
  spaceName: string;
  attemptNo: number;
  objectiveScore: string;
  subjectiveScore: string;
  totalScore: string;
  submittedAt: string;
  status: string;
};

export type ResultActorInput = {
  tenantID: number;
  examID: number;
  actorID: number;
  actorRole: ActorRole;
  spaceID?: number;
};

export type SavePublishConfigInput = ResultActorInput & {
  publishMode: "immediate_score" | "manual_publish";
  scorePublishTime?: number;
};

export type ExportResultsResult = {
  filePath: string;
  fileURL: string;
  rowCount: number;
};

export type ResultsAPI = {
  listResults(input: ResultActorInput): Promise<{ items: ResultRow[] }>;
  savePublishConfig(input: SavePublishConfigInput): Promise<void>;
  exportResults(input: ResultActorInput): Promise<ExportResultsResult>;
};

type ResultAPIResponse = {
  id: number;
  student_name: string;
  space_name: string;
  attempt_no: number;
  objective_score: string;
  subjective_score: string;
  total_score: string;
  submitted_at: number;
  status: string;
};

type ExportResultsAPIResponse = {
  file_path: string;
  file_url: string;
  row_count: number;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const resultsApi = createResultsAPI(defaultApiClient);

export function createResultsAPI(apiClient: ApiClient): ResultsAPI {
  return {
    async listResults(input) {
      const data = await apiClient.get<{ items: ResultAPIResponse[] }>(`/api/v1/results?${actorQuery(input)}`);
      return { items: data.items.map(mapResultResponse) };
    },
    async savePublishConfig(input) {
      await apiClient.post("/api/v1/results/publish-config", {
        tenant_id: input.tenantID,
        exam_id: input.examID,
        actor_id: input.actorID,
        actor_role: input.actorRole,
        space_id: input.spaceID,
        publish_mode: input.publishMode,
        score_publish_time: input.scorePublishTime,
      });
    },
    async exportResults(input) {
      const data = await apiClient.post<ExportResultsAPIResponse>("/api/v1/results/export", {
        tenant_id: input.tenantID,
        exam_id: input.examID,
        actor_id: input.actorID,
        actor_role: input.actorRole,
        space_id: input.spaceID,
      });
      return {
        filePath: data.file_path,
        fileURL: data.file_url,
        rowCount: data.row_count,
      };
    },
  };
}

function actorQuery(input: ResultActorInput) {
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

function mapResultResponse(row: ResultAPIResponse): ResultRow {
  return {
    id: row.id,
    studentName: row.student_name,
    spaceName: row.space_name,
    attemptNo: row.attempt_no,
    objectiveScore: row.objective_score,
    subjectiveScore: row.subjective_score,
    totalScore: row.total_score,
    submittedAt: formatDateTime(row.submitted_at),
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
