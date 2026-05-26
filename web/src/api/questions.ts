import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type QuestionStatus = "draft" | "ready";

export type QuestionRow = {
  id: number;
  tenantID: number;
  title: string;
  stem: string;
  options: string[];
  analysis: string;
  tag: string;
  status: QuestionStatus;
};

export type ListQuestionsInput = {
  tenantID: number;
  spaceID?: number;
};

export type CreateQuestionInput = {
  tenantID: number;
  title: string;
  stem: string;
  options: string[];
  analysis: string;
  tag: string;
};

export type QuestionListResult = {
  items: QuestionRow[];
};

export type QuestionBankAPI = {
  listQuestions(input: ListQuestionsInput): Promise<QuestionListResult>;
  createQuestion(input: CreateQuestionInput): Promise<QuestionRow>;
};

type QuestionAPIResponse = {
  id: number;
  tenant_id: number;
  title: string;
  analysis: string;
  status: "draft" | "enabled" | "disabled";
  tag: string;
  tags: string[];
  options: Array<{
    option_key: string;
    content: string;
    is_correct: boolean;
    is_distractor: boolean;
  }>;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const questionApi = createQuestionAPI(defaultApiClient);

export function createQuestionAPI(apiClient: ApiClient): QuestionBankAPI {
  return {
    async listQuestions(input) {
      const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
      if (input.spaceID !== undefined) {
        params.set("space_id", String(input.spaceID));
      }
      const data = await apiClient.get<PageData<QuestionAPIResponse>>(`/api/v1/questions?${params.toString()}`);
      return { items: data.items.map(mapQuestionResponse) };
    },
    async createQuestion(input) {
      const data = await apiClient.post<QuestionAPIResponse>("/api/v1/questions", {
        tenant_id: input.tenantID,
        type: "single",
        difficulty: "medium",
        title: input.stem,
        analysis: input.analysis,
        score_default: "2",
        tags: [input.tag],
        options: input.options.map((content, index) => ({
          option_key: optionKeyByIndex(index),
          content,
          is_correct: index === 0,
          is_distractor: index !== 0,
        })),
      });
      return { ...mapQuestionResponse(data), title: input.title, stem: input.stem };
    },
  };
}

function mapQuestionResponse(row: QuestionAPIResponse): QuestionRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    title: row.title,
    stem: row.title,
    options: row.options.map((option) => option.content),
    analysis: row.analysis,
    tag: row.tag || row.tags[0] || "",
    status: row.status === "enabled" ? "ready" : "draft",
  };
}

function optionKeyByIndex(index: number) {
  return String.fromCharCode("A".charCodeAt(0) + index);
}
