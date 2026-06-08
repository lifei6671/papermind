import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { expect, test, vi } from "vitest";
import { ApiError } from "../../api/client";
import { FeedbackProvider } from "../../app/feedback";
import { ExamManagementPage } from "./ExamManagementPage";

function renderExamManagementPage(page: ReactNode) {
  return render(<FeedbackProvider>{page}</FeedbackProvider>);
}

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
        status: "enabled",
        totalScore: "12",
        createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
        creatorName: "teacher.exam",
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
    listSpaceMembers: vi.fn(),
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
      }, {
        id: 556,
        tenantID: 10,
        name: "李老师",
        username: "teacher01",
        role: "teacher",
        avatarFileName: "未上传",
        status: "enabled",
      }],
    }),
  };
  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  expect(screen.getByRole("tab", { name: "考试列表" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("tab", { name: "考试发布" })).not.toBeInTheDocument();
  expect(screen.queryByText("占位")).not.toBeInTheDocument();
  expect(await screen.findByText("高一语文期中考试")).toBeInTheDocument();
  expect(screen.getByText("PM2026")).toBeInTheDocument();
  expect(api.listExams).toHaveBeenCalledWith({ tenantID: 10 });
  expect(paperApi.listPapers).toHaveBeenCalledWith({ tenantID: 10 });
  expect(spaceApi.listSpaces).toHaveBeenCalledWith(10);
  expect(spaceApi.listSpaceMembers).not.toHaveBeenCalled();
  expect(userApi.listUsers).toHaveBeenCalledWith(10);

  await user.click(screen.getByRole("button", { name: "发布考试" }));

  const dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  expect(within(dialog).queryByText("高二数学阶段测评")).not.toBeInTheDocument();
  for (const labelText of ["发布试卷", "考试日期", "开始时间", "结束时间", "单次作答时长"]) {
    const field = within(dialog).getByLabelText(labelText).closest("label");
    expect(field).not.toBeNull();
    const requiredMarker = within(field as HTMLElement).getByText("*");
    expect(requiredMarker).toHaveClass("required-marker");
    expect(requiredMarker).toHaveAttribute("aria-hidden", "true");
  }
  const publishScopeLegend = within(dialog).getByText("发布范围").closest("legend");
  expect(publishScopeLegend).not.toBeNull();
  expect(within(publishScopeLegend as HTMLElement).getByText("*")).toHaveClass("required-marker");
  expect(within(dialog).getByRole("radio", { name: "班级范围" })).toBeChecked();
  expect(within(dialog).getByRole("radio", { name: "指定人群" })).not.toBeChecked();
  const classRangeSelect = within(dialog).getByLabelText("选择班级范围");
  expect(within(classRangeSelect).getByRole("option", { name: "联调班级" })).toBeInTheDocument();
  expect(within(classRangeSelect).queryByRole("option", { name: "张同学（个人）" })).not.toBeInTheDocument();
  await user.selectOptions(within(dialog).getByLabelText("发布试卷"), "333");
  await user.type(within(dialog).getByLabelText("考试日期"), "2026-05-30");
  await user.type(within(dialog).getByLabelText("开始时间"), "09:00");
  await user.type(within(dialog).getByLabelText("结束时间"), "11:00");
  await user.clear(within(dialog).getByLabelText("单次作答时长"));
  await user.type(within(dialog).getByLabelText("单次作答时长"), "150");
  await user.click(within(dialog).getByRole("button", { name: "确认发布" }));

  expect(screen.getByRole("alert")).toHaveTextContent("作答时长不能超过考试时间窗口");
  expect(screen.getByRole("alert").closest(".feedback-toast-stack")).toBeInTheDocument();
  expect(document.querySelector(".tenant-admin-warning")).not.toBeInTheDocument();

  await user.clear(within(dialog).getByLabelText("单次作答时长"));
  await user.type(within(dialog).getByLabelText("单次作答时长"), "120");
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));
  await user.click(within(dialog).getByRole("button", { name: "指定同学" }));
  expect(within(dialog).getByRole("checkbox", { name: "张同学（个人）" })).toBeInTheDocument();
  expect(within(dialog).queryByRole("checkbox", { name: "李老师（个人）" })).not.toBeInTheDocument();
  await user.click(within(dialog).getByRole("checkbox", { name: "张同学（个人）" }));
  await user.click(within(dialog).getByRole("button", { name: "确认发布" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    durationMinutes: 120,
    name: "联调语文试卷",
    paperID: 333,
    targets: [
      { targetID: 555, targetType: "user" },
    ],
    targetID: 555,
    targetType: "user",
    tenantID: 10,
  }));
  expect(screen.queryByRole("dialog", { name: "发布考试弹窗" })).not.toBeInTheDocument();
  expect(await screen.findByRole("status", { name: "exam-publish-result" })).toHaveTextContent("PM2027");
  const publishedRow = await screen.findByRole("row", { name: /高一 1 班/ });
  expect(publishedRow).toHaveTextContent("联调语文试卷");
  expect(publishedRow).toHaveTextContent("已发布");
});
test("空间教师发布考试时只使用当前空间作为发布范围", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn().mockResolvedValue({
      id: 3,
      tenantID: 10,
      paperID: 333,
      name: "空间语文试卷",
      paperName: "空间语文试卷",
      inviteCode: "PM301",
      target: "当前空间",
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
        name: "空间语文试卷",
        description: "空间教师可发布",
        buildMode: "manual",
        status: "enabled",
        totalScore: "12",
        createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
        creatorName: "teacher.exam",
      }],
    }),
  };
  const spaceApi = {
    listSpaces: vi.fn(),
    listSpaceMembers: vi.fn().mockResolvedValue({
      items: [
        {
          id: 901,
          userID: 777,
          name: "王同学",
          role: "student",
          status: "enabled",
        },
        {
          id: 902,
          userID: 778,
          name: "禁用学生",
          role: "student",
          status: "disabled",
        },
        {
          id: 903,
          userID: 779,
          name: "空间教师",
          role: "teacher",
          status: "enabled",
        },
      ],
    }),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(
    <ExamManagementPage
      api={api}
      canManageTenantTargets={false}
      paperApi={paperApi}
      spaceApi={spaceApi}
      userApi={userApi}
      tenantID={10}
      spaceID={301}
    />,
  );

  await waitFor(() => expect(paperApi.listPapers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 }));
  expect(spaceApi.listSpaces).not.toHaveBeenCalled();
  expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 });
  expect(userApi.listUsers).not.toHaveBeenCalled();

  await user.click(screen.getByRole("button", { name: "发布考试" }));

  const dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  const classRangeSelect = within(dialog).getByLabelText("选择班级范围") as HTMLSelectElement;
  expect(classRangeSelect).toHaveValue("space:301");
  expect(within(classRangeSelect).getByRole("option", { name: "当前空间 301" })).toBeInTheDocument();
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));
  await user.click(within(dialog).getByRole("button", { name: "指定同学" }));
  expect(within(dialog).getByRole("checkbox", { name: "王同学（个人）" })).toBeInTheDocument();
  expect(within(dialog).queryByRole("checkbox", { name: "禁用学生（个人）" })).not.toBeInTheDocument();
  expect(within(dialog).queryByRole("checkbox", { name: "空间教师（个人）" })).not.toBeInTheDocument();
  await user.click(within(dialog).getByRole("radio", { name: "班级范围" }));
  await user.type(within(dialog).getByLabelText("考试日期"), "2026-05-30");
  await user.type(within(dialog).getByLabelText("开始时间"), "09:00");
  await user.type(within(dialog).getByLabelText("结束时间"), "11:00");
  await user.click(within(dialog).getByRole("button", { name: "确认发布" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    targets: [
      { targetID: 301, targetType: "space" },
    ],
    targetID: 301,
    targetType: "space",
  }));
});

