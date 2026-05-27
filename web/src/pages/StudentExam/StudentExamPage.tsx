import { Button } from "../../components/ui/Button";
import { useEffect, useState } from "react";
import { examApi } from "../../api/exams";
import type { StudentExamAPI, StudentExamOption, StudentExamQuestion as APIStudentExamQuestion } from "../../api/exams";
import {
  AlertTriangle,
  Award,
  Bookmark,
  Calculator,
  CalendarDays,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  ClipboardList,
  ClipboardPenLine,
  Clock,
  DoorOpen,
  Eraser,
  FileText,
  Grid2X2,
  Link,
  List,
  ListOrdered,
  RefreshCw,
  Save,
  ShieldCheck,
  Star,
  TriangleAlert,
  Underline,
  X,
} from "lucide-react";
import "./StudentExamPage.css";

type QuestionState = "current" | "answered" | "blank" | "marked";

type QuestionNavItem = {
  number: number;
  state?: QuestionState;
};

type QuestionGroup = {
  title: string;
  subtitle: string;
  questions: QuestionNavItem[];
};

type ExamQuestionType = "single" | "multiple" | "judge" | "fill_blank" | "short_text";

type ExamQuestion = {
  id?: number;
  number: number;
  type: ExamQuestionType;
  sectionTitle: string;
  sectionSubtitle: string;
  stem: string;
  score: number;
  options?: string[];
  optionMeta?: StudentExamOption[];
  analysis: string;
};

type ExamAnswer = string | string[];

const narrowExamViewportQuery = "(max-width: 1100px)";

function shouldCollapseQuestionListByDefault() {
  return typeof window !== "undefined"
    && typeof window.matchMedia === "function"
    && window.matchMedia(narrowExamViewportQuery).matches;
}

const questionGroups: QuestionGroup[] = [
  {
    title: "一、单项选择题",
    subtitle: "共20题，每题2分",
    questions: Array.from({ length: 20 }, (_, index) => ({
      number: index + 1,
      state: index === 0 ? "current" : [2, 4, 5, 10, 16].includes(index + 1) ? "answered" : [3, 12, 18].includes(index + 1) ? "marked" : "blank",
    })),
  },
  {
    title: "二、多项选择题",
    subtitle: "共10题，每题2分",
    questions: Array.from({ length: 10 }, (_, index) => ({ number: index + 21 })),
  },
  {
    title: "三、判断题",
    subtitle: "共5题，每题1分",
    questions: Array.from({ length: 5 }, (_, index) => ({ number: index + 31 })),
  },
  {
    title: "三、填空题",
    subtitle: "共5题，每题2分",
    questions: Array.from({ length: 5 }, (_, index) => ({ number: index + 36 })),
  },
  {
    title: "四、简答题",
    subtitle: "共5题，共30分",
    questions: Array.from({ length: 5 }, (_, index) => ({ number: index + 41 })),
  },
];

const optionRows = [
  ["A.", "瞻望（zhān）", "娑娑（suō）", "岑参（cén）", "怅然（chàng）"],
  ["B.", "棹蕴（yùn）", "弥散（mí）", "蹊跷（qī）", "摇曳（yè）"],
  ["C.", "腌臜（yān）", "提防（dī）", "颔首（hàn）", "睥睨（pì）"],
  ["D.", "龟裂（guī）", "谙熟（ān）", "憧憬（chōng）", "饕餮（tāo）"],
];

const desktopQuestionSamples: Record<number, ExamQuestion> = {
  1: {
    number: 1,
    type: "single",
    sectionTitle: "一、单项选择题",
    sectionSubtitle: "共20题，每题2分",
    stem: "下列加点字的注音完全正确的一项是（ ）",
    score: 2,
    options: optionRows.map((row) => row.join(" ")),
    analysis: "岑参（cén），怅然（chàng）。",
  },
  21: {
    number: 21,
    type: "multiple",
    sectionTitle: "二、多项选择题",
    sectionSubtitle: "共10题，每题2分",
    stem: "下列属于李白诗歌艺术特色的有（ ）",
    score: 2,
    options: ["A. 豪放飘逸", "B. 沉郁顿挫", "C. 想象奇特", "D. 语言质朴无华"],
    analysis: "李白诗歌以豪放飘逸和想象奇特见长。",
  },
  31: {
    number: 31,
    type: "judge",
    sectionTitle: "三、判断题",
    sectionSubtitle: "共5题，每题1分",
    stem: "《岳阳楼记》的作者是范仲淹。",
    score: 1,
    options: ["正确", "错误"],
    analysis: "《岳阳楼记》作者为范仲淹。",
  },
  36: {
    number: 36,
    type: "fill_blank",
    sectionTitle: "三、填空题",
    sectionSubtitle: "共5题，每题2分",
    stem: "补写名句：________，后天下之乐而乐。",
    score: 2,
    analysis: "标准答案：先天下之忧而忧。",
  },
  41: {
    number: 41,
    type: "short_text",
    sectionTitle: "四、简答题",
    sectionSubtitle: "共5题，共30分",
    stem: "请简要分析《岳阳楼记》中“先天下之忧而忧，后天下之乐而乐”的思想内涵。",
    score: 10,
    analysis: "本题重点考查忧乐观、士大夫责任意识和家国情怀。",
  },
};

