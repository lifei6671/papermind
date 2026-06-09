import { readFileSync } from "node:fs";
import { join } from "node:path";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import ConfigProvider from "antd/es/config-provider";
import zhCN from "antd/es/locale/zh_CN";
import type { ReactNode } from "react";
import { expect, test, vi } from "vitest";
import { ApiError } from "../../api/client";
import { FeedbackProvider } from "../../app/feedback";
import { ExamManagementPage } from "./ExamManagementPage";

function renderExamManagementPage(page: ReactNode) {
  return render(withExamManagementProviders(page));
}

function withExamManagementProviders(page: ReactNode) {
  return (
    <ConfigProvider locale={zhCN}>
      <FeedbackProvider>{page}</FeedbackProvider>
    </ConfigProvider>
  );
}

function changePickerInput(field: HTMLElement, value: string) {
  const input = field.querySelector("input");
  expect(input).not.toBeNull();
  fireEvent.change(input as HTMLInputElement, { target: { value } });
  fireEvent.blur(input as HTMLInputElement);
}

function changeRangePickerInputs(field: HTMLElement, startValue: string, endValue: string) {
  const inputs = field.querySelectorAll("input");
  expect(inputs).toHaveLength(2);
  fireEvent.change(inputs[0], { target: { value: startValue } });
  fireEvent.blur(inputs[0]);
  fireEvent.change(inputs[1], { target: { value: endValue } });
  fireEvent.blur(inputs[1]);
}

async function selectStudentTarget(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement, keyword: string, label: string) {
  const studentAutocomplete = within(dialog).getByRole("combobox", { name: "指定同学" });
  await user.clear(studentAutocomplete);
  await user.type(studentAutocomplete, keyword);
  await user.click(await within(dialog).findByText(label));
}

async function selectAntOption(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement, label: string, optionName: string) {
  const combobox = within(dialog).getByRole("combobox", { name: label });
  await user.click(combobox);
  await user.click(await screen.findByRole("option", { name: optionName }));
}

async function selectExamStatus(user: ReturnType<typeof userEvent.setup>, dialog: HTMLElement, status: "draft" | "published") {
  await selectAntOption(user, dialog, "考试状态", status === "draft" ? "草稿" : "已发布");
}

function getVisibleDateCell(container: HTMLElement, date: string) {
  const cell = container.querySelector(`td.ant-picker-cell-in-view[title="${date}"] .ant-picker-cell-inner`);
  expect(cell).toBeInTheDocument();
  return cell as HTMLElement;
}

