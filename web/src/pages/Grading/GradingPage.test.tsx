import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useNavigate } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { GradingAPI } from "../../api/grading";
import { GradingPage } from "./GradingPage";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

test("阅卷页从真实 API 加载待阅卷列表并保存评分", async () => {
  const user = userEvent.setup();
  const api: GradingAPI = {
    listPendingAttempts: vi.fn().mockResolvedValue({
      items: [{
        attemptID: 900,
        attemptQuestionID: 901,
        studentName: "张三",
        spaceName: "高一 1 班",
        examName: "高一语文期中考试",
        questionTitle: "岳阳楼记思想内涵",
        answerContent: "先忧后乐体现了责任意识。",
        submittedAt: "2026-05-26 18:40",
        maxScore: "10",
        answerVersion: 7,
        pendingShortTextCount: 1,
        status: "pending",
      }],
    }),
    gradeShortText: vi.fn().mockResolvedValue(undefined),
  };

  render(
    <MemoryRouter>
      <GradingPage api={api} tenantID={10} examID={1} actorID={501} actorRole="teacher" spaceID={301} />
    </MemoryRouter>,
  );

  expect(await screen.findByText("张三")).toBeInTheDocument();
  expect(screen.getByText("岳阳楼记思想内涵")).toBeInTheDocument();
  const refreshResult = deferred<Awaited<ReturnType<GradingAPI["listPendingAttempts"]>>>();
  vi.mocked(api.listPendingAttempts).mockReturnValueOnce(refreshResult.promise);

  await user.click(screen.getByRole("button", { name: "刷新待阅卷" }));
  expect(screen.getByRole("button", { name: "刷新待阅卷" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );
  refreshResult.resolve({
    items: [{
      attemptID: 900,
      attemptQuestionID: 901,
      studentName: "张三",
      spaceName: "高一 1 班",
      examName: "高一语文期中考试",
      questionTitle: "岳阳楼记思想内涵",
      answerContent: "先忧后乐体现了责任意识。",
      submittedAt: "2026-05-26 18:40",
      maxScore: "10",
      answerVersion: 7,
      pendingShortTextCount: 1,
      status: "pending",
    }],
  });
  expect(await screen.findByText("岳阳楼记思想内涵")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /张三/ })).getByRole("button", { name: "开始阅卷" }));
  expect(screen.getByText("学生作答：先忧后乐体现了责任意识。")).toBeInTheDocument();
  await user.clear(screen.getByLabelText("评分"));
  await user.type(screen.getByLabelText("评分"), "4.5");
  await user.type(screen.getByLabelText("阅卷评语"), "要点完整");
  await user.click(screen.getByRole("button", { name: "保存阅卷" }));

  expect(api.gradeShortText).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 1,
    actorID: 501,
    actorRole: "teacher",
    spaceID: 301,
    attemptID: 900,
    attemptQuestionID: 901,
    answerVersion: 7,
    score: "4.5",
    comment: "要点完整",
  });
  expect(await screen.findByRole("status", { name: "grading-save-result" })).toHaveTextContent("已保存 4.5 分");
  expect(screen.getByRole("row", { name: /张三/ })).toHaveTextContent("已完成");
});

test("阅卷页缺少考试时不请求后端", () => {
  const api: GradingAPI = {
    listPendingAttempts: vi.fn(),
    gradeShortText: vi.fn(),
  };

  render(
    <MemoryRouter>
      <GradingPage api={api} tenantID={10} actorID={501} actorRole="teacher" spaceID={301} />
    </MemoryRouter>,
  );

  expect(screen.getByText("请先选择考试后再进入阅卷中心。")).toBeInTheDocument();
  expect(api.listPendingAttempts).not.toHaveBeenCalled();
  expect(api.gradeShortText).not.toHaveBeenCalled();
});

test("阅卷页带 attempt_id 时自动打开对应作答", async () => {
  const api: GradingAPI = {
    listPendingAttempts: vi.fn().mockResolvedValue({
      items: [{
        attemptID: 900,
        attemptQuestionID: 901,
        studentName: "张三",
        spaceName: "高一 1 班",
        examName: "高一语文期中考试",
        questionTitle: "岳阳楼记思想内涵",
        answerContent: "先忧后乐体现了责任意识。",
        submittedAt: "2026-05-26 18:40",
        maxScore: "10",
        answerVersion: 7,
        pendingShortTextCount: 1,
        status: "pending",
      }],
    }),
    gradeShortText: vi.fn().mockResolvedValue(undefined),
  };

  render(
    <MemoryRouter initialEntries={["/grading?exam_id=1&space_id=301&attempt_id=900"]}>
      <GradingPage api={api} tenantID={10} actorID={501} actorRole="teacher" />
    </MemoryRouter>,
  );

  expect(await screen.findByText("学生作答：先忧后乐体现了责任意识。")).toBeInTheDocument();
  expect(screen.getByLabelText("评分")).toHaveValue(7);
});

test("阅卷页切换 attempt_id 时会自动打开新的作答记录", async () => {
  const user = userEvent.setup();
  const api: GradingAPI = {
    listPendingAttempts: vi.fn().mockResolvedValue({
      items: [
        {
          attemptID: 900,
          attemptQuestionID: 901,
          studentName: "张三",
          spaceName: "高一 1 班",
          examName: "高一语文期中考试",
          questionTitle: "第一题",
          answerContent: "第一份答案",
          submittedAt: "2026-05-26 18:40",
          maxScore: "10",
          answerVersion: 7,
          pendingShortTextCount: 1,
          status: "pending",
        },
        {
          attemptID: 901,
          attemptQuestionID: 902,
          studentName: "李四",
          spaceName: "高一 1 班",
          examName: "高一语文期中考试",
          questionTitle: "第二题",
          answerContent: "第二份答案",
          submittedAt: "2026-05-26 18:41",
          maxScore: "8",
          answerVersion: 3,
          pendingShortTextCount: 1,
          status: "pending",
        },
      ],
    }),
    gradeShortText: vi.fn().mockResolvedValue(undefined),
  };

  render(
    <MemoryRouter initialEntries={["/grading?exam_id=1&space_id=301&attempt_id=900"]}>
      <Routes>
        <Route path="/grading" element={<GradingRouteHarness api={api} />} />
      </Routes>
    </MemoryRouter>,
  );

  expect(await screen.findByText("学生作答：第一份答案")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "切换到第二份作答" }));

  expect(await screen.findByText("学生作答：第二份答案")).toBeInTheDocument();
  expect(screen.queryByText("学生作答：第一份答案")).not.toBeInTheDocument();
});

function GradingRouteHarness({ api }: { api: GradingAPI }) {
  const navigate = useNavigate();

  return (
    <>
      <button onClick={() => navigate("/grading?exam_id=1&space_id=301&attempt_id=901")} type="button">
        切换到第二份作答
      </button>
      <GradingPage api={api} tenantID={10} actorID={501} actorRole="teacher" />
    </>
  );
}