function getDesktopQuestion(number: number): ExamQuestion {
  if (desktopQuestionSamples[number]) {
    return desktopQuestionSamples[number];
  }

  if (number >= 21 && number <= 30) {
    return { ...desktopQuestionSamples[21], number };
  }

  if (number >= 31 && number <= 35) {
    return { ...desktopQuestionSamples[31], number };
  }

  if (number >= 36 && number <= 40) {
    return { ...desktopQuestionSamples[36], number };
  }

  if (number >= 41 && number <= 45) {
    return { ...desktopQuestionSamples[41], number };
  }

  return { ...desktopQuestionSamples[1], number };
}

function mapAPIQuestion(question: APIStudentExamQuestion): ExamQuestion {
  return {
    id: question.id,
    number: question.number,
    type: question.type,
    sectionTitle: question.sectionTitle,
    sectionSubtitle: question.sectionSubtitle || `第 ${question.number} 题 / ${question.score} 分`,
    stem: question.stem,
    score: question.score,
    options: question.options.map((option) => `${option.key}. ${option.content}`),
    optionMeta: question.options,
    analysis: "题目解析将在成绩公布后展示。",
  };
}

function answerToOptionIDs(question: ExamQuestion, answer: ExamAnswer) {
  if (!question.optionMeta || question.optionMeta.length === 0) {
    return [];
  }
  const values = Array.isArray(answer) ? answer : [answer];
  const normalizedValues = values.map((value) => value.replace(/\.$/, ""));
  return question.optionMeta
    .filter((option) => normalizedValues.includes(option.key))
    .map((option) => option.id);
}

function hasAnswer(answer: ExamAnswer | undefined) {
  return Array.isArray(answer) ? answer.length > 0 : Boolean(answer);
}

function QuestionButton({
  item,
  onSelect,
  selectedNumber,
  answers = {},
}: {
  item: QuestionNavItem;
  onSelect?: (number: number) => void;
  selectedNumber?: number;
  answers?: Record<number, ExamAnswer>;
}) {
  const state = selectedNumber === item.number ? "current" : hasAnswer(answers[item.number]) ? "answered" : item.state ?? "blank";

  return (
    <Button className={`exam-qnav__button exam-qnav__button--${state}`} onClick={() => onSelect?.(item.number)} type="button">
      {item.number}
      {state === "answered" && <span className="exam-qnav__dot" />}
      {state === "marked" && <span className="exam-qnav__mark" />}
    </Button>
  );
}

function QuestionGrid({
  compact = false,
  onSelect,
  selectedNumber,
  answers,
}: {
  compact?: boolean;
  onSelect?: (number: number) => void;
  selectedNumber?: number;
  answers?: Record<number, ExamAnswer>;
}) {
  return (
    <div className={compact ? "exam-qnav exam-qnav--compact" : "exam-qnav"}>
      {questionGroups.map((group) => (
        <section className="exam-qnav__group" key={group.title}>
          <div className="exam-qnav__title">
            <strong>{group.title}</strong>
            <span>{group.subtitle}</span>
            {compact && <ChevronDown aria-hidden="true" size={14} />}
          </div>
          {!compact || group.title.startsWith("一") ? (
            <div className="exam-qnav__grid">
              {group.questions.map((item) => (
                <QuestionButton answers={answers} item={item} key={item.number} onSelect={onSelect} selectedNumber={selectedNumber} />
              ))}
            </div>
          ) : null}
        </section>
      ))}
    </div>
  );
}

function useExamViewportMode() {
  const [isNarrowViewport, setIsNarrowViewport] = useState(() => shouldCollapseQuestionListByDefault());

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return undefined;
    }

    const mediaQueryList = window.matchMedia(narrowExamViewportQuery);
    const handleViewportChange = (event: MediaQueryListEvent) => {
      setIsNarrowViewport(event.matches);
    };

    mediaQueryList.addEventListener("change", handleViewportChange);

    return () => {
      mediaQueryList.removeEventListener("change", handleViewportChange);
    };
  }, []);

  return isNarrowViewport;
}