test("考试列表支持分页和切换每页条数", async () => {
  const user = userEvent.setup();
  const exams = Array.from({ length: 7 }, (_, index) => ({
    id: index + 1,
    tenantID: 10,
    paperID: 100 + index,
    name: `分页考试 ${index + 1}`,
    paperName: `分页试卷 ${index + 1}`,
    inviteCode: `PM${index + 1}`,
    target: "高一全年级",
    status: "published" as const,
    startAt: "2026-05-30 09:00",
    endAt: "2026-05-30 11:00",
    durationMinutes: 120,
  }));
  const api = {
    listExams: vi.fn().mockImplementation(async ({ page, pageSize }: { page: number; pageSize: number }) => ({
      items: exams.slice((page - 1) * pageSize, page * pageSize),
      total: exams.length,
    })),
    publishExam: vi.fn(),
  };
  const paperApi = { listPublishPaperCandidates: vi.fn().mockResolvedValue({ items: [] }) };
  const spaceApi = {
    listSpaces: vi.fn().mockResolvedValue({ items: [] }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  expect(await screen.findByText("分页考试 1")).toBeInTheDocument();
  expect(api.listExams).toHaveBeenCalledWith({ tenantID: 10, page: 1, pageSize: 5 });
  expect(screen.getByText("分页考试 5")).toBeInTheDocument();
  expect(screen.queryByText("分页考试 6")).not.toBeInTheDocument();

  const pagination = screen.getByRole("navigation", { name: "分页" });
  expect(within(pagination).getByText("共 7 条")).toBeInTheDocument();
  expect(within(pagination).getByText("第 1 / 2 页")).toBeInTheDocument();
  expect(within(pagination).getByRole("combobox", { name: "每页条数" })).toBeInTheDocument();

  await user.click(within(pagination).getByRole("button", { name: "下一页" }));
  await waitFor(() => expect(api.listExams).toHaveBeenLastCalledWith({ tenantID: 10, page: 2, pageSize: 5 }));
  expect(await screen.findByText("分页考试 6")).toBeInTheDocument();
  expect(screen.queryByText("分页考试 1")).not.toBeInTheDocument();

  await user.click(within(pagination).getByRole("combobox", { name: "每页条数" }));
  await user.click(screen.getByRole("option", { name: "10 条 / 页" }));
  await waitFor(() => expect(api.listExams).toHaveBeenLastCalledWith({ tenantID: 10, page: 1, pageSize: 10 }));
  expect(await screen.findByText("分页考试 1")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 1 页")).toBeInTheDocument();
  expect(screen.getByText("分页考试 7")).toBeInTheDocument();
});

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
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
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
      items: [
        {
          id: 444,
          tenantID: 10,
          name: "联调班级",
          description: "从空间接口加载",
          logoFileName: "未上传",
          status: "enabled",
          members: [],
        },
        {
          id: 445,
          tenantID: 10,
          name: "已禁用班级",
          description: "禁用后不应发布考试",
          logoFileName: "未上传",
          status: "disabled",
          members: [],
        },
      ],
    }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = {
    listUsers: vi.fn()
      .mockResolvedValue({
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
        total: 2,
      }),
  };
  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  expect(screen.getByRole("tab", { name: "考试列表" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("tab", { name: "考试发布" })).not.toBeInTheDocument();
  expect(screen.queryByText("占位")).not.toBeInTheDocument();
  expect(await screen.findByText("高一语文期中考试")).toBeInTheDocument();
  expect(screen.getByText("PM2026")).toBeInTheDocument();
  expect(api.listExams).toHaveBeenCalledWith({ tenantID: 10, page: 1, pageSize: 5 });
  expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" });
  expect(spaceApi.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
    tenantID: 10,
    page: 1,
    pageSize: 100,
    filters: { status: "enabled" },
  }));
  expect(spaceApi.listSpaceMembers).not.toHaveBeenCalled();
  expect(userApi.listUsers).not.toHaveBeenCalledWith(expect.objectContaining({ tenantID: 10, page: 1, pageSize: 100 }));

  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(within(dialog).queryByLabelText("搜索发布试卷")).not.toBeInTheDocument();
  expect(within(dialog).queryByText("高二数学阶段测评")).not.toBeInTheDocument();
  for (const labelText of ["发布试卷", "考试名称", "考试日期", "开始时间", "结束时间"]) {
    const field = within(dialog).getByLabelText(labelText).closest("label");
    expect(field).not.toBeNull();
    const requiredMarker = within(field as HTMLElement).getByText("*");
    expect(requiredMarker).toHaveClass("required-marker");
    expect(requiredMarker).toHaveAttribute("aria-hidden", "true");
  }
  expect(within(dialog).getByRole("radio", { name: "固定场次" })).toBeChecked();
  expect(within(dialog).getByRole("radio", { name: "开放时间窗" })).not.toBeChecked();
  expect(within(dialog).queryByLabelText("单次作答时长")).not.toBeInTheDocument();
  expect(within(dialog).getByRole("combobox", { name: "考试状态" })).toBeInTheDocument();
  const publishScopeLegend = within(dialog).getByText("发布范围").closest("legend");
  expect(publishScopeLegend).not.toBeNull();
  expect(within(publishScopeLegend as HTMLElement).getByText("*")).toHaveClass("required-marker");
  expect(within(dialog).getByRole("radio", { name: "班级范围" })).toBeChecked();
  expect(within(dialog).getByRole("radio", { name: "指定人群" })).not.toBeChecked();
  const classRangeSelect = within(dialog).getByRole("combobox", { name: "选择班级范围" });
  await user.click(classRangeSelect);
  expect(await screen.findByRole("option", { name: "联调班级" })).toBeInTheDocument();
  expect(within(dialog).queryByRole("option", { name: "张同学（个人）" })).not.toBeInTheDocument();
  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));
  await selectExamStatus(user, dialog, "published");
  await selectExamStatus(user, dialog, "published");
  const studentTargetInput = within(dialog).getByRole("combobox", { name: "指定同学" });
  expect(studentTargetInput).toHaveClass("exam-user-target-input");
  expect(studentTargetInput.closest(".ant-select-auto-complete")).not.toBeInTheDocument();
  await selectStudentTarget(user, dialog, "张", "张同学（个人）");
  await waitFor(() => expect(userApi.listUsers).toHaveBeenLastCalledWith(expect.objectContaining({
    tenantID: 10,
    page: 1,
    pageSize: 10,
    search: "张",
    filters: { role: "student", status: "enabled" },
  })));
  expect(within(dialog).queryByText("李老师（个人）")).not.toBeInTheDocument();
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

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
    status: "published",
  }));
  await waitFor(() => expect(screen.queryByRole("dialog", { name: "新建考试抽屉" })).not.toBeInTheDocument());
  expect(screen.queryByRole("status", { name: "exam-publish-result" })).not.toBeInTheDocument();
  expect(await screen.findByText("联调语文试卷 已发布，邀请码 PM2027")).toBeInTheDocument();
  const publishedRow = await screen.findByRole("row", { name: /高一 1 班/ });
  expect(publishedRow).toHaveTextContent("联调语文试卷");
  expect(publishedRow).toHaveTextContent("已发布");
});

