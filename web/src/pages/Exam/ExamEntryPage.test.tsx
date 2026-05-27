import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { ExamEntryAPI } from "../../api/exams";
import { ExamEntryPage } from "./ExamEntryPage";

test("考试入口提交邀请码前先调用真实 API 校验", async () => {
  const user = userEvent.setup();
  const api: ExamEntryAPI = {
    resolveInvite: vi.fn(async () => ({
      id: 1,
      tenantID: 10,
      paperID: 100,
      name: "高一语文期中考试",
      paperName: "高一语文月考试卷",
      inviteCode: "PM2026",
      target: "未配置",
      status: "published" as const,
      startAt: "2026-05-26 18:40",
      endAt: "2026-05-26 20:40",
      durationMinutes: 120,
    })),
  };

  render(
    <MemoryRouter>
      <ExamEntryPage api={api} userID={20} />
    </MemoryRouter>,
  );

  await user.type(screen.getByLabelText("邀请码"), "pm2026");
  await user.click(screen.getByRole("button", { name: "进入考试" }));

  await waitFor(() => {
    expect(api.resolveInvite).toHaveBeenCalledWith({ inviteCode: "PM2026", userID: 20 });
  });
  expect(screen.getByRole("status", { name: "entry-resolve-result" })).toHaveTextContent("高一语文期中考试");
});

test("考试入口邀请码校验失败时停留在入口页", async () => {
  const user = userEvent.setup();
  const api: ExamEntryAPI = {
    resolveInvite: vi.fn(async () => {
      throw new Error("login or register required for invite");
    }),
  };

  render(
    <MemoryRouter>
      <ExamEntryPage api={api} userID={0} />
    </MemoryRouter>,
  );

  await user.type(screen.getByLabelText("邀请码"), "PM2026");
  await user.click(screen.getByRole("button", { name: "进入考试" }));

  expect(await screen.findByRole("alert")).toHaveTextContent("login or register required for invite");
});
