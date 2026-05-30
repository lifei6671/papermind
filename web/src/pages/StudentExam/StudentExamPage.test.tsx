import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { StudentExamPage } from "./StudentExamPage";
import type { StudentExamAPI, StudentExamQuestion } from "../../api/exams";

describe("StudentExamPage", () => {
  const originalMatchMedia = window.matchMedia;

  afterEach(() => {
    Object.defineProperty(window, "matchMedia", {
      value: originalMatchMedia,
      writable: true,
    });
  });

  test("桌面答题页通过真实 API 开考、保存答案并交卷", async () => {
    const user = userEvent.setup();
    const api = createStudentExamApiDouble();

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    expect(await screen.findByText("服务端题干")).toBeInTheDocument();
    await user.click(screen.getByLabelText(/A\./));

    await waitFor(() => {
      expect(api.saveAnswer).toHaveBeenCalledWith({
        tenantID: 10,
        attemptID: 99,
        attemptQuestionID: 9001,
        examToken: "exam-token",
        questionType: "single",
        optionIDs: [101],
        text: "A",
      });
    });

    await user.click(screen.getByRole("button", { name: "考试信息与答题卡" }));
    await user.click(screen.getByRole("button", { name: "交 卷" }));
    const dialog = screen.getByRole("dialog", { name: "确认交卷" });
    await user.click(within(dialog).getByRole("button", { name: "确认交卷" }));

    await waitFor(() => {
      expect(api.submitAttempt).toHaveBeenCalledWith({
        tenantID: 10,
        attemptID: 99,
        examToken: "exam-token",
      });
    });
    expect(await screen.findByText("总分 8 分")).toBeInTheDocument();
  });

  test("答题页不展示固定考试信息，按服务端题目派生统计", async () => {
    const user = userEvent.setup();
    const api = createStudentExamApiDouble({
      questions: [
        createStudentExamQuestion({ id: 9001, number: 1, score: 2, stem: "服务端题干 1" }),
        createStudentExamQuestion({ id: 9002, number: 2, score: 10, stem: "服务端题干 2" }),
      ],
    });

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    expect(await screen.findByRole("heading", { name: "在线考试" })).toBeInTheDocument();
    expect(screen.queryByText("期中考试（高一语文）")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "考试信息与答题卡" }));

    expect(screen.getByText("考试名称：在线考试")).toBeInTheDocument();
    expect(screen.getByText("考试时长：以服务端截止时间为准")).toBeInTheDocument();
    expect(screen.getByText("总题目数：2 题")).toBeInTheDocument();
    expect(screen.getByText("总分：12 分")).toBeInTheDocument();
    expect(screen.queryByText("考试时长：120 分钟")).not.toBeInTheDocument();
    expect(screen.queryByText("总题目数：45 题")).not.toBeInTheDocument();
    expect(screen.queryByText("总分：100 分")).not.toBeInTheDocument();
  });

  test("窄屏答题页同样使用真实 API 题目", async () => {
    mockExamViewport(true);
    const api = createStudentExamApiDouble();

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    expect(await screen.findByText("服务端题干")).toBeInTheDocument();
    expect(api.startAttempt).toHaveBeenCalledWith({ tenantID: 10, examID: 1 });
    expect(screen.queryByLabelText("开考前说明")).not.toBeInTheDocument();
  });

  test("选项作答按 optionMeta 提交 optionIDs 而不是反解析展示文案", async () => {
    const user = userEvent.setup();
    const api = createStudentExamApiDouble({
      options: [
        { id: 101, key: "A", content: "第一个选项" },
        { id: 105, key: "E", content: "第五个选项" },
      ],
    });

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    await user.click(await screen.findByLabelText("E.第五个选项"));

    await waitFor(() => {
      expect(api.saveAnswer).toHaveBeenCalledWith(expect.objectContaining({
        optionIDs: [105],
        text: "E",
      }));
    });
  });

  test("上一题和下一题按服务端题目顺序切换当前题目", async () => {
    const user = userEvent.setup();
    const api = createStudentExamApiDouble({
      questions: [
        createStudentExamQuestion({ id: 9001, number: 1, stem: "服务端题干 1" }),
        createStudentExamQuestion({ id: 9002, number: 2, stem: "服务端题干 2" }),
      ],
    });

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    expect(await screen.findByText("服务端题干 1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "下一题" }));

    expect(screen.getByText("服务端题干 2")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "上一题" }));

    expect(screen.getByText("服务端题干 1")).toBeInTheDocument();
  });

  test("清空选择会移除当前题答案并向后端同步空答案", async () => {
    const user = userEvent.setup();
    const api = createStudentExamApiDouble();

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    const option = await screen.findByLabelText(/A\./);
    await user.click(option);
    await waitFor(() => expect(api.saveAnswer).toHaveBeenCalledWith(expect.objectContaining({
      optionIDs: [101],
      text: "A",
    })));

    await user.click(screen.getByRole("button", { name: "清空选择" }));

    expect(option).not.toBeChecked();
    await waitFor(() => expect(api.saveAnswer).toHaveBeenLastCalledWith({
      tenantID: 10,
      attemptID: 99,
      attemptQuestionID: 9001,
      examToken: "exam-token",
      questionType: "single",
      optionIDs: [],
      text: "",
    }));
  });

  test("答题页不展示伪造的固定考生身份", async () => {
    const user = userEvent.setup();
    const api = createStudentExamApiDouble();

    renderStudentExam(<StudentExamPage api={api} tenantID={10} examID={1} userID={20} />);

    const profileButton = await screen.findByRole("button", { name: "考生" });
    expect(screen.queryByRole("button", { name: "张三" })).not.toBeInTheDocument();

    await user.click(profileButton);

    expect(screen.getByRole("menu", { name: "考生信息" })).toHaveTextContent("身份已校验");
    expect(screen.queryByText("考生编号：S1001001")).not.toBeInTheDocument();
  });
});

function mockExamViewport(matchesNarrow: boolean) {
  Object.defineProperty(window, "matchMedia", {
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query === "(max-width: 1100px)" ? matchesNarrow : false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
    writable: true,
  });
}

function renderStudentExam(element: ReactElement) {
  return render(<MemoryRouter>{element}</MemoryRouter>);
}

function createStudentExamApiDouble(options: {
  options?: Array<{ id: number; key: string; content: string }>;
  questions?: StudentExamQuestion[];
} = {}): StudentExamAPI {
  return {
    startAttempt: vi.fn(async () => ({
      attemptID: 99,
      examToken: "exam-token",
      answerDeadline: 1779795600000,
      questions: options.questions ?? [createStudentExamQuestion(options.options ? { options: options.options } : undefined)],
    })),
    saveAnswer: vi.fn(async () => undefined),
    submitAttempt: vi.fn(async () => undefined),
    getVisibleResult: vi.fn(async () => ({
      attemptID: 99,
      examID: 1,
      attemptNo: 1,
      objectiveScore: "6",
      subjectiveScore: "2",
      totalScore: "8",
      analysisVisible: true,
    })),
    recordEvent: vi.fn(async () => undefined),
  };
}

function createStudentExamQuestion(overrides: Partial<StudentExamQuestion> = {}): StudentExamQuestion {
  return {
    id: 9001,
    number: 1,
    sectionTitle: "一、单项选择题",
    sectionSubtitle: "每题 2 分",
    type: "single",
    stem: "服务端题干",
    score: 2,
    options: [
      { id: 101, key: "A", content: "正确选项" },
      { id: 102, key: "B", content: "干扰项" },
    ],
    ...overrides,
  };
}
