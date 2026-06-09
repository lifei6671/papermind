import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";

export type QuestionStatus = "draft" | "ready" | "disabled";
export type QuestionType = "single" | "multiple" | "judge" | "fill_blank" | "short_text";
export type QuestionDifficulty = "easy" | "medium" | "hard";

export type QuestionRow = {
  id: number;
  tenantID: number;
  spaceID?: number;
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
  qualityScore?: number;
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
  scope?: "public";
  page?: number;
  pageSize?: number;
  search?: string;
  type?: QuestionType;
  difficulty?: QuestionDifficulty;
  tag?: string;
  status?: QuestionStatus | "ready";
};

export type ListQuestionTagsInput = {
  tenantID: number;
  spaceID?: number;
  scope?: "public";
  status?: QuestionStatus | "ready";
  search?: string;
};

export type QuestionAvailabilityInput = {
  tenantID: number;
  spaceID?: number;
  scope?: "public";
  status?: QuestionStatus | "ready";
  tags?: string[];
  excludeQuestionIDs?: number[];
};

export type CreateQuestionInput = {
  tenantID: number;
  spaceID?: number | null;
  type: QuestionType;
  difficulty: QuestionDifficulty;
  title: string;
  options: string[];
  correctOptionIndexes?: number[];
  analysis: string;
  scoreDefault: string;
  qualityScore?: number;
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
  spaceID?: number | null;
  file: File;
  status?: "draft" | "enabled";
};

export type ImportQuestionError = {
  rowNumber: number;
  reason: string;
};

export type ImportQuestionsResult = {
  successCount: number;
  duplicateCount: number;
  errors: ImportQuestionError[];
};

export type StartQuestionImportJobResult = {
  jobID: string;
};

export type QuestionImportJobEvent = {
  jobID: string;
  status: "queued" | "running" | "completed" | "failed";
  fileName: string;
  totalRows: number;
  processedRows: number;
  successCount: number;
  errorCount: number;
  duplicateCount: number;
  errors: ImportQuestionError[];
  message?: string;
};

export type QuestionImportJobInput = {
  jobID: string;
};

export type QuestionListResult = {
  items: QuestionRow[];
  page: number;
  pageSize: number;
  total: number;
};

