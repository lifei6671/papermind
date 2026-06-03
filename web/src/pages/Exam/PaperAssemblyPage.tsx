import { Search } from "lucide-react";
import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import type { PaperAssemblyAPI, PaperBuildMode, PaperRow, PaperRuleRow, PaperSectionRow } from "../../api/papers";
import { paperApi } from "../../api/papers";
import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { Panel } from "../../components/ui/Panel";
import { RefreshIcon } from "../../components/ui/RefreshIcon";
import { withRefreshFeedback } from "../../components/ui/refreshFeedback";
import { StatusBadge } from "../../components/ui/StatusBadge";

type PaperSubMenu = "papers" | "rules";

type EditableRuleState = {
  ruleID: number;
  sectionID: number;
  sortOrder: number;
  count: number;
  tag: string;
  scorePerQuestion: string;
  difficulty?: string;
  shuffleOptions?: boolean;
};

type PaperAssemblyPageProps = {
  api?: PaperAssemblyAPI;
  tenantID?: number;
  spaceID?: number;
};

const paperBuildModeOptions: Array<{
  value: PaperBuildMode;
  label: string;
  hint: string;
}> = [
  { value: "manual", label: "手动组卷", hint: "教师逐题确认，适合精细编排试卷结构。" },
  { value: "rule_fixed", label: "固定规则组卷", hint: "按规则先生成试卷，再进行审题和替换。" },
  { value: "rule_live", label: "实时抽题组卷", hint: "考试开始时再抽题，适合动态题池场景。" },
];

