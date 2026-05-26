import { Button } from "../../components/ui/Button";
import { RefreshCw, Search } from "lucide-react";
import { useState } from "react";
import { FileUploadField } from "../../components/ui/FileUploadField";
import { Panel } from "../../components/ui/Panel";
import { StatusBadge } from "../../components/ui/StatusBadge";

type Question = {
  id: number;
  title: string;
  stem: string;
  options: string[];
  analysis: string;
  tag: string;
  status: "draft" | "ready";
};

const initialQuestions: Question[] = [
  {
    id: 1,
    title: "现代文阅读主旨题",
    stem: "下列选项最能概括文章中心的是哪一项？",
    options: ["把握中心句", "复述细节", "替换概念", "扩展背景"],
    analysis: "定位中心句并排除以偏概全选项。",
    tag: "阅读理解",
    status: "ready",
  },
];

export function QuestionBankPage() {
  const [questions, setQuestions] = useState<Question[]>(initialQuestions);
  const [tags, setTags] = useState(["选择题", "语言文字"]);
  const [title, setTitle] = useState("");
  const [stem, setStem] = useState("");
  const [optionA, setOptionA] = useState("选项 A");
  const [optionB, setOptionB] = useState("选项 B");
  const [analysis, setAnalysis] = useState("");
  const [questionTag, setQuestionTag] = useState("");
  const [newTag, setNewTag] = useState("");
  const [importFileName, setImportFileName] = useState("");
  const [importMessage, setImportMessage] = useState("");
  const [isQuestionDialogOpen, setIsQuestionDialogOpen] = useState(false);
  const [isTagDialogOpen, setIsTagDialogOpen] = useState(false);
  const [isImportDialogOpen, setIsImportDialogOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [appliedSearchQuery, setAppliedSearchQuery] = useState("");

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

  function handleSaveQuestion(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    // 在线出题同时保存选项、解析和标签，题目进入题库后可直接被组卷页选择。
    setQuestions((items) => [
      ...items,
      {
        id: Date.now(),
        title,
        stem,
        options: [optionA, optionB],
        analysis,
        tag: questionTag,
        status: "draft",
      },
    ]);
    setTitle("");
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

  function handleImportQuestions() {
    if (!importFileName) {
      setImportMessage("请选择题目导入文件");
      return;
    }

    // 导入先展示解析摘要，行级错误后续由导入任务接口返回。
    setImportMessage(`${importFileName} 已解析 18 道题`);
    setIsImportDialogOpen(false);
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
            <Button variant="toolbarSecondary" onClick={() => setIsImportDialogOpen(true)} type="button">
              导入题目
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
        {importMessage && <div className="tenant-admin-status" role="status">{importMessage}</div>}
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
                <span>题目标题</span>
                <input onChange={(event) => setTitle(event.target.value)} required value={title} />
              </label>
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

      {isImportDialogOpen && (
        <div className="platform-dialog" role="dialog" aria-modal="true" aria-label="导入题目弹窗">
          <div className="platform-dialog__card">
            <h2>导入题目</h2>
            <div className="platform-form">
              <FileUploadField
                accept={["text/csv", "application/vnd.ms-excel"]}
                label="题目导入文件"
                maxSizeBytes={2 * 1024 * 1024}
                onFileAccepted={(file) => setImportFileName(file.name)}
              />
              <div className="platform-dialog__actions">
                <Button variant="secondary" onClick={() => setIsImportDialogOpen(false)} type="button">
                  取消
                </Button>
                <Button variant="primary" onClick={handleImportQuestions} type="button">
                  确认导入
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </section>
  );
}