test("空间教师读取成员返回 403 时回退到当前空间并提示范围受限", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn().mockResolvedValue({
      id: 4,
      tenantID: 10,
      paperID: 333,
      name: "空间回退发布试卷",
      paperName: "空间回退发布试卷",
      inviteCode: "PM302",
      target: "当前空间",
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
        name: "空间回退发布试卷",
        description: "成员列表失败时仍应可发布",
        buildMode: "manual",
        status: "enabled",
        totalScore: "12",
        createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
        creatorName: "teacher.exam",
      }],
    }),
  };
  const spaceApi = {
    listSpaces: vi.fn(),
    listSpaceMembers: vi.fn().mockRejectedValue(new ApiError("forbidden", 403, 403, { message: "forbidden" })),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(
    <ExamManagementPage
      api={api}
      canManageTenantTargets={false}
      paperApi={paperApi}
      spaceApi={spaceApi}
      userApi={userApi}
      tenantID={10}
      spaceID={301}
    />,
  );

  await waitFor(() => expect(paperApi.listPapers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 }));
  expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 });
  expect(await screen.findByRole("button", { name: "发布考试" })).toBeInTheDocument();
  expect(screen.queryByText("当前身份无法读取空间成员列表，仅支持向当前空间发布考试")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "发布考试" }));

  const dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  expect(within(dialog).getByText("当前身份无法读取空间成员列表，仅支持向当前空间发布考试")).toBeInTheDocument();
  const classRangeSelect = within(dialog).getByLabelText("选择班级范围") as HTMLSelectElement;
  expect(within(classRangeSelect).getAllByRole("option")).toHaveLength(1);
  expect(within(classRangeSelect).getByRole("option", { name: "当前空间 301" })).toBeInTheDocument();
  expect(classRangeSelect).toHaveValue("space:301");

  await user.type(within(dialog).getByLabelText("考试日期"), "2026-05-30");
  await user.type(within(dialog).getByLabelText("开始时间"), "09:00");
  await user.type(within(dialog).getByLabelText("结束时间"), "11:00");
  await user.click(within(dialog).getByRole("button", { name: "确认发布" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    targets: [{ targetID: 301, targetType: "space" }],
    targetID: 301,
    targetType: "space",
  }));
});