test("新建考试表单提交考试名称、作答规则、成绩公布配置和多个班级目标", async () => {
  const user = userEvent.setup();
  const scorePublishTime = new Date("2026-05-31T18:00:00+08:00").getTime();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [], page: 1, pageSize: 5, total: 0 }),
    publishExam: vi.fn().mockResolvedValue({
      id: 3,
      tenantID: 10,
      paperID: 333,
      name: "高一联考正式考试",
      paperName: "联调语文试卷",
      inviteCode: "",
      target: "联调班级、联调班级二班",
      status: "draft",
      startAt: "2026-05-30 09:00",
      endAt: "2026-05-30 11:00",
      durationMinutes: 120,
    }),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
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
      items: [
        {
          id: 444,
          tenantID: 10,
          name: "联调班级",
          description: "从空间接口加载",
          logoFileName: "未上传",
          status: "enabled",
          members: [],
        },
        {
          id: 446,
          tenantID: 10,
          name: "联调班级二班",
          description: "从空间接口加载",
          logoFileName: "未上传",
          status: "enabled",
          members: [],
        },
      ],
    }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };
  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  await user.type(within(dialog).getByRole("textbox", { name: "考试名称" }), "高一联考正式考试");
  await selectExamStatus(user, dialog, "draft");
  expect(within(dialog).getByRole("spinbutton", { name: "最多作答次数" })).toHaveDisplayValue("");
  expect(within(dialog).getByText("留空表示不限制作答次数；填写数字时必须为正整数。包含主观题的试卷必须填写 1 次。")).toHaveClass("field-note");
  await user.clear(within(dialog).getByRole("spinbutton", { name: "最多作答次数" }));
  await user.type(within(dialog).getByRole("spinbutton", { name: "最多作答次数" }), "2");
  await selectAntOption(user, dialog, "多次作答成绩", "最高分");
  expect(within(dialog).getByText("多次作答时按所选策略计算最终成绩。")).toHaveClass("field-note");
  expect(within(dialog).getByText("手动发布时成绩由教师确认后展示；立即出分会在提交后展示客观题和已完成评分的成绩。")).toHaveClass("field-note");
  await selectAntOption(user, dialog, "成绩发布方式", "手动发布成绩");
  expect(within(dialog).getByText("可选；留空时按成绩发布方式和系统默认规则展示成绩。")).toHaveClass("field-note");
  changePickerInput(within(dialog).getByLabelText("成绩公布时间").closest("label") as HTMLElement, "2026-05-31 18:00");
  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");

  await selectAntOption(user, dialog, "选择班级范围", "联调班级二班");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    name: "高一联考正式考试",
    maxAttempts: 2,
    resultStrategy: "highest",
    publishMode: "manual_publish",
    scorePublishTime,
    targets: [
      { targetID: 444, targetType: "space" },
      { targetID: 446, targetType: "space" },
    ],
    targetID: 444,
    targetType: "space",
  }));
});

test("新建考试默认不限制作答次数", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn().mockResolvedValue({
      id: 88,
      tenantID: 10,
      paperID: 333,
      name: "默认不限制考试",
      paperName: "高二语文期末考试",
      inviteCode: "PM8888",
      target: "联调班级",
      status: "draft",
      startAt: "2026-05-30 09:00",
      endAt: "2026-05-30 11:00",
      durationMinutes: 120,
    }),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "高二语文期末考试",
        status: "enabled",
      }],
    }),
  };
  const spaceApi = {
    listSpaces: vi.fn().mockResolvedValue({
      items: [{
        id: 444,
        tenantID: 10,
        name: "联调班级",
        description: "",
        logoFileName: "未上传",
        status: "enabled",
        members: [],
      }],
    }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };
  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  await selectExamStatus(user, dialog, "draft");
  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");

  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    maxAttempts: 0,
  }));
});

test("考试发布班级范围会读取后端空间分页的后续页", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "分页空间试卷",
        description: "发布范围测试",
        buildMode: "manual",
        status: "enabled",
        totalScore: "12",
        createdAt: new Date("2026-06-02T09:00:00+08:00").getTime(),
        creatorName: "teacher.exam",
      }],
    }),
  };
  const firstPageSpaces = Array.from({ length: 100 }, (_, index) => ({
    id: 300 + index,
    tenantID: 10,
    name: `第 ${index + 1} 班`,
    description: "分页空间",
    logoFileName: "class.png",
    status: "enabled" as const,
    members: [],
  }));
  const spaceApi = {
    listSpaces: vi.fn().mockImplementation(async (input) => ({
      items: input.page === 1
        ? firstPageSpaces
        : [{
          id: 999,
          tenantID: 10,
          name: "第 101 班",
          description: "后续页空间",
          logoFileName: "class-101.png",
          status: "enabled",
          members: [],
        }],
      total: 101,
    })),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  await waitFor(() => expect(spaceApi.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
    tenantID: 10,
    page: 2,
    pageSize: 100,
    filters: { status: "enabled" },
  })));
  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });

  await user.click(within(dialog).getByRole("combobox", { name: "选择班级范围" }));
  expect(await screen.findByRole("option", { name: "第 101 班" })).toBeInTheDocument();
});

test("发布试卷支持输入后延迟走后端检索前十项", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockImplementation(async (input) => ({
      items: input.search === "期末"
        ? [{
          id: 901,
          tenantID: 10,
          name: "高三语文期末考试",
          description: "后端检索结果",
          buildMode: "manual",
          status: "enabled",
          totalScore: "100",
          createdAt: new Date("2026-06-08T09:00:00+08:00").getTime(),
          creatorName: "teacher.search",
        }]
        : [{
          id: 333,
          tenantID: 10,
          name: "最新可用试卷",
          description: "默认前十项",
          buildMode: "manual",
          status: "enabled",
          totalScore: "80",
          createdAt: new Date("2026-06-09T09:00:00+08:00").getTime(),
          creatorName: "teacher.latest",
        }],
      total: input.search === "期末" ? 1 : 10,
    })),
  };
  const spaceApi = {
    listSpaces: vi.fn().mockResolvedValue({ items: [] }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));

  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  const paperCombobox = within(dialog).getByRole("combobox", { name: "发布试卷" });
  expect(paperCombobox.tagName).toBe("INPUT");

  await user.type(paperCombobox, "期末");
  expect(paperApi.listPublishPaperCandidates).not.toHaveBeenCalledWith({ tenantID: 10, search: "期末" });

  await act(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 350));
  });
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "期末" }));
});

