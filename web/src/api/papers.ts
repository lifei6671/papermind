import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";
import { readStoredAccessToken } from "./session-token";

export type PaperRow = {
  id: number;
  tenantID: number;
  spaceID?: number;
  name: string;
  description?: string;
  buildMode: string;
  status: "draft" | "ready";
  totalScore: string;
};

export type PaperSectionRow = {
  id: number;
  tenantID: number;
  paperID: number;
  sortOrder: number;
  name: string;
  questionType: string;
  instructions?: string;
  totalScore: string;
  questionCount: number;
};

export type ListPapersInput = {
  tenantID: number;
  spaceID?: number;
};

export type ListPaperSectionsInput = {
  tenantID: number;
  paperID: number;
};

export type CreatePaperInput = {
  tenantID: number;
  spaceID?: number;
  name: string;
  description?: string;
  shuffleQuestions?: boolean;
  showAnalysis?: boolean;
};

export type DeletePaperInput = {
  tenantID: number;
  paperID: number;
};

export type CreatePaperSectionInput = {
  tenantID: number;
  paperID: number;
  name: string;
  questionType: string;
  instructions: string;
};

export type AddManualQuestionInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  questionID: number;
  score: string;
};

export type ManualQuestionRow = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  questionID: number;
  sortOrder: number;
  score: string;
};

export type PaperRuleRow = {
  id: number;
  tenantID: number;
  paperID: number;
  sectionID: number;
  sortOrder: number;
  difficulty?: string;
  tagIDs: number[];
  questionCount: number;
  scorePerQuestion: string;
  shuffleOptions?: boolean;
};

export type CreatePaperRuleInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  sortOrder: number;
  difficulty?: string;
  tagIDs: number[];
  questionCount: number;
  scorePerQuestion: string;
  shuffleOptions?: boolean;
};

export type RuleFixedGenerateInput = {
  tenantID: number;
  paperID: number;
};

export type RuleFixedGenerateResult = {
  paperID: number;
  generated: boolean;
};

export type RuleLivePrecheckInput = {
  tenantID: number;
  paperID: number;
};

export type RuleLivePrecheckResult = {
  candidateQuestionIDs: number[];
  candidateCount: number;
};

export type PaperListResult = {
  items: PaperRow[];
};

export type PaperSectionListResult = {
  items: PaperSectionRow[];
};

export type PaperRuleListResult = {
  items: PaperRuleRow[];
};

export type PaperAPI = {
  listPapers(input: ListPapersInput): Promise<PaperListResult>;
  createPaper(input: CreatePaperInput): Promise<PaperRow>;
  deletePaper(input: DeletePaperInput): Promise<void>;
  listSections(input: ListPaperSectionsInput): Promise<PaperSectionListResult>;
  createSection(input: CreatePaperSectionInput): Promise<PaperSectionRow>;
  addManualQuestion(input: AddManualQuestionInput): Promise<ManualQuestionRow>;
  listRules(input: ListPaperSectionsInput): Promise<PaperRuleListResult>;
  createRule(input: CreatePaperRuleInput): Promise<PaperRuleRow>;
  generateRuleFixed(input: RuleFixedGenerateInput): Promise<RuleFixedGenerateResult>;
  precheckRuleLive(input: RuleLivePrecheckInput): Promise<RuleLivePrecheckResult>;
};

export type PaperAssemblyAPI = PaperAPI;

type PaperAPIResponse = {
  id: number;
  tenant_id: number;
  space_id?: number | null;
  name: string;
  description?: string;
  total_score: string;
  build_mode: string;
  status: "draft" | "enabled" | "disabled";
};

type PaperSectionAPIResponse = {
  id: number;
  tenant_id: number;
  paper_id: number;
  sort_order: number;
  name: string;
  question_type: string;
  instructions: string;
  total_score: string;
  question_count: number;
};

type ManualQuestionAPIResponse = {
  tenant_id: number;
  paper_id: number;
  section_id: number;
  question_id: number;
  sort_order: number;
  score: string;
};

type PaperRuleAPIResponse = {
  id: number;
  tenant_id: number;
  paper_id: number;
  section_id: number;
  sort_order: number;
  difficulty?: string | null;
  tag_ids: number[];
  question_count: number;
  score_per_question: string;
  shuffle_options?: boolean | null;
};

type RuleFixedGenerateAPIResponse = {
  paper_id: number;
  generated: boolean;
};

type RuleLivePrecheckAPIResponse = {
  candidate_question_ids: number[];
  candidate_count: number;
};

const defaultApiClient = createApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? "",
  getAccessToken: readStoredAccessToken,
});

export const paperApi = createPaperAPI(defaultApiClient);

