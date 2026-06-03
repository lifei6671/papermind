import { Eye, FilePlus2, GripVertical, Trash2 } from "lucide-react";
import type { CSSProperties } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { formatApiErrorMessage } from "../../api/client";
import type {
  ManualQuestionRow,
  PaperAPI,
  PaperBuildMode,
  PaperRuleRow,
  PaperRow,
  PaperSectionRow,
  SmartDifficultyPercentages,
  SmartQuestionScope,
} from "../../api/papers";
import { paperApi } from "../../api/papers";
import { useFeedback } from "../../app/feedback-context";
import type { QuestionAPI, QuestionDifficulty, QuestionRow, QuestionType } from "../../api/questions";
import { questionApi } from "../../api/questions";
import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Panel } from "../../components/ui/Panel";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { Select } from "../../components/ui/Select";
import { SegmentTabs } from "../../components/ui/SegmentTabs";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { Tooltip } from "../../components/ui/Tooltip";

type PaperEditorPageProps = {
  paperApi?: PaperAPI;
  questionApi?: QuestionAPI;
  paperID?: number;
  tenantID?: number;
  spaceID?: number;
};

type SelectedQuestionGroup = {
  section: PaperSectionRow;
  questions: Array<ManualQuestionRow & { question: QuestionRow | null }>;
};

type FilterSelectOption = {
  value: string;
  label: string;
};

type SmartRuleDraft = {
  ruleID?: number;
  questionCount: string;
  scorePerQuestion: string;
};

const questionTypeLabels: Record<QuestionType, string> = {
  single: "选择题",
  multiple: "多选题",
  judge: "判断题",
  fill_blank: "填空题",
  short_text: "简答题",
};

const difficultyLabels: Record<QuestionDifficulty, string> = {
  easy: "较易",
  medium: "中等",
  hard: "较难",
};

const defaultDifficultyPercentages: SmartDifficultyPercentages = {
  easy: 30,
  medium: 50,
  hard: 20,
};

const sectionTemplates: Record<QuestionType, { name: string; instructions: string }> = {
  single: { name: "一、单项选择题", instructions: "每题 5 分" },
  multiple: { name: "二、多项选择题", instructions: "每题 5 分" },
  judge: { name: "三、判断题", instructions: "每题 2 分" },
  fill_blank: { name: "四、填空题", instructions: "每题 5 分" },
  short_text: { name: "五、解答题", instructions: "按采分点给分" },
};

const AUTO_SAVE_DRAFT_DELAY_MS = 30_000;

type DraggedQuestion = {
  sectionID: number;
  questionID: number;
};

type DropIndicatorDirection = "up" | "down";

