import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { ExamEntryAPI } from "../../api/exams";
import { SessionContext } from "../../auth/session-context";
import { ExamEntryPage } from "./ExamEntryPage";

test("考试入口提交邀请码前先调用真实 API 校验", async () => {
  const user = userEvent.setup();
  const api: ExamEntryAPI = {
    resolveInvite: vi.fn(async () => ({
      id: 1,
      tenantID: 10,
      paperID: 100,
      name: "高一语文期中考试",
      paperName: "试卷 100",
      inviteCode: "PM2026",
      target: "未配置",
      createdBy: 501,
      status: "published" as const,
      startAt: "2026-05-26 18:40",
      endAt: "2026-05-26 20:40",
      startTime: new Date("2026-05-26T18:40:00+08:00").getTime(),
      endTime: new Date("2026-05-26T20:40:00+08:00").getTime(),
      durationMinutes: 120,
      maxAttempts: 1,
      resultStrategy: "latest" as const,
      publishMode: "manual_publish" as const,
      scorePublishTime: null,
      targets: [],
    })),
  };

  render(
    <MemoryRouter>
      <SessionContext.Provider value={{
        session: {
          user: { userID: 20, displayName: "目标考生", role: "student", tenantID: 10 },
        },
        signIn: vi.fn(),
        signOut: vi.fn(),
      }}>
        <ExamEntryPage api={api} />
      </SessionContext.Provider>
    </MemoryRouter>,
  );

  expect(screen.queryByText("高一语文期中考试")).not.toBeInTheDocument();
  expect(screen.queryByText("2026-05-30 09:00 - 11:00")).not.toBeInTheDocument();
  await user.type(screen.getByLabelText("邀请码"), "pm2026");
  await user.click(screen.getByRole("button", { name: "进入考试" }));

  await waitFor(() => {
    expect(api.resolveInvite).toHaveBeenCalledWith({ inviteCode: "PM2026" });
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
      <SessionContext.Provider value={{
        session: {
          user: { userID: 20, displayName: "目标考生", role: "student", tenantID: 10 },
        },
        signIn: vi.fn(),
        signOut: vi.fn(),
      }}>
        <ExamEntryPage api={api} />
      </SessionContext.Provider>
    </MemoryRouter>,
  );

  await user.type(screen.getByLabelText("邀请码"), "PM2026");
  await user.click(screen.getByRole("button", { name: "进入考试" }));

  expect(await screen.findByRole("alert")).toHaveTextContent("login or register required for invite");
});

test("考试入口允许当前选中空间内的学生身份进入", async () => {
  const user = userEvent.setup();
  const api: ExamEntryAPI = {
    resolveInvite: vi.fn(async () => ({
      id: 1,
      tenantID: 10,
      paperID: 100,
      name: "补考",
      paperName: "试卷 100",
      inviteCode: "PM2026",
      target: "未配置",
      createdBy: 501,
      status: "published" as const,
      startAt: "2026-05-26 18:40",
      endAt: "2026-05-26 20:40",
      startTime: new Date("2026-05-26T18:40:00+08:00").getTime(),
      endTime: new Date("2026-05-26T20:40:00+08:00").getTime(),
      durationMinutes: 120,
      maxAttempts: 1,
      resultStrategy: "latest" as const,
      publishMode: "manual_publish" as const,
      scorePublishTime: null,
      targets: [],
    })),
  };

  render(
    <MemoryRouter>
      <SessionContext.Provider value={{
        session: {
          selectedSpaceID: 302,
          profileSpaces: [{
            id: 4,
            tenantID: 10,
            tenantName: "明德学校",
            spaceID: 302,
            spaceName: "补考空间",
            role: "student",
            status: "enabled",
          }],
          user: { userID: 20, displayName: "多身份用户", role: "teacher", tenantID: 10 },
        },
        signIn: vi.fn(),
        signOut: vi.fn(),
      }}>
        <ExamEntryPage api={api} />
      </SessionContext.Provider>
    </MemoryRouter>,
  );

  await user.type(screen.getByLabelText("邀请码"), "PM2026");
  await user.click(screen.getByRole("button", { name: "进入考试" }));

  await waitFor(() => {
    expect(api.resolveInvite).toHaveBeenCalledWith({ inviteCode: "PM2026" });
  });
});
