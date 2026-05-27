import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, test, vi } from "vitest";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { StudentExamPage } from "./StudentExamPage";
import type { StudentExamAPI } from "../../api/exams";

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

function createStudentExamApiDouble(options: { options?: Array<{ id: number; key: string; content: string }> } = {}): StudentExamAPI {
  return {
    startAttempt: vi.fn(async () => ({
      attemptID: 99,
      examToken: "exam-token",
      answerDeadline: 1779795600000,
      questions: [{
        id: 9001,
        number: 1,
        sectionTitle: "一、单项选择题",
        sectionSubtitle: "每题 2 分",
        type: "single" as const,
        stem: "服务端题干",
        score: 2,
        options: options.options ?? [
          { id: 101, key: "A", content: "正确选项" },
          { id: 102, key: "B", content: "干扰项" },
        ],
      }],
    })),
    saveAnswer: vi.fn(async () => undefined),
    submitAttempt: vi.fn(async () => undefined),
    recordEvent: vi.fn(async () => undefined),
  };
}
