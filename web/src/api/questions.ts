import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type QuestionStatus = "draft" | "ready";
export type QuestionType = "single" | "multiple" | "judge" | "fill_blank" | "short_text";
export type QuestionDifficulty = "easy" | "medium" | "hard";

export type QuestionRow = {
  id: number;
  tenantID: number;
  type: QuestionType;
  title: string;
  stem: string;
  options: string[];
  analysis: string;
  difficulty: QuestionDifficulty;
  tag: string;
  tags: string[];
  scoreDefault?: string;
  status: QuestionStatus;
};

export type ListQuestionsInput = {
  tenantID: number;
  spaceID?: number;
};

export type CreateQuestionInput = {
  tenantID: number;
  spaceID?: number;
  type: QuestionType;
  difficulty: QuestionDifficulty;
  title: string;
  options: string[];
  correctOptionIndexes?: number[];
  analysis: string;
  scoreDefault: string;
  tags: string[];
  standardAnswer?: string;
  referenceAnswer?: string;
  blankCount?: number;
};

export type ImportQuestionsInput = {
  tenantID: number;
  spaceID?: number;
  file: File;
};

export type ImportQuestionError = {
  rowNumber: number;
  reason: string;
};

export type ImportQuestionsResult = {
  successCount: number;
  errors: ImportQuestionError[];
};

export type QuestionListResult = {
  items: QuestionRow[];
};

export type QuestionBankAPI = {
  listQuestions(input: ListQuestionsInput): Promise<QuestionListResult>;
  createQuestion(input: CreateQuestionInput): Promise<QuestionRow>;
};

export type QuestionImportAPI = {
  importQuestions(input: ImportQuestionsInput): Promise<ImportQuestionsResult>;
};

export type QuestionAPI = QuestionBankAPI & QuestionImportAPI;

type QuestionAPIResponse = {
  id: number;
  tenant_id: number;
  type: QuestionType;
  difficulty: QuestionDifficulty;
  title: string;
  analysis: string;
  standard_answer?: string;
  reference_answer?: string;
  blank_count?: number;
  score_default?: string;
  status: "draft" | "enabled" | "disabled";
  tag: string;
  tags?: string[];
  options: Array<{
    option_key: string;
    content: string;
    is_correct: boolean;
    is_distractor: boolean;
  }>;
};

type ImportQuestionsAPIResponse = {
  success_count: number;
  errors: Array<{
    row_number: number;
    reason: string;
  }>;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
});

export const questionApi = createQuestionAPI(defaultApiClient);

export function createQuestionAPI(apiClient: ApiClient): QuestionAPI {
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
        ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }),
        type: input.type,
        difficulty: input.difficulty,
        title: input.title,
        analysis: input.analysis,
        score_default: input.scoreDefault,
        standard_answer: input.standardAnswer,
        reference_answer: input.referenceAnswer,
        blank_count: input.blankCount,
        tags: input.tags,
        options: input.options.map((content, index) => ({
          option_key: optionKeyByIndex(index),
          content,
          is_correct: input.correctOptionIndexes?.includes(index) ?? index === 0,
          is_distractor: !(input.correctOptionIndexes?.includes(index) ?? index === 0),
        })),
      });
      return mapQuestionResponse(data);
    },
    async importQuestions(input) {
      const formData = new FormData();
      formData.append("tenant_id", String(input.tenantID));
      if (input.spaceID !== undefined) {
        formData.append("space_id", String(input.spaceID));
      }
      formData.append("file", input.file);
      const data = await apiClient.upload<ImportQuestionsAPIResponse>("/api/v1/questions/import", formData);
      return mapImportQuestionsResponse(data);
    },
  };
}

function mapQuestionResponse(row: QuestionAPIResponse): QuestionRow {
  const tags = row.tags && row.tags.length > 0 ? row.tags : row.tag ? [row.tag] : [];

  return {
    id: row.id,
    tenantID: row.tenant_id,
    type: row.type,
    title: row.title,
    stem: row.title,
    options: row.options.map((option) => option.content),
    analysis: row.analysis,
    difficulty: row.difficulty ?? "medium",
    tag: tags[0] ?? "",
    tags,
    scoreDefault: row.score_default ?? "0",
    status: row.status === "enabled" ? "ready" : "draft",
  };
}

function optionKeyByIndex(index: number) {
  return String.fromCharCode("A".charCodeAt(0) + index);
}

function mapImportQuestionsResponse(response: ImportQuestionsAPIResponse): ImportQuestionsResult {
  return {
    successCount: response.success_count,
    errors: response.errors.map((item) => ({
      rowNumber: item.row_number,
      reason: item.reason,
    })),
  };
}