test("新建考试可保存为草稿并使用 toast 提示", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn().mockResolvedValue({
      id: 9,
      tenantID: 10,
      paperID: 333,
      name: "草稿语文试卷",
      paperName: "草稿语文试卷",
      inviteCode: "",
      target: "联调班级",
      status: "draft",
      startAt: "2026-05-30 09:00",
      endAt: "2026-05-30 11:00",
      durationMinutes: 120,
    }),
    updateExamStatus: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "草稿语文试卷",
        description: "保存草稿",
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
        status: "enabled",
        members: [],
      }],
    }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");
  await selectExamStatus(user, dialog, "draft");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    status: "draft",
    targets: [{ targetID: 444, targetType: "space" }],
  }));
  expect(await screen.findByText("草稿语文试卷 已保存为草稿")).toBeInTheDocument();
  expect(await screen.findByRole("row", { name: /草稿语文试卷/ })).toHaveTextContent("草稿");
  await waitFor(() => expect(screen.queryByRole("dialog", { name: "新建考试抽屉" })).not.toBeInTheDocument());

  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const reopenedDialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(within(reopenedDialog).getByRole("combobox", { name: "考试状态" })).toHaveValue("");
});

test("考试列表支持发布、提前结束和禁用考试", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({
      items: [
        {
          id: 11,
          tenantID: 10,
          paperID: 333,
          name: "草稿考试",
          paperName: "语文试卷",
          inviteCode: "",
          target: "联调班级",
          status: "draft",
          startAt: "2026-05-30 09:00",
          endAt: "2026-05-30 11:00",
          durationMinutes: 120,
        },
        {
          id: 12,
          tenantID: 10,
          paperID: 334,
          name: "进行中考试",
          paperName: "数学试卷",
          inviteCode: "PM1200",
          target: "联调班级",
          status: "published",
          startAt: "2026-05-30 09:00",
          endAt: "2026-05-30 11:00",
          durationMinutes: 120,
        },
      ],
    }),
    publishExam: vi.fn(),
    updateExamStatus: vi.fn()
      .mockResolvedValueOnce({
        id: 11,
        tenantID: 10,
        paperID: 333,
        name: "草稿考试",
        paperName: "语文试卷",
        inviteCode: "PM1100",
        target: "联调班级",
        status: "published",
        startAt: "2026-05-30 09:00",
        endAt: "2026-05-30 11:00",
        durationMinutes: 120,
      })
      .mockResolvedValueOnce({
        id: 12,
        tenantID: 10,
        paperID: 334,
        name: "进行中考试",
        paperName: "数学试卷",
        inviteCode: "PM1200",
        target: "联调班级",
        status: "closed",
        startAt: "2026-05-30 09:00",
        endAt: "2026-05-30 10:00",
        durationMinutes: 120,
      }),
  };
  const paperApi = { listPublishPaperCandidates: vi.fn().mockResolvedValue({ items: [] }) };
  const spaceApi = {
    listSpaces: vi.fn().mockResolvedValue({ items: [] }),
    listSpaceMembers: vi.fn(),
  };
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  const draftRow = await screen.findByRole("row", { name: /草稿考试/ });
  expect(within(draftRow).queryByRole("button", { name: "禁用考试" })).not.toBeInTheDocument();
  await user.click(within(draftRow).getByRole("button", { name: "发布考试" }));
  expect(api.updateExamStatus).not.toHaveBeenCalled();
  const publishConfirm = screen.getByRole("dialog", { name: "确认发布考试" });
  expect(publishConfirm).toHaveTextContent("发布后将生成邀请码");
  await user.click(within(publishConfirm).getByRole("button", { name: "取消" }));
  expect(api.updateExamStatus).not.toHaveBeenCalled();

  await user.click(within(draftRow).getByRole("button", { name: "发布考试" }));
  await user.click(within(screen.getByRole("dialog", { name: "确认发布考试" })).getByRole("button", { name: "确认发布" }));
  expect(api.updateExamStatus).toHaveBeenNthCalledWith(1, { tenantID: 10, examID: 11, status: "published" });
  expect(await screen.findByText("草稿考试 已发布，邀请码 PM1100")).toBeInTheDocument();

  const publishedRow = await screen.findByRole("row", { name: /进行中考试/ });
  await user.click(within(publishedRow).getByRole("button", { name: "提前结束考试" }));
  const closeConfirm = screen.getByRole("dialog", { name: "确认提前结束考试" });
  expect(closeConfirm).toHaveTextContent("考生将不能继续开始或提交本场考试");
  await user.click(within(closeConfirm).getByRole("button", { name: "确认结束" }));
  expect(api.updateExamStatus).toHaveBeenNthCalledWith(2, { tenantID: 10, examID: 12, status: "closed" });
  expect(await screen.findByText("进行中考试 已提前结束")).toBeInTheDocument();

  const closedRow = await screen.findByRole("row", { name: /进行中考试/ });
  expect(within(closedRow).queryByRole("button", { name: "禁用考试" })).not.toBeInTheDocument();
  expect(closedRow).toHaveTextContent("暂无操作");
});

