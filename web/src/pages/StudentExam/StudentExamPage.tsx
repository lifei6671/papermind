import { Button } from "../../components/ui/Button";
import { useEffect, useState } from "react";
import { examApi } from "../../api/exams";
import type { StudentExamAPI, StudentExamOption, StudentExamQuestion as APIStudentExamQuestion } from "../../api/exams";
import {
  Bookmark,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  DoorOpen,
  TriangleAlert,
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

function buildQuestionGroups(questions: ExamQuestion[]): QuestionGroup[] {
  const groups = new Map<string, QuestionGroup>();
  for (const question of questions) {
    const key = `${question.sectionTitle}\n${question.sectionSubtitle}`;
    const currentGroup = groups.get(key);
    if (currentGroup) {
      currentGroup.questions.push({ number: question.number });
      continue;
    }
    groups.set(key, {
      title: question.sectionTitle,
      subtitle: question.sectionSubtitle,
      questions: [{ number: question.number }],
    });
  }
  return Array.from(groups.values());
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
  groups,
  onSelect,
  selectedNumber,
  answers,
}: {
  compact?: boolean;
  groups: QuestionGroup[];
  onSelect?: (number: number) => void;
  selectedNumber?: number;
  answers?: Record<number, ExamAnswer>;
}) {
  return (
    <div className={compact ? "exam-qnav exam-qnav--compact" : "exam-qnav"}>
      {groups.map((group) => (
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
  const [loadError, setLoadError] = useState("");

  const questionGroups = buildQuestionGroups(apiQuestions);
  const currentQuestion = apiQuestions.find((question) => question.number === currentQuestionNumber);

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
        setLoadError("");
        setCurrentQuestionNumber(questions[0]?.number ?? 1);
      } catch {
        setApiQuestions([]);
        setExamSession(null);
        setLoadError("考试加载失败，请稍后重试。");
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
    if (!currentQuestion) {
      return;
    }
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

  if (!currentQuestion) {
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
          <section className="exam-card exam-result-card" aria-live="polite">
            <strong>{loadError || "正在加载考试题目..."}</strong>
          </section>
        </main>
      </div>
    );
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
          <QuestionGrid
            answers={answers}
            groups={questionGroups}
            onSelect={setCurrentQuestionNumber}
            selectedNumber={currentQuestionNumber}
          />
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
            <span className="question-progress"><strong>{currentQuestion.number}</strong> / {apiQuestions.length}</span>
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
            <QuestionGrid
              answers={answers}
              compact
              groups={questionGroups}
              onSelect={setCurrentQuestionNumber}
              selectedNumber={currentQuestionNumber}
            />
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
  return <DesktopStudentExamPage api={api} tenantID={tenantID} examID={examID} userID={userID} />;
}
