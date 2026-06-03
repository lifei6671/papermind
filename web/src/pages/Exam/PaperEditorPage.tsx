import { Eye, FilePlus2, GripVertical, Sparkles, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { formatApiErrorMessage } from "../../api/client";
import type {
  ManualQuestionRow,
  PaperAPI,
  PaperBuildMode,
  PaperRow,
  PaperSectionRow,
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

const sectionTemplates: Record<QuestionType, { name: string; instructions: string }> = {
  single: { name: "一、单项选择题", instructions: "每题 5 分" },
  multiple: { name: "二、多项选择题", instructions: "每题 5 分" },
  judge: { name: "三、判断题", instructions: "每题 2 分" },
  fill_blank: { name: "四、填空题", instructions: "每题 5 分" },
  short_text: { name: "五、解答题", instructions: "按采分点给分" },
};

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
      };
    }

    const [paperData, sectionData, sectionQuestionData] = await Promise.all([
      providedPaperApi.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) }),
      providedPaperApi.listSections({ tenantID, paperID }),
      providedPaperApi.listSectionQuestions({ tenantID, paperID }),
    ]);
    return {
      currentPaper: paperData.items.find((item) => item.id === paperID) ?? null,
      nextSections: sectionData.items,
      nextSectionQuestions: sectionQuestionData.items,
    };
  }, [paperID, providedPaperApi, spaceID, tenantID]);

  const applyWorkspaceSnapshot = useCallback((
    currentPaper: PaperRow | null,
    nextSections: PaperSectionRow[],
    nextSectionQuestions: ManualQuestionRow[],
    nextQuestionPool: QuestionRow[],
  ) => {
    setQuestionPool(nextQuestionPool);
    setPaper(currentPaper);
    setPaperName(currentPaper?.name ?? "");
    setPaperDescription(currentPaper?.description ?? "");
    setGradeText(currentPaper?.gradeLevel?.trim() ? currentPaper.gradeLevel.trim() : "高一");
    setDurationText(String(currentPaper?.durationMinutes ?? 120));
    setBuildMode((currentPaper?.buildMode as PaperBuildMode | undefined) ?? "manual");
    setSections(nextSections.map((item) => ({ ...item })));
    setPersistedSections(nextSections.map((item) => ({ ...item })));
    setSectionQuestions(nextSectionQuestions.map(cloneSectionQuestion));
    setPersistedSectionQuestions(nextSectionQuestions.map(cloneSectionQuestion));
    setEditingScores({});
  }, []);

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
  const tags = useMemo(() => {
    const values = Array.from(new Set(questionPool.flatMap((item) => item.tags.length > 0 ? item.tags : [item.tag]).filter(Boolean)));
    values.sort((left, right) => left.localeCompare(right, "zh-Hans-CN"));
    return values;
  }, [questionPool]);

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
  const canAssemble = paper !== null;
  const isRuleLivePaper = buildMode === "rule_live";
  const sectionOrderDirty = serializeSections(sections) !== serializeSections(persistedSections);
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
      || buildMode !== ((paper.buildMode as PaperBuildMode | undefined) ?? "manual")
      || sectionOrderDirty
      || serializeSectionQuestions(sectionQuestions) !== serializeSectionQuestions(persistedSectionQuestions);
  }, [buildMode, durationText, gradeText, paper, paperDescription, paperName, persistedSectionQuestions, sectionOrderDirty, sectionQuestions]);
  const lastSavedLabel = paper === null
    ? "草稿尚未保存"
    : draftDirty ? "草稿有未保存调整" : "草稿已保存";

  async function handleSaveDraft(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
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
            buildModeSyncError = error
          }
        }
        navigate(`/papers/${nextPaper.id}/edit${pageSearch}`, { replace: true });
        if (buildModeSyncError !== null) {
          showError(`试卷草稿已创建，${formatApiErrorMessage(buildModeSyncError, "切换组卷方式失败")}`);
        } else {
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
        if (buildMode !== ((paper.buildMode as PaperBuildMode | undefined) ?? "manual")) {
          const updatedModePaper = await providedPaperApi.updateBuildMode({
            tenantID,
            paperID: paper.id,
            buildMode,
          });
          nextPaper = {
            ...nextPaper,
            ...updatedModePaper,
            name: paperName.trim(),
            description: paperDescription.trim(),
            buildMode,
          };
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
        setPaper(nextPaper);
        showSuccess("试卷草稿已保存。");
      }
    } catch (error) {
      showError(formatApiErrorMessage(error, "试卷草稿保存失败"));
    } finally {
      setIsSavingDraft(false);
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
    configureDragPreview(event, event.currentTarget.closest(".exam-paper-editor__selected-section"));
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
              <Button variant="toolbarPrimary" type="button">
                <FilePlus2 aria-hidden="true" size={16} />
                <span>新建组卷</span>
              </Button>
              <Button disabled variant="toolbarSecondary" type="button">
                <Sparkles aria-hidden="true" size={16} />
                <span>从模板创建</span>
              </Button>
              <Button
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

          <div className="exam-paper-editor__summarybar">
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
                onChange={(event) => setGradeText(event.target.value)}
                placeholder="高一"
                value={gradeText}
              />
            </label>
            <div className="exam-paper-editor__summary-item exam-paper-editor__summary-item--modes">
              <span>组卷方式：</span>
              <SegmentTabs
                active={buildMode === "manual" ? "手动组卷" : "智能组卷"}
                ariaLabel="组卷方式"
                items={["手动组卷", "智能组卷"]}
                onChange={(item) => setBuildMode(item === "手动组卷" ? "manual" : "rule_fixed")}
              />
            </div>
            <div className="exam-paper-editor__summary-item exam-paper-editor__summary-item--status">
              <span className="exam-paper-editor__status">
                <StatusBadge tone={paperStatusTone(paper?.status ?? "draft")}>
                  {paperStatusLabel(paper?.status ?? "draft")}
                </StatusBadge>
              </span>
            </div>
          </div>
          <label className="sr-only">
            <span>试卷说明</span>
            <input aria-label="试卷说明" onChange={(event) => setPaperDescription(event.target.value)} value={paperDescription} />
          </label>
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
                                      <span
                                        className="exam-paper-editor__selected-title"
                                        title={`${index + 1}. ${selectedTitle}`}
                                      >
                                        {index + 1}. {selectedTitle}
                                      </span>
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
                disabled={!canAssemble || selectedQuestionCount === 0}
                onClick={() => setIsPreviewOpen(true)}
                type="button"
                variant="toolbarSecondary"
              >
                生成预览
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

function cloneSectionQuestion(item: ManualQuestionRow): ManualQuestionRow {
  return { ...item };
}

function cloneSection(item: PaperSectionRow): PaperSectionRow {
  return { ...item };
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