function MobileExamHeader({ remainingTime = "01:28:36" }: { remainingTime?: string }) {
  return (
    <header className="mobile-exam-header">
      <div className="mobile-brand">
        <span className="mobile-brand__mark">P</span>
        <span>PaperMind</span>
      </div>
      <div className="mobile-timer">
        <Clock aria-hidden="true" size={22} />
        <span>剩余时间</span>
        <strong>{remainingTime}</strong>
      </div>
    </header>
  );
}

function MobileExamStart({ onStart }: { onStart: () => void }) {
  const noticeItems = [
    "开考后系统自动开始计时；",
    "答案将自动保存；",
    "考试过程中请勿频繁切换页面；",
    "到达截止时间将自动交卷；",
    "成绩公布后可查看解析。",
  ];

  return (
    <section className="mobile-start-view" aria-label="开考前说明">
      <MobileExamHeader />
      <main className="mobile-start-main">
        <h1>期中考试（高一语文）</h1>
        <span className="mobile-status-pill"><Clock aria-hidden="true" size={16} />开考前</span>
        <p className="mobile-muted">请仔细阅读考试须知，确认后开始考试</p>

        <section className="mobile-info-card" aria-label="考试信息">
          <div><CalendarDays aria-hidden="true" /><span>考试时间</span><strong>2025-05-28 09:00 - 11:00</strong></div>
          <div><Clock aria-hidden="true" /><span>作答时长</span><strong>120 分钟</strong></div>
          <div><FileText aria-hidden="true" /><span>总题数</span><strong>45 题</strong></div>
          <div><Award aria-hidden="true" /><span>总分</span><strong>100 分</strong></div>
          <div><RefreshCw aria-hidden="true" /><span>作答次数</span><strong>1 次</strong></div>
        </section>

        <section className="mobile-notice-card">
          <h2>考试须知</h2>
          <ol>
            {noticeItems.map((item, index) => (
              <li key={item}><span>{index + 1}</span>{item}</li>
            ))}
          </ol>
        </section>

        <section className="mobile-tip-card">
          <ShieldCheck aria-hidden="true" size={32} />
          <div>
            <strong>温馨提示</strong>
            <p>为保障考试的稳定进行，建议使用稳定的网络环境，如遇网络异常，请不要刷新或关闭页面。</p>
          </div>
        </section>

        <label className="mobile-agree-check">
          <input type="checkbox" />
          <span>我已阅读并同意<strong>考试规则</strong></span>
        </label>
      </main>
      <footer className="mobile-start-actions">
        <Button className="mobile-secondary-button" type="button"><ClipboardList aria-hidden="true" />查看考试说明</Button>
        <Button className="mobile-primary-button" onClick={onStart} type="button">开始考试</Button>
        <span><ShieldCheck aria-hidden="true" size={18} />建议在稳定网络环境下完成考试</span>
      </footer>
    </section>
  );
}

function MobileQuestionOptions() {
  return (
    <div className="mobile-option-list">
      {optionRows.map((row, index) => (
        <label className={index === 0 ? "mobile-option mobile-option--selected" : "mobile-option"} key={row[0]}>
          <span className="mobile-option__radio" aria-hidden="true" />
          <strong>{row[0]}</strong>
          <span className="mobile-option__terms">
            {row.slice(1).map((text) => <span key={text}>{text}</span>)}
          </span>
        </label>
      ))}
    </div>
  );
}

function MobileChoiceQuestion({ onAnswerCard, onEssay }: { onAnswerCard: () => void; onEssay: () => void }) {
  return (
    <section className="mobile-exam-view" aria-label="移动端单选题作答">
      <MobileExamHeader />
      <main className="mobile-question-main">
        <div className="mobile-question-title">
          <div>
            <h1>期中考试（高一语文）</h1>
            <span><CheckCircle2 aria-hidden="true" size={18} />已自动保存&nbsp;&nbsp;14:32:18</span>
          </div>
          <p><strong>1</strong> / 45</p>
        </div>

        <section className="mobile-question-card">
          <header>
            <h2>一、单项选择题 <span>（共20题，每题2分）</span></h2>
            <Button type="button"><Bookmark aria-hidden="true" />标记本题</Button>
          </header>
          <p className="mobile-question-stem">下列加点字的注音完全正确的一项是（ ）</p>
          <MobileQuestionOptions />
          <div className="mobile-analysis">
            <strong>题目解析（考后公布）</strong>
            <p>岑参（cén）、怅然（chàng）。</p>
            <p>B项：棹蕴（yún）错误，应为棹蕴（yǔn）；C项：提防（dī）错误，应为提防（dī）；D项：龟裂（guī）错误，应为龟裂（jūn）。</p>
          </div>
        </section>

        <div className="mobile-warning-toast">
          <AlertTriangle aria-hidden="true" />
          <span>考试中请勿切换页面或离开考试</span>
          <X aria-hidden="true" size={18} />
        </div>
      </main>
      <MobileExamNav onAnswerCard={onAnswerCard} onNext={onEssay} />
    </section>
  );
}