export function PaperEditorPage({
  paperApi: providedPaperApi = paperApi,
  questionApi: providedQuestionApi = questionApi,
  paperID,
  tenantID = 10,
  spaceID,
}: PaperEditorPageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const { showError, showSuccess } = useFeedback();
  const pageSearch = scopedPaperSearch(location.search, spaceID);
  const listPath = `/papers${pageSearch}`;
  const [paper, setPaper] = useState<PaperRow | null>(null);
  const [sections, setSections] = useState<PaperSectionRow[]>([]);
  const [persistedSections, setPersistedSections] = useState<PaperSectionRow[]>([]);
  const [sectionQuestions, setSectionQuestions] = useState<ManualQuestionRow[]>([]);
  const [persistedSectionQuestions, setPersistedSectionQuestions] = useState<ManualQuestionRow[]>([]);
  const [questionPool, setQuestionPool] = useState<QuestionRow[]>([]);
  const [paperName, setPaperName] = useState("");
  const [paperDescription, setPaperDescription] = useState("");
  const [gradeText, setGradeText] = useState("高一");
  const [durationText, setDurationText] = useState("120");
  const [buildMode, setBuildMode] = useState<PaperBuildMode>("manual");
  const [smartQuestionScope, setSmartQuestionScope] = useState<SmartQuestionScope>("space_all");
  const [smartDifficultyPercentages, setSmartDifficultyPercentages] = useState<SmartDifficultyPercentages>(defaultDifficultyPercentages);
  const [smartSelectedTags, setSmartSelectedTags] = useState<string[]>([]);
  const [smartPrioritizeQuality, setSmartPrioritizeQuality] = useState(true);
  const [smartExcludeRecentExamQuestions, setSmartExcludeRecentExamQuestions] = useState(true);
  const [smartExcludeUsedQuestions, setSmartExcludeUsedQuestions] = useState(true);
  const [blockedSmartQuestionIDs, setBlockedSmartQuestionIDs] = useState<number[]>([]);
  const [smartRuleDrafts, setSmartRuleDrafts] = useState<Record<number, SmartRuleDraft>>({});
  const [persistedSmartRuleSnapshot, setPersistedSmartRuleSnapshot] = useState("");
  const [isAddingSmartSection, setIsAddingSmartSection] = useState(false);
  const [newSmartSectionType, setNewSmartSectionType] = useState<QuestionType>("single");
  const [newSmartSectionName, setNewSmartSectionName] = useState(defaultSectionName("single"));
  const [isCreatingSmartSection, setIsCreatingSmartSection] = useState(false);
  const [smartTagQuery, setSmartTagQuery] = useState("");
  const [isSmartTagPickerOpen, setIsSmartTagPickerOpen] = useState(false);
  const [selectedType, setSelectedType] = useState<"all" | QuestionType>("all");
  const [selectedDifficulty, setSelectedDifficulty] = useState<"all" | QuestionDifficulty>("all");
  const [selectedTag, setSelectedTag] = useState("all");
  const [page, setPage] = useState(1);
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);
  const [isSavingDraft, setIsSavingDraft] = useState(false);
  const [isRefreshingWorkspace, setIsRefreshingWorkspace] = useState(false);
  const [draggingSectionID, setDraggingSectionID] = useState<number | null>(null);
  const [draggingQuestion, setDraggingQuestion] = useState<DraggedQuestion | null>(null);
  const [sectionDropTargetID, setSectionDropTargetID] = useState<number | null>(null);
  const [sectionDropDirection, setSectionDropDirection] = useState<DropIndicatorDirection | null>(null);
  const [questionDropTargetKey, setQuestionDropTargetKey] = useState<string | null>(null);
  const [questionDropDirection, setQuestionDropDirection] = useState<DropIndicatorDirection | null>(null);
  const [editingScores, setEditingScores] = useState<Record<string, string>>({});

  const loadQuestionPool = useCallback(async () => {
    const pageSize = 100;
    const allItems: QuestionRow[] = [];
    let page = 1;
    let loadedCount = 0;
    let total = 0;

    while (page === 1 || loadedCount < total) {
      const data = await providedQuestionApi.listQuestions({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        page,
        pageSize,
      });
      total = data.total;
      loadedCount += data.items.length;
      allItems.push(...data.items);
      if (data.items.length === 0) {
        break;
      }
      page += 1;
    }

    return allItems.filter((item) => item.status === "ready");
  }, [providedQuestionApi, spaceID, tenantID]);

  const loadWorkspaceSnapshot = useCallback(async () => {
    if (paperID === undefined) {
      return {
        currentPaper: null,
        nextSections: [],
        nextSectionQuestions: [],
        nextRules: [],
      };
    }

    const [paperData, sectionData, sectionQuestionData, ruleData] = await Promise.all([
      providedPaperApi.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) }),
      providedPaperApi.listSections({ tenantID, paperID }),
      providedPaperApi.listSectionQuestions({ tenantID, paperID }),
      providedPaperApi.listRules({ tenantID, paperID }),
    ]);
    return {
      currentPaper: paperData.items.find((item) => item.id === paperID) ?? null,
      nextSections: sectionData.items,
      nextSectionQuestions: sectionQuestionData.items,
      nextRules: ruleData.items,
    };
  }, [paperID, providedPaperApi, spaceID, tenantID]);

  const applySmartRuleState = useCallback((
    nextSections: PaperSectionRow[],
    nextSectionQuestions: ManualQuestionRow[],
    nextRules: PaperRuleRow[],
  ) => {
    const firstRule = nextRules[0];
    const nextQuestionScope = firstRule?.questionScope ?? "space_all";
    const nextDifficultyPercentages = firstRule?.difficultyPercentages ?? defaultDifficultyPercentages;
    const nextSelectedTags = firstRule?.tagNames ?? [];
    const nextPrioritizeQuality = firstRule?.prioritizeQuality ?? true;
    const nextExcludeRecentExamQuestions = firstRule?.excludeRecentExamQuestions ?? true;
    const nextExcludeUsedQuestions = firstRule?.excludeUsedQuestions ?? true;
    const nextRuleDrafts = buildSmartRuleDrafts(nextSections, nextSectionQuestions, nextRules);

    setSmartQuestionScope(nextQuestionScope);
    setSmartDifficultyPercentages(nextDifficultyPercentages);
    setSmartSelectedTags(nextSelectedTags);
    setSmartTagQuery("");
    setIsSmartTagPickerOpen(false);
    setSmartPrioritizeQuality(nextPrioritizeQuality);
    setSmartExcludeRecentExamQuestions(nextExcludeRecentExamQuestions);
    setSmartExcludeUsedQuestions(nextExcludeUsedQuestions);
    setSmartRuleDrafts(nextRuleDrafts);
    setPersistedSmartRuleSnapshot(serializeSmartRuleState({
      sections: nextSections,
      ruleDrafts: nextRuleDrafts,
      questionScope: nextQuestionScope,
      selectedTags: nextSelectedTags,
      difficultyPercentages: nextDifficultyPercentages,
      prioritizeQuality: nextPrioritizeQuality,
      excludeRecentExamQuestions: nextExcludeRecentExamQuestions,
      excludeUsedQuestions: nextExcludeUsedQuestions,
    }));
  }, []);

  const applyWorkspaceSnapshot = useCallback((
    currentPaper: PaperRow | null,
    nextSections: PaperSectionRow[],
    nextSectionQuestions: ManualQuestionRow[],
    nextRules: PaperRuleRow[],
    nextQuestionPool: QuestionRow[],
  ) => {
    setQuestionPool(nextQuestionPool);
    setPaper(currentPaper);
    setPaperName(currentPaper?.name ?? "");
    setPaperDescription(currentPaper?.description ?? "");
    setGradeText(currentPaper?.gradeLevel?.trim() ? currentPaper.gradeLevel.trim() : "高一");
    setDurationText(String(currentPaper?.durationMinutes ?? 120));
    setBuildMode((currentPaper?.buildMode as PaperBuildMode | undefined) ?? "manual");
    applySmartRuleState(nextSections, nextSectionQuestions, nextRules);
    setSections(nextSections.map((item) => ({ ...item })));
    setPersistedSections(nextSections.map((item) => ({ ...item })));
    setSectionQuestions(nextSectionQuestions.map(cloneSectionQuestion));
    setPersistedSectionQuestions(nextSectionQuestions.map(cloneSectionQuestion));
    setEditingScores({});
    setBlockedSmartQuestionIDs([]);
  }, [applySmartRuleState]);

  useEffect(() => {
    let ignore = false;

    void (async () => {
      try {
        const [nextQuestionPool, snapshot] = await Promise.all([
          loadQuestionPool(),
          loadWorkspaceSnapshot(),
        ]);
        if (ignore) {
          return;
        }
        applyWorkspaceSnapshot(
          snapshot.currentPaper,
          snapshot.nextSections,
          snapshot.nextSectionQuestions,
          snapshot.nextRules,
          nextQuestionPool,
        );
      } catch (error) {
        if (!ignore) {
          showError(formatApiErrorMessage(error, paperID === undefined ? "题库候选题加载失败" : "试卷工作台加载失败"));
        }
      }
    })();

    return () => {
      ignore = true;
    };
  }, [applyWorkspaceSnapshot, loadQuestionPool, loadWorkspaceSnapshot, paperID, showError]);

  const questionMap = useMemo(() => new Map(questionPool.map((item) => [item.id, item])), [questionPool]);
  const blockedSmartQuestions = useMemo(() => blockedSmartQuestionIDs.map((questionID) => {
    const question = questionMap.get(questionID);

    return {
      id: questionID,
      title: question?.title ?? `题目 ${questionID}`,
      tag: question?.tag ?? "",
      tags: question?.tags ?? [],
    };
  }), [blockedSmartQuestionIDs, questionMap]);
  const tags = useMemo(() => {
    const values = Array.from(new Set(questionPool.flatMap((item) => item.tags.length > 0 ? item.tags : [item.tag]).filter(Boolean)));
    values.sort((left, right) => left.localeCompare(right, "zh-Hans-CN"));
    return values;
  }, [questionPool]);
  const smartTagOptions = useMemo(() => {
    const query = smartTagQuery.trim().toLocaleLowerCase();
    return tags.filter((tag) => !smartSelectedTags.includes(tag) && (query.length === 0 || tag.toLocaleLowerCase().includes(query)));
  }, [smartSelectedTags, smartTagQuery, tags]);

  function selectSmartTag(tag: string) {
    setSmartSelectedTags((items) => items.includes(tag) ? items : [...items, tag]);
    setSmartTagQuery("");
    setIsSmartTagPickerOpen(false);
  }

  function removeSmartTag(tag: string) {
    setSmartSelectedTags((items) => items.filter((item) => item !== tag));
  }

  function updateEasyBoundary(value: number) {
    setSmartDifficultyPercentages((current) => {
      const mediumEnd = current.easy + current.medium;
      const easy = clamp(value, 0, mediumEnd);
      return {
        easy,
        medium: mediumEnd - easy,
        hard: 100 - mediumEnd,
      };
    });
  }

  function updateMediumBoundary(value: number) {
    setSmartDifficultyPercentages((current) => {
      const easy = current.easy;
      const mediumEnd = clamp(value, easy, 100);
      return {
        easy,
        medium: mediumEnd - easy,
        hard: 100 - mediumEnd,
      };
    });
  }

  function handleNewSmartSectionTypeChange(nextType: QuestionType) {
    setNewSmartSectionType(nextType);
    setNewSmartSectionName(defaultSectionName(nextType));
  }

  async function handleCreateSmartSection() {
    if (paper === null) {
      showError("请先保存试卷基础信息后再添加题型");
      return;
    }
    const name = newSmartSectionName.trim();
    if (!name) {
      showError("题型名称不能为空");
      return;
    }
    const template = sectionTemplates[newSmartSectionType];
    setIsCreatingSmartSection(true);
    try {
      const created = await providedPaperApi.createSection({
        tenantID,
        paperID: paper.id,
        name,
        questionType: newSmartSectionType,
        instructions: template.instructions,
      });
      setSections((items) => normalizeSectionOrders([...items, created]));
      setPersistedSections((items) => [...items, cloneSection(created)]);
      setSmartRuleDrafts((items) => ({
        ...items,
        [created.id]: defaultRuleDraftForSection(created, sectionQuestions),
      }));
      setIsAddingSmartSection(false);
      setNewSmartSectionType("single");
      setNewSmartSectionName(defaultSectionName("single"));
    } catch (error) {
      showError(formatApiErrorMessage(error, "新增题型失败"));
    } finally {
      setIsCreatingSmartSection(false);
    }
  }

  async function handleDeleteSmartSection(section: PaperSectionRow) {
    if (paper === null) {
      return;
    }
    try {
      await providedPaperApi.deleteSection({
        tenantID,
        paperID: paper.id,
        sectionID: section.id,
      });
      const nextSections = normalizeSectionOrders(sections.filter((item) => item.id !== section.id));
      setSections(nextSections);
      setPersistedSections((items) => items.filter((item) => item.id !== section.id));
      setSectionQuestions((items) => items.filter((item) => item.sectionID !== section.id));
      setPersistedSectionQuestions((items) => items.filter((item) => item.sectionID !== section.id));
      setSmartRuleDrafts((items) => {
        const next = { ...items };
        delete next[section.id];
        return next;
      });
    } catch (error) {
      showError(formatApiErrorMessage(error, "删除题型失败"));
    }
  }

  const filteredQuestions = questionPool.filter((item) => {
    if (selectedType !== "all" && item.type !== selectedType) {
      return false;
    }
    if (selectedDifficulty !== "all" && item.difficulty !== selectedDifficulty) {
      return false;
    }
    if (selectedTag !== "all" && !(item.tags.length > 0 ? item.tags : [item.tag]).includes(selectedTag)) {
      return false;
    }
    return true;
  });

  const pagedQuestions = filteredQuestions.slice((page - 1) * 10, page * 10);
  const groupedSelectedQuestions = buildSelectedGroups(sections, sectionQuestions, questionMap);
  const selectedQuestionCount = sectionQuestions.length;
  const totalScore = sectionQuestions.reduce((sum, item) => sum + Number(item.score || "0"), 0);
  const easyBoundary = clamp(smartDifficultyPercentages.easy, 0, 100);
  const mediumBoundary = clamp(smartDifficultyPercentages.easy + smartDifficultyPercentages.medium, easyBoundary, 100);
  const difficultySliderStyle = {
    "--easy-percent": `${easyBoundary}%`,
    "--medium-end-percent": `${mediumBoundary}%`,
  } as CSSProperties;
  const canAssemble = paper !== null;
  const isRuleLivePaper = buildMode === "rule_live";
  const isSmartPaper = buildMode === "rule_fixed";
  const canChangeBuildMode = paper === null;
  const sectionOrderDirty = serializeSections(sections) !== serializeSections(persistedSections);
  const currentSmartRuleSnapshot = useMemo(() => serializeSmartRuleState({
    sections,
    ruleDrafts: smartRuleDrafts,
    questionScope: smartQuestionScope,
    selectedTags: smartSelectedTags,
    difficultyPercentages: smartDifficultyPercentages,
    prioritizeQuality: smartPrioritizeQuality,
    excludeRecentExamQuestions: smartExcludeRecentExamQuestions,
    excludeUsedQuestions: smartExcludeUsedQuestions,
  }), [
    sections,
    smartDifficultyPercentages,
    smartExcludeRecentExamQuestions,
    smartExcludeUsedQuestions,
    smartPrioritizeQuality,
    smartQuestionScope,
    smartRuleDrafts,
    smartSelectedTags,
  ]);
  const smartRulesDirty = isSmartPaper && currentSmartRuleSnapshot !== persistedSmartRuleSnapshot;
  const draftDirty = useMemo(() => {
    if (paper === null) {
      return paperName.trim().length > 0
        || paperDescription.trim().length > 0
        || gradeText.trim() !== "高一"
        || durationText.trim() !== "120";
    }
    return paperName !== paper.name
      || paperDescription !== (paper.description ?? "")
      || gradeText !== (paper.gradeLevel?.trim() ? paper.gradeLevel.trim() : "高一")
      || durationText !== String(paper.durationMinutes ?? 120)
      || sectionOrderDirty
      || smartRulesDirty
      || serializeSectionQuestions(sectionQuestions) !== serializeSectionQuestions(persistedSectionQuestions);
  }, [durationText, gradeText, paper, paperDescription, paperName, persistedSectionQuestions, sectionOrderDirty, sectionQuestions, smartRulesDirty]);
  const lastSavedLabel = paper === null
    ? "草稿尚未保存"
    : draftDirty ? "草稿有未保存调整" : "草稿已保存";

  const persistSmartRules = useCallback(async (targetPaperID: number) => {
    const nextRules: PaperRuleRow[] = [];
    const zeroRuleDrafts: Record<number, SmartRuleDraft> = {};
    for (const section of sections) {
      const draft = smartRuleDrafts[section.id] ?? defaultRuleDraftForSection(section, sectionQuestions);
      const questionCountText = draft.questionCount.trim();
      if (questionCountText === "") {
        showError(`${section.name} 的题量不能为空`);
        throw new Error("empty smart rule question count");
      }
      const questionCount = Number(questionCountText);
      const scorePerQuestion = draft.scorePerQuestion.trim();
      if (!Number.isInteger(questionCount) || questionCount < 0) {
        showError(`${section.name} 的题量必须是非负整数`);
        throw new Error("invalid smart rule question count");
      }
      if (questionCount === 0) {
        if (draft.ruleID !== undefined) {
          await providedPaperApi.deleteRule({
            tenantID,
            paperID: targetPaperID,
            ruleID: draft.ruleID,
          });
        }
        zeroRuleDrafts[section.id] = { ...draft, ruleID: undefined, questionCount: "0" };
        continue;
      }
      if (!isNonNegativeScore(scorePerQuestion)) {
        showError(`${section.name} 的每题分值必须是非负数字`);
        throw new Error("invalid smart rule score");
      }
      const input = {
        tenantID,
        paperID: targetPaperID,
        sectionID: section.id,
        sortOrder: section.sortOrder,
        tagIDs: [],
        tagNames: smartQuestionScope === "tag_filter" ? smartSelectedTags : [],
        questionScope: smartQuestionScope,
        difficultyPercentages: smartDifficultyPercentages,
        questionCount,
        scorePerQuestion,
        prioritizeQuality: smartPrioritizeQuality,
        excludeRecentExamQuestions: smartExcludeRecentExamQuestions,
        excludeUsedQuestions: smartExcludeUsedQuestions,
      };
      const saved = draft.ruleID === undefined
        ? await providedPaperApi.createRule(input)
        : await providedPaperApi.updateRule({ ...input, ruleID: draft.ruleID });
      nextRules.push(saved);
    }
    const nextRuleDrafts = {
      ...buildSmartRuleDrafts(sections, sectionQuestions, nextRules),
      ...zeroRuleDrafts,
    };
    setSmartRuleDrafts(nextRuleDrafts);
    setPersistedSmartRuleSnapshot(serializeSmartRuleState({
      sections,
      ruleDrafts: nextRuleDrafts,
      questionScope: smartQuestionScope,
      selectedTags: smartQuestionScope === "tag_filter" ? smartSelectedTags : [],
      difficultyPercentages: smartDifficultyPercentages,
      prioritizeQuality: smartPrioritizeQuality,
      excludeRecentExamQuestions: smartExcludeRecentExamQuestions,
      excludeUsedQuestions: smartExcludeUsedQuestions,
    }));
  }, [
    providedPaperApi,
    sectionQuestions,
    sections,
    showError,
    smartDifficultyPercentages,
    smartExcludeRecentExamQuestions,
    smartExcludeUsedQuestions,
    smartPrioritizeQuality,
    smartQuestionScope,
    smartRuleDrafts,
    smartSelectedTags,
    tenantID,
  ]);

  const saveDraft = useCallback(async ({ notifySuccess }: { notifySuccess: boolean }) => {
    if (!paperName.trim()) {
      showError("试卷名称不能为空");
      return;
    }
    const durationMinutes = Number.parseInt(durationText.trim(), 10);
    const normalizedGradeText = gradeText.trim();
    if (!Number.isInteger(durationMinutes) || durationMinutes <= 0) {
      showError("考试时长必须是正整数");
      return;
    }

    setIsSavingDraft(true);
    try {
      if (paper === null) {
        // 先接管已创建的草稿，再尝试同步可失败的附加配置，避免第二步失败时重复创建。
        let nextPaper = await providedPaperApi.createPaper({
          tenantID,
          ...(spaceID === undefined ? {} : { spaceID }),
          name: paperName.trim(),
          description: paperDescription.trim(),
          durationMinutes,
          gradeLevel: normalizedGradeText,
        });
        setPaper(nextPaper);
        let buildModeSyncError: unknown = null;
        if (buildMode !== "manual") {
          try {
            nextPaper = await providedPaperApi.updateBuildMode({
              tenantID,
              paperID: nextPaper.id,
              buildMode,
            });
            setPaper(nextPaper);
            setBuildMode((nextPaper.buildMode as PaperBuildMode | undefined) ?? buildMode);
          } catch (error) {
            buildModeSyncError = error;
          }
        }
        navigate(`/papers/${nextPaper.id}/edit${pageSearch}`, { replace: true });
        if (buildModeSyncError !== null) {
          showError(`试卷草稿已创建，${formatApiErrorMessage(buildModeSyncError, "切换组卷方式失败")}`);
        } else if (notifySuccess) {
          showSuccess(buildMode === "manual" ? "试卷草稿已创建，可以继续手动组卷。" : "试卷草稿已创建，可以继续完善组卷。");
        }
      } else {
        const changedSectionOrders = collectChangedSectionOrders(sections, persistedSections);
        const changedQuestionOrders = collectChangedQuestionOrders(sectionQuestions, persistedSectionQuestions);
        let nextPaper = paper;

        if (paperName !== paper.name || paperDescription !== (paper.description ?? "")) {
          nextPaper = await providedPaperApi.updatePaper({
            tenantID,
            paperID: paper.id,
            name: paperName.trim(),
            description: paperDescription.trim(),
            durationMinutes,
            gradeLevel: normalizedGradeText,
          });
        } else if (durationMinutes !== (paper.durationMinutes ?? 120)) {
          nextPaper = await providedPaperApi.updatePaper({
            tenantID,
            paperID: paper.id,
            name: paperName.trim(),
            description: paperDescription.trim(),
            durationMinutes,
            gradeLevel: normalizedGradeText,
          });
        } else if (normalizedGradeText !== (paper.gradeLevel?.trim() ? paper.gradeLevel.trim() : "高一")) {
          nextPaper = await providedPaperApi.updatePaper({
            tenantID,
            paperID: paper.id,
            name: paperName.trim(),
            description: paperDescription.trim(),
            durationMinutes,
            gradeLevel: normalizedGradeText,
          });
        }
        if (changedSectionOrders.length > 0) {
          await providedPaperApi.reorderSections({
            tenantID,
            paperID: paper.id,
            orders: changedSectionOrders,
          });
          setPersistedSections(sections.map(cloneSection));
        }
        if (changedQuestionOrders.length > 0) {
          for (const item of changedQuestionOrders) {
            await providedPaperApi.updateSectionQuestion({
              tenantID,
              paperID: paper.id,
              sectionID: item.sectionID,
              questionID: item.questionID,
              sortOrder: item.sortOrder,
              score: item.score,
            });
          }
          setPersistedSectionQuestions(sectionQuestions.map(cloneSectionQuestion));
        }
        if (buildMode !== "manual") {
          await persistSmartRules(paper.id);
        }
        setPaper(nextPaper);
        if (notifySuccess) {
          showSuccess("试卷草稿已保存。");
        }
      }
    } catch (error) {
      showError(formatApiErrorMessage(error, "试卷草稿保存失败"));
    } finally {
      setIsSavingDraft(false);
    }
  }, [
    buildMode,
    durationText,
    gradeText,
    navigate,
    pageSearch,
    paper,
    paperDescription,
    paperName,
    persistSmartRules,
    persistedSectionQuestions,
    persistedSections,
    providedPaperApi,
    sectionQuestions,
    sections,
    showError,
    showSuccess,
    spaceID,
    tenantID,
  ]);

  async function handleSaveDraft(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    await saveDraft({ notifySuccess: true });
  }

  useEffect(() => {
    if (paper === null || !draftDirty || isSavingDraft || !paperName.trim()) {
      return undefined;
    }

    const timer = window.setTimeout(() => {
      void saveDraft({ notifySuccess: false });
    }, AUTO_SAVE_DRAFT_DELAY_MS);

    return () => window.clearTimeout(timer);
  }, [draftDirty, isSavingDraft, paper, paperName, saveDraft]);

  async function handleGenerateSmartPaper() {
    if (paper === null) {
      showError("请先保存试卷基础信息后再智能组卷");
      return;
    }
    try {
      await persistSmartRules(paper.id);
      await providedPaperApi.generateRuleFixed({ tenantID, paperID: paper.id, blockedQuestionIDs: blockedSmartQuestionIDs });
      const [nextQuestionPool, snapshot] = await withRefreshFeedback(Promise.all([
        loadQuestionPool(),
        loadWorkspaceSnapshot(),
      ]));
      applyWorkspaceSnapshot(
        snapshot.currentPaper,
        snapshot.nextSections,
        snapshot.nextSectionQuestions,
        snapshot.nextRules,
        nextQuestionPool,
      );
      showSuccess("智能组卷已生成。");
    } catch (error) {
      showError(formatApiErrorMessage(error, "智能组卷生成失败"));
    }
  }

  async function handleAddQuestion(question: QuestionRow) {
    try {
      if (paper === null) {
        showError("请先保存试卷基础信息后再开始组卷");
        return;
      }
      if (isRuleLivePaper) {
        showError("rule_live 试卷请到组卷规则页维护题池和分值。");
        return;
      }
      let targetSection = sections.find((item) => item.questionType === question.type) ?? null;
      if (targetSection === null) {
        const template = sectionTemplates[question.type];
        targetSection = await providedPaperApi.createSection({
          tenantID,
          paperID: paper.id,
          name: template.name,
          questionType: question.type,
          instructions: template.instructions,
        });
        setSections((items) => [...items, targetSection!]);
        setPersistedSections((items) => [...items, targetSection!]);
      }

      const nextQuestion = await providedPaperApi.addManualQuestion({
        tenantID,
        paperID: paper.id,
        sectionID: targetSection.id,
        questionID: question.id,
        score: question.scoreDefault ?? "5",
      });
      setSectionQuestions((items) => [...items, nextQuestion]);
      setPersistedSectionQuestions((items) => [...items, nextQuestion]);
    } catch (error) {
      showError(formatApiErrorMessage(error, "加入试卷失败"));
    }
  }

  async function handleRefreshWorkspace() {
    setIsRefreshingWorkspace(true);
    try {
      const [nextQuestionPool, snapshot] = await withRefreshFeedback(Promise.all([
        loadQuestionPool(),
        loadWorkspaceSnapshot(),
      ]));
      applyWorkspaceSnapshot(
        snapshot.currentPaper,
        snapshot.nextSections,
        snapshot.nextSectionQuestions,
        snapshot.nextRules,
        nextQuestionPool,
      );
      clearDragState();
    } catch (error) {
      showError(formatApiErrorMessage(error, "组卷数据刷新失败"));
    } finally {
      setIsRefreshingWorkspace(false);
    }
  }

  async function handleRemoveQuestion(item: ManualQuestionRow) {
    if (paper === null) {
      return;
    }
    if (isRuleLivePaper) {
      showError("rule_live 试卷请到组卷规则页维护题池和分值。");
      return;
    }

    try {
      await providedPaperApi.deleteSectionQuestion({
        tenantID,
        paperID: paper.id,
        sectionID: item.sectionID,
        questionID: item.questionID,
      });
      setSectionQuestions((items) => items.filter((current) => !(current.sectionID === item.sectionID && current.questionID === item.questionID)));
      setPersistedSectionQuestions((items) => items.filter((current) => !(current.sectionID === item.sectionID && current.questionID === item.questionID)));
      setEditingScores((items) => {
        const next = { ...items };
        delete next[toQuestionKey(item)];
        return next;
      });
    } catch (error) {
      showError(formatApiErrorMessage(error, "移除试题失败"));
    }
  }

  async function handleBlockSmartQuestion(item: ManualQuestionRow) {
    if (paper === null) {
      return;
    }
    try {
      await providedPaperApi.deleteSectionQuestion({
        tenantID,
        paperID: paper.id,
        sectionID: item.sectionID,
        questionID: item.questionID,
      });
      setBlockedSmartQuestionIDs((ids) => (ids.includes(item.questionID) ? ids : [...ids, item.questionID]));
      setSectionQuestions((items) => items.filter((current) => !(current.sectionID === item.sectionID && current.questionID === item.questionID)));
      setPersistedSectionQuestions((items) => items.filter((current) => !(current.sectionID === item.sectionID && current.questionID === item.questionID)));
      setEditingScores((items) => {
        const next = { ...items };
        delete next[toQuestionKey(item)];
        return next;
      });
      showSuccess("已屏蔽该题，本次智能组卷会排除。");
    } catch (error) {
      showError(formatApiErrorMessage(error, "屏蔽题目失败"));
    }
  }

  function handleUnblockSmartQuestion(questionID: number) {
    setBlockedSmartQuestionIDs((ids) => ids.filter((id) => id !== questionID));
  }

  async function handleScoreBlur(item: ManualQuestionRow) {
    if (paper === null) {
      return;
    }
    if (isRuleLivePaper) {
      return;
    }
    const key = toQuestionKey(item);
    const nextScore = (editingScores[key] ?? item.score).trim();
    if (nextScore === item.score) {
      return;
    }
    if (!isNonNegativeScore(nextScore)) {
      showError("分值必须是非负数字");
      setEditingScores((items) => ({ ...items, [key]: item.score }));
      return;
    }

    const persistedQuestion = persistedSectionQuestions.find((current) => current.sectionID === item.sectionID && current.questionID === item.questionID);
    if (persistedQuestion === undefined) {
      showError("未找到可保存的题目分值快照");
      setEditingScores((items) => ({ ...items, [key]: item.score }));
      return;
    }

    try {
      const saved = await providedPaperApi.updateSectionQuestion({
        tenantID,
        paperID: paper.id,
        sectionID: item.sectionID,
        questionID: item.questionID,
        sortOrder: persistedQuestion.sortOrder,
        score: nextScore,
      });
      setSectionQuestions((items) => items.map((current) => (
        current.sectionID === item.sectionID && current.questionID === item.questionID
          ? { ...current, score: saved.score }
          : current
      )));
      setPersistedSectionQuestions((items) => items.map((current) => (
        current.sectionID === item.sectionID && current.questionID === item.questionID
          ? { ...current, score: saved.score }
          : current
      )));
      setEditingScores((items) => ({ ...items, [key]: saved.score }));
    } catch (error) {
      showError(formatApiErrorMessage(error, "更新题目分值失败"));
      setEditingScores((items) => ({ ...items, [key]: item.score }));
    }
  }

  function handleSectionDrop(targetSectionID: number) {
    if (isRuleLivePaper) {
      clearDragState();
      return;
    }
    if (draggingSectionID === null || draggingSectionID === targetSectionID) {
      return;
    }

    setSections((items) => moveSectionOrder(items, draggingSectionID, targetSectionID));
    clearDragState();
  }

  function handleSectionDragStart(event: React.DragEvent<HTMLButtonElement>, sectionID: number) {
    if (isRuleLivePaper) {
      event.preventDefault();
      return;
    }
    setDraggingSectionID(sectionID);
    configureDragPreview(event, event.currentTarget.closest(".exam-paper-editor__selected-section, .exam-paper-editor__rule-row"));
  }

  function handleSectionDragOver(targetSectionID: number) {
    if (draggingSectionID === null || draggingSectionID === targetSectionID) {
      setSectionDropTargetID(null);
      setSectionDropDirection(null);
      return;
    }
    const direction = detectSectionDropDirection(sections, draggingSectionID, targetSectionID);
    setSectionDropTargetID(targetSectionID);
    setSectionDropDirection(direction);
  }

  function handleSectionDragEnd() {
    clearDragState();
  }

  function handleQuestionDrop(target: ManualQuestionRow) {
    if (isRuleLivePaper) {
      clearDragState();
      return;
    }
    if (draggingQuestion === null) {
      return;
    }
    if (draggingQuestion.sectionID !== target.sectionID) {
      setDraggingQuestion(null);
      return;
    }
    if (draggingQuestion.questionID === target.questionID) {
      setDraggingQuestion(null);
      return;
    }

    setSectionQuestions((items) => moveQuestionOrder(items, draggingQuestion, target));
    clearDragState();
  }

  function handleQuestionDragStart(
    event: React.DragEvent<HTMLButtonElement>,
    item: Pick<ManualQuestionRow, "sectionID" | "questionID">,
  ) {
    if (isRuleLivePaper) {
      event.preventDefault();
      return;
    }
    setDraggingQuestion({ sectionID: item.sectionID, questionID: item.questionID });
    configureDragPreview(event, event.currentTarget.closest("tr"));
  }

  function handleQuestionDragOver(target: ManualQuestionRow) {
    if (draggingQuestion === null || draggingQuestion.sectionID !== target.sectionID || draggingQuestion.questionID === target.questionID) {
      setQuestionDropTargetKey(null);
      setQuestionDropDirection(null);
      return;
    }
    const direction = detectQuestionDropDirection(sectionQuestions, draggingQuestion, target);
    setQuestionDropTargetKey(toQuestionKey(target));
    setQuestionDropDirection(direction);
  }

  function handleQuestionDragEnd() {
    clearDragState();
  }

  function clearDragState() {
    setDraggingSectionID(null);
    setDraggingQuestion(null);
    setSectionDropTargetID(null);
    setSectionDropDirection(null);
    setQuestionDropTargetKey(null);
    setQuestionDropDirection(null);
  }

  const previewDialog = isPreviewOpen ? (
    <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="试卷预览弹窗">
      <div className="platform-dialog__card exam-paper-preview-dialog">
        <div className="exam-paper-preview-dialog__head">
          <div>
            <h2>{paperName || "未命名试卷"}</h2>
            <p>{selectedQuestionCount} 题 / 总分 {totalScore} 分</p>
          </div>
          <Button variant="secondary" onClick={() => setIsPreviewOpen(false)} type="button">
            关闭预览
          </Button>
        </div>
        <div className="exam-paper-preview-dialog__body">
          {groupedSelectedQuestions.map((group, groupIndex) => (
            <section key={group.section.id} className="exam-paper-preview-dialog__section">
              <h3>{formatSectionTitle(group.section, groupIndex)}</h3>
              <ol>
                {group.questions.map((item) => (
                  <li key={`${item.sectionID}-${item.questionID}`}>
                    <span>{item.question?.title ?? `题目 ${item.questionID}`}</span>
                    <strong>{item.score} 分</strong>
                  </li>
                ))}
              </ol>
            </section>
          ))}
        </div>
      </div>
    </div>
  ) : null;

  return (
    <section className="page platform-page exam-builder-page exam-paper-editor-page">
      <nav aria-label="试卷编辑菜单" className="platform-tabbar" role="tablist">
        <Link className="platform-tab" to={listPath} role="tab" aria-selected="false">
          试卷
        </Link>
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          组卷管理
        </span>
      </nav>

      <Panel>
        <form className="exam-paper-editor" onSubmit={handleSaveDraft}>
          <div className="tenant-list-toolbar exam-paper-editor__toolbar">
            <div className="tenant-list-actions" aria-label="组卷操作区">
              <Button
                className="exam-paper-editor__toolbar-button--icon-center"
                disabled={!canAssemble || selectedQuestionCount === 0}
                onClick={() => setIsPreviewOpen(true)}
                variant="toolbarSecondary"
                type="button"
              >
                <Eye aria-hidden="true" size={16} />
                <span>预览试卷</span>
              </Button>
            </div>

            <div className="tenant-search-actions">
              <Button
                aria-label="刷新组卷数据"
                disabled={isRefreshingWorkspace}
                onClick={() => void handleRefreshWorkspace()}
                variant="icon"
                type="button"
              >
                <RefreshIcon active={isRefreshingWorkspace} />
              </Button>
            </div>
          </div>

          <div className="exam-paper-editor__summarybar exam-paper-editor__summarybar--responsive">
            <label className="exam-paper-editor__summary-item exam-paper-editor__summary-item--name">
              <span>试卷名称：</span>
              <input
                aria-label="试卷名称"
                onChange={(event) => setPaperName(event.target.value)}
                placeholder="请输入试卷名称"
                value={paperName}
              />
            </label>
            <label className="exam-paper-editor__summary-item exam-paper-editor__summary-item--duration">
              <span>考试时长：</span>
              <input
                aria-label="考试时长"
                className="exam-paper-editor__summary-input--compact exam-paper-editor__summary-input--duration"
                onChange={(event) => setDurationText(event.target.value)}
                placeholder="120"
                value={durationText}
              />
              <em>分钟</em>
            </label>
            <div className="exam-paper-editor__summary-item">
              <span>总分：</span>
              <strong>{totalScore}</strong>
              <em>分</em>
            </div>
            <label className="exam-paper-editor__summary-item exam-paper-editor__summary-item--grade">
              <span>适用年级：</span>
              <input
                aria-label="适用年级"
                className="exam-paper-editor__summary-input--compact exam-paper-editor__summary-input--grade"
                onChange={(event) => setGradeText(event.target.value)}
                placeholder="高一"
                value={gradeText}
              />
            </label>
            <div className="exam-paper-editor__summary-item exam-paper-editor__summary-item--modes">
              <span>组卷方式：</span>
              {canChangeBuildMode ? (
                <SegmentTabs
                  active={buildMode === "manual" ? "手动组卷" : "智能组卷"}
                  ariaLabel="组卷方式"
                  className="exam-paper-editor__summary-chip--compact"
                  items={["手动组卷", "智能组卷"]}
                  onChange={(item) => setBuildMode(item === "手动组卷" ? "manual" : "rule_fixed")}
                />
              ) : (
                <strong aria-label="组卷方式" className="exam-paper-editor__mode-value exam-paper-editor__summary-chip--compact">
                  {editorBuildModeLabel(buildMode)}
                </strong>
              )}
            </div>
            <div className="exam-paper-editor__summary-item exam-paper-editor__summary-item--status exam-paper-editor__summary-item--inline">
              <span className="exam-paper-editor__status">
                <StatusBadge className="exam-paper-editor__summary-chip--compact" tone={paperStatusTone(paper?.status ?? "draft")}>
                  {paperStatusLabel(paper?.status ?? "draft")}
                </StatusBadge>
              </span>
            </div>
          </div>
          <label className="sr-only">
            <span>试卷说明</span>
            <input aria-label="试卷说明" onChange={(event) => setPaperDescription(event.target.value)} value={paperDescription} />
          </label>
          {!isSmartPaper ? (
          <div className="exam-paper-editor__workspace">
            <section className="exam-paper-editor__panel">
              <div className="exam-paper-editor__panel-head">
                <h2>题库筛选</h2>
              </div>

              {isRuleLivePaper && (
                <div className="exam-paper-editor__locked-state">
                  rule_live 试卷请到组卷规则页维护题池和分值。
                </div>
              )}

              <div className="exam-paper-editor__filters">
                <FilterSelect
                  ariaLabel="题型筛选"
                  label="题型"
                  onChange={(value) => {
                    setSelectedType(value as "all" | QuestionType);
                    setPage(1);
                  }}
                  options={[
                    { value: "all", label: "全部题型" },
                    ...Object.entries(questionTypeLabels).map(([value, label]) => ({ value, label })),
                  ]}
                  value={selectedType}
                />
                <FilterSelect
                  ariaLabel="难度筛选"
                  label="难度"
                  onChange={(value) => {
                    setSelectedDifficulty(value as "all" | QuestionDifficulty);
                    setPage(1);
                  }}
                  options={[
                    { value: "all", label: "全部难度" },
                    ...Object.entries(difficultyLabels).map(([value, label]) => ({ value, label })),
                  ]}
                  value={selectedDifficulty}
                />
                <FilterSelect
                  ariaLabel="知识点筛选"
                  label="知识点"
                  onChange={(value) => {
                    setSelectedTag(value);
                    setPage(1);
                  }}
                  options={[
                    { value: "all", label: "全部标签" },
                    ...tags.map((tag) => ({ value: tag, label: tag })),
                  ]}
                  value={selectedTag}
                />
              </div>

              <div className="table-wrap">
                <table className="data-table tenant-admin-table exam-paper-editor__table exam-paper-editor__question-table">
                  <thead>
                    <tr>
                      <th scope="col">题号</th>
                      <th scope="col">题型</th>
                      <th scope="col">难度</th>
                      <th scope="col">知识点</th>
                      <th scope="col">分值</th>
                      <th scope="col">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {pagedQuestions.length === 0 && !canAssemble ? (
                      <tr>
                        <td colSpan={6}>
                          <div className="exam-paper-editor__locked-state">
                            请先保存试卷基础信息后再开始组卷
                          </div>
                        </td>
                      </tr>
                    ) : (
                      <>
                        {pagedQuestions.length === 0 && <EmptyTableRow colSpan={6} />}
                        {pagedQuestions.map((item, index) => {
                          const selectedQuestion = sectionQuestions.find((current) => current.questionID === item.id);
                          const isSelected = selectedQuestion !== undefined;

                          return (
                            <tr key={item.id}>
                              <td>{index + 1 + ((page - 1) * 10)}</td>
                              <td>{questionTypeLabels[item.type]}</td>
                              <td>
                                <span className={`exam-paper-editor__difficulty exam-paper-editor__difficulty--${item.difficulty}`}>
                                  {difficultyLabels[item.difficulty]}
                                </span>
                              </td>
                              <td>
                                <div className="exam-paper-editor__question-meta">
                                  <strong title={item.title}>{item.title}</strong>
                                  <span>{(item.tags.length > 0 ? item.tags : [item.tag]).join(" / ")}</span>
                                </div>
                              </td>
                              <td>{item.scoreDefault ?? "5"} 分</td>
                              <td>
                                <Button
                                  aria-label={`${isSelected ? "移除试卷" : "加入试卷"} 题目 ${item.id}`}
                                  disabled={isRuleLivePaper}
                                  onClick={() => {
                                    if (selectedQuestion !== undefined) {
                                      void handleRemoveQuestion(selectedQuestion);
                                      return;
                                    }
                                    void handleAddQuestion(item);
                                  }}
                                  variant={isSelected ? "actionClose" : "actionReset"}
                                  type="button"
                                >
                                  {isSelected ? "移除试卷" : "加入试卷"}
                                </Button>
                              </td>
                            </tr>
                          );
                        })}
                      </>
                    )}
                  </tbody>
                </table>
              </div>
              <div className="exam-paper-editor__pager">
                <span>共 {filteredQuestions.length} 条</span>
                <div className="exam-paper-editor__pager-actions">
                  <Button disabled={page === 1} onClick={() => setPage((current) => Math.max(1, current - 1))} type="button" variant="secondary">
                    上一页
                  </Button>
                  <Button
                    disabled={page >= Math.max(1, Math.ceil(filteredQuestions.length / 10))}
                    onClick={() => setPage((current) => current + 1)}
                    type="button"
                    variant="secondary"
                  >
                    下一页
                  </Button>
                </div>
              </div>
            </section>

            <section className="exam-paper-editor__panel">
              <div className="exam-paper-editor__panel-head">
                <h2>已选试题</h2>
                <span>共 {selectedQuestionCount} 题 / 总分 {totalScore} 分</span>
              </div>

              {groupedSelectedQuestions.length === 0 ? (
                <div className="exam-paper-editor__selected-empty">
                  {canAssemble ? "当前还没有加入试题，先从左侧题库加入。" : "请先保存试卷基础信息后再开始组卷"}
                </div>
              ) : (
                <div className="exam-paper-editor__selected-groups">
                  {groupedSelectedQuestions.map((group, groupIndex) => (
                    <section
                      className={buildSectionClassName(
                        group.section.id,
                        draggingSectionID,
                        sectionDropTargetID,
                        sectionDropDirection,
                      )}
                      key={group.section.id}
                      onDragOver={(event) => {
                        event.preventDefault();
                        handleSectionDragOver(group.section.id);
                      }}
                      onDrop={() => handleSectionDrop(group.section.id)}
                    >
                      <div className="exam-paper-editor__selected-section-head">
                        <div className="exam-paper-editor__selected-section-title">
                          <button
                            aria-label={`拖拽排序大题 ${group.section.id}`}
                            className="exam-paper-editor__drag-handle"
                            disabled={isRuleLivePaper}
                            draggable
                            onDragEnd={handleSectionDragEnd}
                            onDragStart={(event) => handleSectionDragStart(event, group.section.id)}
                            type="button"
                          >
                            <GripVertical aria-hidden="true" size={14} />
                          </button>
                          <h3>{formatSectionTitle(group.section, groupIndex)}</h3>
                        </div>
                        <span>共 {group.questions.length} 题</span>
                      </div>
                      <div className="table-wrap">
                        <table className="data-table tenant-admin-table exam-paper-editor__table exam-paper-editor__selected-table">
                          <colgroup>
                            <col className="exam-paper-editor__selected-col exam-paper-editor__selected-col--question" />
                            <col className="exam-paper-editor__selected-col exam-paper-editor__selected-col--source" />
                            <col className="exam-paper-editor__selected-col exam-paper-editor__selected-col--score" />
                            <col className="exam-paper-editor__selected-col exam-paper-editor__selected-col--actions" />
                          </colgroup>
                          <thead>
                            <tr>
                              <th scope="col">题目</th>
                              <th scope="col">来源题库</th>
                              <th scope="col">分值</th>
                              <th scope="col">操作</th>
                            </tr>
                          </thead>
                          <tbody>
                            {group.questions.map((item, index) => {
                              const selectedTitle = item.question?.title ?? `题目 ${item.questionID}`;
                              const questionKey = toQuestionKey(item);
                              const scoreValue = editingScores[questionKey] ?? item.score;

                              return (
                                <tr
                                  className={buildQuestionRowClassName(
                                    item,
                                    draggingQuestion,
                                    questionDropTargetKey,
                                    questionDropDirection,
                                  )}
                                  key={`${item.sectionID}-${item.questionID}`}
                                  onDragOver={(event) => {
                                    event.preventDefault();
                                    handleQuestionDragOver(item);
                                  }}
                                  onDrop={() => handleQuestionDrop(item)}
                                >
                                  <td>
                                    <div className="exam-paper-editor__selected-title-wrap">
                                      <button
                                        aria-label={`拖拽排序题目 ${item.questionID}`}
                                        className="exam-paper-editor__drag-handle"
                                        disabled={isRuleLivePaper}
                                        draggable
                                        onDragEnd={handleQuestionDragEnd}
                                        onDragStart={(event) => handleQuestionDragStart(event, item)}
                                        type="button"
                                      >
                                        <GripVertical aria-hidden="true" size={14} />
                                      </button>
                                      <Tooltip content={`${index + 1}. ${selectedTitle}`}>
                                        <button className="exam-paper-editor__selected-title" type="button">
                                          {index + 1}. {selectedTitle}
                                        </button>
                                      </Tooltip>
                                    </div>
                                  </td>
                                  <td>{(item.question?.tags.length ?? 0) > 0 ? item.question?.tags.join(" / ") : item.question?.tag ?? "-"}</td>
                                  <td>
                                    <input
                                      aria-label={`题目 ${item.questionID} 分值`}
                                      className="exam-paper-editor__score-input"
                                      disabled={isRuleLivePaper}
                                      inputMode="decimal"
                                      min="0"
                                      onBlur={() => void handleScoreBlur(item)}
                                      onChange={(event) => {
                                        const nextValue = event.target.value;
                                        setEditingScores((items) => ({ ...items, [questionKey]: nextValue }));
                                      }}
                                      onKeyDown={(event) => {
                                        if (event.key === "Enter") {
                                          event.preventDefault();
                                          event.currentTarget.blur();
                                        }
                                        if (event.key === "Escape") {
                                          event.preventDefault();
                                          setEditingScores((items) => ({ ...items, [questionKey]: item.score }));
                                          event.currentTarget.blur();
                                        }
                                      }}
                                      step="any"
                                      type="number"
                                      value={scoreValue}
                                    />
                                    <span className="exam-paper-editor__score-unit">分</span>
                                  </td>
                                  <td className="exam-inline-actions">
                                    <Button
                                      aria-label={`移除题目 ${item.questionID}`}
                                      disabled={isRuleLivePaper}
                                      onClick={() => void handleRemoveQuestion(item)}
                                      type="button"
                                      variant="actionClose"
                                    >
                                      <Trash2 aria-hidden="true" size={14} />
                                      <span>移除</span>
                                    </Button>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </section>
                  ))}
                </div>
              )}
            </section>
          </div>
          ) : (
          <div className="exam-paper-editor__workspace exam-paper-editor__workspace--smart">
            <section className="exam-paper-editor__panel exam-paper-editor__panel--separated exam-paper-editor__smart-rules">
              <div className="exam-paper-editor__panel-head">
                <h2>组卷规则</h2>
              </div>

              <fieldset className="exam-paper-editor__smart-fieldset exam-paper-editor__smart-rule-section">
                <legend>题库范围</legend>
                <label>
                  <input
                    checked={smartQuestionScope === "space_all"}
                    onChange={() => setSmartQuestionScope("space_all")}
                    type="radio"
                  />
                  本空间全部题库
                </label>
                <label>
                  <input
                    checked={smartQuestionScope === "tag_filter"}
                    onChange={() => setSmartQuestionScope("tag_filter")}
                    type="radio"
                  />
                  根据知识点筛选
                </label>
              </fieldset>

              <section className="exam-paper-editor__smart-block exam-paper-editor__smart-rule-section">
                <h3>难度分布</h3>
                <div
                  aria-label="难度分布滑块"
                  className="exam-paper-editor__difficulty-slider"
                  role="group"
                  style={difficultySliderStyle}
                >
                  <div className="exam-paper-editor__difficulty-values">
                    <span>{difficultyLabels.easy} {smartDifficultyPercentages.easy}%</span>
                    <span>{difficultyLabels.medium} {smartDifficultyPercentages.medium}%</span>
                    <span>{difficultyLabels.hard} {smartDifficultyPercentages.hard}%</span>
                  </div>
                  <div className="exam-paper-editor__difficulty-track">
                    <span className="exam-paper-editor__difficulty-fill" aria-hidden="true" />
                    <input
                      aria-label="较易占比边界"
                      max={100}
                      min={0}
                      onChange={(event) => updateEasyBoundary(Number.parseInt(event.target.value || "0", 10))}
                      type="range"
                      value={easyBoundary}
                    />
                    <input
                      aria-label="中等占比边界"
                      max={100}
                      min={0}
                      onChange={(event) => updateMediumBoundary(Number.parseInt(event.target.value || "0", 10))}
                      type="range"
                      value={mediumBoundary}
                    />
                  </div>
                </div>
              </section>

              {smartQuestionScope === "tag_filter" ? (
                <section className="exam-paper-editor__smart-block exam-paper-editor__smart-rule-section">
                  <h3>知识点覆盖</h3>
                  <div
                    className="exam-paper-editor__tag-combobox"
                    onBlur={(event) => {
                      const nextTarget = event.relatedTarget;
                      if (nextTarget instanceof Node && event.currentTarget.contains(nextTarget)) {
                        return;
                      }
                      setIsSmartTagPickerOpen(false);
                    }}
                  >
                    <label className="exam-paper-editor__tag-input">
                      <span>选择知识点</span>
                      <input
                        aria-label="搜索知识点标签"
                        disabled={tags.length === 0}
                        onChange={(event) => {
                          setSmartTagQuery(event.target.value);
                          setIsSmartTagPickerOpen(true);
                        }}
                        onFocus={() => setIsSmartTagPickerOpen(true)}
                        placeholder={tags.length === 0 ? "暂无可用标签" : "输入关键词选择标签"}
                        value={smartTagQuery}
                      />
                    </label>
                    {isSmartTagPickerOpen && tags.length > 0 ? (
                      <div className="exam-paper-editor__tag-options" role="listbox">
                        {smartTagOptions.length === 0 ? (
                          <span className="exam-paper-editor__tag-empty">没有匹配的标签</span>
                        ) : smartTagOptions.map((tag) => (
                          <button
                            key={tag}
                            onMouseDown={(event) => event.preventDefault()}
                            onClick={() => selectSmartTag(tag)}
                            role="option"
                            type="button"
                          >
                            {tag}
                          </button>
                        ))}
                      </div>
                    ) : null}
                  </div>
                  <div className="exam-paper-editor__selected-tags" aria-label="已选知识点标签">
                    {smartSelectedTags.length === 0 ? (
                      <span className="exam-paper-editor__muted">未选择知识点时，将按当前题库范围生成。</span>
                    ) : smartSelectedTags.map((tag) => (
                      <span className="exam-paper-editor__selected-tag" key={tag}>
                        {tag}
                        <button
                          aria-label={`移除知识点 ${tag}`}
                          onClick={() => removeSmartTag(tag)}
                          type="button"
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                </section>
              ) : null}

              <section className="exam-paper-editor__smart-block exam-paper-editor__smart-rule-section">
                <div className="exam-paper-editor__smart-block-head">
                  <h3>题型数量与分值</h3>
                  <Button
                    onClick={() => setIsAddingSmartSection((value) => !value)}
                    type="button"
                    variant="toolbarSecondary"
                  >
                    <FilePlus2 aria-hidden="true" size={14} />
                    <span>添加题型</span>
                  </Button>
                </div>
                {isAddingSmartSection ? (
                  <div className="exam-paper-editor__section-form">
                    <label>
                      <span>题型</span>
                      <select
                        aria-label="新增题型类型"
                        onChange={(event) => handleNewSmartSectionTypeChange(event.target.value as QuestionType)}
                        value={newSmartSectionType}
                      >
                        {Object.entries(sectionTemplates).map(([value, template]) => (
                          <option key={value} value={value}>{defaultSectionName(value as QuestionType) || template.name}</option>
                        ))}
                      </select>
                    </label>
                    <label>
                      <span>名称</span>
                      <input
                        aria-label="新增题型名称"
                        onChange={(event) => setNewSmartSectionName(event.target.value)}
                        value={newSmartSectionName}
                      />
                    </label>
                    <Button
                      disabled={isCreatingSmartSection}
                      onClick={() => void handleCreateSmartSection()}
                      type="button"
                      variant="primary"
                    >
                      保存题型
                    </Button>
                  </div>
                ) : null}
                <div className="exam-paper-editor__rule-rows exam-paper-editor__rule-rows--separated">
                  {sections.map((section, sectionIndex) => {
                    const draft = smartRuleDrafts[section.id] ?? defaultRuleDraftForSection(section, sectionQuestions);
                    const sectionTitle = formatSectionTitle(section, sectionIndex);
                    return (
                      <div
                        className={buildRuleRowClassName(section.id, draggingSectionID, sectionDropTargetID, sectionDropDirection)}
                        key={section.id}
                        onDragOver={(event) => {
                          event.preventDefault();
                          handleSectionDragOver(section.id);
                        }}
                        onDrop={() => handleSectionDrop(section.id)}
                      >
                        <button
                          aria-label={`拖拽排序题型 ${section.id}`}
                          className="exam-paper-editor__drag-handle"
                          draggable
                          onDragEnd={handleSectionDragEnd}
                          onDragStart={(event) => handleSectionDragStart(event, section.id)}
                          type="button"
                        >
                          <GripVertical aria-hidden="true" size={14} />
                        </button>
                        <span className="exam-paper-editor__rule-title">{sectionTitle}</span>
                        <div className="exam-paper-editor__rule-controls">
                          <label>
                            <span>题数</span>
                            <input
                              aria-label={`${sectionTitle}题数`}
                              min={0}
                              onChange={(event) => setSmartRuleDrafts((items) => ({
                                ...items,
                                [section.id]: { ...draft, questionCount: event.target.value },
                              }))}
                              type="number"
                              value={draft.questionCount}
                            />
                          </label>
                        </div>
                        <Button
                          aria-label={`删除题型 ${section.id}`}
                          onClick={() => void handleDeleteSmartSection(section)}
                          type="button"
                          variant="actionClose"
                        >
                          <Trash2 aria-hidden="true" size={14} />
                        </Button>
                      </div>
                    );
                  })}
                </div>
              </section>

              <section className="exam-paper-editor__smart-block exam-paper-editor__smart-rule-section exam-paper-editor__constraint-block">
                <h3>组卷约束</h3>
                <div className="exam-paper-editor__constraint-grid">
                  <label>
                    <input checked={smartPrioritizeQuality} onChange={(event) => setSmartPrioritizeQuality(event.target.checked)} type="checkbox" />
                    优先高质量题目
                  </label>
                  <label>
                    <input checked={smartExcludeRecentExamQuestions} onChange={(event) => setSmartExcludeRecentExamQuestions(event.target.checked)} type="checkbox" />
                    近三次考试不重复
                  </label>
                  <label>
                    <input checked={smartExcludeUsedQuestions} onChange={(event) => setSmartExcludeUsedQuestions(event.target.checked)} type="checkbox" />
                    排除已用题目
                  </label>
                </div>
              </section>
              {blockedSmartQuestions.length > 0 ? (
                <section className="exam-paper-editor__smart-block exam-paper-editor__smart-rule-section exam-paper-editor__blocked-block">
                  <h3>已屏蔽题目</h3>
                  <div className="exam-paper-editor__blocked-list">
                    {blockedSmartQuestions.map((question) => {
                      const tagNames = question.tags?.length > 0 ? question.tags : [question.tag].filter(Boolean);

                      return (
                        <div className="exam-paper-editor__blocked-item" key={question.id}>
                          <div className="exam-paper-editor__question-meta">
                            <strong className="exam-paper-editor__blocked-title--regular" title={question.title}>{question.title}</strong>
                            {tagNames.length > 0 ? <span>{tagNames.join(" / ")}</span> : null}
                          </div>
                          <Button
                            aria-label={`移除屏蔽题目 ${question.id}`}
                            className="exam-paper-editor__action-button--regular"
                            onClick={() => handleUnblockSmartQuestion(question.id)}
                            type="button"
                            variant="actionReset"
                          >
                            移除屏蔽
                          </Button>
                        </div>
                      );
                    })}
                  </div>
                </section>
              ) : null}
            </section>

            <section className="exam-paper-editor__panel exam-paper-editor__panel--separated exam-paper-editor__smart-results">
              <div className="exam-paper-editor__panel-head">
                <h2>已生成试卷 / 智能推荐结果</h2>
                <span>共 {selectedQuestionCount} 题 / 总分 {totalScore} 分</span>
              </div>
              <div className="exam-paper-editor__smart-metrics exam-paper-editor__smart-metrics--separated">
                <div><span>可用题量</span><strong>{filteredQuestions.length}</strong></div>
                <div><span>知识点覆盖率</span><strong>{smartQuestionScope === "tag_filter" && smartSelectedTags.length > 0 ? "已筛选" : "全部"}</strong></div>
                <div><span>预计生成耗时</span><strong>3 秒</strong></div>
              </div>

              {groupedSelectedQuestions.length === 0 ? (
                <div className="exam-paper-editor__selected-empty">
                  {canAssemble ? "配置规则后点击一键智能组卷生成试题。" : "请先保存试卷基础信息后再智能组卷"}
                </div>
              ) : (
                <div className="exam-paper-editor__selected-groups">
                  {groupedSelectedQuestions.map((group, groupIndex) => (
                    <section className="exam-paper-editor__selected-section exam-paper-editor__selected-section--separated" key={group.section.id}>
                      <div className="exam-paper-editor__selected-section-head">
                        <h3>{formatSectionTitle(group.section, groupIndex)}</h3>
                        <span>共 {group.questions.length} 题</span>
                      </div>
                      <div className="table-wrap">
                        <table className="data-table tenant-admin-table exam-paper-editor__selected-table exam-paper-editor__smart-table">
                          <thead>
                            <tr>
                              <th scope="col">题目</th>
                              <th scope="col">来源题库</th>
                              <th scope="col">难度</th>
                              <th scope="col">分值</th>
                              <th scope="col">操作</th>
                            </tr>
                          </thead>
                          <tbody>
                            {group.questions.map((item, index) => {
                              const selectedTitle = item.question?.title ?? `题目 ${item.questionID}`;
                              return (
                                <tr key={`${item.sectionID}-${item.questionID}`}>
                                  <td>
                                    <Tooltip content={`${index + 1}. ${selectedTitle}`}>
                                      <button className="exam-paper-editor__selected-title" type="button">
                                        {index + 1}. {selectedTitle}
                                      </button>
                                    </Tooltip>
                                  </td>
                                  <td>{item.question === null ? "题库" : (item.question.tags.length > 0 ? item.question.tags : [item.question.tag]).join(" / ")}</td>
                                  <td>
                                    {item.question === null ? "-" : (
                                      <span className={`exam-paper-editor__difficulty exam-paper-editor__difficulty--${item.question.difficulty}`}>
                                        {difficultyLabels[item.question.difficulty]}
                                      </span>
                                    )}
                                  </td>
                                  <td>{item.score} 分</td>
                                  <td className="exam-inline-actions">
                                    <Button
                                      aria-label={`屏蔽题目 ${item.questionID}`}
                                      className="exam-paper-editor__action-button--regular"
                                      onClick={() => void handleBlockSmartQuestion(item)}
                                      type="button"
                                      variant="actionReset"
                                    >
                                      屏蔽
                                    </Button>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </section>
                  ))}
                </div>
              )}
            </section>
          </div>
          )}

          <div className="exam-paper-editor__footer">
            <div className="exam-paper-editor__footer-note">
              <span>{lastSavedLabel}</span>
              <p>发布能力将与考试发布流程联动，当前版本先完成真实组卷与预览。</p>
            </div>
            <div className="exam-paper-editor__footer-actions">
              <Button disabled={isSavingDraft} type="submit" variant="secondary">
                保存草稿
              </Button>
              <Button
                disabled={!canAssemble || (!isSmartPaper && selectedQuestionCount === 0)}
                onClick={() => {
                  if (isSmartPaper) {
                    void handleGenerateSmartPaper();
                    return;
                  }
                  setIsPreviewOpen(true);
                }}
                type="button"
                variant={isSmartPaper ? "toolbarPrimary" : "toolbarSecondary"}
              >
                {isSmartPaper ? "一键智能组卷" : "生成预览"}
              </Button>
              <Button disabled variant="toolbarPrimary" type="button">
                发布试卷
              </Button>
            </div>
          </div>
        </form>
      </Panel>

      {previewDialog}
    </section>
  );
}

function FilterSelect({
  ariaLabel,
  label,
  onChange,
  options,
  value,
}: {
  ariaLabel: string;
  label: string;
  onChange: (value: string) => void;
  options: FilterSelectOption[];
  value: string;
}) {
  return (
    <div className="exam-paper-editor__filter-select">
      <span className="exam-paper-editor__filter-label">{label}</span>
      <Select ariaLabel={ariaLabel} onChange={onChange} options={options} value={value} />
    </div>
  );
}

function buildSelectedGroups(
  sections: PaperSectionRow[],
  sectionQuestions: ManualQuestionRow[],
  questionMap: Map<number, QuestionRow>,
): SelectedQuestionGroup[] {
  return [...sections]
    .sort((left, right) => left.sortOrder - right.sortOrder)
    .map((section) => ({
      section,
      questions: sectionQuestions
        .filter((item) => item.sectionID === section.id)
        .sort((left, right) => left.sortOrder - right.sortOrder)
        .map((item) => ({ ...item, question: questionMap.get(item.questionID) ?? null })),
    }))
    .filter((group) => group.questions.length > 0);
}

function moveQuestionOrder(
  items: ManualQuestionRow[],
  draggedQuestion: DraggedQuestion,
  targetQuestion: ManualQuestionRow,
) {
  const siblings = items
    .filter((item) => item.sectionID === draggedQuestion.sectionID)
    .sort((left, right) => left.sortOrder - right.sortOrder);
  const currentIndex = siblings.findIndex((item) => item.questionID === draggedQuestion.questionID);
  const targetIndex = siblings.findIndex((item) => item.questionID === targetQuestion.questionID);
  if (currentIndex < 0 || targetIndex < 0 || currentIndex === targetIndex) {
    return items;
  }

  const reordered = [...siblings];
  const [movedItem] = reordered.splice(currentIndex, 1);
  reordered.splice(targetIndex, 0, movedItem);
  const nextSortOrders = new Map(reordered.map((item, index) => [item.questionID, index + 1]));

  return items.map((item) => {
    if (item.sectionID !== draggedQuestion.sectionID) {
      return item;
    }
    return { ...item, sortOrder: nextSortOrders.get(item.questionID) ?? item.sortOrder };
  });
}

function buildSmartRuleDrafts(
  sections: PaperSectionRow[],
  sectionQuestions: ManualQuestionRow[],
  rules: PaperRuleRow[],
): Record<number, SmartRuleDraft> {
  const ruleBySection = new Map(rules.map((rule) => [rule.sectionID, rule]));
  return Object.fromEntries(sections.map((section) => {
    const rule = ruleBySection.get(section.id);
    const draft = rule === undefined
      ? defaultRuleDraftForSection(section, sectionQuestions)
      : {
        ruleID: rule.id,
        questionCount: String(rule.questionCount),
        scorePerQuestion: rule.scorePerQuestion,
      };
    return [section.id, draft];
  }));
}

function defaultRuleDraftForSection(section: PaperSectionRow, sectionQuestions: ManualQuestionRow[]): SmartRuleDraft {
  const questions = sectionQuestions.filter((item) => item.sectionID === section.id);
  const firstScore = questions[0]?.score ?? "5";
  return {
    questionCount: String(questions.length || section.questionCount || 0),
    scorePerQuestion: firstScore,
  };
}

function clamp(value: number, min: number, max: number) {
  return Math.min(Math.max(value, min), max);
}

function cloneSectionQuestion(item: ManualQuestionRow): ManualQuestionRow {
  return { ...item };
}

function cloneSection(item: PaperSectionRow): PaperSectionRow {
  return { ...item };
}

function normalizeSectionOrders(items: PaperSectionRow[]) {
  return items.map((item, index) => ({ ...item, sortOrder: index + 1 }));
}

function serializeSections(items: PaperSectionRow[]) {
  return [...items]
    .sort((left, right) => left.id - right.id)
    .map((item) => `${item.id}:${item.sortOrder}`)
    .join("|");
}

function serializeSectionQuestions(items: ManualQuestionRow[]) {
  return [...items]
    .sort((left, right) => {
      if (left.sectionID !== right.sectionID) {
        return left.sectionID - right.sectionID;
      }
      return left.questionID - right.questionID;
    })
    .map((item) => `${item.sectionID}:${item.questionID}:${item.sortOrder}:${item.score}`)
    .join("|");
}

function serializeSmartRuleState({
  sections,
  ruleDrafts,
  questionScope,
  selectedTags,
  difficultyPercentages,
  prioritizeQuality,
  excludeRecentExamQuestions,
  excludeUsedQuestions,
}: {
  sections: PaperSectionRow[];
  ruleDrafts: Record<number, SmartRuleDraft>;
  questionScope: SmartQuestionScope;
  selectedTags: string[];
  difficultyPercentages: SmartDifficultyPercentages;
  prioritizeQuality: boolean;
  excludeRecentExamQuestions: boolean;
  excludeUsedQuestions: boolean;
}) {
  const normalizedTags = questionScope === "tag_filter" ? [...selectedTags].sort() : [];
  const rules = [...sections]
    .sort((left, right) => left.id - right.id)
    .map((section) => {
      const draft = ruleDrafts[section.id];
      return [
        section.id,
        section.sortOrder,
        draft?.ruleID ?? "",
        draft?.questionCount ?? "",
        draft?.scorePerQuestion.trim() ?? "",
      ].join(":");
    });

  return JSON.stringify({
    rules,
    questionScope,
    selectedTags: normalizedTags,
    difficultyPercentages,
    prioritizeQuality,
    excludeRecentExamQuestions,
    excludeUsedQuestions,
  });
}

function collectChangedQuestionOrders(
  currentItems: ManualQuestionRow[],
  persistedItems: ManualQuestionRow[],
) {
  const persistedMap = new Map(
    persistedItems.map((item) => [toQuestionKey(item), item]),
  );
  return currentItems
    .filter((item) => {
      const persisted = persistedMap.get(toQuestionKey(item));
      return persisted !== undefined && (
        persisted.sortOrder !== item.sortOrder
        || persisted.score !== item.score
      );
    })
    .sort((left, right) => {
      if (left.sectionID !== right.sectionID) {
        return left.sectionID - right.sectionID;
      }
      return left.sortOrder - right.sortOrder;
    });
}

function collectChangedSectionOrders(
  currentItems: PaperSectionRow[],
  persistedItems: PaperSectionRow[],
) {
  const persistedMap = new Map(
    persistedItems.map((item) => [item.id, item]),
  );
  return [...currentItems]
    .sort((left, right) => left.sortOrder - right.sortOrder)
    .filter((item) => {
      const persisted = persistedMap.get(item.id);
      return persisted !== undefined && persisted.sortOrder !== item.sortOrder;
    })
    .map((item) => ({
      sectionID: item.id,
      sortOrder: item.sortOrder,
    }));
}

function moveSectionOrder(items: PaperSectionRow[], draggedSectionID: number, targetSectionID: number) {
  const ordered = [...items].sort((left, right) => left.sortOrder - right.sortOrder);
  const currentIndex = ordered.findIndex((item) => item.id === draggedSectionID);
  const targetIndex = ordered.findIndex((item) => item.id === targetSectionID);
  if (currentIndex < 0 || targetIndex < 0 || currentIndex === targetIndex) {
    return items;
  }

  const reordered = [...ordered];
  const [movedSection] = reordered.splice(currentIndex, 1);
  reordered.splice(targetIndex, 0, movedSection);
  return reordered.map((item, index) => ({ ...item, sortOrder: index + 1 }));
}

function formatSectionTitle(section: PaperSectionRow, index: number) {
  return `${toChineseSectionIndex(index + 1)}、${stripSectionPrefix(section.name)}`;
}

function defaultSectionName(questionType: QuestionType) {
  return stripSectionPrefix(sectionTemplates[questionType].name);
}

function editorBuildModeLabel(buildMode: PaperBuildMode) {
  if (buildMode === "rule_fixed") {
    return "策略组卷";
  }
  if (buildMode === "rule_live") {
    return "实时抽题组卷";
  }
  return "手动组卷";
}

function stripSectionPrefix(name: string) {
  return name.replace(/^[一二三四五六七八九十]+、/, "") || name;
}

function toChineseSectionIndex(value: number) {
  const numbers = ["零", "一", "二", "三", "四", "五", "六", "七", "八", "九"];
  if (value <= 10) {
    return value === 10 ? "十" : numbers[value];
  }
  if (value < 20) {
    return `十${numbers[value - 10]}`;
  }
  const tens = Math.floor(value / 10);
  const units = value % 10;
  return `${numbers[tens]}十${units === 0 ? "" : numbers[units]}`;
}

function buildSectionClassName(
  sectionID: number,
  draggingSectionID: number | null,
  sectionDropTargetID: number | null,
  sectionDropDirection: DropIndicatorDirection | null,
) {
  const classes = ["exam-paper-editor__selected-section"];
  if (draggingSectionID === sectionID) {
    classes.push("exam-paper-editor__selected-section--dragging");
  }
  if (sectionDropTargetID === sectionID && sectionDropDirection !== null) {
    classes.push(`exam-paper-editor__selected-section--drop-${sectionDropDirection}`);
  }
  return classes.join(" ");
}

function buildRuleRowClassName(
  sectionID: number,
  draggingSectionID: number | null,
  sectionDropTargetID: number | null,
  sectionDropDirection: DropIndicatorDirection | null,
) {
  const classes = ["exam-paper-editor__rule-row"];
  if (draggingSectionID === sectionID) {
    classes.push("exam-paper-editor__rule-row--dragging");
  }
  if (sectionDropTargetID === sectionID && sectionDropDirection !== null) {
    classes.push(`exam-paper-editor__rule-row--drop-${sectionDropDirection}`);
  }
  return classes.join(" ");
}

function buildQuestionRowClassName(
  item: Pick<ManualQuestionRow, "sectionID" | "questionID">,
  draggingQuestion: DraggedQuestion | null,
  questionDropTargetKey: string | null,
  questionDropDirection: DropIndicatorDirection | null,
) {
  const classes: string[] = [];
  if (draggingQuestion?.sectionID === item.sectionID && draggingQuestion.questionID === item.questionID) {
    classes.push("exam-paper-editor__selected-row--dragging");
  }
  if (questionDropTargetKey === toQuestionKey(item) && questionDropDirection !== null) {
    classes.push(`exam-paper-editor__selected-row--drop-${questionDropDirection}`);
  }
  return classes.length > 0 ? classes.join(" ") : undefined;
}

function detectSectionDropDirection(
  sections: PaperSectionRow[],
  draggingSectionID: number,
  targetSectionID: number,
): DropIndicatorDirection {
  const ordered = [...sections].sort((left, right) => left.sortOrder - right.sortOrder);
  const draggingIndex = ordered.findIndex((item) => item.id === draggingSectionID);
  const targetIndex = ordered.findIndex((item) => item.id === targetSectionID);
  return draggingIndex < targetIndex ? "up" : "down";
}

function detectQuestionDropDirection(
  sectionQuestions: ManualQuestionRow[],
  draggingQuestion: DraggedQuestion,
  targetQuestion: ManualQuestionRow,
): DropIndicatorDirection {
  const siblings = sectionQuestions
    .filter((item) => item.sectionID === targetQuestion.sectionID)
    .sort((left, right) => left.sortOrder - right.sortOrder);
  const draggingIndex = siblings.findIndex((item) => item.questionID === draggingQuestion.questionID);
  const targetIndex = siblings.findIndex((item) => item.questionID === targetQuestion.questionID);
  return draggingIndex < targetIndex ? "up" : "down";
}

function toQuestionKey(item: Pick<ManualQuestionRow, "sectionID" | "questionID">) {
  return `${item.sectionID}-${item.questionID}`;
}

function configureDragPreview(event: React.DragEvent<HTMLElement>, dragRoot: Element | null) {
  if (event.dataTransfer) {
    event.dataTransfer.setData("text/plain", "papermind-drag");
    event.dataTransfer.effectAllowed = "move";
  }

  if (!(dragRoot instanceof HTMLElement)) {
    return;
  }

  dragRoot.classList.add("exam-paper-editor__drag-preview");
  event.dataTransfer?.setDragImage(dragRoot, 24, 24);
  requestAnimationFrame(() => {
    dragRoot.classList.remove("exam-paper-editor__drag-preview");
  });
}

function isNonNegativeScore(value: string) {
  if (value.trim() === "") {
    return false;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0;
}

function paperStatusTone(status: PaperRow["status"]) {
  switch (status) {
    case "enabled":
      return "success";
    case "disabled":
      return "danger";
    default:
      return "info";
  }
}

function paperStatusLabel(status: PaperRow["status"]) {
  switch (status) {
    case "enabled":
      return "已发布";
    case "disabled":
      return "已禁用";
    default:
      return "未发布";
  }
}

function scopedPaperSearch(currentSearch: string, spaceID: number | undefined) {
  if (spaceID !== undefined) {
    return currentSearch;
  }
  const params = new URLSearchParams(currentSearch);
  params.delete("space_id");
  const query = params.toString();
  return query ? `?${query}` : "";
}
