import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

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

export type ExamListResult = {
  items: ExamRow[];
};

export type ExamManagementAPI = {
  listExams(tenantID: number): Promise<ExamListResult>;
  publishExam(input: PublishExamInput): Promise<ExamRow>;
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

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const examApi = createExamAPI(defaultApiClient);

export function createExamAPI(apiClient: ApiClient): ExamManagementAPI {
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
