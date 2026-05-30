import { Button } from "../../components/ui/Button";
import { EmptyTableRow } from "../../components/ui/EmptyTableRow";
import { RefreshCw, Search } from "lucide-react";
import { useEffect, useState } from "react";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";
import { questionApi } from "../../api/questions";
import type { QuestionBankAPI, QuestionRow } from "../../api/questions";

type QuestionBankPageProps = {
  api?: QuestionBankAPI;
  tenantID?: number;
  spaceID?: number;
};

export function QuestionBankPage({ api = questionApi, tenantID = 10, spaceID }: QuestionBankPageProps) {
  const [questions, setQuestions] = useState<QuestionRow[]>([]);
  const [tags, setTags] = useState(["选择题", "语言文字"]);
  const [stem, setStem] = useState("");
  const [optionA, setOptionA] = useState("选项 A");
  const [optionB, setOptionB] = useState("选项 B");
  const [analysis, setAnalysis] = useState("");
  const [questionTag, setQuestionTag] = useState("");
  const [newTag, setNewTag] = useState("");
  const [isQuestionDialogOpen, setIsQuestionDialogOpen] = useState(false);
  const [isTagDialogOpen, setIsTagDialogOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    let ignore = false;

    api.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then((data) => {
        if (!ignore) {
          setQuestions(data.items);
          setTags((items) => mergeTags(items, data.items.map((item) => item.tag)));
          setLoadError("");
        }
      })
      .catch(() => {
        if (!ignore) {
          setLoadError("题目列表加载失败");
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, spaceID]);

  const filteredQuestions = questions.filter((item) => {
    const keyword = appliedSearchQuery.trim().toLowerCase();
    if (!keyword) {
      return true;
    }

    // 题库搜索只匹配题目列表可见字段，便于按题干、标签、选项或解析快速定位。
    return [item.title, item.stem, item.tag, item.options.join(" "), item.analysis, item.status === "ready" ? "可用" : "草稿"].some(
      (value) => value.toLowerCase().includes(keyword),
    );
  });

  async function handleSaveQuestion(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 在线出题同时保存选项、解析和标签，题目进入题库后可直接被组卷页选择。
    const nextQuestion = await api.createQuestion({
      tenantID,
      ...(spaceID === undefined ? {} : { spaceID }),
      title: stem,
      options: [optionA, optionB],
      analysis,
      tag: questionTag,
    });
    setQuestions((items) => [...items, nextQuestion]);
    setTags((items) => mergeTags(items, [nextQuestion.tag]));
    setStem("");
    setOptionA("选项 A");
    setOptionB("选项 B");
    setAnalysis("");
    setQuestionTag("");
    setIsQuestionDialogOpen(false);
  }

  function handleAddTag(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!newTag.trim()) {
      return;
    }

    // 标签只维护当前租户内题目分类，重复标签不再追加到列表里。
    setTags((items) => (items.includes(newTag.trim()) ? items : [...items, newTag.trim()]));
    setNewTag("");
    setIsTagDialogOpen(false);
  }


  function handleSearchQuestions() {
    setAppliedSearchQuery(searchQuery);
  }

  function handleRefreshQuestions() {
    setSearchQuery("");
    setAppliedSearchQuery("");
  }

  return (
    <section className="page platform-page exam-builder-page">
      <nav aria-label="题库菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab platform-tab--active" href="/questions" role="tab" aria-selected="true">
          题库
        </a>
      </nav>

      <Panel>
        <div className="tenant-list-toolbar">
          <div className="tenant-list-actions" aria-label="题库操作区">
            <Button variant="toolbarPrimary" onClick={() => setIsQuestionDialogOpen(true)} type="button">
              保存题目
            </Button>
            <Button variant="toolbarSecondary" onClick={() => setIsTagDialogOpen(true)} type="button">
              新增标签
            </Button>
          </div>
          <div className="tenant-search-actions">
            <label className="tenant-search-field">
              <span className="sr-only">搜索题目</span>
              <input
                onChange={(event) => setSearchQuery(event.target.value)}
                placeholder="输入题目、标签、选项或解析"
                value={searchQuery}
              />
            </label>
            <Button aria-label="搜索" variant="icon" onClick={handleSearchQuestions} type="button">
              <Search aria-hidden="true" size={16} />
            </Button>
            <Button
              aria-label="刷新题目列表"
              variant="icon"
              onClick={handleRefreshQuestions}
              type="button"
            >
              <RefreshCw aria-hidden="true" size={16} />
            </Button>
          </div>
        </div>
        <div aria-label="题目标签列表" className="exam-tag-list">
          {tags.map((item) => (
            <span className="exam-tag" key={item}>{item}</span>
          ))}
        </div>
        {loadError && <div className="tenant-admin-warning" role="alert">{loadError}</div>}
        <div className="table-wrap">
          <table className="data-table tenant-admin-table">
            <thead>
              <tr>
                <th scope="col">题目</th>
                <th scope="col">标签</th>
                <th scope="col">选项</th>
                <th scope="col">解析</th>
                <th scope="col">状态</th>
              </tr>
            </thead>
            <tbody>
              {filteredQuestions.length === 0 && <EmptyTableRow colSpan={5} />}
              {filteredQuestions.map((item) => (
                <tr key={item.id}>
                  <td>
                    <strong>{item.title}</strong>
                    <p className="tenant-admin-muted">{item.stem}</p>
                  </td>
                  <td>{item.tag}</td>
                  <td>{item.options.join(" / ")}</td>
                  <td>{`解析：${item.analysis}`}</td>
                  <td>
                    <StatusBadge tone={item.status === "ready" ? "success" : "info"}>
                      {item.status === "ready" ? "可用" : "草稿"}
                    </StatusBadge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Panel>

      {isQuestionDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="保存题目弹窗">
          <div className="platform-dialog__card">
            <h2>保存题目</h2>
            <form className="platform-form" onSubmit={handleSaveQuestion}>
              <label className="field">
                <span>题干</span>
                <textarea onChange={(event) => setStem(event.target.value)} required value={stem} />
              </label>
              <div className="exam-option-grid">
                <label className="field">
                  <span>选项 A</span>
                  <input onChange={(event) => setOptionA(event.target.value)} required value={optionA} />
                </label>
                <label className="field">
                  <span>选项 B</span>
                  <input onChange={(event) => setOptionB(event.target.value)} required value={optionB} />
                </label>
              </div>
              <label className="field">
                <span>题目解析</span>
                <textarea onChange={(event) => setAnalysis(event.target.value)} required value={analysis} />
              </label>
              <label className="field">
                <span>题目标签</span>
                <input onChange={(event) => setQuestionTag(event.target.value)} required value={questionTag} />
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsQuestionDialogOpen(false)} type="button">
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

      {isTagDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="新增标签弹窗">
          <div className="platform-dialog__card">
            <h2>新增标签</h2>
            <form className="platform-form" onSubmit={handleAddTag}>
              <label className="field">
                <span>新标签名称</span>
                <input onChange={(event) => setNewTag(event.target.value)} value={newTag} />
              </label>
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsTagDialogOpen(false)} type="button">
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

    </section>
  );
}

function mergeTags(current: string[], incoming: string[]) {
  const next = [...current];
  for (const tag of incoming) {
    if (tag && !next.includes(tag)) {
      next.push(tag);
    }
  }
  return next;
}