test("空间教师读取成员服务异常时不会静默回退成整空间发布", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPapers: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "空间异常发布试卷",
        description: "成员列表异常时不应自动扩大发布范围",
        buildMode: "manual",
        status: "enabled",
        totalScore: "12",
        createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
        creatorName: "teacher.exam",
      }],
    }),
  };
  const spaceApi = {
    listSpaces: vi.fn(),
    listSpaceMembers: vi.fn().mockRejectedValue(new ApiError("server exploded", 50000, 500, { message: "server exploded" })),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(
    <ExamManagementPage
      api={api}
      canManageTenantTargets={false}
      paperApi={paperApi}
      spaceApi={spaceApi}
      userApi={userApi}
      tenantID={10}
      spaceID={301}
    />,
  );

  await waitFor(() => expect(paperApi.listPapers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 }));
  expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301 });
  expect(await screen.findByRole("alert")).toHaveTextContent("发布选项加载失败");
  expect(screen.queryByText("当前身份无法读取空间成员列表，仅支持向当前空间发布考试")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "发布考试" }));

  const dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  const classRangeSelect = within(dialog).getByLabelText("选择班级范围") as HTMLSelectElement;
  expect(within(classRangeSelect).queryByRole("option", { name: "当前空间 301" })).not.toBeInTheDocument();
  expect(classRangeSelect.options).toHaveLength(0);
});

