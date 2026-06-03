import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type QuestionStatus = "draft" | "ready" | "disabled";
export type QuestionType = "single" | "multiple" | "judge" | "fill_blank" | "short_text";
export type QuestionDifficulty = "easy" | "medium" | "hard";

export type QuestionRow = {
  id: number;
  tenantID: number;
  type: QuestionType;
  title: string;
  stem: string;
  options: string[];
  correctOptionIndexes?: number[];
  analysis: string;
  difficulty: QuestionDifficulty;
  tag: string;
  tags: string[];
  scoreDefault?: string;
  standardAnswer?: string;
  blankAnswers?: string[];
  referenceAnswer?: string;
  authorName?: string;
  authorRole?: string;
  createdAt?: number;
  status: QuestionStatus;
};

export type ListQuestionsInput = {
  tenantID: number;
  spaceID?: number;
  page?: number;
  pageSize?: number;
  search?: string;
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

export type GetQuestionInput = {
  tenantID: number;
  questionID: number;
};

export type UpdateQuestionInput = CreateQuestionInput & {
  questionID: number;
};

export type QuestionActionInput = {
  tenantID: number;
  questionID: number;
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
  page: number;
  pageSize: number;
  total: number;
};

export type QuestionBankAPI = {
  listQuestions(input: ListQuestionsInput): Promise<QuestionListResult>;
  createQuestion(input: CreateQuestionInput): Promise<QuestionRow>;
};

export type QuestionManagementAPI = {
  getQuestion(input: GetQuestionInput): Promise<QuestionRow>;
  updateQuestion(input: UpdateQuestionInput): Promise<QuestionRow>;
  disableQuestion(input: QuestionActionInput): Promise<QuestionRow>;
  enableQuestion(input: QuestionActionInput): Promise<QuestionRow>;
  deleteQuestion(input: QuestionActionInput): Promise<void>;
};

export type QuestionImportAPI = {
  importQuestions(input: ImportQuestionsInput): Promise<ImportQuestionsResult>;
};

export type QuestionAPI = QuestionBankAPI & QuestionManagementAPI & QuestionImportAPI;

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
  author_name?: string;
  author_role?: string;
  created_at?: number;
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
      params.set("page", String(input.page ?? 1));
      params.set("page_size", String(input.pageSize ?? 20));
      const search = input.search?.trim();
      if (search) {
        params.set("search", search);
      }
      const data = await apiClient.get<PageData<QuestionAPIResponse>>(`/api/v1/questions?${params.toString()}`);
      return {
        items: data.items.map(mapQuestionResponse),
        page: data.page,
        pageSize: data.page_size,
        total: data.total,
      };
    },
    async getQuestion(input) {
      const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
      const data = await apiClient.get<QuestionAPIResponse>(`/api/v1/questions/${input.questionID}?${params.toString()}`);
      return mapQuestionResponse(data);
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
    async updateQuestion(input) {
      const data = await apiClient.request<QuestionAPIResponse>(`/api/v1/questions/${input.questionID}`, {
        method: "PUT",
        body: questionMutationBody(input),
      });
      return mapQuestionResponse(data);
    },
    async disableQuestion(input) {
      const data = await apiClient.post<QuestionAPIResponse>(`/api/v1/questions/${input.questionID}/disable`, {
        tenant_id: input.tenantID,
      });
      return mapQuestionResponse(data);
    },
    async enableQuestion(input) {
      const data = await apiClient.post<QuestionAPIResponse>(`/api/v1/questions/${input.questionID}/enable`, {
        tenant_id: input.tenantID,
      });
      return mapQuestionResponse(data);
    },
    async deleteQuestion(input) {
      await apiClient.request<null>(`/api/v1/questions/${input.questionID}`, {
        method: "DELETE",
        body: { tenant_id: input.tenantID },
      });
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

function questionMutationBody(input: CreateQuestionInput) {
  return {
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
  };
}

function mapQuestionResponse(row: QuestionAPIResponse): QuestionRow {
  const tags = row.tags && row.tags.length > 0 ? row.tags : row.tag ? [row.tag] : [];
  const blankAnswers = parseBlankAnswers(row.type, row.standard_answer);

  return {
    id: row.id,
    tenantID: row.tenant_id,
    type: row.type,
    title: row.title,
    stem: row.title,
    options: row.options.map((option) => option.content),
    correctOptionIndexes: row.options.flatMap((option, index) => option.is_correct ? [index] : []),
    analysis: row.analysis,
    difficulty: row.difficulty ?? "medium",
    tag: tags[0] ?? "",
    tags,
    scoreDefault: row.score_default ?? "0",
    standardAnswer: row.standard_answer,
    blankAnswers,
    referenceAnswer: row.reference_answer,
    authorName: row.author_name ?? "",
    authorRole: row.author_role ?? "",
    createdAt: row.created_at ?? 0,
    status: mapQuestionStatus(row.status),
  };
}

function mapQuestionStatus(status: QuestionAPIResponse["status"]): QuestionStatus {
  if (status === "enabled") {
    return "ready";
  }
  if (status === "disabled") {
    return "disabled";
  }
  return "draft";
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

function parseBlankAnswers(type: QuestionType, standardAnswer: string | undefined) {
  if (type !== "fill_blank" || !standardAnswer) {
    return undefined;
  }
  const trimmed = standardAnswer.trim();
  if (!trimmed) {
    return [];
  }
  if (!trimmed.startsWith("[")) {
    return [trimmed];
  }
  try {
    const items = JSON.parse(trimmed);
    if (!Array.isArray(items)) {
      return [trimmed];
    }
    return items
      .map((item) => typeof item === "string" ? item.trim() : "")
      .filter((item) => item !== "");
  } catch {
    return [trimmed];
  }
}