test("空间教师查看考试列表时不展示租户级考试状态操作", async () => {
  const api = {
    listExams: vi.fn().mockResolvedValue({
      items: [
        {
          id: 21,
          tenantID: 10,
          paperID: 333,
          name: "空间草稿考试",
          paperName: "语文试卷",
          inviteCode: "",
          target: "当前空间",
          status: "draft",
          startAt: "2026-05-30 09:00",
          endAt: "2026-05-30 11:00",
          durationMinutes: 120,
        },
        {
          id: 22,
          tenantID: 10,
          paperID: 334,
          name: "空间进行中考试",
          paperName: "数学试卷",
          inviteCode: "PM2200",
          target: "当前空间",
          status: "published",
          startAt: "2026-05-30 09:00",
          endAt: "2026-05-30 11:00",
          durationMinutes: 120,
        },
      ],
      total: 2,
    }),
    publishExam: vi.fn(),
    updateExamStatus: vi.fn(),
  };
  const paperApi = { listPublishPaperCandidates: vi.fn().mockResolvedValue({ items: [] }) };
  const spaceApi = {
    listSpaces: vi.fn(),
    listSpaceMembers: vi.fn().mockResolvedValue({ items: [] }),
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

  const draftRow = await screen.findByRole("row", { name: /空间草稿考试/ });
  const publishedRow = await screen.findByRole("row", { name: /空间进行中考试/ });
  expect(within(draftRow).queryByRole("button", { name: "发布考试" })).not.toBeInTheDocument();
  expect(within(publishedRow).queryByRole("button", { name: "提前结束考试" })).not.toBeInTheDocument();
  expect(within(publishedRow).queryByRole("button", { name: "禁用考试" })).not.toBeInTheDocument();
  expect(draftRow).toHaveTextContent("暂无操作");
  expect(publishedRow).toHaveTextContent("暂无操作");
});

test("指定同名学生时按用户 ID 移除已选目标", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn().mockResolvedValue({
      id: 2,
      tenantID: 10,
      paperID: 333,
      name: "同名学生测试",
      paperName: "同名学生测试",
      inviteCode: "PM2028",
      target: "张同学",
      status: "published",
      startAt: "2026-05-30 09:00",
      endAt: "2026-05-30 11:00",
      durationMinutes: 120,
    }),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "同名学生测试",
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
      items: [
        {
          id: 444,
          tenantID: 10,
          name: "联调班级",
          description: "从空间接口加载",
          logoFileName: "未上传",
          status: "enabled",
          members: [],
        },
        {
          id: 445,
          tenantID: 10,
          name: "已禁用班级",
          description: "禁用后不应发布考试",
          logoFileName: "未上传",
          status: "disabled",
          members: [],
        },
      ],
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
        id: 557,
        tenantID: 10,
        name: "张同学",
        username: "student02",
        role: "student",
        avatarFileName: "未上传",
        status: "enabled",
      }],
    }),
  };
  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await waitFor(() => {
    expect(spaceApi.listSpaces.mock.calls.some((call) => call[0]?.filters?.status === "enabled")).toBe(true);
  });
  await user.click(screen.getByRole("button", { name: "新建考试" }));
  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));

  const studentAutocomplete = within(dialog).getByRole("combobox", { name: "指定同学" });
  await user.type(studentAutocomplete, "张");
  let userListbox = await within(dialog).findByRole("listbox", { name: "指定同学列表" });
  await user.click(within(userListbox).getAllByText("张同学（个人）")[0]);
  await user.clear(studentAutocomplete);
  await user.type(studentAutocomplete, "张");
  userListbox = await within(dialog).findByRole("listbox", { name: "指定同学列表" });
  await user.click(within(userListbox).getByText("张同学（个人）"));

  const selectedTargets = within(dialog).getByLabelText("已选指定同学");
  const selectedButtons = within(selectedTargets).getAllByRole("button", { name: /张同学（个人）/ });
  expect(selectedButtons).toHaveLength(2);
  await user.click(selectedButtons[1]);
  await selectExamStatus(user, dialog, "published");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    targets: [
      { targetID: 555, targetType: "user" },
    ],
    targetID: 555,
    targetType: "user",
    status: "published",
  }));
});