export function createPaperAPI(apiClient: ApiClient): PaperAPI {
  return {
    async listPapers(input) {
      const params = new URLSearchParams({ tenant_id: String(input.tenantID) });
      if (input.spaceID !== undefined) {
        params.set("space_id", String(input.spaceID));
      }
      const data = await apiClient.get<PageData<PaperAPIResponse>>(`/api/v1/papers?${params.toString()}`);
      return { items: data.items.map(mapPaperResponse) };
    },
    async createPaper(input) {
      const data = await apiClient.post<PaperAPIResponse>("/api/v1/papers", {
        tenant_id: input.tenantID,
        ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }),
        name: input.name,
        description: input.description ?? "",
        shuffle_questions: input.shuffleQuestions ?? false,
        show_analysis: input.showAnalysis ?? false,
      });
      return mapPaperResponse(data);
    },
    async deletePaper(input) {
      await apiClient.request(`/api/v1/papers/${input.paperID}?tenant_id=${input.tenantID}`, { method: "DELETE" });
    },
    async listSections(input) {
      const data = await apiClient.get<PageData<PaperSectionAPIResponse>>(
        `/api/v1/papers/${input.paperID}/sections?tenant_id=${input.tenantID}`,
      );
      return { items: data.items.map(mapPaperSectionResponse) };
    },
    async createSection(input) {
      const data = await apiClient.post<PaperSectionAPIResponse>(`/api/v1/papers/${input.paperID}/sections`, {
        tenant_id: input.tenantID,
        name: input.name,
        question_type: input.questionType,
        instructions: input.instructions,
      });
      return mapPaperSectionResponse(data);
    },
    async addManualQuestion(input) {
      const data = await apiClient.post<ManualQuestionAPIResponse>(
        `/api/v1/papers/${input.paperID}/sections/${input.sectionID}/questions`,
        {
          tenant_id: input.tenantID,
          question_id: input.questionID,
          score: input.score,
        },
      );
      return mapManualQuestionResponse(data);
    },
    async listRules(input) {
      const data = await apiClient.get<PageData<PaperRuleAPIResponse>>(
        `/api/v1/papers/${input.paperID}/rules?tenant_id=${input.tenantID}`,
      );
      return { items: data.items.map(mapPaperRuleResponse) };
    },
    async createRule(input) {
      const data = await apiClient.post<PaperRuleAPIResponse>(
        `/api/v1/papers/${input.paperID}/sections/${input.sectionID}/rules`,
        {
          tenant_id: input.tenantID,
          sort_order: input.sortOrder,
          ...(input.difficulty === undefined ? {} : { difficulty: input.difficulty }),
          tag_ids: input.tagIDs,
          question_count: input.questionCount,
          score_per_question: input.scorePerQuestion,
          ...(input.shuffleOptions === undefined ? {} : { shuffle_options: input.shuffleOptions }),
        },
      );
      return mapPaperRuleResponse(data);
    },
    async generateRuleFixed(input) {
      const data = await apiClient.post<RuleFixedGenerateAPIResponse>(
        `/api/v1/papers/${input.paperID}/rule-fixed/generate`,
        { tenant_id: input.tenantID },
      );
      return {
        paperID: data.paper_id,
        generated: data.generated,
      };
    },
    async precheckRuleLive(input) {
      const data = await apiClient.post<RuleLivePrecheckAPIResponse>(
        `/api/v1/papers/${input.paperID}/rule-live/precheck`,
        { tenant_id: input.tenantID },
      );
      return {
        candidateQuestionIDs: data.candidate_question_ids,
        candidateCount: data.candidate_count,
      };
    },
  };
}

function mapPaperResponse(row: PaperAPIResponse): PaperRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    ...(row.space_id === undefined || row.space_id === null ? {} : { spaceID: row.space_id }),
    name: row.name,
    description: row.description ?? "",
    buildMode: row.build_mode,
    status: row.status === "draft" ? "draft" : "ready",
    totalScore: row.total_score,
  };
}

function mapPaperSectionResponse(row: PaperSectionAPIResponse): PaperSectionRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    paperID: row.paper_id,
    sortOrder: row.sort_order,
    name: row.name,
    questionType: row.question_type,
    instructions: row.instructions,
    totalScore: row.total_score,
    questionCount: row.question_count,
  };
}

function mapManualQuestionResponse(row: ManualQuestionAPIResponse): ManualQuestionRow {
  return {
    tenantID: row.tenant_id,
    paperID: row.paper_id,
    sectionID: row.section_id,
    questionID: row.question_id,
    sortOrder: row.sort_order,
    score: row.score,
  };
}

function mapPaperRuleResponse(row: PaperRuleAPIResponse): PaperRuleRow {
  return {
    id: row.id,
    tenantID: row.tenant_id,
    paperID: row.paper_id,
    sectionID: row.section_id,
    sortOrder: row.sort_order,
    ...(row.difficulty === undefined || row.difficulty === null ? {} : { difficulty: row.difficulty }),
    tagIDs: row.tag_ids,
    questionCount: row.question_count,
    scorePerQuestion: row.score_per_question,
    ...(row.shuffle_options === undefined || row.shuffle_options === null ? {} : { shuffleOptions: row.shuffle_options }),
  };
}
