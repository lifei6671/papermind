import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { GradingAPI } from "../../api/grading";
import { GradingPage } from "./GradingPage";

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