test("开放时间窗模式使用 Ant Design 日期范围并保留单次作答时长", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn().mockResolvedValue({
      id: 5,
      tenantID: 10,
      paperID: 333,
      name: "开放时间窗试卷",
      paperName: "开放时间窗试卷",
      inviteCode: "PM501",
      target: "联调班级",
      status: "published",
      startAt: "2026-05-30 09:00",
      endAt: "2026-06-02 18:00",
      durationMinutes: 90,
    }),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "开放时间窗试卷",
        description: "支持一段日期范围内作答",
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
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  await user.click(within(dialog).getByRole("radio", { name: "开放时间窗" }));

  const rangeField = within(dialog).getByRole("group", { name: "考试开放范围" });
  expect(rangeField.querySelector(".ant-picker")).toBeInTheDocument();
  expect(within(dialog).getByLabelText("单次作答时长")).toBeInTheDocument();

  changeRangePickerInputs(rangeField, "2026-05-30", "2026-05-30");
  await selectExamStatus(user, dialog, "published");
  await user.clear(within(dialog).getByLabelText("单次作答时长"));
  await user.type(within(dialog).getByLabelText("单次作答时长"), "2000");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  const alert = await screen.findByRole("alert");
  expect(alert).toHaveTextContent("作答时长不能超过考试时间窗口");
  expect(alert.closest(".ant-message")).toBeInTheDocument();

  changeRangePickerInputs(rangeField, "2026-05-30", "2026-06-02");
  await user.clear(within(dialog).getByLabelText("单次作答时长"));
  await user.type(within(dialog).getByLabelText("单次作答时长"), "90");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    durationMinutes: 90,
    name: "开放时间窗试卷",
    paperID: 333,
    targets: [
      { targetID: 444, targetType: "space" },
    ],
    targetID: 444,
    targetType: "space",
    tenantID: 10,
    status: "published",
  }));
});

test("发布考试表单使用右侧抽屉并在抽屉内提供日期双月范围和指定同学自动完成", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "抽屉发布试卷",
        description: "验证发布抽屉",
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
      }],
    }),
  };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const drawer = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(document.querySelector(".platform-ant-modal")).not.toBeInTheDocument();

  changePickerInput(within(drawer).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(drawer).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(drawer).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");

  await user.click(within(drawer).getByRole("radio", { name: "开放时间窗" }));
  const rangeField = within(drawer).getByRole("group", { name: "考试开放范围" });
  await user.click(rangeField.querySelector("input") as HTMLInputElement);
  expect(within(drawer).getByRole("dialog", { name: "考试开放范围日期面板" })).toBeInTheDocument();
  expect(within(drawer).getByRole("dialog", { name: "考试开放范围日期面板" }).querySelectorAll(".ant-picker-panel")).toHaveLength(2);

  await user.click(within(drawer).getByRole("radio", { name: "指定人群" }));
  const studentAutocomplete = within(drawer).getByRole("combobox", { name: "指定同学" });
  expect(studentAutocomplete).toHaveClass("exam-user-target-input");
  expect(studentAutocomplete.closest(".ant-select-auto-complete")).not.toBeInTheDocument();
  await selectStudentTarget(user, drawer, "张", "张同学（个人）");
  expect(within(within(drawer).getByLabelText("已选指定同学")).getByText("张同学（个人）")).toBeInTheDocument();
});

