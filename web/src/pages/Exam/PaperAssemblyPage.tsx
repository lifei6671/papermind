import { Button } from "../../components/ui/Button";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";

type Section = {
  id: number;
  name: string;
  score: number;
};

type CandidateQuestion = {
  id: number;
  title: string;
  tag: string;
  score: number;
};

type Paper = {
  id: number;
  name: string;
  strategy: string;
  status: "draft" | "ready";
};

const initialPapers: Paper[] = [
  { id: 1, name: "高一语文月考试卷", strategy: "rule_fixed", status: "draft" },
];

const initialSections: Section[] = [
  { id: 1, name: "一、现代文阅读", score: 30 },
  { id: 2, name: "二、语言文字运用", score: 20 },
];

const candidateQuestions: CandidateQuestion[] = [
  { id: 1, title: "现代文阅读主旨题", tag: "阅读理解", score: 6 },
  { id: 2, title: "病句辨析题", tag: "语言文字", score: 4 },
];

export function PaperAssemblyPage() {
  const [papers] = useState<Paper[]>(initialPapers);
  const [sections, setSections] = useState<Section[]>(initialSections);
  const [sectionName, setSectionName] = useState("");
  const [sectionScore, setSectionScore] = useState("");
  const [manualCount, setManualCount] = useState(0);
  const [fixedCount, setFixedCount] = useState(4);
  const [fixedTag, setFixedTag] = useState("阅读理解");
  const [fixedResult, setFixedResult] = useState("");
  const [fixedReview, setFixedReview] = useState("");
  const [liveRule, setLiveRule] = useState("");
  const [liveResult, setLiveResult] = useState("");
  const [precheckMessage, setPrecheckMessage] = useState("");
  const [isSectionDialogOpen, setIsSectionDialogOpen] = useState(false);
  const [isFixedDialogOpen, setIsFixedDialogOpen] = useState(false);
  const [isLiveDialogOpen, setIsLiveDialogOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");

  const filteredPapers = papers.filter((paper) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 试卷列表搜索只匹配当前可见字段，便于按试卷名称、策略或状态快速定位。
    return [paper.name, paper.strategy, paper.status === "draft" ? "草稿" : "可用"].some((value) =>
      value.toLowerCase().includes(keyword),
    );
  });

  function handleAddSection(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 大题结构先维护名称和分值，题目归属后续由手动选题或规则组卷填充。
    setSections((items) => [
      ...items,
      { id: Date.now(), name: sectionName, score: Number(sectionScore) },
    ]);
    setSectionName("");
    setSectionScore("");
    setIsSectionDialogOpen(false);
  }

  function handleGenerateFixedPaper(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // rule_fixed 按固定标签和题量生成，审题替换会基于生成结果继续处理。
    setFixedResult(`已按${fixedTag}生成 ${fixedCount} 道题`);
    setFixedReview("待替换低匹配题");
    setIsFixedDialogOpen(false);
  }

  function handleSaveLiveRule(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // rule_live 只保存动态规则说明，实时选题策略由后续服务端调度执行。
    setLiveResult(`rule_live 已保存：${liveRule}`);
    setIsLiveDialogOpen(false);
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
        <a className="platform-tab platform-tab--active" href="/papers" role="tab" aria-selected="true">
          试卷
        </a>
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="试卷操作区">
            <Button variant="toolbarPrimary" onClick={() => setIsSectionDialogOpen(true)} type="button">
              新增大题
            </Button>
            <Button variant="toolbarSecondary" onClick={() => setIsFixedDialogOpen(true)} type="button">
              生成 rule_fixed 试卷
            </Button>
            <Button variant="toolbarSecondary" onClick={() => setIsLiveDialogOpen(true)} type="button">
              保存 rule_live 规则
            </Button>
            <Button
              variant="toolbarSecondary"
              onClick={() => setPrecheckMessage(`预检查通过，已覆盖 ${sections.length} 个大题`)}
              type="button"
            >
              运行组卷预检查
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
              {filteredPapers.map((paper) => (
                <tr key={paper.id}>
                  <td>{paper.name}</td>
                  <td>{paper.strategy}</td>
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
              <strong>{item.score} 分</strong>
            </li>
          ))}
        </ul>
        {precheckMessage && <div className="tenant-admin-warning" role="alert">{precheckMessage}</div>}
      </Panel>

      <Panel>
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
              {candidateQuestions.map((item) => (
                <tr key={item.id}>
                  <td>{item.title}</td>
                  <td>{item.tag}</td>
                  <td>{item.score}</td>
                  <td>
                    <Button
                      variant="actionReset"
                      onClick={() => setManualCount((value) => value + 1)}
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

      {(fixedResult || fixedReview || liveResult) && (
        <Panel>
          {fixedResult && (
            <div aria-label="rule-fixed-result" className="tenant-admin-status" role="status">{fixedResult}</div>
          )}
          <Button variant="toolbarSecondary" onClick={() => setFixedReview("已替换 1 道低匹配题")} type="button">
            替换低匹配题
          </Button>
          {fixedReview && (
            <div aria-label="rule-fixed-review" className="tenant-admin-status" role="status">{fixedReview}</div>
          )}
          {liveResult && (
            <div aria-label="rule-live-result" className="tenant-admin-status" role="status">{liveResult}</div>
          )}
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
            <h2>生成 rule_fixed 试卷</h2>
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
                <span>动态规则说明</span>
                <textarea onChange={(event) => setLiveRule(event.target.value)} required value={liveRule} />
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
