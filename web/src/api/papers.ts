import { createApiClient } from "./client";
import type { ApiClient, PageData } from "./client";
import type { QuestionType } from "./questions";

export type PaperRow = {
  id: number;
  tenantID: number;
  spaceID?: number;
  name: string;
  description?: string;
  durationMinutes?: number;
  gradeLevel?: string;
  buildMode: string;
  status: "draft" | "enabled" | "disabled";
  totalScore: string;
  createdAt: number;
  creatorName: string;
};

export type PaperBuildMode = "manual" | "rule_fixed" | "rule_live";
export type SmartQuestionScope = "space_all" | "tag_filter";
export type SmartDifficultyPercentages = {
  easy: number;
  medium: number;
  hard: number;
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
  spaceID?: number | null;
  name: string;
  description?: string;
  durationMinutes?: number;
  gradeLevel?: string;
  shuffleQuestions?: boolean;
  showAnalysis?: boolean;
};

export type DeletePaperInput = {
  tenantID: number;
  paperID: number;
};

export type DeletePaperSectionInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
};

export type UpdatePaperInput = {
  tenantID: number;
  paperID: number;
  spaceID?: number | null;
  name: string;
  description?: string;
  durationMinutes?: number;
  gradeLevel?: string;
};

export type DisablePaperInput = {
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

export type SectionOrderInput = {
  sectionID: number;
  sortOrder: number;
};

export type ReorderPaperSectionsInput = {
  tenantID: number;
  paperID: number;
  orders: SectionOrderInput[];
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
  questionType?: QuestionType;
  title?: string;
  options?: string[];
  blankCount?: number;
};

export type SectionQuestionListResult = {
  items: ManualQuestionRow[];
};

export type PaperRuleRow = {
  id: number;
  tenantID: number;
  paperID: number;
  sectionID: number;
  sortOrder: number;
  difficulty?: string;
  tagIDs: number[];
  tagNames?: string[];
  questionScope?: SmartQuestionScope;
  difficultyPercentages?: SmartDifficultyPercentages;
  questionCount: number;
  scorePerQuestion: string;
  shuffleOptions?: boolean;
  prioritizeQuality?: boolean;
  excludeRecentExamQuestions?: boolean;
  excludeUsedQuestions?: boolean;
};

export type CreatePaperRuleInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  sortOrder: number;
  difficulty?: string;
  tagIDs: number[];
  tagNames?: string[];
  questionScope?: SmartQuestionScope;
  difficultyPercentages?: SmartDifficultyPercentages;
  questionCount: number;
  scorePerQuestion: string;
  shuffleOptions?: boolean;
  prioritizeQuality?: boolean;
  excludeRecentExamQuestions?: boolean;
  excludeUsedQuestions?: boolean;
};

export type UpdatePaperRuleInput = CreatePaperRuleInput & {
  ruleID: number;
};

export type UpdateSectionQuestionInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  questionID: number;
  sortOrder: number;
  score: string;
};

export type ReplaceSectionQuestionInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  questionID: number;
  newQuestionID: number;
  sortOrder: number;
  score: string;
};

export type DeleteSectionQuestionInput = {
  tenantID: number;
  paperID: number;
  sectionID: number;
  questionID: number;
};

export type DeletePaperRuleInput = {
  tenantID: number;
  paperID: number;
  ruleID: number;
};

export type UpdatePaperBuildModeInput = {
  tenantID: number;
  paperID: number;
  buildMode: PaperBuildMode;
};

export type RuleFixedGenerateInput = {
  tenantID: number;
  paperID: number;
  blockedQuestionIDs?: number[];
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
  updatePaper(input: UpdatePaperInput): Promise<PaperRow>;
  enablePaper(input: DisablePaperInput): Promise<PaperRow>;
  disablePaper(input: DisablePaperInput): Promise<PaperRow>;
  deletePaper(input: DeletePaperInput): Promise<void>;
  listSections(input: ListPaperSectionsInput): Promise<PaperSectionListResult>;
  createSection(input: CreatePaperSectionInput): Promise<PaperSectionRow>;
  reorderSections(input: ReorderPaperSectionsInput): Promise<void>;
  deleteSection(input: DeletePaperSectionInput): Promise<void>;
  addManualQuestion(input: AddManualQuestionInput): Promise<ManualQuestionRow>;
  listSectionQuestions(input: ListPaperSectionsInput): Promise<SectionQuestionListResult>;
  updateSectionQuestion(input: UpdateSectionQuestionInput): Promise<ManualQuestionRow>;
  replaceSectionQuestion(input: ReplaceSectionQuestionInput): Promise<ManualQuestionRow>;
  deleteSectionQuestion(input: DeleteSectionQuestionInput): Promise<void>;
  listRules(input: ListPaperSectionsInput): Promise<PaperRuleListResult>;
  createRule(input: CreatePaperRuleInput): Promise<PaperRuleRow>;
  updateRule(input: UpdatePaperRuleInput): Promise<PaperRuleRow>;
  deleteRule(input: DeletePaperRuleInput): Promise<void>;
  generateRuleFixed(input: RuleFixedGenerateInput): Promise<RuleFixedGenerateResult>;
  precheckRuleLive(input: RuleLivePrecheckInput): Promise<RuleLivePrecheckResult>;
  updateBuildMode(input: UpdatePaperBuildModeInput): Promise<PaperRow>;
};