export function PaperAssemblyPage({
  api = paperApi,
  tenantID = 10,
  spaceID,
}: PaperAssemblyPageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [papers, setPapers] = useState<PaperRow[]>([]);
  const [activeTab, setActiveTab] = useState<PaperSubMenu>("papers");
  const [sections, setSections] = useState<PaperSectionRow[]>([]);
  const [rules, setRules] = useState<PaperRuleRow[]>([]);
  const [activePaperID, setActivePaperID] = useState<number | null>(null);
  const [sectionName, setSectionName] = useState("");
  const [sectionScore, setSectionScore] = useState("");
  const [fixedCount, setFixedCount] = useState(4);
  const [fixedTag, setFixedTag] = useState("阅读理解");
  const [fixedResult, setFixedResult] = useState("");
  const [fixedReview, setFixedReview] = useState("");
  const [liveCount, setLiveCount] = useState(4);
  const [liveTag, setLiveTag] = useState("阅读理解");
  const [liveResult, setLiveResult] = useState("");
  const [precheckMessage, setPrecheckMessage] = useState("");
  const [editableRule, setEditableRule] = useState<EditableRuleState | null>(null);
  const [isSectionDialogOpen, setIsSectionDialogOpen] = useState(false);
  const [isCreatePaperDialogOpen, setIsCreatePaperDialogOpen] = useState(false);
  const [isFixedDialogOpen, setIsFixedDialogOpen] = useState(false);
  const [isLiveDialogOpen, setIsLiveDialogOpen] = useState(false);
  const [newPaperName, setNewPaperName] = useState("");
  const [newPaperDescription, setNewPaperDescription] = useState("");
  const [newPaperBuildMode, setNewPaperBuildMode] = useState<PaperBuildMode>("manual");
  const [newPaperDurationText, setNewPaperDurationText] = useState("120");
  const [newPaperGradeText, setNewPaperGradeText] = useState("高一");
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [loadError, setLoadError] = useState("");
  const [isPaperListRefreshing, setIsPaperListRefreshing] = useState(false);
  const [isCreatingPaper, setIsCreatingPaper] = useState(false);

  const activePaper = papers.find((paper) => paper.id === activePaperID) ?? null;
  const activeBuildMode = activePaper?.buildMode ?? "manual";

  useEffect(() => {
    let ignore = false;

    api.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then((data) => {
        if (ignore) {
          return;
        }
        setPapers(data.items);
        setActivePaperID((current) => {
          if (current !== null && data.items.some((paper) => paper.id === current)) {
            return current;
          }
          return data.items[0]?.id ?? null;
        });
        if (!data.items[0]) {
          setSections([]);
          setRules([]);
        }
        setLoadError("");
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("试卷数据加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, spaceID]);

  useEffect(() => {
    if (activePaperID === null) {
      return;
    }

    let ignore = false;
    void (async () => {
      try {
        const [sectionData, ruleData] = await Promise.all([
          api.listSections({ tenantID, paperID: activePaperID }),
          api.listRules({ tenantID, paperID: activePaperID }),
        ]);
        if (ignore) {
          return;
        }
        setSections(sectionData.items);
        setRules(ruleData.items);
        setLoadError("");
      } catch {
        if (!ignore) {
          setLoadError("试卷工作台加载失败");
        }
      }
    })();

    return () => {
      ignore = true;
    };
  }, [activePaperID, api, tenantID]);

  const filteredPapers = papers.filter((paper) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }
    return [paper.name, paper.creatorName, paper.buildMode, buildModeLabel(paper.buildMode), paperStatusLabel(paper.status)].some((value) =>
      value.toLowerCase().includes(keyword),
    );
  });

  async function loadWorkspace(paperID: number) {
    try {
      const [sectionData, ruleData] = await Promise.all([
        api.listSections({ tenantID, paperID }),
        api.listRules({ tenantID, paperID }),
      ]);
      setSections(sectionData.items);
      setRules(ruleData.items);
      setLoadError("");
    } catch {
      setLoadError("试卷工作台加载失败");
    }
  }

  async function reloadActiveWorkspace() {
    if (activePaperID === null) {
      return;
    }
    await loadWorkspace(activePaperID);
  }

  async function handleCreatePaper(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const name = newPaperName.trim();
    const durationMinutes = Number.parseInt(newPaperDurationText, 10);
    if (!name) {
      setLoadError("请输入试卷名称");
      return;
    }
    if (!Number.isFinite(durationMinutes) || durationMinutes <= 0) {
      setLoadError("考试时长必须是正整数");
      return;
    }

    setIsCreatingPaper(true);
    try {
      const createdPaper = await api.createPaper({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        name,
        description: newPaperDescription.trim(),
        durationMinutes,
        gradeLevel: newPaperGradeText.trim() || "高一",
      });
      if (newPaperBuildMode !== "manual") {
        await api.updateBuildMode({
          tenantID,
          paperID: createdPaper.id,
          buildMode: newPaperBuildMode,
        });
      }
      setIsCreatePaperDialogOpen(false);
      setNewPaperName("");
      setNewPaperDescription("");
      setNewPaperBuildMode("manual");
      setNewPaperDurationText("120");
      setNewPaperGradeText("高一");
      setLoadError("");
      navigateToPaperEditor(createdPaper.id);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "创建试卷失败");
    } finally {
      setIsCreatingPaper(false);
    }
  }

  async function handleAddSection(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (activePaperID === null) {
      setLoadError("请先选择试卷");
      return;
    }

    try {
      const section = await api.createSection({
        tenantID,
        paperID: activePaperID,
        name: sectionName,
        questionType: "single",
        instructions: sectionScore ? `计划 ${sectionScore} 分` : "",
      });
      setSections((items) => [...items, section]);
      setSectionName("");
      setSectionScore("");
      setIsSectionDialogOpen(false);
      setLoadError("");
    } catch {
      setLoadError("创建大题失败");
    }
  }

  async function handleGenerateFixedPaper(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (activePaperID === null || sections.length === 0) {
      setLoadError("请先创建试卷大题");
      return;
    }

    try {
      const targetSection = sections[0];
      const existingRule = rules.find((item) => item.sectionID === targetSection.id) ?? null;
      const staleRules = rules.filter((item) => existingRule === null ? true : item.id !== existingRule.id);
      const tagIDs = [tagIDByName(fixedTag)];
      const scorePerQuestion = defaultRuleScorePerQuestion(targetSection);
      const rule = existingRule === null
        ? await api.createRule({
            tenantID,
            paperID: activePaperID,
            sectionID: targetSection.id,
            sortOrder: rules.length + 1,
            tagIDs,
            questionCount: fixedCount,
            scorePerQuestion,
          })
        : await api.updateRule({
            tenantID,
            paperID: activePaperID,
            ruleID: existingRule.id,
            sectionID: targetSection.id,
            sortOrder: existingRule.sortOrder,
            difficulty: existingRule.difficulty,
            tagIDs,
            questionCount: fixedCount,
            scorePerQuestion,
            shuffleOptions: existingRule.shuffleOptions,
          });
      for (const staleRule of staleRules) {
        await api.deleteRule({ tenantID, paperID: activePaperID, ruleID: staleRule.id });
      }
      await api.generateRuleFixed({ tenantID, paperID: activePaperID });
      await syncPaperBuildMode("rule_fixed");
      setFixedResult(`已按${tagNameByID(rule.tagIDs[0])}生成 ${rule.questionCount} 道题`);
      setFixedReview("待替换低匹配题");
      setIsFixedDialogOpen(false);
      setLoadError("");
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "生成固定规则试卷失败");
    }
  }

  async function handleSaveLiveRule(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (activePaperID === null || sections.length === 0) {
      setLoadError("请先创建试卷大题");
      return;
    }

    try {
      const rule = await api.createRule({
        tenantID,
        paperID: activePaperID,
        sectionID: sections[0].id,
        sortOrder: rules.length + 1,
        tagIDs: [tagIDByName(liveTag)],
        questionCount: liveCount,
        scorePerQuestion: defaultRuleScorePerQuestion(sections[0]),
      });
      await reloadActiveWorkspace();
      setLiveResult(`rule_live 已保存：${tagNameByID(rule.tagIDs[0])} ${rule.questionCount} 道题`);
      setIsLiveDialogOpen(false);
      setLoadError("");
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "保存 rule_live 规则失败");
    }
  }

  async function handlePrecheckRuleLive() {
    if (activePaperID === null) {
      setLoadError("请先选择试卷");
      return;
    }

    try {
      const result = await api.precheckRuleLive({ tenantID, paperID: activePaperID });
      setPrecheckMessage(`预检查通过，候选题池 ${result.candidateCount} 道题`);
      setLoadError("");
    } catch (err) {
      setPrecheckMessage("");
      setLoadError(err instanceof Error ? err.message : "组卷预检查失败");
    }
  }

  async function syncPaperBuildMode(nextBuildMode: PaperBuildMode) {
    if (activePaperID === null) {
      return;
    }
    if (activePaper !== null && activePaper.buildMode !== nextBuildMode) {
      throw new Error("已创建试卷不能修改组卷方式");
    }
    if (activePaper !== null && activePaper.buildMode === nextBuildMode) {
      return;
    }
    const nextPaper = await api.updateBuildMode({ tenantID, paperID: activePaperID, buildMode: nextBuildMode });
    setPapers((items) => items.map((paper) => (paper.id === nextPaper.id ? nextPaper : paper)));
    await reloadActiveWorkspace();
  }

  async function handleDeleteRule(ruleID: number) {
    if (activePaperID === null) {
      return;
    }
    try {
      await api.deleteRule({ tenantID, paperID: activePaperID, ruleID });
      await reloadActiveWorkspace();
      setLoadError("");
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "删除规则失败");
    }
  }

  async function handleSaveEditableRule(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (editableRule === null || activePaperID === null) {
      return;
    }

    try {
      await api.updateRule({
        tenantID,
        paperID: activePaperID,
        ruleID: editableRule.ruleID,
        sectionID: editableRule.sectionID,
        sortOrder: editableRule.sortOrder,
        difficulty: editableRule.difficulty,
        tagIDs: [tagIDByName(editableRule.tag)],
        questionCount: editableRule.count,
        scorePerQuestion: editableRule.scorePerQuestion,
        shuffleOptions: editableRule.shuffleOptions,
      });
      await reloadActiveWorkspace();
      setEditableRule(null);
      setLoadError("");
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "保存规则失败");
    }
  }

  function handleSearchPapers() {
    setAppliedSearchQuery(searchQuery);
  }

  async function handleRefreshPapers() {
    setSearchQuery("");
    setAppliedSearchQuery("");
    setIsPaperListRefreshing(true);
    try {
      const data = await withRefreshFeedback(api.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) }));
      setPapers(data.items);
      setActivePaperID((current) => {
        if (current !== null && data.items.some((paper) => paper.id === current)) {
          return current;
        }
        return data.items[0]?.id ?? null;
      });
      if (!data.items[0]) {
        setSections([]);
        setRules([]);
      }
      setLoadError("");
    } catch {
      setLoadError("试卷数据加载失败");
    } finally {
      setIsPaperListRefreshing(false);
    }
  }

  function navigateToPaperEditor(target: number) {
    const path = `/papers/${target}/edit`;
    navigate(`${path}${scopedPaperSearch(location.search, spaceID)}`);
  }

  async function handlePreviewPaper(paperID: number) {
    setActivePaperID(paperID);
    await loadWorkspace(paperID);
    setActiveTab("rules");
  }

  async function handleTogglePaperStatus(paper: PaperRow) {
    try {
      const nextPaper = paper.status === "enabled"
        ? await api.disablePaper({ tenantID, paperID: paper.id })
        : await api.enablePaper({ tenantID, paperID: paper.id });
      setPapers((items) => items.map((item) => (item.id === nextPaper.id ? nextPaper : item)));
      setLoadError("");
    } catch {
      setLoadError(paper.status === "enabled" ? "禁用试卷失败" : "启用试卷失败");
    }
  }

  const paperOverviewList = (
    <div
      className={[
        "table-wrap",
        "tenant-list-transition",
        isPaperListRefreshing ? "tenant-list-transition--refreshing" : "",
      ].filter(Boolean).join(" ")}
    >
      <table className="data-table tenant-admin-table exam-paper-overview-table">
        <thead>
          <tr>
            <th scope="col">试卷名称</th>
            <th scope="col">策略</th>
            <th scope="col">试卷分数</th>
            <th scope="col">状态</th>
            <th scope="col">创建时间</th>
            <th scope="col">创建人</th>
            <th scope="col">操作区</th>
          </tr>
        </thead>
        <tbody>
          {filteredPapers.length === 0 && <EmptyTableRow colSpan={7} />}
          {filteredPapers.map((paper) => (
            <tr key={paper.id}>
              <td>{paper.name}</td>
              <td>
                <span className={`exam-paper-mode-badge exam-paper-mode-badge--${paper.buildMode}`}>
                  {buildModeLabel(paper.buildMode)}
                </span>
              </td>
              <td>{paper.totalScore}</td>
              <td>
                <StatusBadge tone={paperStatusTone(paper.status)}>
                  {paperStatusLabel(paper.status)}
                </StatusBadge>
              </td>
              <td>{formatPaperCreatedAt(paper.createdAt)}</td>
              <td>{paper.creatorName || "-"}</td>
              <td>
                <div className="tenant-actions">
                  <Button
                    aria-label={`编辑试卷 ${paper.name}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      navigateToPaperEditor(paper.id);
                    }}
                    variant="actionEdit"
                  >
                    编辑
                  </Button>
                  <Button
                    aria-label={`预览试卷 ${paper.name}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      void handlePreviewPaper(paper.id);
                    }}
                    variant="actionReset"
                  >
                    预览
                  </Button>
                  <Button
                    aria-label={`${paper.status === "enabled" ? "禁用试卷" : "启用试卷"} ${paper.name}`}
                    onClick={(event) => {
                      event.stopPropagation();
                      void handleTogglePaperStatus(paper);
                    }}
                    variant="actionClose"
                  >
                    {paper.status === "enabled" ? "禁用" : "启用"}
                  </Button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  const paperWorkspaceList = (
    <div className="table-wrap">
      <table className="data-table tenant-admin-table exam-paper-list-table">
        <thead>
          <tr>
            <th scope="col">试卷</th>
            <th scope="col">策略</th>
            <th scope="col">状态</th>
          </tr>
        </thead>
        <tbody>
          {filteredPapers.length === 0 && <EmptyTableRow colSpan={3} />}
          {filteredPapers.map((paper) => (
            <tr
              aria-selected={activePaperID === paper.id}
              className={activePaperID === paper.id ? "exam-paper-row exam-paper-row--active" : "exam-paper-row"}
              key={paper.id}
              onClick={() => setActivePaperID(paper.id)}
            >
              <td>{paper.name}</td>
              <td>
                <span className={`exam-paper-mode-badge exam-paper-mode-badge--${paper.buildMode}`}>
                  {buildModeLabel(paper.buildMode)}
                </span>
              </td>
              <td>
                <StatusBadge tone={paperStatusTone(paper.status)}>
                  {paperStatusLabel(paper.status)}
                </StatusBadge>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  return (
    <section className="page platform-page exam-builder-page">
      <nav aria-label="试卷菜单" className="platform-tabbar" role="tablist">
        {[
          ["papers", "试卷"],
          ["rules", "组卷规则"],
        ].map(([tab, label]) => (
          <button
            aria-selected={activeTab === tab}
            className={activeTab === tab ? "platform-tab platform-tab--active" : "platform-tab"}
            key={tab}
            onClick={() => setActiveTab(tab as PaperSubMenu)}
            role="tab"
            type="button"
          >
            {label}
          </button>
        ))}
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="试卷操作区">
            {activeTab === "papers" ? (
              <Button variant="toolbarPrimary" onClick={() => setIsCreatePaperDialogOpen(true)} type="button">
                新建试卷
              </Button>
            ) : (
              <>
                <Button variant="toolbarPrimary" onClick={() => setIsSectionDialogOpen(true)} type="button">
                  新增大题
                </Button>
                <div className="exam-paper-mode-field field" aria-label="当前组卷方式">
                  <span className="exam-paper-mode-field__label">组卷模式</span>
                  <strong className="exam-paper-mode-field__value">{buildModeLabel(activeBuildMode)}</strong>
                  <span className="exam-paper-mode-field__hint">{buildModeHint(activeBuildMode)}</span>
                </div>
              </>
            )}
          </div>
          <div className="tenant-search-actions">
            <label className="tenant-search-field">
              <span className="sr-only">搜索试卷</span>
              <input
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="输入试卷名称、策略或状态"
                value={searchQuery}
              />
            </label>
            <Button aria-label="搜索" variant="icon" onClick={handleSearchPapers} type="button">
              <Search aria-hidden="true" size={16} />
            </Button>
            <Button
              aria-label="刷新试卷列表"
              variant="icon"
              disabled={isPaperListRefreshing}
              onClick={() => void handleRefreshPapers()}
              type="button"
            >
              <RefreshIcon active={isPaperListRefreshing} />
            </Button>
          </div>
        </div>

        {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}

        {activeTab === "papers" ? (
          paperOverviewList
        ) : (
          <div className="exam-paper-workspace">
            <div className="exam-paper-workspace__column">
              {paperWorkspaceList}
            </div>

            <div className="exam-paper-workspace__column exam-paper-workspace__column--wide">
              <div className="exam-workspace-card">
                <div className="exam-workspace-card__head">
                  <h2>规则列表</h2>
                  <span>{activePaper ? `当前试卷：${activePaper.name}` : "请选择试卷"}</span>
                </div>
                <div className="tenant-list-toolbar">
                  <div className="tenant-list-actions" aria-label="组卷规则操作区">
                    <Button variant="toolbarPrimary" onClick={() => setIsFixedDialogOpen(true)} type="button">
                      生成固定规则试卷
                    </Button>
                    <Button variant="toolbarSecondary" onClick={() => setIsLiveDialogOpen(true)} type="button">
                      保存 rule_live 规则
                    </Button>
                    <Button variant="toolbarSecondary" onClick={() => void handlePrecheckRuleLive()} type="button">
                      运行组卷预检查
                    </Button>
                  </div>
                </div>
                {fixedResult && <div aria-label="rule-fixed-result" className="tenant-admin-status" role="status">{fixedResult}</div>}
                {fixedResult && (
                  <Button
                    variant="toolbarSecondary"
                    onClick={() => setFixedReview("已替换 1 道低匹配题")}
                    type="button"
                  >
                    替换低匹配题
                  </Button>
                )}
                {fixedReview && <div aria-label="rule-fixed-review" className="tenant-admin-status" role="status">{fixedReview}</div>}
                {liveResult && <div aria-label="rule-live-result" className="tenant-admin-status" role="status">{liveResult}</div>}
                {precheckMessage && <div className="tenant-admin-warning" role="alert">{precheckMessage}</div>}

                <div className="table-wrap">
                  <table className="data-table tenant-admin-table">
                    <thead>
                      <tr>
                        <th scope="col">规则 ID</th>
                        <th scope="col">标签</th>
                        <th scope="col">题量</th>
                        <th scope="col">每题分值</th>
                        <th scope="col">操作</th>
                      </tr>
                    </thead>
                    <tbody>
                      {rules.length === 0 && <EmptyTableRow colSpan={5} />}
                      {rules.map((rule) => (
                        <tr key={rule.id}>
                          <td>{rule.id}</td>
                          <td>{tagNameByID(rule.tagIDs[0])}</td>
                          <td>{rule.questionCount}</td>
                          <td>{rule.scorePerQuestion}</td>
                          <td className="exam-inline-actions">
                            <Button
                              aria-label={`编辑规则 ${rule.id}`}
                              variant="actionEdit"
                              onClick={() => setEditableRule({
                                ruleID: rule.id,
                                sectionID: rule.sectionID,
                                sortOrder: rule.sortOrder,
                                count: rule.questionCount,
                                tag: tagNameByID(rule.tagIDs[0]),
                                scorePerQuestion: rule.scorePerQuestion,
                                difficulty: rule.difficulty,
                                shuffleOptions: rule.shuffleOptions,
                              })}
                              type="button"
                            >
                              编辑
                            </Button>
                            <Button
                              aria-label={`删除规则 ${rule.id}`}
                              variant="actionClose"
                              onClick={() => void handleDeleteRule(rule.id)}
                              type="button"
                            >
                              删除
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>
        )}
      </Panel>

      {isCreatePaperDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="新建试卷">
          <div className="platform-dialog__card">
            <h2>新建试卷</h2>
            <form className="platform-form" onSubmit={handleCreatePaper}>
              <label className="field">
                <span>试卷名称</span>
                <input
                  aria-label="试卷名称"
                  onChange={(event) => setNewPaperName(event.target.value)}
                  required
                  value={newPaperName}
                />
              </label>
              <label className="field">
                <span>组卷方式</span>
                <select
                  aria-label="组卷方式"
                  onChange={(event) => setNewPaperBuildMode(event.target.value as PaperBuildMode)}
                  value={newPaperBuildMode}
                >
                  {paperBuildModeOptions.map((option) => (
                    <option key={option.value} value={option.value}>{option.label}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                <span>考试时长</span>
                <input
                  aria-label="考试时长"
                  min={1}
                  onChange={(event) => setNewPaperDurationText(event.target.value)}
                  required
                  type="number"
                  value={newPaperDurationText}
                />
              </label>
              <label className="field">
                <span>适用年级</span>
                <input
                  aria-label="适用年级"
                  onChange={(event) => setNewPaperGradeText(event.target.value)}
                  required
                  value={newPaperGradeText}
                />
              </label>
              <label className="field">
                <span>试卷说明</span>
                <input
                  aria-label="试卷说明"
                  onChange={(event) => setNewPaperDescription(event.target.value)}
                  value={newPaperDescription}
                />
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsCreatePaperDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" disabled={isCreatingPaper} type="submit">
                  保存并组卷
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {isSectionDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="新增大题弹窗">
          <div className="platform-dialog__card">
            <h2>新增大题</h2>
            <form className="platform-form" onSubmit={handleAddSection}>
              <label className="field">
                <span>大题名称</span>
                <input onChange={(event) => setSectionName(event.target.value)} required value={sectionName} />
              </label>
              <label className="field">
                <span>大题分值</span>
                <input min={1} onChange={(event) => setSectionScore(event.target.value)} required type="number" value={sectionScore} />
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsSectionDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认新增
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {isFixedDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="rule_fixed 弹窗">
          <div className="platform-dialog__card">
            <h2>生成固定规则试卷</h2>
            <form className="platform-form" onSubmit={handleGenerateFixedPaper}>
              <label className="field">
                <span>固定规则题量</span>
                <input min={1} onChange={(event) => setFixedCount(Number(event.target.value))} type="number" value={fixedCount} />
              </label>
              <label className="field">
                <span>固定规则标签</span>
                <select onChange={(event) => setFixedTag(event.target.value)} value={fixedTag}>
                  <option value="阅读理解">阅读理解</option>
                  <option value="语言文字">语言文字</option>
                </select>
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsFixedDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认生成
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {isLiveDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="rule_live 弹窗">
          <div className="platform-dialog__card">
            <h2>保存 rule_live 规则</h2>
            <form className="platform-form" onSubmit={handleSaveLiveRule}>
              <label className="field">
                <span>动态规则题量</span>
                <input min={1} onChange={(event) => setLiveCount(Number(event.target.value))} type="number" value={liveCount} />
              </label>
              <label className="field">
                <span>动态规则标签</span>
                <select onChange={(event) => setLiveTag(event.target.value)} value={liveTag}>
                  <option value="阅读理解">阅读理解</option>
                  <option value="语言文字">语言文字</option>
                </select>
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsLiveDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认保存
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}

      {editableRule && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label={`编辑规则 ${editableRule.ruleID}`}>
          <div className="platform-dialog__card">
            <h2>编辑组卷规则</h2>
            <form className="platform-form" onSubmit={handleSaveEditableRule}>
              <label className="field">
                <span>规则题量</span>
                <input
                  min={1}
                  onChange={(event) => setEditableRule((current) => (current === null ? current : { ...current, count: Number(event.target.value) }))}
                  type="number"
                  value={editableRule.count}
                />
              </label>
              <label className="field">
                <span>规则标签</span>
                <select
                  onChange={(event) => setEditableRule((current) => (current === null ? current : { ...current, tag: event.target.value }))}
                  value={editableRule.tag}
                >
                  <option value="阅读理解">阅读理解</option>
                  <option value="语言文字">语言文字</option>
                </select>
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setEditableRule(null)} type="button">
                  取消
                </Button>
                <Button variant="primary" type="submit">
                  确认保存规则
                </Button>
              </div>
            </form>
          </div>
        </div>
      )}
    </section>
  );
}

function tagIDByName(name: string): number {
  return name === "语言文字" ? 2 : 1;
}

function tagNameByID(id: number | undefined): string {
  return id === 2 ? "语言文字" : "阅读理解";
}

function defaultRuleScorePerQuestion(section: PaperSectionRow): string {
  if (section.questionCount <= 0) {
    return "4";
  }

  const totalScore = Number(section.totalScore);
  if (!Number.isFinite(totalScore) || totalScore <= 0) {
    return "4";
  }
  return String(totalScore / section.questionCount);
}

function buildModeLabel(mode: string): string {
  return paperBuildModeOptions.find((option) => option.value === mode)?.label ?? mode;
}

function buildModeHint(mode: string): string {
  return paperBuildModeOptions.find((option) => option.value === mode)?.hint ?? "按当前试卷策略继续维护试题与规则。";
}

function paperStatusLabel(status: PaperRow["status"]) {
  if (status === "enabled") {
    return "可用";
  }
  if (status === "disabled") {
    return "已禁用";
  }
  return "草稿";
}

function paperStatusTone(status: PaperRow["status"]) {
  if (status === "enabled") {
    return "success";
  }
  if (status === "disabled") {
    return "warning";
  }
  return "info";
}

function formatPaperCreatedAt(value: number) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  const pad = (item: number) => String(item).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
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
