import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { expect, test, vi } from "vitest";
import type { ResultsAPI } from "../../api/results";
import { ResultsPage } from "./ResultsPage";

test("成绩页从真实 API 加载成绩、保存发布配置并导出", async () => {
  const user = userEvent.setup();
  const api: ResultsAPI = {
    listResults: vi.fn().mockResolvedValue({
      items: [{
        id: 1,
        studentName: "张三",
        spaceName: "高一 1 班",
        attemptNo: 1,
        objectiveScore: "2",
        subjectiveScore: "4.5",
        totalScore: "6.5",
        submittedAt: "2026-05-26 18:40",
        status: "可发布",
      }],
    }),
    savePublishConfig: vi.fn().mockResolvedValue(undefined),
    exportResults: vi.fn().mockResolvedValue({
      filePath: "server/data/exports/exam-1-scores.csv",
      rowCount: 1,
    }),
  };

  render(
    <MemoryRouter>
      <ResultsPage api={api} tenantID={10} examID={1} actorID={501} actorRole="tenant_admin" />
    </MemoryRouter>,
  );

  expect(await screen.findByText("张三")).toBeInTheDocument();
  expect(screen.getByText("6.5")).toBeInTheDocument();

  await user.selectOptions(screen.getByLabelText("成绩发布模式"), "manual_publish");
  await user.type(screen.getByLabelText("统一公布时间"), "2026-05-30T10:00");
  await user.click(screen.getByRole("button", { name: "保存发布配置" }));

  expect(api.savePublishConfig).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 1,
    actorID: 501,
    actorRole: "tenant_admin",
    spaceID: undefined,
    publishMode: "manual_publish",
    scorePublishTime: new Date("2026-05-30T10:00").getTime(),
  });
  expect(await screen.findByRole("status", { name: "result-publish-config" })).toHaveTextContent("2026-05-30 10:00");

  await user.click(screen.getByRole("button", { name: "导出成绩" }));

  expect(api.exportResults).toHaveBeenCalledWith({
    tenantID: 10,
    examID: 1,
    actorID: 501,
    actorRole: "tenant_admin",
    spaceID: undefined,
  });
  expect(await screen.findByRole("status", { name: "result-export" })).toHaveTextContent("已导出 1 行");
  expect(screen.getByRole("link", { name: "下载导出文件" })).toHaveAttribute("href", "server/data/exports/exam-1-scores.csv");
});

test("教师成绩页不展示导出入口", async () => {
  const api: ResultsAPI = {
    listResults: vi.fn().mockResolvedValue({ items: [] }),
    savePublishConfig: vi.fn(),
    exportResults: vi.fn(),
  };

  render(
    <MemoryRouter>
      <ResultsPage api={api} tenantID={10} examID={1} actorID={501} actorRole="teacher" spaceID={301} />
    </MemoryRouter>,
  );

  expect(await screen.findByRole("heading", { name: "成绩" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "导出成绩" })).not.toBeInTheDocument();
  expect(api.exportResults).not.toHaveBeenCalled();
});