function MobileEssayQuestion({ onAnswerCard, onChoice }: { onAnswerCard: () => void; onChoice: () => void }) {
  return (
    <section className="mobile-exam-view" aria-label="移动端简答题作答">
      <MobileExamHeader remainingTime="00:36:12" />
      <main className="mobile-question-main">
        <div className="mobile-question-title">
          <div>
            <h1>期中考试（高一语文）</h1>
            <span><CheckCircle2 aria-hidden="true" size={18} />已自动保存&nbsp;&nbsp;14:45:09</span>
          </div>
          <p><strong>41</strong> / 45</p>
        </div>

        <section className="mobile-question-card">
          <header>
            <h2>四、简答题 <span>（共5题，共30分）</span></h2>
            <Button type="button"><Bookmark aria-hidden="true" />标记本题</Button>
          </header>
          <div className="mobile-reading-box">
            <strong>阅读材料</strong>
            <p>庆历四年春，滕子京谪守巴陵郡。越明年，政通人和，百废具兴。乃重修岳阳楼，增其旧制，刻唐贤今人诗赋于其上。属予作文以记之。</p>
          </div>
          <p className="mobile-essay-stem">41. 请简要分析《岳阳楼记》中“先天下之忧而忧，后天下之乐而乐”的思想内涵。（10分）</p>
          <div className="mobile-editor">
            <div className="mobile-editor-toolbar">
              <Button type="button">B</Button>
              <Button type="button">I</Button>
              <Button type="button"><Underline aria-hidden="true" /></Button>
              <i />
              <Button type="button"><List aria-hidden="true" /></Button>
              <Button type="button"><ListOrdered aria-hidden="true" /></Button>
              <i />
              <Button type="button"><Link aria-hidden="true" /></Button>
              <Button type="button"><Eraser aria-hidden="true" /></Button>
              <Button type="button">清除格式</Button>
            </div>
            <textarea placeholder="请输入作答内容..." defaultValue="" />
            <footer><span>已输入 <strong>126</strong> 字</span><Button type="button"><Save aria-hidden="true" />保存</Button></footer>
          </div>
        </section>
      </main>
      <MobileExamNav onAnswerCard={onAnswerCard} onPrev={onChoice} />
    </section>
  );
}

function MobileExamNav({ onAnswerCard, onNext, onPrev }: { onAnswerCard: () => void; onNext?: () => void; onPrev?: () => void }) {
  return (
    <footer className="mobile-bottom-nav">
      <div className="mobile-nav-buttons">
        <Button onClick={onPrev} type="button"><ChevronLeft aria-hidden="true" />上一题</Button>
        <Button onClick={onAnswerCard} type="button"><Grid2X2 aria-hidden="true" />答题卡</Button>
        <Button className="mobile-nav-primary" onClick={onNext} type="button">下一题<ChevronRight aria-hidden="true" /></Button>
      </div>
      <div className="mobile-tool-row">
        <Button type="button"><Calculator aria-hidden="true" />计算器</Button>
        <Button type="button"><ClipboardPenLine aria-hidden="true" />草稿纸</Button>
      </div>
    </footer>
  );
}

