import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { ExamManagementPage } from "./ExamManagementPage";

test("考试页支持配置发布范围、邀请码和发布考试", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({
      items: [
        {
          id: 1,
          tenantID: 10,
          paperID: 100,
          name: "高一语文期中考试",
          paperName: "高一语文月考试卷",
          inviteCode: "PM2026",
          target: "高一全年级",
          status: "published",
          startAt: "2026-05-30 09:00",
          endAt: "2026-05-30 11:00",
          durationMinutes: 120,
        },
      ],
    }),
    publishExam: vi.fn().mockResolvedValue({
      id: 2,
      tenantID: 10,
      paperID: 100,
      name: "联调语文试卷",
      paperName: "联调语文试卷",
      inviteCode: "PM2027",
      target: "高一 1 班",
      status: "published",
      startAt: "2026-05-30 09:00",
      endAt: "2026-05-30 11:00",
      durationMinutes: 120,
    }),
  };
  const paperApi = {
    listPapers: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "联调语文试卷",
        description: "从试卷接口加载",
        buildMode: "manual",
        status: "ready",
        totalScore: "12",
      }],
    }),
  };
  const spaceApi = {
    listSpaces: vi.fn().mockResolvedValue({
      items: [{
        id: 444,
        tenantID: 10,
        name: "联调班级",
        description: "从空间接口加载",
        logoFileName: "未上传",
        members: [],
      }],
    }),
  };
  const userApi = {
    listUsers: vi.fn().mockResolvedValue({
      items: [{
        id: 555,
        tenantID: 10,
        name: "张同学",
        username: "student01",
        role: "student",
        avatarFileName: "未上传",
        status: "enabled",
      }],
    }),
  };
  render(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  expect(screen.getByRole("tab", { name: "考试发布" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByText("占位")).not.toBeInTheDocument();
  expect(await screen.findByText("高一语文期中考试")).toBeInTheDocument();
  expect(screen.getByText("PM2026")).toBeInTheDocument();
  expect(api.listExams).toHaveBeenCalledWith(10);
  expect(paperApi.listPapers).toHaveBeenCalledWith({ tenantID: 10 });
  expect(spaceApi.listSpaces).toHaveBeenCalledWith(10);
  expect(userApi.listUsers).toHaveBeenCalledWith(10);

  await user.click(screen.getByRole("button", { name: "发布考试" }));

  const dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  expect(within(dialog).queryByText("高二数学阶段测评")).not.toBeInTheDocument();
  await user.selectOptions(within(dialog).getByLabelText("发布试卷"), "333");
  await user.type(within(dialog).getByLabelText("考试开始时间"), "2026-05-30T09:00");
  await user.type(within(dialog).getByLabelText("考试结束时间"), "2026-05-30T11:00");
  await user.clear(within(dialog).getByLabelText("单次作答时长"));
  await user.type(within(dialog).getByLabelText("单次作答时长"), "150");
  await user.click(within(dialog).getByRole("button", { name: "确认发布" }));

  expect(screen.getByRole("alert")).toHaveTextContent("作答时长不能超过考试时间窗口");

  await user.clear(within(dialog).getByLabelText("单次作答时长"));
  await user.type(within(dialog).getByLabelText("单次作答时长"), "120");
  await user.selectOptions(within(dialog).getByLabelText("发布范围"), "space:444");
  await user.click(within(dialog).getByRole("button", { name: "确认发布" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    durationMinutes: 120,
    name: "联调语文试卷",
    paperID: 333,
    targetID: 444,
    targetType: "space",
    tenantID: 10,
  }));
  expect(screen.queryByRole("dialog", { name: "发布考试弹窗" })).not.toBeInTheDocument();
  expect(await screen.findByRole("status", { name: "exam-publish-result" })).toHaveTextContent("PM2027");
  const publishedRow = await screen.findByRole("row", { name: /高一 1 班/ });
  expect(publishedRow).toHaveTextContent("联调语文试卷");
  expect(publishedRow).toHaveTextContent("已发布");
});