test("发布考试表单控件高度与普通输入框一致", () => {
  const css = readFileSync(join(process.cwd(), "src/styles/global.css"), "utf8");
  const publishFormRule = css.match(/\.exam-publish-form\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const nativeControlRule = css.match(/\.exam-publish-form \.field > input,[\s\S]*?\.exam-publish-form \.field > select,[\s\S]*?\.exam-publish-form \.exam-ant-picker\.ant-picker\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const antSelectContentRule = css.match(/\.exam-publish-form \.exam-publish-select\.ant-select-single \.ant-select-content\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const pickerInputRule = css.match(/\.exam-publish-form \.exam-ant-picker \.ant-picker-input > input\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const fieldNoteRule = css.match(/\.exam-publish-form \.field-note\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const userTargetInputRule = css.match(/\.exam-publish-form \.exam-user-target-input\s*\{[\s\S]*?\}/)?.[0] ?? "";

  expect(publishFormRule).toContain("align-content: start");
  expect(nativeControlRule).toContain("height: 38px !important");
  expect(nativeControlRule).toContain("min-height: 38px !important");
  expect(antSelectContentRule).toContain("height: 38px !important");
  expect(antSelectContentRule).toContain("line-height: 38px !important");
  expect(antSelectContentRule).not.toContain("border:");
  expect(antSelectContentRule).not.toContain("background:");
  expect(pickerInputRule).toContain("height: 36px !important");
  expect(pickerInputRule).toContain("line-height: 36px !important");
  expect(fieldNoteRule).toContain("font-size: 12px");
  expect(fieldNoteRule).toContain("color: var(--text-muted)");
  expect(userTargetInputRule).toContain("height: 38px !important");
  expect(userTargetInputRule).toContain("line-height: normal !important");
  expect(userTargetInputRule).toContain("box-sizing: border-box");
  expect(css).not.toContain(".exam-user-autocomplete");
});

test("发布考试日期和时间选择弹层挂载到抽屉内而不是表单 label 内", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "弹层挂载测试试卷",
        description: "验证日期时间弹层",
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
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const drawer = screen.getByRole("dialog", { name: "新建考试抽屉" });
  const dateLabel = within(drawer).getByLabelText("考试日期").closest("label");
  expect(dateLabel).not.toBeNull();
  await user.click(within(drawer).getByLabelText("考试日期"));
  const dateDropdown = within(drawer).getByRole("dialog", { name: "考试日期选择面板" }).closest(".ant-picker-dropdown");
  expect(dateDropdown).toBeInTheDocument();
  expect(dateDropdown).toHaveClass("ant-picker-dropdown-placement-bottomLeft");
  expect(dateLabel as HTMLElement).not.toContainElement(dateDropdown as HTMLElement);
  expect(within(drawer).getByText("今天")).toBeInTheDocument();
  expect(dateDropdown).not.toHaveTextContent("Jun");
  expect(dateDropdown).toHaveTextContent("2026");
  expect(dateDropdown).toHaveTextContent("6月");
  expect(dateDropdown).toHaveTextContent("日");

  await user.click(within(drawer).getByLabelText("开始时间"));
  const timeDropdown = within(drawer).getByRole("dialog", { name: "开始时间选择面板" }).closest(".ant-picker-dropdown");
  expect(timeDropdown).toBeInTheDocument();
  expect(timeDropdown).toHaveClass("ant-picker-dropdown-placement-bottomLeft");
});

test("发布考试日期和时间选择组件固定向下展开", () => {
  const source = readFileSync(join(process.cwd(), "src/pages/Exam/ExamManagementPage.tsx"), "utf8");

  expect(source.match(/placement="bottomLeft"/g)).toHaveLength(5);
  expect(source).toContain("publishExamPickerPlacements");
  expect(source.match(/builtinPlacements=\{publishExamPickerPlacements\}/g)).toHaveLength(5);
  expect(source).toContain("adjustY: false");
});

test("发布考试从日期和时间面板选择后保留输入值", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 333,
        tenantID: 10,
        name: "面板选择测试试卷",
        description: "验证面板选择值不会被清空",
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
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);
  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const drawer = screen.getByRole("dialog", { name: "新建考试抽屉" });
  const examDateInput = within(drawer).getByLabelText("考试日期") as HTMLInputElement;
  await user.click(examDateInput);
  await user.click(getVisibleDateCell(drawer, "2026-06-30"));
  expect(examDateInput).toHaveValue("2026-06-30");

  const startTimeInput = within(drawer).getByLabelText("开始时间") as HTMLInputElement;
  await user.click(startTimeInput);
  const startTimePanel = within(drawer).getByRole("dialog", { name: "开始时间选择面板" });
  const [hourColumn, minuteColumn] = Array.from(startTimePanel.querySelectorAll(".ant-picker-time-panel-column"));
  expect(hourColumn).toBeInTheDocument();
  expect(minuteColumn).toBeInTheDocument();
  await user.click(within(hourColumn as HTMLElement).getByText("09"));
  await user.click(within(minuteColumn as HTMLElement).getByText("00"));
  expect(startTimeInput).toHaveValue("09:00");

  await user.click(within(drawer).getByRole("radio", { name: "开放时间窗" }));
  const rangeField = within(drawer).getByRole("group", { name: "考试开放范围" });
  const [rangeStartInput, rangeEndInput] = Array.from(rangeField.querySelectorAll("input"));
  await user.click(rangeStartInput);
  await user.click(getVisibleDateCell(drawer, "2026-06-30"));
  await user.click(getVisibleDateCell(drawer, "2026-07-02"));
  expect(rangeStartInput).toHaveValue("2026-06-30");
  expect(rangeEndInput).toHaveValue("2026-07-02");
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
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
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

  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, search: "" }));
  expect(spaceApi.listSpaces).not.toHaveBeenCalled();
  expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, page: 1, pageSize: 1 });
  expect(userApi.listUsers).not.toHaveBeenCalled();

  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(within(dialog).getByText("当前空间 301")).toBeInTheDocument();
  await user.click(within(dialog).getByRole("combobox", { name: "选择班级范围" }));
  expect(await screen.findByRole("option", { name: "当前空间 301" })).toBeInTheDocument();
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));
  await user.type(within(dialog).getByRole("combobox", { name: "指定同学" }), "王");
  await waitFor(() => expect(spaceApi.listSpaceMembers).toHaveBeenLastCalledWith({
    tenantID: 10,
    spaceID: 301,
    page: 1,
    pageSize: 10,
    search: "王",
    role: "student",
    status: "enabled",
  }));
  expect(within(dialog).getByText("王同学（个人）")).toBeInTheDocument();
  expect(within(dialog).queryByText("禁用学生（个人）")).not.toBeInTheDocument();
  expect(within(dialog).queryByText("空间教师（个人）")).not.toBeInTheDocument();
  await user.click(within(dialog).getByRole("radio", { name: "班级范围" }));
  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");
  await selectExamStatus(user, dialog, "published");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    targets: [
      { targetID: 301, targetType: "space" },
    ],
    targetID: 301,
    targetType: "space",
    status: "published",
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
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
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

  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, search: "" }));
  expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, page: 1, pageSize: 1 });
  expect(await screen.findByRole("button", { name: "新建考试" })).toBeInTheDocument();
  expect(screen.queryByText("当前身份无法读取空间成员列表，仅支持向当前空间发布考试")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(within(dialog).getByText("当前身份无法读取空间成员列表，仅支持向当前空间发布考试")).toBeInTheDocument();
  expect(within(dialog).getByText("当前空间 301")).toBeInTheDocument();
  await user.click(within(dialog).getByRole("combobox", { name: "选择班级范围" }));
  expect(await screen.findByRole("option", { name: "当前空间 301" })).toBeInTheDocument();
  expect(screen.getAllByRole("option", { name: "当前空间 301" })).toHaveLength(1);

  changePickerInput(within(dialog).getByLabelText("考试日期").closest("label") as HTMLElement, "2026-05-30");
  changePickerInput(within(dialog).getByLabelText("开始时间").closest("label") as HTMLElement, "09:00");
  changePickerInput(within(dialog).getByLabelText("结束时间").closest("label") as HTMLElement, "11:00");
  await selectExamStatus(user, dialog, "published");
  await user.click(within(dialog).getByRole("button", { name: "确认创建" }));

  expect(api.publishExam).toHaveBeenCalledWith(expect.objectContaining({
    targets: [{ targetID: 301, targetType: "space" }],
    targetID: 301,
    targetType: "space",
    status: "published",
  }));
});