export type QuestionBankAPI = {
  listQuestions(input: ListQuestionsInput): Promise<QuestionListResult>;
  listQuestionTags(input: ListQuestionTagsInput): Promise<string[]>;
  countAvailableQuestions(input: QuestionAvailabilityInput): Promise<Partial<Record<QuestionType, number>>>;
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

export type QuestionImportJobAPI = {
  startQuestionImportJob(input: ImportQuestionsInput): Promise<StartQuestionImportJobResult>;
  subscribeQuestionImportJob(
    input: QuestionImportJobInput,
    onEvent: (event: QuestionImportJobEvent) => void,
    onError: (error: Error) => void,
  ): () => void;
};

export type QuestionAPI = QuestionBankAPI & QuestionManagementAPI & QuestionImportAPI & QuestionImportJobAPI;

type QuestionAPIResponse = {
  id: number;
  tenant_id: number;
  space_id?: number | null;
  type: QuestionType;
  difficulty: QuestionDifficulty;
  title: string;
  analysis: string;
  standard_answer?: string;
  reference_answer?: string;
  blank_count?: number;
  score_default?: string;
  quality_score?: number;
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
  duplicate_count?: number;
  errors: Array<{
    row_number: number;
    reason: string;
  }>;
};

type StartQuestionImportJobAPIResponse = {
  job_id: string;
};

type QuestionImportJobEventAPIResponse = {
  job_id: string;
  status: QuestionImportJobEvent["status"];
  file_name: string;
  total_rows: number;
  processed_rows: number;
  success_count: number;
  error_count: number;
  duplicate_count: number;
  errors: Array<{
    row_number: number;
    reason: string;
  }>;
  message?: string;
};

type QuestionTagsAPIResponse = {
  items: string[];
};

type QuestionAvailabilityAPIResponse = {
  items: Array<{
    type: QuestionType;
    count: number;
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
      if (input.scope) {
        params.set("scope", input.scope);
      }
      params.set("page", String(input.page ?? 1));
      params.set("page_size", String(input.pageSize ?? 20));
      const search = input.search?.trim();
      if (search) {
        params.set("search", search);
      }
      if (input.type) {
        params.set("type", input.type);
      }
      if (input.difficulty) {
        params.set("difficulty", input.difficulty);
      }
      const tag = input.tag?.trim();
      if (tag) {
        params.set("tag", tag);
      }
      if (input.status) {
        params.set("status", input.status);
      }
      const data = await apiClient.get<PageData<QuestionAPIResponse>>(`/api/v1/questions?${params.toString()}`);
      return {
        items: data.items.map(mapQuestionResponse),
        page: data.page,
        pageSize: data.page_size,
        total: data.total,
      };
    },
    async listQuestionTags(input) {
      const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
      if (input.spaceID !== undefined) {
        params.set("space_id", String(input.spaceID));
      }
      if (input.scope) {
        params.set("scope", input.scope);
      }
      if (input.status) {
        params.set("status", input.status);
      }
      const search = input.search?.trim();
      if (search) {
        params.set("search", search);
      }
      const data = await apiClient.get<QuestionTagsAPIResponse>(`/api/v1/questions/tags?${params.toString()}`);
      return data.items;
    },
    async countAvailableQuestions(input) {
      const data = await apiClient.post<QuestionAvailabilityAPIResponse>("/api/v1/questions/availability-counts", {
        tenant_id: input.tenantID,
        ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }),
        ...(input.scope === undefined ? {} : { scope: input.scope }),
        ...(input.status === undefined ? {} : { status: input.status }),
        tags: input.tags ?? [],
        exclude_question_ids: input.excludeQuestionIDs ?? [],
      });
      return Object.fromEntries(data.items.map((item) => [item.type, item.count])) as Partial<Record<QuestionType, number>>;
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
        ...(input.qualityScore === undefined ? {} : { quality_score: input.qualityScore }),
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
      if (input.spaceID !== undefined && input.spaceID !== null) {
        formData.append("space_id", String(input.spaceID));
      }
      if (input.status !== undefined) {
        formData.append("status", input.status);
      }
      formData.append("file", input.file);
      const data = await apiClient.upload<ImportQuestionsAPIResponse>("/api/v1/questions/import", formData);
      return mapImportQuestionsResponse(data);
    },
    async startQuestionImportJob(input) {
      const formData = new FormData();
      formData.append("tenant_id", String(input.tenantID));
      if (input.spaceID !== undefined && input.spaceID !== null) {
        formData.append("space_id", String(input.spaceID));
      }
      if (input.status !== undefined) {
        formData.append("status", input.status);
      }
      formData.append("file", input.file);
      const data = await apiClient.upload<StartQuestionImportJobAPIResponse>("/api/v1/questions/import/jobs", formData);
      return { jobID: data.job_id };
    },
    subscribeQuestionImportJob(input, onEvent, onError) {
      const source = new EventSource(apiClient.url(`/api/v1/questions/import/jobs/${input.jobID}/events`), { withCredentials: true });
      source.addEventListener("import_progress", (message) => {
        onEvent(mapQuestionImportJobEventResponse(JSON.parse((message as MessageEvent).data) as QuestionImportJobEventAPIResponse));
      });
      source.onerror = () => {
        onError(new Error("题目导入进度连接失败"));
        source.close();
      };
      return () => source.close();
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
    ...(input.qualityScore === undefined ? {} : { quality_score: input.qualityScore }),
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
    spaceID: row.space_id ?? undefined,
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
    qualityScore: row.quality_score ?? 5,
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
    duplicateCount: response.duplicate_count ?? 0,
    errors: response.errors.map((item) => ({
      rowNumber: item.row_number,
      reason: item.reason,
    })),
  };
}

function mapQuestionImportJobEventResponse(response: QuestionImportJobEventAPIResponse): QuestionImportJobEvent {
  return {
    jobID: response.job_id,
    status: response.status,
    fileName: response.file_name,
    totalRows: response.total_rows,
    processedRows: response.processed_rows,
    successCount: response.success_count,
    errorCount: response.error_count,
    duplicateCount: response.duplicate_count,
    errors: response.errors.map((item) => ({
      rowNumber: item.row_number,
      reason: item.reason,
    })),
    message: response.message,
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
