import MDEditor from "@uiw/react-md-editor";
import "@uiw/react-md-editor/markdown-editor.css";
import "@uiw/react-markdown-preview/markdown.css";
import { ArrowLeft, X } from "lucide-react";
import { useEffect, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { questionApi } from "../../api/questions";
import type { QuestionAPI, QuestionDifficulty, QuestionType } from "../../api/questions";
import { Button } from "../../components/ui/Button";
import { Panel } from "../../components/ui/Panel";

type QuestionCreatePageProps = {
  api?: QuestionAPI;
  tenantID?: number;
  spaceID?: number;
};

const questionTypeLabels: Record<QuestionType, string> = {
  single: "单选题",
  multiple: "多选题",
  judge: "判断题",
  fill_blank: "填空题",
  short_text: "简答题",
};

const questionDifficultyLabels: Record<QuestionDifficulty, string> = {
  easy: "简单",
  medium: "中等",
  hard: "困难",
};

const defaultChoiceOptions = ["选项 A", "选项 B"];

export function QuestionCreatePage({ api = questionApi, tenantID = 10, spaceID }: QuestionCreatePageProps) {
  const navigate = useNavigate();
  const location = useLocation();
  const [tags, setTags] = useState(["选择题", "语言文字"]);
  const [questionType, setQuestionType] = useState<QuestionType>("single");
  const [difficulty, setDifficulty] = useState<QuestionDifficulty>("medium");
  const [scoreDefault, setScoreDefault] = useState("2");
  const [stem, setStem] = useState("");
  const [optionValues, setOptionValues] = useState(defaultChoiceOptions);
  const [singleCorrectIndex, setSingleCorrectIndex] = useState(0);
  const [multipleCorrectIndexes, setMultipleCorrectIndexes] = useState([0, 1]);
  const [judgeAnswer, setJudgeAnswer] = useState("true");
  const [standardAnswer, setStandardAnswer] = useState("");
  const [referenceAnswer, setReferenceAnswer] = useState("");
  const [analysis, setAnalysis] = useState("");
  const [selectedQuestionTags, setSelectedQuestionTags] = useState<string[]>([]);
  const [questionTagQuery, setQuestionTagQuery] = useState("");
  const [isQuestionTagInputFocused, setIsQuestionTagInputFocused] = useState(false);
  const [saveError, setSaveError] = useState("");

  const questionTagSuggestions = tags
    .filter((item) => item.toLowerCase().includes(questionTagQuery.trim().toLowerCase()))
    .filter((item) => !selectedQuestionTags.includes(item))
    .slice(0, 10);
  const shouldShowQuestionTagSuggestions = isQuestionTagInputFocused && questionTagQuery.trim() !== "" && questionTagSuggestions.length > 0;
  const questionListPath = `/questions${location.search}`;

  useEffect(() => {
    let ignore = false;

    api.listQuestions({ tenantID, ...(spaceID === undefined ? {} : { spaceID }) })
      .then((data) => {
        if (!ignore) {
          setTags((items) => mergeTags(items, data.items.flatMap((item) => item.tags.length > 0 ? item.tags : [item.tag])));
        }
      })
      .catch(() => {
        if (!ignore) {
          setTags((items) => items);
        }
      });

    return () => {
      ignore = true;
    };
  }, [api, tenantID, spaceID]);

  async function handleSaveQuestion(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!stem.trim() || !analysis.trim()) {
      setSaveError("题干和题目解析不能为空");
      return;
    }

    setSaveError("");
    const optionPayload = buildOptionPayload(questionType, optionValues, singleCorrectIndex, multipleCorrectIndexes);
    if (optionPayload.hasBlankCorrectOption) {
      setSaveError("正确答案选项不能为空");
      return;
    }
    const questionTags = normalizeTags([...selectedQuestionTags, questionTagQuery]);

    try {
      await api.createQuestion({
        tenantID,
        ...(spaceID === undefined ? {} : { spaceID }),
        type: questionType,
        difficulty,
        title: stem,
        options: optionPayload.options,
        correctOptionIndexes: optionPayload.correctOptionIndexes,
        analysis,
        scoreDefault,
        tags: questionTags,
        standardAnswer: readStandardAnswer(questionType, judgeAnswer, standardAnswer),
        referenceAnswer: questionType === "short_text" ? referenceAnswer : undefined,
        blankCount: questionType === "fill_blank" ? 1 : undefined,
      });
      navigate(questionListPath);
    } catch {
      setSaveError("题目保存失败");
    }
  }

  function handleAddChoiceOption() {
    setOptionValues((items) => [...items, `选项 ${optionLabelByIndex(items.length)}`]);
  }

  function handleRemoveChoiceOption(index: number) {
    setOptionValues((items) => items.filter((_, itemIndex) => itemIndex !== index));
    setSingleCorrectIndex((current) => {
      if (current === index) {
        return 0;
      }
      return current > index ? current - 1 : current;
    });
    setMultipleCorrectIndexes((items) => {
      const next = items
        .filter((item) => item !== index)
        .map((item) => item > index ? item - 1 : item);
      return next.length > 0 ? next : [0];
    });
  }

  function handleSelectQuestionTag(tag: string) {
    setSelectedQuestionTags((items) => normalizeTags([...items, tag]));
    setTags((items) => mergeTags(items, [tag]));
    setQuestionTagQuery("");
    setIsQuestionTagInputFocused(false);
  }

  function handleRemoveQuestionTag(tag: string) {
    setSelectedQuestionTags((items) => items.filter((item) => item !== tag));
  }

  return (
    <section className="page platform-page exam-builder-page exam-question-create-page">
      <nav aria-label="题库菜单" className="platform-tabbar" role="tablist">
        <a className="platform-tab" href={questionListPath} role="tab" aria-selected="false">
          题库
        </a>
        <span className="platform-tab platform-tab--active" role="tab" aria-selected="true">
          新增题目
        </span>
      </nav>

      <Panel>
        <div className="exam-question-create-head">
          <a className="exam-question-create-head__back tenant-resource-drawer__back" href={questionListPath}>
            <ArrowLeft aria-hidden="true" size={19} />
            <span>返回题库</span>
          </a>
          <span aria-hidden="true" className="exam-question-create-head__divider">|</span>
          <h1 className="exam-question-create-head__title">新增题目</h1>
        </div>
        {saveError && <div className="tenant-admin-warning" role="alert">{saveError}</div>}
        <form className="platform-form exam-question-page-form" onSubmit={handleSaveQuestion}>
          <div className="exam-question-form-grid">
            <label className="field">
              <span>题型</span>
              <select
                aria-label="题型"
                onChange={(event) => setQuestionType(event.target.value as QuestionType)}
                value={questionType}
              >
                {Object.entries(questionTypeLabels).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span>题目难度</span>
              <select
                aria-label="题目难度"
                onChange={(event) => setDifficulty(event.target.value as QuestionDifficulty)}
                value={difficulty}
              >
                {Object.entries(questionDifficultyLabels).map(([value, label]) => (
                  <option key={value} value={value}>
                    {label}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span>默认分值</span>
              <input
                aria-label="默认分值"
                min="0"
                onChange={(event) => setScoreDefault(event.target.value)}
                required
                step="0.5"
                type="number"
                value={scoreDefault}
              />
            </label>
          </div>

          <MarkdownEditor label="题干" onChange={setStem} required value={stem} />

          {(questionType === "single" || questionType === "multiple") && (
            <section className="exam-option-list" aria-label="选择题选项">
              <div className="exam-option-list__head">
                <span>选项</span>
                <button className="secondary-button tenant-create-button" onClick={handleAddChoiceOption} type="button">
                  添加选项
                </button>
              </div>
              {optionValues.map((value, index) => {
                const optionLabel = optionLabelByIndex(index);

                return (
                  <div className="exam-option-item" key={index}>
                    <label className="field">
                      <span>{`选项 ${optionLabel}`}</span>
                      <input
                        aria-label={`选项 ${optionLabel}`}
                        onChange={(event) => setOptionValues((items) =>
                          items.map((item, itemIndex) => itemIndex === index ? event.target.value : item),
                        )}
                        required={index < 2}
                        value={value}
                      />
                    </label>
                    <div className="exam-option-item__controls">
                      <label className="platform-check">
                        <input
                          aria-label={`设为正确答案 ${optionLabel}`}
                          checked={
                            questionType === "single"
                              ? singleCorrectIndex === index
                              : multipleCorrectIndexes.includes(index)
                          }
                          name={questionType === "single" ? "single-question-correct-option" : undefined}
                          onChange={(event) => {
                            if (questionType === "single") {
                              setSingleCorrectIndex(index);
                              return;
                            }
                            setMultipleCorrectIndexes((items) =>
                              event.target.checked
                                ? Array.from(new Set([...items, index])).sort((left, right) => left - right)
                                : items.length > 1
                                  ? items.filter((item) => item !== index)
                                  : items,
                            );
                          }}
                          type={questionType === "single" ? "radio" : "checkbox"}
                        />
                        正确答案
                      </label>
                      {optionValues.length > 2 && (
                        <button
                          aria-label={`删除选项 ${optionLabel}`}
                          className="tenant-action-button tenant-action--danger"
                          onClick={() => handleRemoveChoiceOption(index)}
                          type="button"
                        >
                          删除
                        </button>
                      )}
                    </div>
                  </div>
                );
              })}
            </section>
          )}

          {questionType === "judge" && (
            <label className="field">
              <span>正确答案</span>
              <select aria-label="判断题正确答案" onChange={(event) => setJudgeAnswer(event.target.value)} value={judgeAnswer}>
                <option value="true">正确</option>
                <option value="false">错误</option>
              </select>
            </label>
          )}
          {questionType === "fill_blank" && (
            <label className="field">
              <span>标准答案</span>
              <input
                aria-label="填空题标准答案"
                onChange={(event) => setStandardAnswer(event.target.value)}
                required
                value={standardAnswer}
              />
            </label>
          )}
          {questionType === "short_text" && (
            <label className="field">
              <span>参考答案</span>
              <textarea
                aria-label="简答题参考答案"
                onChange={(event) => setReferenceAnswer(event.target.value)}
                required
                value={referenceAnswer}
              />
            </label>
          )}

          <MarkdownEditor label="题目解析" onChange={setAnalysis} required value={analysis} />

          <div className="field exam-tag-picker">
            <span>题目标签</span>
            {selectedQuestionTags.length > 0 && (
              <div className="exam-selected-tags" aria-label="已选题目标签">
                {selectedQuestionTags.map((tag) => (
                  <span className="exam-selected-tag" key={tag}>
                    {tag}
                    <button aria-label={`移除题目标签 ${tag}`} onClick={() => handleRemoveQuestionTag(tag)} type="button">
                      <X aria-hidden="true" size={13} />
                    </button>
                  </span>
                ))}
              </div>
            )}
            <div className="exam-tag-input-row">
              <input
                aria-label="题目标签"
                onBlur={() => window.setTimeout(() => setIsQuestionTagInputFocused(false), 120)}
                onChange={(event) => setQuestionTagQuery(event.target.value)}
                onFocus={() => setIsQuestionTagInputFocused(true)}
                onKeyDown={(event) => {
                  if (event.key !== "Enter" || questionTagQuery.trim() === "") {
                    return;
                  }
                  event.preventDefault();
                  handleSelectQuestionTag(questionTagQuery);
                }}
                placeholder="输入标签名，回车添加"
                value={questionTagQuery}
              />
              <button
                className="secondary-button tenant-create-button"
                disabled={questionTagQuery.trim() === ""}
                onClick={() => handleSelectQuestionTag(questionTagQuery)}
                type="button"
              >
                添加标签
              </button>
            </div>
            {shouldShowQuestionTagSuggestions && (
              <div aria-label="题目标签候选" className="user-suggestion-list" role="listbox">
                {questionTagSuggestions.map((tag) => (
                  <button
                    className="user-suggestion-option"
                    key={tag}
                    onMouseDown={(event) => {
                      event.preventDefault();
                      handleSelectQuestionTag(tag);
                    }}
                    role="option"
                    type="button"
                  >
                    <span>{tag}</span>
                    <small>已有标签</small>
                  </button>
                ))}
              </div>
            )}
          </div>

          <div className="platform-dialog__actions">
            <Button variant="secondary" onClick={() => navigate(questionListPath)} type="button">
              取消
            </Button>
            <Button variant="primary" type="submit">
              确认新增
            </Button>
          </div>
        </form>
      </Panel>
    </section>
  );
}

function MarkdownEditor({
  label,
  onChange,
  required = false,
  value,
}: {
  label: string;
  onChange(value: string): void;
  required?: boolean;
  value: string;
}) {
  return (
    <div className="field markdown-editor" data-color-mode="light">
      <span>{label}</span>
      <MDEditor
        height="auto"
        onChange={(nextValue) => onChange(nextValue ?? "")}
        preview="live"
        textareaProps={{
          "aria-label": label,
          required,
        }}
        value={value}
        visibleDragbar={false}
      />
    </div>
  );
}

function normalizeTags(tags: string[]) {
  const next: string[] = [];
  for (const tag of tags) {
    const normalized = tag.trim();
    if (normalized && !next.includes(normalized)) {
      next.push(normalized);
    }
  }
  return next;
}

function mergeTags(current: string[], incoming: string[]) {
  const next = [...current];
  for (const tag of normalizeTags(incoming)) {
    if (!next.includes(tag)) {
      next.push(tag);
    }
  }
  return next;
}

function optionLabelByIndex(index: number) {
  return String.fromCharCode("A".charCodeAt(0) + index);
}

function buildOptionPayload(
  questionType: QuestionType,
  optionValues: string[],
  singleCorrectIndex: number,
  multipleCorrectIndexes: number[],
) {
  if (questionType !== "single" && questionType !== "multiple") {
    return { options: [], correctOptionIndexes: [], hasBlankCorrectOption: false };
  }

  const options: string[] = [];
  const correctOptionIndexes: number[] = [];
  let hasBlankCorrectOption = false;
  optionValues.forEach((value, index) => {
    const normalized = value.trim();
    const isCorrect = questionType === "single" ? singleCorrectIndex === index : multipleCorrectIndexes.includes(index);
    if (!normalized) {
      if (isCorrect) {
        hasBlankCorrectOption = true;
      }
      return;
    }
    const nextIndex = options.length;
    options.push(normalized);
    if (isCorrect) {
      correctOptionIndexes.push(nextIndex);
    }
  });
  if (options.length > 0 && correctOptionIndexes.length === 0) {
    correctOptionIndexes.push(0);
  }

  return { options, correctOptionIndexes, hasBlankCorrectOption };
}

function readStandardAnswer(questionType: QuestionType, judgeAnswer: string, standardAnswer: string) {
  if (questionType === "judge") {
    return judgeAnswer;
  }
  if (questionType === "fill_blank") {
    return standardAnswer;
  }
  return undefined;
}