test("空间教师读取成员服务异常时不会静默回退成整空间发布", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
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

  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, search: "" }));
  expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, page: 1, pageSize: 1 });
  expect(await screen.findByRole("alert")).toHaveTextContent("发布选项加载失败");
  expect(screen.queryByText("当前身份无法读取空间成员列表，仅支持向当前空间发布考试")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(within(dialog).queryByText("当前空间 301")).not.toBeInTheDocument();
  await user.click(within(dialog).getByRole("combobox", { name: "选择班级范围" }));
  expect(within(dialog).queryByRole("option", { name: "当前空间 301" })).not.toBeInTheDocument();
});

test("发布选项重载失败时会清空上一次成功加载的试卷和范围", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn()
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
    listSpaceMembers: vi.fn().mockImplementation(async (input) => {
      if (input.spaceID === 302) {
        throw new ApiError("server exploded", 50000, 500, { message: "server exploded" });
      }
      if (input.search === "王") {
        return {
          items: [{
            id: 901,
            userID: 777,
            name: "王同学",
            role: "student",
            status: "enabled",
          }],
        };
      }
      return { items: [] };
    }),
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

  await waitFor(() => expect(spaceApi.listSpaceMembers).toHaveBeenNthCalledWith(1, { tenantID: 10, spaceID: 301, page: 1, pageSize: 1 }));

  await user.click(screen.getByRole("button", { name: "新建考试" }));
  let dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  expect(within(dialog).getByText("空间 301 试卷")).toBeInTheDocument();
  await user.click(within(dialog).getByRole("radio", { name: "指定人群" }));
  await user.type(within(dialog).getByRole("combobox", { name: "指定同学" }), "王");
  expect(within(dialog).getByText("王同学（个人）")).toBeInTheDocument();

  view.rerender(
    withExamManagementProviders(
      <ExamManagementPage
        api={api}
        canManageTenantTargets={false}
        paperApi={paperApi}
        spaceApi={spaceApi}
        userApi={userApi}
        tenantID={10}
        spaceID={302}
      />,
    ),
  );

  await waitFor(() => expect(spaceApi.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 302, page: 1, pageSize: 1 }));
  const alerts = await screen.findAllByRole("alert");
  expect(alerts.some((alert) => alert.textContent?.includes("发布选项加载失败"))).toBe(true);

  dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  await user.click(within(dialog).getByRole("combobox", { name: "选择班级范围" }));
  expect(within(dialog).queryByRole("option", { name: "当前空间 301" })).not.toBeInTheDocument();
  expect(within(dialog).queryByText("空间 301 试卷")).not.toBeInTheDocument();
  expect(within(dialog).queryByText("王同学（个人）")).not.toBeInTheDocument();
});

test("发布考试使用后端返回的 enabled 试卷候选并重置默认选择", async () => {
  const user = userEvent.setup();
  const api = {
    listExams: vi.fn().mockResolvedValue({ items: [] }),
    publishExam: vi.fn(),
  };
  const paperApi = {
    listPublishPaperCandidates: vi.fn().mockResolvedValue({
      items: [{
        id: 200,
        tenantID: 10,
        name: "可发布试卷",
        description: "应作为默认选项",
        buildMode: "manual",
        status: "enabled",
        totalScore: "80",
        createdAt: new Date("2026-06-02T09:10:00+08:00").getTime(),
        creatorName: "teacher.enabled",
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
  const userApi = { listUsers: vi.fn().mockResolvedValue({ items: [] }) };

  renderExamManagementPage(<ExamManagementPage api={api} paperApi={paperApi} spaceApi={spaceApi} userApi={userApi} tenantID={10} />);

  await waitFor(() => expect(paperApi.listPublishPaperCandidates).toHaveBeenCalledWith({ tenantID: 10, search: "" }));
  await user.click(screen.getByRole("button", { name: "新建考试" }));

  const dialog = screen.getByRole("dialog", { name: "新建考试抽屉" });
  const paperSelect = within(dialog).getByRole("combobox", { name: "发布试卷" });

  expect(within(dialog).queryByText("已禁用试卷")).not.toBeInTheDocument();
  expect(within(dialog).queryByText("草稿试卷")).not.toBeInTheDocument();
  expect(within(dialog).getByText("可发布试卷")).toBeInTheDocument();
  await user.click(paperSelect);
  expect(await screen.findByRole("option", { name: "可发布试卷" })).toBeInTheDocument();
  await user.keyboard("{Escape}");
  await user.click(within(dialog).getByRole("combobox", { name: "选择班级范围" }));
  expect(await screen.findByRole("option", { name: "联调班级" })).toBeInTheDocument();
  expect(within(dialog).queryByRole("option", { name: "已禁用班级" })).not.toBeInTheDocument();
});