test("发布选项重载失败时会清空上一次成功加载的试卷和范围", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPapers: vi.fn()
      .mockResolvedValueOnce({
        items: [{
          id: 333,
          tenantID: 10,
          name: "空间 301 试卷",
          description: "首次加载成功",
          buildMode: "manual",
          status: "enabled",
          totalScore: "12",
          createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
          creatorName: "teacher.exam",
        }],
      })
      .mockResolvedValueOnce({
        items: [{
          id: 444,
          tenantID: 10,
          name: "空间 302 试卷",
          description: "本次不会生效，因为整体加载失败",
          buildMode: "manual",
          status: "enabled",
          totalScore: "12",
          createdAt: new Date("2026-06-02T09:10:00+08:00").getTime(),
          creatorName: "teacher.exam",
        }],
      }),
  };
  const spaceApi = {
    listSpaces: vi.fn(),
    listSpaceMembers: vi.fn()
      .mockResolvedValueOnce({
        items: [{
          id: 901,
          userID: 777,
          name: "王同学",
          role: "student",
          status: "enabled",
        }],
      })
      .mockRejectedValueOnce(new ApiError("server exploded", 50000, 500, { message: "server exploded" })),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  const view = renderExamManagementPage(
    <ExamManagementPage
      api={api}
      canManageTenantTargets={false}
      paperApi={paperApi}
      spaceApi={spaceApi}
      userApi={userApi}
      tenantID={10}
      spaceID={301}
    />,
  );

  await waitFor(() => expect(spaceApi.listSpaceMembers).toHaveBeenNthCalledWith(1, { tenantID: 10, spaceID: 301 }));

  await user.click(screen.getByRole("button", { name: "发布考试" }));
  let dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  expect(within(dialog).getByRole("option", { name: "空间 301 试卷" })).toBeInTheDocument();
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));
  await user.click(within(dialog).getByRole("button", { name: "指定同学" }));
  expect(within(dialog).getByRole("checkbox", { name: "王同学（个人）" })).toBeInTheDocument();

  view.rerender(
    <FeedbackProvider>
      <ExamManagementPage
        api={api}
        canManageTenantTargets={false}
        paperApi={paperApi}
        spaceApi={spaceApi}
        userApi={userApi}
        tenantID={10}
        spaceID={302}
      />
    </FeedbackProvider>,
  );

  await waitFor(() => expect(spaceApi.listSpaceMembers).toHaveBeenNthCalledWith(2, { tenantID: 10, spaceID: 302 }));
  expect(await screen.findByRole("alert")).toHaveTextContent("发布选项加载失败");

  dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  const paperSelect = within(dialog).getByLabelText("发布试卷") as HTMLSelectElement;
  const classRangeSelect = within(dialog).getByLabelText("选择班级范围") as HTMLSelectElement;
  expect(paperSelect.options).toHaveLength(0);
  expect(classRangeSelect.options).toHaveLength(0);
  expect(within(dialog).queryByRole("option", { name: "空间 301 试卷" })).not.toBeInTheDocument();
  expect(within(dialog).queryByRole("checkbox", { name: "王同学（个人）" })).not.toBeInTheDocument();
});

test("发布考试时只保留 enabled 试卷并重置默认选择", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPapers: vi.fn().mockResolvedValue({
      items: [
        {
          id: 100,
          tenantID: 10,
          name: "已禁用试卷",
          description: "不应出现在发布候选里",
          buildMode: "manual",
          status: "disabled",
          totalScore: "60",
          createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
          creatorName: "teacher.disabled",
        },
        {
          id: 150,
          tenantID: 10,
          name: "草稿试卷",
          description: "未启用前不应出现在发布候选里",
          buildMode: "manual",
          status: "draft",
          totalScore: "40",
          createdAt: new Date("2026-06-02T09:05:00+08:00").getTime(),
          creatorName: "teacher.draft",
        },
        {
          id: 200,
          tenantID: 10,
          name: "可发布试卷",
          description: "应作为默认选项",
          buildMode: "manual",
          status: "enabled",
          totalScore: "80",
          createdAt: new Date("2026-06-02T09:10:00+08:00").getTime(),
          creatorName: "teacher.enabled",
        },
      ],
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
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  await waitFor(() => expect(paperApi.listPapers).toHaveBeenCalledWith({ tenantID: 10 }));
  await user.click(screen.getByRole("button", { name: "发布考试" }));

  const dialog = screen.getByRole("dialog", { name: "发布考试弹窗" });
  const paperSelect = within(dialog).getByLabelText("发布试卷");

  expect(within(paperSelect).queryByRole("option", { name: "已禁用试卷" })).not.toBeInTheDocument();
  expect(within(paperSelect).queryByRole("option", { name: "草稿试卷" })).not.toBeInTheDocument();
  expect(within(paperSelect).getByRole("option", { name: "可发布试卷" })).toBeInTheDocument();
  expect(paperSelect).toHaveValue("200");
});