export type PaperAssemblyAPI = PaperAPI;

type PaperAPIResponse = {
  id: number;
  tenant_id: number;
  space_id?: number | null;
  name: string;
  description?: string;
  duration_minutes?: number;
  grade_level?: string;
  total_score: string;
  build_mode: string;
  status: "draft" | "enabled" | "disabled";
  created_at: number;
  creator_name?: string;
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
  question_type?: QuestionType;
  title?: string;
  options?: string[];
  blank_count?: number;
};

type PaperRuleAPIResponse = {
  id: number;
  tenant_id: number;
  paper_id: number;
  section_id: number;
  sort_order: number;
  difficulty?: string | null;
  tag_ids: number[];
  tag_names?: string[];
  question_scope?: SmartQuestionScope;
  difficulty_percentages?: SmartDifficultyPercentages;
  question_count: number;
  score_per_question: string;
  shuffle_options?: boolean | null;
  prioritize_quality?: boolean;
  exclude_recent_exam_questions?: boolean;
  exclude_used_questions?: boolean;
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
        ...(input.spaceID == null ? {} : { space_id: input.spaceID }),
        name: input.name,
        description: input.description ?? "",
        ...(input.durationMinutes === undefined ? {} : { duration_minutes: input.durationMinutes }),
        ...(input.gradeLevel === undefined ? {} : { grade_level: input.gradeLevel }),
        shuffle_questions: input.shuffleQuestions ?? false,
        show_analysis: input.showAnalysis ?? false,
      });
      return mapPaperResponse(data);
    },
    async updatePaper(input) {
      const data = await apiClient.request<PaperAPIResponse>(`/api/v1/papers/${input.paperID}`, {
        method: "PUT",
        body: {
          tenant_id: input.tenantID,
          ...(input.spaceID === undefined ? {} : { space_id: input.spaceID }),
          name: input.name,
          description: input.description ?? "",
          ...(input.durationMinutes === undefined ? {} : { duration_minutes: input.durationMinutes }),
          ...(input.gradeLevel === undefined ? {} : { grade_level: input.gradeLevel }),
        },
      });
      return mapPaperResponse(data);
    },
    async enablePaper(input) {
      const data = await apiClient.post<PaperAPIResponse>(`/api/v1/papers/${input.paperID}/enable`, {
        tenant_id: input.tenantID,
      });
      return mapPaperResponse(data);
    },
    async disablePaper(input) {
      const data = await apiClient.post<PaperAPIResponse>(`/api/v1/papers/${input.paperID}/disable`, {
        tenant_id: input.tenantID,
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
    async reorderSections(input) {
      await apiClient.request(`/api/v1/papers/${input.paperID}/sections/reorder`, {
        method: "PUT",
        body: {
          tenant_id: input.tenantID,
          orders: input.orders.map((item) => ({
            section_id: item.sectionID,
            sort_order: item.sortOrder,
          })),
        },
      });
    },
    async deleteSection(input) {
      await apiClient.request(
        `/api/v1/papers/${input.paperID}/sections/${input.sectionID}?tenant_id=${input.tenantID}`,
        { method: "DELETE" },
      );
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
    async listSectionQuestions(input) {
      const data = await apiClient.get<PageData<ManualQuestionAPIResponse>>(
        `/api/v1/papers/${input.paperID}/questions?tenant_id=${input.tenantID}`,
      );
      return { items: data.items.map(mapManualQuestionResponse) };
    },
    async updateSectionQuestion(input) {
      const data = await apiClient.request<ManualQuestionAPIResponse>(
        `/api/v1/papers/${input.paperID}/sections/${input.sectionID}/questions/${input.questionID}`,
        {
          method: "PUT",
          body: {
            tenant_id: input.tenantID,
            sort_order: input.sortOrder,
            score: input.score,
          },
        },
      );
      return mapManualQuestionResponse(data);
    },
    async replaceSectionQuestion(input) {
      const data = await apiClient.post<ManualQuestionAPIResponse>(
        `/api/v1/papers/${input.paperID}/sections/${input.sectionID}/questions/${input.questionID}/replace`,
        {
          tenant_id: input.tenantID,
          new_question_id: input.newQuestionID,
          sort_order: input.sortOrder,
          score: input.score,
        },
      );
      return mapManualQuestionResponse(data);
    },
    async deleteSectionQuestion(input) {
      await apiClient.request(
        `/api/v1/papers/${input.paperID}/sections/${input.sectionID}/questions/${input.questionID}?tenant_id=${input.tenantID}`,
        { method: "DELETE" },
      );
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
          ...(input.tagNames === undefined ? {} : { tag_names: input.tagNames }),
          ...(input.questionScope === undefined ? {} : { question_scope: input.questionScope }),
          ...(input.difficultyPercentages === undefined ? {} : { difficulty_percentages: input.difficultyPercentages }),
          question_count: input.questionCount,
          score_per_question: input.scorePerQuestion,
          ...(input.shuffleOptions === undefined ? {} : { shuffle_options: input.shuffleOptions }),
          ...(input.prioritizeQuality === undefined ? {} : { prioritize_quality: input.prioritizeQuality }),
          ...(input.excludeRecentExamQuestions === undefined ? {} : { exclude_recent_exam_questions: input.excludeRecentExamQuestions }),
          ...(input.excludeUsedQuestions === undefined ? {} : { exclude_used_questions: input.excludeUsedQuestions }),
        },
      );
      return mapPaperRuleResponse(data);
    },
    async updateRule(input) {
      const data = await apiClient.request<PaperRuleAPIResponse>(`/api/v1/papers/${input.paperID}/rules/${input.ruleID}`, {
        method: "PUT",
        body: {
          tenant_id: input.tenantID,
          section_id: input.sectionID,
          sort_order: input.sortOrder,
          ...(input.difficulty === undefined ? {} : { difficulty: input.difficulty }),
          tag_ids: input.tagIDs,
          ...(input.tagNames === undefined ? {} : { tag_names: input.tagNames }),
          ...(input.questionScope === undefined ? {} : { question_scope: input.questionScope }),
          ...(input.difficultyPercentages === undefined ? {} : { difficulty_percentages: input.difficultyPercentages }),
          question_count: input.questionCount,
          score_per_question: input.scorePerQuestion,
          ...(input.shuffleOptions === undefined ? {} : { shuffle_options: input.shuffleOptions }),
          ...(input.prioritizeQuality === undefined ? {} : { prioritize_quality: input.prioritizeQuality }),
          ...(input.excludeRecentExamQuestions === undefined ? {} : { exclude_recent_exam_questions: input.excludeRecentExamQuestions }),
          ...(input.excludeUsedQuestions === undefined ? {} : { exclude_used_questions: input.excludeUsedQuestions }),
        },
      });
      return mapPaperRuleResponse(data);
    },
    async deleteRule(input) {
      await apiClient.request(`/api/v1/papers/${input.paperID}/rules/${input.ruleID}?tenant_id=${input.tenantID}`, {
        method: "DELETE",
      });
    },
    async generateRuleFixed(input) {
      const data = await apiClient.post<RuleFixedGenerateAPIResponse>(
        `/api/v1/papers/${input.paperID}/rule-fixed/generate`,
        {
          tenant_id: input.tenantID,
          ...(input.blockedQuestionIDs === undefined || input.blockedQuestionIDs.length === 0
            ? {}
            : { blocked_question_ids: input.blockedQuestionIDs }),
        },
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
    async updateBuildMode(input) {
      const data = await apiClient.request<PaperAPIResponse>(`/api/v1/papers/${input.paperID}/mode`, {
        method: "PUT",
        body: {
          tenant_id: input.tenantID,
          build_mode: input.buildMode,
        },
      });
      return mapPaperResponse(data);
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
    durationMinutes: row.duration_minutes ?? 120,
    ...(row.grade_level === undefined ? {} : { gradeLevel: row.grade_level }),
    buildMode: row.build_mode,
    status: row.status,
    totalScore: row.total_score,
    createdAt: row.created_at,
    creatorName: row.creator_name ?? "",
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
    ...(row.question_type === undefined ? {} : { questionType: row.question_type }),
    ...(row.title === undefined ? {} : { title: row.title }),
    ...(row.options === undefined ? {} : { options: row.options }),
    ...(row.blank_count === undefined ? {} : { blankCount: row.blank_count }),
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
    ...(row.tag_names === undefined ? {} : { tagNames: row.tag_names }),
    ...(row.question_scope === undefined ? {} : { questionScope: row.question_scope }),
    ...(row.difficulty_percentages === undefined ? {} : { difficultyPercentages: row.difficulty_percentages }),
    questionCount: row.question_count,
    scorePerQuestion: row.score_per_question,
    ...(row.shuffle_options === undefined || row.shuffle_options === null ? {} : { shuffleOptions: row.shuffle_options }),
    ...(row.prioritize_quality === undefined ? {} : { prioritizeQuality: row.prioritize_quality }),
    ...(row.exclude_recent_exam_questions === undefined ? {} : { excludeRecentExamQuestions: row.exclude_recent_exam_questions }),
    ...(row.exclude_used_questions === undefined ? {} : { excludeUsedQuestions: row.exclude_used_questions }),
  };
}
