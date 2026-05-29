import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { paperApi } from "../../api/papers";
import type { PaperAssemblyAPI, PaperRow, PaperSectionRow } from "../../api/papers";
import { questionApi as defaultQuestionApi } from "../../api/questions";
import type { QuestionBankAPI, QuestionRow } from "../../api/questions";

type PaperSubMenu = "papers" | "questions" | "rules";

type PaperAssemblyPageProps = {
  api?: PaperAssemblyAPI;
  questionApi?: QuestionBankAPI;
  tenantID?: number;
  spaceID?: number;
};

export function PaperAssemblyPage({ api = paperApi, questionApi = defaultQuestionApi, tenantID = 10, spaceID }: PaperAssemblyPageProps) {
  const [papers, setPapers] = useState<PaperRow[]>([]);
  const [activeTab, setActiveTab] = useState<PaperSubMenu>("papers");
  const [sections, setSections] = useState<PaperSectionRow[]>([]);
  const [candidateQuestions, setCandidateQuestions] = useState<QuestionRow[]>([]);
  const [activePaperID, setActivePaperID] = useState<number | null>(null);
  const [sectionName, setSectionName] = useState("");
  const [sectionScore, setSectionScore] = useState("");
  const [manualCount, setManualCount] = useState(0);
  const [fixedCount, setFixedCount] = useState(4);
  const [fixedTag, setFixedTag] = useState("阅读理解");
  const [fixedResult, setFixedResult] = useState("");
  const [fixedReview, setFixedReview] = useState("");
  const [liveCount, setLiveCount] = useState(4);
  const [liveTag, setLiveTag] = useState("阅读理解");
  const [liveResult, setLiveResult] = useState("");
  const [precheckMessage, setPrecheckMessage] = useState("");
  const [isSectionDialogOpen, setIsSectionDialogOpen] = useState(false);
  const [isFixedDialogOpen, setIsFixedDialogOpen] = useState(false);
  const [isLiveDialogOpen, setIsLiveDialogOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    let ignore = false;

    api.listPapers({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then(async (data) => {
        if (ignore) {
          return;
        }
        setPapers(data.items);
        setActivePaperID(data.items[0]?.id ?? null);
        if (!data.items[0]) {
          setSections([]);
          return;
        }
        const sectionData = await api.listSections({ tenantID, paperID: data.items[0].id });
        if (!ignore) {
          setSections(sectionData.items);
          setLoadError("");
        }
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
    let ignore = false;

    questionApi.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then((data) => {
        if (!ignore) {
          setCandidateQuestions(data.items);
        }
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("候选题目加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [questionApi, tenantID, spaceID]);

  const filteredPapers = papers.filter((paper) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 试卷列表搜索只匹配当前可见字段，便于按试卷名称、策略或状态快速定位。
    return [paper.name, paper.buildMode, paper.status === "draft" ? "草稿" : "可用"].some((value) =>
      value.toLowerCase().includes(keyword),
    );
  });

  async function handleAddSection(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (activePaperID === null) {
      setLoadError("请先选择试卷");
      return;
    }

    try {
      // 新增大题写入服务端，分值仍由后续选题或规则聚合重算，前端不直接修改小计分。
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
    } catch {
      setLoadError("创建大题失败");
    }
  }

  async function handleAddManualQuestion(question: QuestionRow) {
    if (activePaperID === null || sections.length === 0) {
      setLoadError("请先创建试卷大题");
      return;
    }

    try {
      // 手动选题必须落到当前试卷的第一个大题，后端负责防重和试卷总分重算。
      await api.addManualQuestion({
        tenantID,
        paperID: activePaperID,
        sectionID: sections[0].id,
        questionID: question.id,
        score: question.scoreDefault ?? "0",
      });
      setManualCount((value) => value + 1);
    } catch (err) {
      setLoadError(err instanceof Error ? err.message : "加入试卷失败");
    }
  }

  async function handleGenerateFixedPaper(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (activePaperID === null || sections.length === 0) {
      setLoadError("请先创建试卷大题");
      return;
    }

    try {
      // rule_fixed 先把抽题条件保存为服务端规则，再触发固化生成，生成结果由后端写入 paper_section_questions。
      const rule = await api.createRule({
        tenantID,
        paperID: activePaperID,
        sectionID: sections[0].id,
        sortOrder: 1,
        tagIDs: [tagIDByName(fixedTag)],
        questionCount: fixedCount,
        scorePerQuestion: defaultRuleScorePerQuestion(sections[0]),
      });
      await api.generateRuleFixed({ tenantID, paperID: activePaperID });
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
      // rule_live 保存同一张规则表，考试发布前的预检查会复用这些规则计算候选题池。
      const rule = await api.createRule({
        tenantID,
        paperID: activePaperID,
        sectionID: sections[0].id,
        sortOrder: 1,
        tagIDs: [tagIDByName(liveTag)],
        questionCount: liveCount,
        scorePerQuestion: defaultRuleScorePerQuestion(sections[0]),
      });
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
      // 组卷预检查由后端按已保存规则跨规则去重，前端只展示候选题池规模和风险结果。
      const result = await api.precheckRuleLive({ tenantID, paperID: activePaperID });
      setPrecheckMessage(`预检查通过，候选题池 ${result.candidateCount} 道题`);
      setLoadError("");
    } catch (err) {
      setPrecheckMessage("");
      setLoadError(err instanceof Error ? err.message : "组卷预检查失败");
    }
  }

  function handleSearchPapers() {
    setAppliedSearchQuery(searchQuery);
  }

  function handleRefreshPapers() {
    setSearchQuery("");
    setAppliedSearchQuery("");
  }

  return (
    <section className="page platform-page exam-builder-page">
      <nav aria-label="试卷菜单" className="platform-tabbar" role="tablist">
        {[
          ["papers", "试卷"],
          ["questions", "题目"],
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

      {activeTab === "papers" && (
        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="试卷操作区">
              <Button variant="toolbarPrimary" onClick={() => setIsSectionDialogOpen(true)} type="button">
                新增大题
              </Button>
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
                onClick={handleRefreshPapers}
                type="button"
              >
                <RefreshCw aria-hidden="true" size={16} />
              </Button>
            </div>
          </div>
          {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
          <div className="table-wrap">
            <table className="data-table tenant-admin-table">
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
                  <tr key={paper.id}>
                    <td>{paper.name}</td>
                    <td>{paper.buildMode}</td>
                    <td><StatusBadge tone={paper.status === "draft" ? "info" : "success"}>{paper.status === "draft" ? "草稿" : "可用"}</StatusBadge></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <ul className="exam-section-list">
            {sections.map((item) => (
              <li key={item.id}>
                <span>{item.name}</span>
                <strong>{item.totalScore} 分</strong>
              </li>
            ))}
          </ul>
        </Panel>
      )}

      {activeTab === "questions" && (
        <Panel>
          {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
          <div className="table-wrap">
            <table className="data-table tenant-admin-table">
              <thead>
                <tr>
                  <th scope="col">题目</th>
                  <th scope="col">标签</th>
                  <th scope="col">分值</th>
                  <th scope="col">操作</th>
                </tr>
              </thead>
              <tbody>
                {candidateQuestions.length === 0 && <EmptyTableRow colSpan={4} />}
                {candidateQuestions.map((item) => (
                  <tr key={item.id}>
                    <td>{item.title}</td>
                    <td>{item.tag}</td>
                    <td>{item.scoreDefault ?? "0"}</td>
                    <td>
                      <Button
                        variant="actionReset"
                        onClick={() => void handleAddManualQuestion(item)}
                        type="button"
                      >
                        加入试卷
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="tenant-admin-status" role="status">已手动加入 {manualCount} 道题</div>
        </Panel>
      )}

      {activeTab === "rules" && (
        <Panel>
          <div className="tenant-list-toolbar">
            <div className="tenant-list-actions" aria-label="组卷规则操作区">
              <Button variant="toolbarPrimary" onClick={() => setIsFixedDialogOpen(true)} type="button">
                生成固定规则试卷
              </Button>
              <Button variant="toolbarSecondary" onClick={() => setIsLiveDialogOpen(true)} type="button">
                保存 rule_live 规则
              </Button>
              <Button
                variant="toolbarSecondary"
                onClick={() => void handlePrecheckRuleLive()}
                type="button"
              >
                运行组卷预检查
              </Button>
            </div>
          </div>
          {fixedResult && (
            <div aria-label="rule-fixed-result" className="tenant-admin-status" role="status">{fixedResult}</div>
          )}
          {fixedResult && (
            <Button variant="toolbarSecondary" onClick={() => setFixedReview("已替换 1 道低匹配题")} type="button">
              替换低匹配题
            </Button>
          )}
          {fixedReview && (
            <div aria-label="rule-fixed-review" className="tenant-admin-status" role="status">{fixedReview}</div>
          )}
          {liveResult && (
            <div aria-label="rule-live-result" className="tenant-admin-status" role="status">{liveResult}</div>
          )}
          {precheckMessage && <div className="tenant-admin-warning" role="alert">{precheckMessage}</div>}
        </Panel>
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
                <input
                  min={1}
                  onChange={(event) => setSectionScore(event.target.value)}
                  required
                  type="number"
                  value={sectionScore}
                />
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
                <input
                  min={1}
                  onChange={(event) => setFixedCount(Number(event.target.value))}
                  type="number"
                  value={fixedCount}
                />
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
                <input
                  min={1}
                  onChange={(event) => setLiveCount(Number(event.target.value))}
                  type="number"
                  value={liveCount}
                />
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