function MobileAnswerCard({ onBack, onSubmit }: { onBack: () => void; onSubmit: () => void }) {
  const stats = [
    ["已答", "28", "answered"],
    ["未答", "15", "blank"],
    ["标记", "2", "marked"],
    ["总题数", "45", "total"],
  ];

  return (
    <section className="mobile-answer-view" aria-label="移动端答题卡">
      <MobileExamHeader />
      <main className="mobile-answer-main">
        <div className="mobile-answer-title">
          <Button aria-label="返回作答" onClick={onBack} type="button"><ChevronLeft aria-hidden="true" /></Button>
          <div><h1>答题卡</h1><p>期中考试（高一语文）</p></div>
        </div>
        <section aria-label="答题统计" className="mobile-answer-stats">
          {stats.map(([label, value, type]) => (
            <div className={`mobile-answer-stat mobile-answer-stat--${type}`} key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
        </section>
        <div className="mobile-answer-legend">
          <span><i className="is-answered" />已答</span>
          <span><i />未答</span>
          <span><Star aria-hidden="true" size={18} />标记</span>
          <span><i className="is-current" />当前</span>
        </div>
        <MobileAnswerGroups />
      </main>
      <footer className="mobile-answer-actions">
        <Button onClick={onBack} type="button">返回作答</Button>
        <Button onClick={onSubmit} type="button">提交试卷</Button>
      </footer>
    </section>
  );
}

function MobileAnswerGroups() {
  return (
    <div className="mobile-answer-groups">
      {questionGroups.map((group) => (
        <section key={group.title}>
          <h2>{group.title} <span>({group.questions[0].number}-{group.questions[group.questions.length - 1].number})</span></h2>
          <div>
            {group.questions.map((item) => {
              const state = item.number === 6 ? "current" : [9, 15, 37, 43].includes(item.number) ? "marked" : [1, 2, 3, 4, 5, 7, 10, 11, 12, 13, 16, 18, 19, 21, 31, 32, 38, 40, 41, 44].includes(item.number) ? "answered" : "blank";
              return <Button className={`mobile-answer-number mobile-answer-number--${state}`} key={item.number} type="button">{item.number}{state === "marked" ? <Star aria-hidden="true" size={12} /> : null}</Button>;
            })}
          </div>
        </section>
      ))}
    </div>
  );
}

function MobileSubmitDialog({ onCancel }: { onCancel: () => void }) {
  return (
    <div className="mobile-submit-layer">
      <div aria-labelledby="mobile-submit-title" aria-modal="true" className="mobile-submit-dialog" role="dialog">
        <span>!</span>
        <h2 id="mobile-submit-title">确认交卷</h2>
        <p>交卷后将无法继续作答，请确认是否提交？</p>
        <div><Button onClick={onCancel} type="button">取消</Button><Button type="button">确认交卷</Button></div>
      </div>
    </div>
  );
}

function MobileExamExperience() {
  const [screen, setScreen] = useState<"start" | "choice" | "essay" | "answerCard">("start");
  const [answerCardReturnScreen, setAnswerCardReturnScreen] = useState<"choice" | "essay">("choice");
  const [isSubmitDialogOpen, setIsSubmitDialogOpen] = useState(false);

  const openAnswerCard = (returnScreen: "choice" | "essay") => {
    setAnswerCardReturnScreen(returnScreen);
    setScreen("answerCard");
  };

  if (screen === "start") {
    return <MobileExamStart onStart={() => setScreen("choice")} />;
  }

  return (
    <>
      {screen === "choice" ? <MobileChoiceQuestion onAnswerCard={() => openAnswerCard("choice")} onEssay={() => setScreen("essay")} /> : null}
      {screen === "essay" ? <MobileEssayQuestion onAnswerCard={() => openAnswerCard("essay")} onChoice={() => setScreen("choice")} /> : null}
      {screen === "answerCard" ? <MobileAnswerCard onBack={() => setScreen(answerCardReturnScreen)} onSubmit={() => setIsSubmitDialogOpen(true)} /> : null}
      {isSubmitDialogOpen ? <MobileSubmitDialog onCancel={() => setIsSubmitDialogOpen(false)} /> : null}
    </>
  );
}

function DesktopQuestionBody({
  answer,
  onAnswer,
  question,
}: {
  answer: ExamAnswer | undefined;
  onAnswer: (value: ExamAnswer) => void;
  question: ExamQuestion;
}) {
  const answerValue = Array.isArray(answer) ? answer : typeof answer === "string" ? answer : "";

  if (question.type === "fill_blank") {
    return (
      <div className="written-answer">
        <label htmlFor="fill-blank-answer">填空题答案</label>
        <input
          id="fill-blank-answer"
          onChange={(event) => onAnswer(event.target.value)}
          placeholder="请输入答案"
          value={answerValue}
        />
      </div>
    );
  }

  if (question.type === "short_text") {
    return (
      <div className="written-answer written-answer--essay">
        <label htmlFor="short-text-answer">简答题答案</label>
        <textarea
          id="short-text-answer"
          onChange={(event) => onAnswer(event.target.value)}
          placeholder="请输入作答内容..."
          value={answerValue}
        />
      </div>
    );
  }

  return (
    <div className="option-list">
      {question.options?.map((option) => {
        const optionValue = option.replace(/^\s*([A-D]\.|正确|错误).*$/, "$1");
        const checked = Array.isArray(answer)
          ? answer.includes(optionValue)
          : answer === optionValue;

        return (
          <label className={checked ? "option-row option-row--selected" : "option-row"} key={option}>
            <span className="option-row__prefix">
              <input
                checked={checked}
                name={`question-${question.number}`}
                onChange={() => {
                  if (question.type === "multiple") {
                    const currentValues = Array.isArray(answer) ? answer : [];
                    onAnswer(currentValues.includes(optionValue)
                      ? currentValues.filter((value) => value !== optionValue)
                      : [...currentValues, optionValue].sort());
                    return;
                  }

                  onAnswer(optionValue);
                }}
                type={question.type === "multiple" ? "checkbox" : "radio"}
              />
              <strong>{optionValue}</strong>
            </span>
            <span className="option-row__terms">{option.replace(optionValue, "").trim()}</span>
          </label>
        );
      })}
    </div>
  );
}

function DesktopResultView() {
  return (
    <div className="student-exam-shell">
      <header className="student-exam-header">
        <div className="student-brand">
          <span className="student-brand__mark">P</span>
          <span>PaperMind</span>
        </div>
        <h1>期中考试（高一语文）</h1>
      </header>
      <main className="exam-result-page">
        <section className="exam-card exam-result-card">
          <span className="mobile-status-pill"><CheckCircle2 aria-hidden="true" size={16} />已交卷</span>
          <h2>成绩可见页</h2>
          <strong>总分 86 分</strong>
          <p>客观题 56 分，主观题 30 分。成绩已按统一公布策略展示。</p>
          <div className="analysis-box">
            <strong>题目解析</strong>
            <p>岑参（cén），怅然（chàng）。</p>
          </div>
        </section>
      </main>
    </div>
  );
}

type StudentExamPageProps = {
  api?: StudentExamAPI;
  tenantID?: number;
  examID?: number;
  userID?: number;
};

type ExamSession = {
  attemptID: number;
  examToken: string;
};

function DesktopStudentExamPage({
  api,
  tenantID,
  examID,
  userID,
}: Required<StudentExamPageProps>) {
  const [isExamDrawerOpen, setIsExamDrawerOpen] = useState(false);
  const [isSubmitDialogOpen, setIsSubmitDialogOpen] = useState(false);
  const [isLeaveDialogOpen, setIsLeaveDialogOpen] = useState(false);
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);
  const [isQuestionListOpen, setIsQuestionListOpen] = useState(() => !shouldCollapseQuestionListByDefault());
  const [answers, setAnswers] = useState<Record<number, ExamAnswer>>({});
  const [currentQuestionNumber, setCurrentQuestionNumber] = useState(1);
  const [saveMessage, setSaveMessage] = useState("");
  const [eventReportMessage, setEventReportMessage] = useState("");
  const [isSubmitted, setIsSubmitted] = useState(false);
  const [apiQuestions, setApiQuestions] = useState<ExamQuestion[]>([]);
  const [examSession, setExamSession] = useState<ExamSession | null>(null);

  const currentQuestion = apiQuestions.find((question) => question.number === currentQuestionNumber) ?? getDesktopQuestion(currentQuestionNumber);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return undefined;
    }

    const mediaQueryList = window.matchMedia(narrowExamViewportQuery);
    const handleViewportChange = (event: MediaQueryListEvent) => {
      setIsQuestionListOpen(!event.matches);
    };

    mediaQueryList.addEventListener("change", handleViewportChange);

    return () => {
      mediaQueryList.removeEventListener("change", handleViewportChange);
    };
  }, []);

  useEffect(() => {
    let isCancelled = false;
    async function startAttempt() {
      try {
        const started = await api.startAttempt({ tenantID, examID, userID });
        if (isCancelled) {
          return;
        }
        const questions = started.questions.map(mapAPIQuestion);
        setApiQuestions(questions);
        setExamSession({ attemptID: started.attemptID, examToken: started.examToken });
        setCurrentQuestionNumber(questions[0]?.number ?? 1);
      } catch {
        // 本地样例作为离线降级视图，避免后端未启动时考试页面直接不可用。
        setApiQuestions([]);
        setExamSession(null);
      }
    }
    void startAttempt();
    return () => {
      isCancelled = true;
    };
  }, [api, tenantID, examID, userID]);

  useEffect(() => {
    const reportTabSwitch = () => {
      setEventReportMessage("已上报切屏事件：blur");
      if (examSession) {
        void api.recordEvent({
          tenantID,
          attemptID: examSession.attemptID,
          examToken: examSession.examToken,
          eventType: "blur",
          payload: "{}",
        });
      }
    };

    window.addEventListener("blur", reportTabSwitch);

    return () => {
      window.removeEventListener("blur", reportTabSwitch);
    };
  }, [api, examSession, tenantID]);

  const saveAnswer = async (value: ExamAnswer) => {
    setAnswers((currentAnswers) => ({ ...currentAnswers, [currentQuestionNumber]: value }));
    try {
      if (examSession && currentQuestion.id) {
        await api.saveAnswer({
          tenantID,
          attemptID: examSession.attemptID,
          attemptQuestionID: currentQuestion.id,
          examToken: examSession.examToken,
          questionType: currentQuestion.type,
          optionIDs: answerToOptionIDs(currentQuestion, value),
          text: Array.isArray(value) ? "" : value,
        });
      }
      setSaveMessage(`第 ${currentQuestionNumber} 题已自动保存`);
    } catch {
      setSaveMessage(`第 ${currentQuestionNumber} 题保存失败`);
    }
  };

  const submitExam = async () => {
    try {
      if (examSession) {
        await api.submitAttempt({
          tenantID,
          attemptID: examSession.attemptID,
          examToken: examSession.examToken,
        });
      }
      setIsSubmitted(true);
    } catch {
      setSaveMessage("交卷失败，请稍后重试");
    }
  };

  if (isSubmitted) {
    return <DesktopResultView />;
  }

  return (
    <div className="student-exam-shell">
      <header className="student-exam-header">
        <div className="student-brand">
          <span className="student-brand__mark">P</span>
          <span>PaperMind</span>
        </div>
        <h1>期中考试（高一语文）</h1>
        <div
          className="student-tools"
          aria-label="考生账户"
          onBlur={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget)) {
              setIsProfileMenuOpen(false);
            }
          }}
        >
          <Button
            aria-label="张三"
            aria-controls="student-profile-menu"
            aria-expanded={isProfileMenuOpen}
            aria-haspopup="menu"
            className="student-profile"
            onClick={() => setIsProfileMenuOpen((isOpen) => !isOpen)}
            type="button"
          >
            <span className="student-avatar" aria-hidden="true">张</span>
            <strong>张三</strong>
            <ChevronDown aria-hidden="true" size={14} />
          </Button>
          {isProfileMenuOpen ? (
            <div aria-label="考生信息" className="student-profile-menu" id="student-profile-menu" role="menu">
              <span>考生编号：S1001001</span>
            </div>
          ) : null}
        </div>
      </header>

      <div className="student-exam-subbar">
        <Button
          aria-controls="exam-question-list"
          aria-expanded={isQuestionListOpen}
          className="ghost-button"
          onClick={() => setIsQuestionListOpen((isOpen) => !isOpen)}
          type="button"
        >
          {isQuestionListOpen ? <ChevronLeft aria-hidden="true" size={16} /> : <ChevronRight aria-hidden="true" size={16} />}
          {isQuestionListOpen ? "收起题目列表" : "展开题目列表"}
        </Button>
        <div className="student-exam-subbar__actions">
          <Button
            aria-controls="exam-helper-drawer"
            aria-expanded={isExamDrawerOpen}
            className="drawer-toggle"
            onClick={() => setIsExamDrawerOpen(true)}
            type="button"
          >
            考试信息与答题卡
          </Button>
          <Button className="danger-link" onClick={() => setIsLeaveDialogOpen(true)} type="button"><DoorOpen aria-hidden="true" size={16} />退出考试</Button>
        </div>
      </div>

      <main className={isQuestionListOpen ? "student-exam-layout" : "student-exam-layout student-exam-layout--qnav-collapsed"}>
        <Button
          aria-label="关闭题目列表抽屉"
          className={isQuestionListOpen ? "qnav-drawer-backdrop qnav-drawer-backdrop--open" : "qnav-drawer-backdrop"}
          onClick={() => setIsQuestionListOpen(false)}
          type="button"
        />

        <aside
          aria-label="题目列表"
          className={isQuestionListOpen ? "exam-card exam-left-panel exam-left-panel--drawer-open" : "exam-card exam-left-panel"}
          hidden={!isQuestionListOpen}
          id="exam-question-list"
        >
          <h2>题目列表</h2>
          <QuestionGrid answers={answers} onSelect={setCurrentQuestionNumber} selectedNumber={currentQuestionNumber} />
          <div className="exam-legend">
            <span><i className="legend-dot legend-dot--answered" />已答</span>
            <span><i className="legend-dot legend-dot--blank" />未答</span>
            <span><i className="legend-triangle" />标记</span>
            <span><i className="legend-current" />当前</span>
          </div>
        </aside>

        <section className="exam-card exam-question-panel">
          <div className="exam-question-head">
            <div>
              <span className="section-index">{currentQuestion.sectionTitle.slice(0, 1)}</span>
              <h2>{currentQuestion.sectionTitle}</h2>
              <strong>{currentQuestion.sectionSubtitle}</strong>
            </div>
            <span className="question-progress"><strong>{currentQuestion.number}</strong> / 45</span>
          </div>

          <div className="question-toolbar">
            <Button className="question-number" type="button">{currentQuestion.number}</Button>
            <Button className="mark-button" type="button"><Bookmark aria-hidden="true" size={16} />标记</Button>
          </div>

          <p className="question-stem">{currentQuestion.stem}<span>（{currentQuestion.score}分）</span></p>

          <DesktopQuestionBody
            answer={answers[currentQuestionNumber]}
            onAnswer={saveAnswer}
            question={currentQuestion}
          />

          <Button className="clear-button" type="button">清空选择</Button>

          {saveMessage ? <div aria-label="自动保存提示" className="save-status" role="status"><CheckCircle2 aria-hidden="true" size={16} />{saveMessage}</div> : null}
          {eventReportMessage ? <div aria-label="切屏事件上报" className="event-status" role="status"><TriangleAlert aria-hidden="true" size={16} />{eventReportMessage}</div> : null}

          <div className="question-actions">
            <Button className="prev-button" type="button"><ChevronLeft aria-hidden="true" size={18} />上一题</Button>
            <label className="favorite-check"><input type="checkbox" />加入收藏</label>
            <Button className="next-button" type="button">下一题<ChevronRight aria-hidden="true" size={18} /></Button>
          </div>
        </section>

        <Button
          aria-label="关闭考试抽屉遮罩"
          className={isExamDrawerOpen ? "drawer-backdrop drawer-backdrop--open" : "drawer-backdrop"}
          onClick={() => setIsExamDrawerOpen(false)}
          type="button"
        />

        <aside
          aria-label="考试辅助抽屉"
          className={isExamDrawerOpen ? "exam-right-column exam-right-column--open" : "exam-right-column"}
          id="exam-helper-drawer"
        >
          <div className="exam-drawer-head">
            <strong>考试辅助</strong>
            <Button aria-label="关闭考试抽屉" onClick={() => setIsExamDrawerOpen(false)} type="button">
              <X aria-hidden="true" size={18} />
            </Button>
          </div>
          <section className="exam-card info-panel">
            <h2>考试信息</h2>
            <p>考试名称：期中考试（高一语文）</p>
            <p>考试时长：120 分钟</p>
            <p>总题目数：45 题</p>
            <p>总分：100 分</p>
          </section>

          <section className="exam-card timer-panel">
            <h2>剩余时间</h2>
            <strong>01:28:36</strong>
            <span><TriangleAlert aria-hidden="true" size={14} />考试中请勿切换页面或离开考试</span>
          </section>

          <section className="exam-card answer-card">
            <h2>答题卡</h2>
            <div className="exam-legend exam-legend--compact">
              <span><i className="legend-dot legend-dot--answered" />已答</span>
              <span><i className="legend-dot legend-dot--blank" />未答</span>
              <span><i className="legend-triangle" />标记</span>
            </div>
            <QuestionGrid answers={answers} compact onSelect={setCurrentQuestionNumber} selectedNumber={currentQuestionNumber} />
            <Button className="submit-button" onClick={() => setIsSubmitDialogOpen(true)} type="button">交 卷</Button>
            <p className="submit-note">交卷后将无法继续作答，请确认已完成</p>
          </section>
        </aside>
      </main>

      {isSubmitDialogOpen && (
        <div className="exam-modal-layer">
          <div
            aria-labelledby="submit-dialog-title"
            aria-modal="true"
            className="mini-dialog exam-submit-dialog"
            role="dialog"
          >
            <span className="alert-icon">!</span>
            <strong id="submit-dialog-title">确认交卷</strong>
            <p>交卷后将无法继续作答，请确认是否交卷？</p>
            <div>
              <Button onClick={() => setIsSubmitDialogOpen(false)} type="button">取消</Button>
              <Button onClick={() => void submitExam()} type="button">确认交卷</Button>
            </div>
          </div>
        </div>
      )}

      {isLeaveDialogOpen && (
        <div className="exam-modal-layer">
          <div
            aria-labelledby="leave-dialog-title"
            aria-modal="true"
            className="leave-dialog exam-leave-dialog"
            role="dialog"
          >
            <TriangleAlert aria-hidden="true" size={28} />
            <strong id="leave-dialog-title">离开页面提醒</strong>
            <p>检测到您将离开考试页面，请确认是否离开？</p>
            <div>
              <Button onClick={() => setIsLeaveDialogOpen(false)} type="button">留在页面</Button>
              <Button type="button">确认离开</Button>
            </div>
          </div>
        </div>
      )}

      <footer className="student-exam-footer">
        <span><i className="legend-dot legend-dot--answered" />自动保存：已开启（每30秒）</span>
        <span>考试过程中如遇问题，请联系监考老师</span>
        <span>当前版本：v1.0.0</span>
      </footer>
    </div>
  );
}

export function StudentExamPage({
  api = examApi,
  tenantID = 10,
  examID = 1,
  userID = 20,
}: StudentExamPageProps) {
  const isMobileExam = useExamViewportMode();

  return isMobileExam ? <MobileExamExperience /> : (
    <DesktopStudentExamPage api={api} tenantID={tenantID} examID={examID} userID={userID} />
  );
}
