import { readFileSync } from "node:fs";
import { join } from "node:path";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import type { ReactElement } from "react";
import type { SpaceMember, SpaceRow } from "../../api/spaces";
import { FeedbackProvider } from "../../app/feedback";
import { SpaceManagementPage } from "./SpaceManagementPage";

function renderWithFeedback(page: ReactElement) {
  return render(<FeedbackProvider>{page}</FeedbackProvider>);
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

function createSpaceAPI() {
  const spaces: SpaceRow[] = [
    {
      id: 100,
      tenantID: 10,
      name: "高一 1 班",
      description: "高一语文月考主空间",
      logoFileName: "class1.png",
      status: "enabled",
      members: [
        {
          id: 1,
          userID: 1,
          name: "李老师",
          username: "teacher_li",
          phone: "13800000020",
          email: "li@example.test",
          registeredAt: 1710000000000,
          registerMethod: "tenant_register",
          role: "space_admin",
          status: "enabled",
        },
        {
          id: 2,
          userID: 2,
          name: "张同学",
          username: "student_zhang",
          phone: "13800000021",
          email: "zhang@example.test",
          registeredAt: 1710000000000,
          registerMethod: "tenant_register",
          role: "student",
          status: "enabled",
        },
        { id: 5, userID: 5, name: "王同学", role: "student", status: "enabled" },
        { id: 6, userID: 6, name: "刘老师", role: "teacher", status: "enabled" },
        { id: 7, userID: 7, name: "孙同学", role: "student", status: "enabled" },
        { id: 8, userID: 8, name: "钱同学", role: "student", status: "enabled" },
        { id: 9, userID: 9, name: "郑同学", role: "student", status: "enabled" },
        { id: 10, userID: 10, name: "周同学", role: "student", status: "enabled" },
      ],
    },
    {
      id: 101,
      tenantID: 10,
      name: "高一 2 班",
      description: "阶段测评与补测空间",
      logoFileName: "class2.png",
      status: "enabled",
      members: [{ id: 3, userID: 3, name: "周老师", role: "space_admin", status: "enabled" }],
    },
    {
      id: 103,
      tenantID: 10,
      name: "高一 3 班",
      description: "分层教学与题库协作空间",
      logoFileName: "class3.png",
      status: "enabled",
      members: [{ id: 4, userID: 4, name: "赵老师", role: "space_admin", status: "enabled" }],
    },
    {
      id: 104,
      tenantID: 10,
      name: "高一 4 班",
      description: "英语听说训练空间",
      logoFileName: "class4.png",
      status: "enabled",
      members: [{ id: 11, userID: 11, name: "吴老师", role: "space_admin", status: "enabled" }],
    },
    {
      id: 105,
      tenantID: 10,
      name: "高一 5 班",
      description: "数学周测与错题整理空间",
      logoFileName: "class5.png",
      status: "enabled",
      members: [{ id: 12, userID: 12, name: "郑老师", role: "space_admin", status: "enabled" }],
    },
    {
      id: 106,
      tenantID: 10,
      name: "高一 6 班",
      description: "物理实验与随堂测评空间",
      logoFileName: "class6.png",
      status: "enabled",
      members: [{ id: 13, userID: 13, name: "钱老师", role: "space_admin", status: "enabled" }],
    },
    {
      id: 107,
      tenantID: 10,
      name: "高一 7 班",
      description: "化学晚自习巩固空间",
      logoFileName: "class7.png",
      status: "enabled",
      members: [{ id: 14, userID: 14, name: "孙老师", role: "space_admin", status: "enabled" }],
    },
  ];

  const members: SpaceMember[] = [
    {
      id: 1,
      userID: 1,
      name: "李老师",
      username: "teacher_li",
      phone: "13800000020",
      email: "li@example.test",
      registeredAt: 1710000000000,
      registerMethod: "tenant_register",
      role: "space_admin" as const,
      status: "enabled" as const,
    },
    {
      id: 2,
      userID: 2,
      name: "张同学",
      username: "student_zhang",
      phone: "13800000021",
      email: "zhang@example.test",
      registeredAt: 1710000000000,
      registerMethod: "tenant_register",
      role: "student" as const,
      status: "enabled" as const,
    },
    { id: 5, userID: 5, name: "王同学", role: "student" as const, status: "enabled" as const },
    { id: 6, userID: 6, name: "刘老师", role: "teacher" as const, status: "enabled" as const },
    { id: 7, userID: 7, name: "孙同学", role: "student" as const, status: "enabled" as const },
    { id: 8, userID: 8, name: "钱同学", role: "student" as const, status: "enabled" as const },
    { id: 9, userID: 9, name: "郑同学", role: "student" as const, status: "enabled" as const },
    { id: 10, userID: 10, name: "周同学", role: "student" as const, status: "enabled" as const },
  ];

  return {
    listSpaces: vi.fn().mockImplementation(async (input) => {
      const keyword = input.search?.trim().toLowerCase() ?? "";
      const matchedSpaces = keyword
        ? spaces.filter((space) => [space.name, space.description, space.logoFileName, space.members.map((member) => member.name).join(" ")].some((value) => value.toLowerCase().includes(keyword)))
        : spaces;
      const filteredSpaces = input.filters?.status === undefined
        ? matchedSpaces
        : matchedSpaces.filter((space) => space.status === input.filters?.status);
      return {
        items: filteredSpaces.slice(((input.page ?? 1) - 1) * (input.pageSize ?? 20), (input.page ?? 1) * (input.pageSize ?? 20)),
        total: filteredSpaces.length,
      };
    }),
    createSpace: vi.fn().mockImplementation(async () => {
      const nextSpace = {
        id: 108,
        tenantID: 10,
        name: "高一 8 班",
        description: "面向月考与补测的学生空间",
        logoFileName: "class8.png",
        status: "enabled" as const,
        members: [{ id: 15, userID: 15, name: "赵老师", role: "space_admin" as const, status: "enabled" as const }],
      };
      spaces.unshift(nextSpace);
      return nextSpace;
    }),
    updateSpace: vi.fn(async (input: { spaceID: number; name: string; description: string; logoFileName: string }) => ({
      id: input.spaceID,
      tenantID: 10,
      name: input.name,
      description: input.description,
      logoFileName: input.logoFileName,
      status: "enabled" as const,
      members: [],
    })),
    disableSpace: vi.fn(async (input: { spaceID: number }) => ({
      id: input.spaceID,
      tenantID: 10,
      name: "高一 1 班",
      description: "高一语文月考主空间",
      logoFileName: "class1.png",
      status: "disabled" as const,
      members: [
        { id: 1, userID: 1, name: "李老师", role: "space_admin" as const, status: "enabled" as const },
        { id: 2, userID: 2, name: "张同学", role: "student" as const, status: "enabled" as const },
      ],
    })),
    listSpaceMembers: vi.fn().mockImplementation(async (input: { spaceID: number; page?: number; pageSize?: number; search?: string }) => {
      const page = input.page ?? 1;
      const pageSize = input.pageSize ?? 20;
      const keyword = input.search?.trim().toLowerCase() ?? "";
      const sourceMembers = input.spaceID === 100
        ? members
        : spaces.find((space) => space.id === input.spaceID)?.members ?? [];
      const matchedMembers = keyword
        ? sourceMembers.filter((member) => [member.name, member.role, member.status].some((value) => value.toLowerCase().includes(keyword)))
        : sourceMembers;
      return {
        items: matchedMembers.slice((page - 1) * pageSize, page * pageSize),
        page,
        pageSize,
        total: matchedMembers.length,
      };
    }),
    createSpaceMember: vi.fn(async (input: { userID: number; role: "space_admin" | "teacher" | "student" }) => {
      const nextMember = {
        id: input.userID,
        userID: input.userID,
        name: input.userID === 22 ? "赵老师" : "陈同学",
        username: input.userID === 22 ? "teacher02" : "student03",
        role: input.role,
        status: "enabled" as const,
      };
      members.push(nextMember);
      return nextMember;
    }),
    updateSpaceMember: vi.fn(async (input: { userID: number; role?: "space_admin" | "teacher" | "student"; status?: "enabled" | "disabled" }) => ({
      id: input.userID,
      userID: input.userID,
      name: input.userID === 2 ? "张同学" : input.userID === 21 ? "李老师" : input.userID === 22 ? "赵老师" : "成员",
      role: input.role ?? "student",
      status: input.status ?? "enabled",
    })),
    removeSpaceMember: vi.fn(),
  };
}

function createUserAPI() {
  const suggestionUsers = Array.from({ length: 12 }, (_, index) => ({
    id: 30 + index,
    tenantID: 10,
    name: `测试同学 ${index + 1}`,
    username: `student_suggest_${index + 1}`,
    role: "student",
    avatarFileName: "未上传",
    status: "enabled",
  }));
  const users = [
    {
      id: 21,
      tenantID: 10,
      name: "李老师",
      username: "teacher01",
      role: "teacher",
      avatarFileName: "未上传",
      status: "enabled",
    },
    {
      id: 22,
      tenantID: 10,
      name: "赵老师",
      username: "teacher02",
      role: "teacher",
      avatarFileName: "未上传",
      status: "enabled",
    },
    {
      id: 23,
      tenantID: 10,
      name: "陈同学",
      username: "student03",
      role: "student",
      avatarFileName: "未上传",
      status: "enabled",
    },
    ...suggestionUsers,
  ];

  return {
    listUsers: vi.fn().mockImplementation(async (input) => {
      const keyword = input.search?.trim().toLowerCase() ?? "";
      const matchedUsers = keyword
        ? users.filter((user) => [user.username, user.name].some((value) => value.toLowerCase().includes(keyword)))
        : users;
      return {
        items: matchedUsers.slice(((input.page ?? 1) - 1) * (input.pageSize ?? 20), (input.page ?? 1) * (input.pageSize ?? 20)),
        total: matchedUsers.length,
      };
    }),
  };
}

test("空间管理页展示空间列表和空间管理员", async () => {
  const api = createSpaceAPI();
  const userApi = createUserAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={userApi} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "空间管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "空间管理" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建空间" })).toHaveClass("tenant-create-button");
  expect(screen.queryByRole("button", { name: "保存空间配置" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("搜索空间")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新空间列表" })).toBeInTheDocument();
  expect(await screen.findByText("高一 1 班")).toBeInTheDocument();
  expect(screen.getByText("空间管理员：李老师")).toBeInTheDocument();
  expect(api.listSpaces).toHaveBeenCalledWith(expect.objectContaining({ tenantID: 10, page: 1, pageSize: 5 }));
  expect(userApi.listUsers).toHaveBeenCalledWith(expect.objectContaining({ tenantID: 10, page: 1, pageSize: 100 }));
  expect(screen.queryByLabelText("成员姓名")).not.toBeInTheDocument();
});

test("缺少租户上下文时不请求空间和用户接口", async () => {
  const api = createSpaceAPI();
  const userApi = createUserAPI();

  renderWithFeedback(<SpaceManagementPage api={api} userApi={userApi} />);

  expect(api.listSpaces).not.toHaveBeenCalled();
  expect(userApi.listUsers).not.toHaveBeenCalled();
  expect(await screen.findByRole("alert")).toHaveTextContent("当前账号没有租户上下文");
});

test("租户管理员可以创建空间并上传 logo", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");


  await user.click(screen.getByRole("button", { name: "创建空间" }));

  const drawer = screen.getByRole("dialog", { name: "创建空间抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(drawer).toHaveClass("tenant-resource-drawer--half");
  expect(screen.queryByRole("dialog", { name: "创建空间弹窗" })).not.toBeInTheDocument();
  expect(within(drawer).getByText("填写空间资料")).toBeInTheDocument();
  expect(within(drawer).getAllByText("*")).toHaveLength(3);
  expect(screen.getByLabelText("空间管理员")).toHaveValue("teacher01");
  expect(screen.getByLabelText("已选空间管理员真实姓名")).toHaveTextContent("李老师");
  await user.type(screen.getByLabelText("空间名称"), "高一 8 班");
  await user.type(screen.getByLabelText("空间描述"), "面向月考与补测的学生空间");
  await user.clear(screen.getByLabelText("空间管理员"));
  await user.type(screen.getByLabelText("空间管理员"), "teacher02");
  const suggestions = await screen.findByRole("listbox", { name: "空间管理员候选" });
  await user.click(await within(suggestions).findByRole("option", { name: /teacher02/ }));
  expect(screen.getByLabelText("空间管理员")).toHaveValue("teacher02");
  expect(screen.getByLabelText("已选空间管理员真实姓名")).toHaveTextContent("赵老师");
  await user.upload(screen.getByLabelText("空间 Logo"), new File(["logo"], "class8.png", { type: "image/png" }));
  await user.click(screen.getByRole("button", { name: "确认创建" }));

  expect(api.createSpace).toHaveBeenCalledWith({
    tenantID: 10,
    name: "高一 8 班",
    description: "面向月考与补测的学生空间",
    logoFileName: "class8.png",
    adminUserID: 22,
  });
  await waitFor(() => expect(screen.queryByRole("dialog", { name: "创建空间抽屉" })).not.toBeInTheDocument());
  const createdSpaceRow = await screen.findByRole("row", { name: /高一 8 班/ });
  expect(within(createdSpaceRow).getByText("空间管理员：赵老师")).toBeInTheDocument();
  expect(within(createdSpaceRow).getByText("class8.png")).toBeInTheDocument();
});

test("创建和编辑空间的管理员选择器高度与普通输入框一致", () => {
  const css = readFileSync(join(process.cwd(), "src/styles/global.css"), "utf8");
  const autocompleteRootRule = css.match(/\.space-profile-drawer-form \.space-admin-autocomplete\.ant-select\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const drawerInputRule = css.match(/\.space-profile-drawer-form \.field > input,[\s\S]*?\.space-admin-autocomplete \.ant-select-selector\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const selectorWrapRule = css.match(/\.space-profile-drawer-form \.space-admin-autocomplete \.ant-select-selection-wrap,[\s\S]*?\.space-admin-autocomplete \.ant-select-selection-search\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const selectorSearchRule = css.match(/\.space-profile-drawer-form \.space-admin-autocomplete \.ant-select-selection-search-input\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const selectorTextRule = css.match(/\.space-profile-drawer-form \.space-admin-autocomplete \.ant-select-selection-placeholder,[\s\S]*?\.space-admin-autocomplete \.ant-select-selection-item\s*\{[\s\S]*?\}/)?.[0] ?? "";

  expect(autocompleteRootRule).toContain("height: 38px");
  expect(autocompleteRootRule).toContain("min-height: 38px");
  expect(drawerInputRule).toContain("height: 38px !important");
  expect(drawerInputRule).toContain("min-height: 38px !important");
  expect(drawerInputRule).toContain("padding: 0 10px !important");
  expect(selectorWrapRule).toContain("height: 36px !important");
  expect(selectorWrapRule).toContain("min-height: 36px !important");
  expect(selectorSearchRule).toContain("height: 36px !important");
  expect(selectorSearchRule).toContain("line-height: 36px !important");
  expect(selectorTextRule).toContain("line-height: 36px !important");
});

test("租户管理员禁用空间前需要确认风险并联动关闭操作入口", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  const row = screen.getByRole("row", { name: /高一 1 班/ });

  await user.click(within(row).getByRole("button", { name: "禁用空间" }));

  expect(api.disableSpace).not.toHaveBeenCalled();
  const confirmDialog = await screen.findByRole("dialog", { name: "禁用空间风险确认" });
  expect(within(confirmDialog).getByText("禁用后该空间下的题库、试卷、考试、阅卷和成绩等业务都将停止操作。")).toBeInTheDocument();
  await user.click(within(confirmDialog).getByRole("button", { name: "确认禁用" }));

  expect(api.disableSpace).toHaveBeenCalledWith({ tenantID: 10, spaceID: 100 });
  await waitFor(() => {
    expect(screen.queryByRole("dialog", { name: "禁用空间风险确认" })).not.toBeInTheDocument();
  });
  const disabledRow = screen.getByRole("row", { name: /高一 1 班/ });
  expect(within(disabledRow).getByText("已禁用")).toBeInTheDocument();
  expect(within(disabledRow).getByRole("button", { name: "编辑空间" })).toBeDisabled();
  expect(within(disabledRow).getByRole("button", { name: "成员管理" })).toBeDisabled();
  expect(within(disabledRow).getByRole("button", { name: "禁用空间" })).toBeDisabled();
});

test("租户管理员可以按账号或姓名将已有用户添加到空间", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  await user.click(screen.getByRole("button", { name: "添加用户" }));

  expect(screen.getByRole("dialog", { name: "添加用户到空间弹窗" })).toBeInTheDocument();
  const dialog = screen.getByRole("dialog", { name: "添加用户到空间弹窗" });
  expect(within(dialog).getAllByText("*")).toHaveLength(3);
  expect(screen.getByLabelText("目标空间")).toHaveValue("");
  expect(screen.getByText("请选择空间")).toBeInTheDocument();
  expect(screen.getByLabelText("空间身份")).toHaveValue("");
  expect(screen.getByText("请选择身份")).toBeInTheDocument();
  await user.type(screen.getByLabelText("用户账号或姓名"), "student03");
  await user.selectOptions(screen.getByLabelText("目标空间"), "100");
  await user.selectOptions(screen.getByLabelText("空间身份"), "student");
  await user.click(screen.getByRole("button", { name: "确认添加" }));

  expect(api.createSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 100, userID: 23, role: "student" });
  expect(screen.queryByRole("dialog", { name: "添加用户到空间弹窗" })).not.toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));
  const drawer = await screen.findByRole("dialog", { name: "高一 1 班成员抽屉" });
  await user.type(within(drawer).getByLabelText("成员检索关键词"), "陈同学");
  await user.click(within(drawer).getByRole("button", { name: "搜索成员" }));
  expect(await within(drawer).findByText("陈同学")).toBeInTheDocument();
});

test("添加用户弹窗的目标空间使用后端启用空间候选，不受当前页限制", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  await user.click(screen.getByRole("button", { name: "添加用户" }));

  const dialog = await screen.findByRole("dialog", { name: "添加用户到空间弹窗" });
  await waitFor(() => {
    expect(api.listSpaces).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      page: 1,
      pageSize: 10,
      search: "",
      filters: { status: "enabled" },
    }));
  });
  expect(within(dialog).getByRole("option", { name: "高一 6 班" })).toBeInTheDocument();
  expect(within(dialog).getByRole("option", { name: "高一 7 班" })).toBeInTheDocument();
});

test("添加用户到空间时按账号或姓名展示最多十个候选用户", async () => {
  const user = userEvent.setup();
  renderWithFeedback(<SpaceManagementPage api={createSpaceAPI()} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  await user.click(screen.getByRole("button", { name: "添加用户" }));
  await user.click(screen.getByLabelText("用户账号或姓名"));

  expect(screen.queryByRole("listbox", { name: "用户账号或姓名候选" })).not.toBeInTheDocument();

  await user.type(screen.getByLabelText("用户账号或姓名"), "student");
  expect(screen.queryByRole("listbox", { name: "用户账号或姓名候选" })).not.toBeInTheDocument();

  const suggestions = await screen.findByRole("listbox", { name: "用户账号或姓名候选" });
  expect(within(suggestions).getAllByRole("option")).toHaveLength(10);
  expect(within(suggestions).getByRole("option", { name: /student03/ })).toBeInTheDocument();
  expect(within(suggestions).queryByRole("option", { name: /student_suggest_10/ })).not.toBeInTheDocument();

  await user.click(within(suggestions).getByRole("option", { name: /student03/ }));

  expect(screen.getByLabelText("用户账号或姓名")).toHaveValue("student03");
  expect(screen.getByLabelText("已选用户真实姓名")).toHaveTextContent("陈同学");
});

test("空间管理列表支持分页并在检索后重置到第一页", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  const pagination = screen.getByRole("navigation", { name: "分页" });
  expect(pagination).toHaveClass("pagination--right");
  expect(pagination.querySelector(".ant-pagination")).toBeInTheDocument();
  expect(within(pagination).getByRole("combobox", { name: "每页条数" })).toBeInTheDocument();
  expect(screen.getByText("共 7 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 2 页")).toBeInTheDocument();
  expect(screen.getByText("高一 5 班")).toBeInTheDocument();
  expect(screen.queryByText("高一 6 班")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "下一页" }));

  await waitFor(() => expect(screen.getByText("第 2 / 2 页")).toBeInTheDocument());
  expect(await screen.findByText("高一 6 班")).toBeInTheDocument();
  expect(screen.getByText("高一 7 班")).toBeInTheDocument();
  expect(screen.queryByText("高一 1 班")).not.toBeInTheDocument();
  expect(api.listSpaces).toHaveBeenLastCalledWith(expect.objectContaining({ tenantID: 10, page: 2, pageSize: 5 }));

  await user.type(screen.getByLabelText("搜索空间"), "高一 2 班");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  await waitFor(() => expect(api.listSpaces).toHaveBeenLastCalledWith(expect.objectContaining({
    tenantID: 10,
    page: 1,
    pageSize: 5,
    search: "高一 2 班",
  })));
  expect(screen.getByText("共 1 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 1 页")).toBeInTheDocument();
  expect(screen.getByText("高一 2 班")).toBeInTheDocument();
  expect(screen.queryByText("高一 6 班")).not.toBeInTheDocument();
});

test("空间搜索覆盖服务端分页后的全部空间", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.type(screen.getByLabelText("搜索空间"), "高一 6 班");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(await screen.findByText("高一 6 班")).toBeInTheDocument();
  expect(screen.queryByText("高一 1 班")).not.toBeInTheDocument();
});

test("空间管理列表支持切换每页条数并将选择器放在分页前", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  const pagination = screen.getByRole("navigation", { name: "分页" });
  const pageSize = within(pagination).getByRole("combobox", { name: "每页条数" });
  const antPagination = pagination.querySelector(".ant-pagination");
  expect(antPagination).not.toBeNull();
  expect(pageSize.compareDocumentPosition(antPagination as Element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  await user.click(pageSize);
  await user.click(screen.getByRole("option", { name: "10 条 / 页" }));

  await waitFor(() => expect(api.listSpaces).toHaveBeenLastCalledWith(expect.objectContaining({ tenantID: 10, page: 1, pageSize: 10 })));
  expect(screen.getByText("第 1 / 1 页")).toBeInTheDocument();
  expect(screen.getByText("高一 7 班")).toBeInTheDocument();
});

test("租户管理员可以搜索和刷新空间列表", async () => {
  const user = userEvent.setup();
  renderWithFeedback(<SpaceManagementPage api={createSpaceAPI()} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.type(screen.getByLabelText("搜索空间"), "高一 2 班");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("高一 1 班")).not.toBeInTheDocument();
  expect(screen.getByText("高一 2 班")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新空间列表" }));

  expect(await screen.findByText("高一 1 班")).toBeInTheDocument();
  expect(screen.getByText("高一 2 班")).toBeInTheDocument();
});

test("刷新空间列表会重新读取后端并播放过渡动画", async () => {
  const user = userEvent.setup();
  const refreshResult = deferred<Awaited<ReturnType<ReturnType<typeof createSpaceAPI>["listSpaces"]>>>();
  const api = createSpaceAPI();
  vi.mocked(api.listSpaces)
    .mockResolvedValueOnce({
      items: [{
        id: 100,
        tenantID: 10,
        name: "高一 1 班",
        description: "高一语文月考主空间",
        logoFileName: "class1.png",
        members: [{ id: 1, name: "李老师", role: "space_admin", status: "enabled" }],
      }],
    })
    .mockReturnValueOnce(refreshResult.promise);

  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(screen.getByRole("button", { name: "刷新空间列表" }));

  expect(screen.getByTestId("space-list-table-wrap")).toHaveClass("tenant-list-transition--refreshing");
  expect(screen.getByRole("button", { name: "刷新空间列表" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );
  expect(api.listSpaces).toHaveBeenCalledTimes(2);

  refreshResult.resolve({
    items: [{
      id: 103,
      tenantID: 10,
      name: "高一 3 班",
      description: "后端刷新后的空间",
      logoFileName: "class3.png",
      members: [{ id: 4, userID: 4, name: "赵老师", role: "space_admin", status: "enabled" }],
    }],
  });

  expect(await screen.findByText("高一 3 班")).toBeInTheDocument();
  expect(screen.queryByText("高一 1 班")).not.toBeInTheDocument();
  await waitFor(() =>
    expect(screen.getByTestId("space-list-table-wrap")).not.toHaveClass("tenant-list-transition--refreshing"),
  );
});

test("租户管理员可以编辑空间信息和管理员并通过抽屉管理成员", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "编辑空间" }));
  const editorDrawer = await screen.findByRole("dialog", { name: "编辑空间抽屉" });
  expect(editorDrawer.closest(".ant-drawer")).toBeInTheDocument();
  await user.clear(screen.getByLabelText("编辑空间名称"));
  await user.type(screen.getByLabelText("编辑空间名称"), "高一实验班");

  const descriptionInput = screen.getByLabelText("编辑空间描述");
  const descriptionEditor = descriptionInput.closest(".markdown-editor");
  expect(descriptionEditor).toBeInTheDocument();
  expect(within(descriptionEditor as HTMLElement).getByTitle("加粗（Ctrl+B）")).toBeInTheDocument();
  expect(within(descriptionEditor as HTMLElement).getByTitle("标题")).toBeInTheDocument();
  expect(within(descriptionEditor as HTMLElement).getByTitle("实时预览（Ctrl+8）")).toBeInTheDocument();
  await user.clear(descriptionInput);
  await user.type(descriptionInput, "**高一语文月考主空间**");

  const adminInput = screen.getByLabelText("编辑空间管理员");
  expect(adminInput.closest(".ant-select")).toBeInTheDocument();
  await user.clear(adminInput);
  await user.type(adminInput, "teacher02");
  const suggestions = await screen.findByRole("listbox", { name: "编辑空间管理员候选" });
  const teacherOption = await within(suggestions).findByRole("option", { name: /赵老师/ });
  expect(teacherOption).toHaveTextContent("teacher02 · 教师");
  await user.click(teacherOption);

  await user.upload(screen.getByLabelText("编辑空间 Logo"), new File(["logo"], "class-next.png", { type: "image/png" }));
  expect(screen.getByLabelText("编辑空间 Logo上传区域")).toHaveTextContent("class-next.png");
  expect(screen.getByLabelText("编辑空间 Logo预览")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "保存空间信息" }));

  expect(api.updateSpace).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 100,
    name: "高一实验班",
    description: "**高一语文月考主空间**",
    logoFileName: "class-next.png",
  });
  expect(api.createSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 100, userID: 22, role: "space_admin" });
  expect(api.updateSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 100, userID: 1, role: "teacher" });
  await waitFor(() => expect(screen.queryByRole("dialog", { name: "编辑空间抽屉" })).not.toBeInTheDocument());
  const updatedSpaceRow = screen.getByRole("row", { name: /高一实验班/ });
  expect(within(updatedSpaceRow).getByText("**高一语文月考主空间**")).toBeInTheDocument();
  expect(within(updatedSpaceRow).getByText("空间管理员：赵老师")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /高一实验班/ })).getByRole("button", { name: "成员管理" }));
  const drawer = await screen.findByRole("dialog", { name: "高一实验班成员抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(document.querySelector(".ant-drawer-mask")).toBeInTheDocument();
  expect(drawer).toHaveClass("tenant-resource-drawer--half");
  await waitFor(() => expect(drawer).toHaveClass("tenant-resource-drawer--open"));
  const drawerHeader = within(drawer).getByTestId("space-member-drawer-header");
  expect(within(drawerHeader).getByRole("button", { name: "返回空间列表" })).toHaveTextContent("返回");
  expect(within(drawerHeader).getByText("|")).toBeInTheDocument();
  expect(within(drawerHeader).getByText("高一实验班")).toBeInTheDocument();
  expect(within(drawerHeader).queryByText("高一实验班成员")).not.toBeInTheDocument();
  expect(within(drawer).getByRole("button", { name: "全屏抽屉" })).toBeInTheDocument();
  expect(within(drawer).queryByRole("button", { name: "关闭抽屉" })).not.toBeInTheDocument();
  const drawerBody = within(drawer).getByTestId("space-member-drawer-body");
  expect(drawerBody).toHaveClass("tenant-resource-drawer__body");
  const drawerToolbar = within(drawer).getByTestId("space-member-toolbar");
  expect(drawerToolbar).toHaveClass("tenant-resource-drawer__toolbar--member-search");
  expect(drawerToolbar.lastElementChild).toHaveClass("tenant-search-actions");
  expect(within(drawer).getByLabelText("成员检索关键词")).toBeInTheDocument();
  expect(within(drawer).getByRole("button", { name: "搜索成员" })).toBeInTheDocument();
  expect(within(drawer).getByRole("button", { name: "刷新成员列表" })).toBeInTheDocument();
  expect(within(drawer).getByRole("button", { name: "添加成员" })).toBeInTheDocument();
  expect(within(drawer).queryByLabelText("成员姓名")).not.toBeInTheDocument();
  expect(within(drawer).queryByRole("columnheader", { name: "操作" })).not.toBeInTheDocument();
  expect(within(drawer).queryByRole("button", { name: "查看详情" })).not.toBeInTheDocument();
  expect(within(drawer).queryByRole("button", { name: "禁用" })).not.toBeInTheDocument();
  expect(within(drawer).queryByRole("button", { name: "启用" })).not.toBeInTheDocument();
  expect(within(drawer).queryByRole("button", { name: "降级为教师" })).not.toBeInTheDocument();

  await user.click(within(drawer).getByRole("button", { name: "全屏抽屉" }));
  expect(drawer).toHaveClass("tenant-resource-drawer--fullscreen");
  await user.click(within(drawer).getByRole("button", { name: "退出全屏抽屉" }));
  expect(drawer).toHaveClass("tenant-resource-drawer--half");


  await user.click(within(drawer).getByRole("button", { name: "返回空间列表" }));
  await waitFor(() => {
    expect(screen.queryByRole("dialog", { name: "高一 1 班成员抽屉" })).not.toBeInTheDocument();
  });
});

test("成员管理抽屉可以打开添加成员弹窗并预选当前空间", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();
  const userApi = createUserAPI();
  renderWithFeedback(<SpaceManagementPage api={api} userApi={userApi} tenantID={10} />);

  await screen.findByText("高一 1 班");
  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));
  const drawer = await screen.findByRole("dialog", { name: "高一 1 班成员抽屉" });
  const initialMemberFetchCount = api.listSpaceMembers.mock.calls.length;
  await user.click(within(drawer).getByRole("button", { name: "添加成员" }));

  const dialog = await screen.findByRole("dialog", { name: "添加用户到空间弹窗" });
  expect(within(dialog).getByLabelText("目标空间")).toHaveValue("100");
  const userInput = within(dialog).getByLabelText("用户账号或姓名");
  expect(userInput).toBeInTheDocument();
  expect(within(dialog).getByLabelText("空间身份")).toBeInTheDocument();

  await user.type(userInput, "student03");
  const suggestions = await screen.findByRole("listbox", { name: "用户账号或姓名候选" });
  expect(userApi.listUsers).toHaveBeenLastCalledWith(expect.objectContaining({
    tenantID: 10,
    page: 1,
    pageSize: 10,
    search: "student03",
    filters: { status: "enabled", excludeSpaceID: 100 },
  }));
  await user.click(within(suggestions).getByRole("option", { name: /陈同学/ }));
  await user.selectOptions(within(dialog).getByLabelText("空间身份"), "student");
  await user.click(within(dialog).getByRole("button", { name: "确认添加" }));

  expect(api.createSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 100, userID: 23, role: "student" });
  await waitFor(() => expect(api.listSpaceMembers).toHaveBeenCalledTimes(initialMemberFetchCount + 1));
  expect(api.listSpaceMembers).toHaveBeenLastCalledWith({ tenantID: 10, spaceID: 100, page: 1, pageSize: 5, search: "" });
});

test("成员管理抽屉只作用于从空间列表选中的空间", async () => {
  const user = userEvent.setup();
  renderWithFeedback(<SpaceManagementPage api={createSpaceAPI()} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");

  await user.click(within(screen.getByRole("row", { name: /高一 2 班/ })).getByRole("button", { name: "成员管理" }));
  const secondDrawer = await screen.findByRole("dialog", { name: "高一 2 班成员抽屉" });

  const secondDrawerHeader = within(secondDrawer).getByTestId("space-member-drawer-header");
  expect(within(secondDrawerHeader).getByText("高一 2 班")).toBeInTheDocument();
  expect(within(secondDrawerHeader).queryByText("高一 2 班成员")).not.toBeInTheDocument();
  expect(within(secondDrawer).getByText("周老师")).toBeInTheDocument();
  expect(within(secondDrawer).queryByText("李老师")).not.toBeInTheDocument();


  await user.click(within(secondDrawer).getByRole("button", { name: "返回空间列表" }));
  await waitFor(() => {
    expect(screen.queryByRole("dialog", { name: "高一 2 班成员抽屉" })).not.toBeInTheDocument();
  });
  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));
  const firstDrawer = await screen.findByRole("dialog", { name: "高一 1 班成员抽屉" });

  const firstDrawerHeader = within(firstDrawer).getByTestId("space-member-drawer-header");
  expect(within(firstDrawerHeader).getByText("高一 1 班")).toBeInTheDocument();
  expect(within(firstDrawerHeader).queryByText("高一 1 班成员")).not.toBeInTheDocument();
});

test("成员管理抽屉支持当前空间内检索和分页", async () => {
  const user = userEvent.setup();
  renderWithFeedback(<SpaceManagementPage api={createSpaceAPI()} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));
  const drawer = await screen.findByRole("dialog", { name: "高一 1 班成员抽屉" });

  const pagination = within(drawer).getByRole("navigation", { name: "分页" });
  expect(pagination).toHaveClass("pagination--right");
  expect(pagination.querySelector(".ant-pagination")).toBeInTheDocument();
  const pageSize = within(pagination).getByRole("combobox", { name: "每页条数" });
  expect(pageSize.compareDocumentPosition(pagination.querySelector(".ant-pagination") as Element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(within(drawer).getByText("共 8 条")).toBeInTheDocument();
  expect(within(drawer).getByText("张同学")).toBeInTheDocument();
  expect(within(drawer).queryByText("钱同学")).not.toBeInTheDocument();

  await user.click(pageSize);
  await user.click(screen.getByRole("option", { name: "10 条 / 页" }));

  expect(within(drawer).getByText("第 1 / 1 页")).toBeInTheDocument();
  expect(within(drawer).getByText("钱同学")).toBeInTheDocument();

  await user.click(pageSize);
  await user.click(screen.getByRole("option", { name: "5 条 / 页" }));

  await user.click(within(drawer).getByRole("button", { name: "下一页" }));

  expect(within(drawer).getByText("钱同学")).toBeInTheDocument();
  expect(within(drawer).queryByText("张同学")).not.toBeInTheDocument();

  await user.type(within(drawer).getByLabelText("成员检索关键词"), "周");
  await user.click(within(drawer).getByRole("button", { name: "搜索成员" }));

  expect(within(drawer).getByText("共 1 条")).toBeInTheDocument();
  expect(within(drawer).getByText("周同学")).toBeInTheDocument();
  expect(within(drawer).queryByText("周老师")).not.toBeInTheDocument();

  await user.click(within(drawer).getByRole("button", { name: "刷新成员列表" }));

  expect(await within(drawer).findByText("共 8 条")).toBeInTheDocument();
  expect(within(drawer).getByText("张同学")).toBeInTheDocument();
});

test("刷新成员列表会重新读取当前空间成员并播放过渡动画", async () => {
  const user = userEvent.setup();
  const api = createSpaceAPI();

  renderWithFeedback(<SpaceManagementPage api={api} userApi={createUserAPI()} tenantID={10} />);

  await screen.findByText("高一 1 班");
  await user.click(within(screen.getByRole("row", { name: /高一 1 班/ })).getByRole("button", { name: "成员管理" }));
  const drawer = await screen.findByRole("dialog", { name: "高一 1 班成员抽屉" });

  expect(await within(drawer).findByText("张同学")).toBeInTheDocument();

  const refreshResult = deferred<Awaited<ReturnType<ReturnType<typeof createSpaceAPI>["listSpaceMembers"]>>>();
  vi.mocked(api.listSpaceMembers).mockReturnValueOnce(refreshResult.promise);
  await user.click(within(drawer).getByRole("button", { name: "刷新成员列表" }));

  expect(api.listSpaceMembers).toHaveBeenCalledWith({ tenantID: 10, spaceID: 100, page: 1, pageSize: 5, search: "" });
  expect(within(drawer).getByRole("button", { name: "刷新成员列表" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );
  expect(within(drawer).getByTestId("space-member-table-wrap")).toHaveClass("tenant-list-transition--refreshing");

  refreshResult.resolve({
    items: [
      { id: 1, userID: 1, name: "李老师", role: "space_admin", status: "enabled" },
      { id: 8, userID: 8, name: "钱同学", role: "student", status: "enabled" },
    ],
  });

  expect(await within(drawer).findByText("钱同学")).toBeInTheDocument();
  expect(within(drawer).queryByText("张同学")).not.toBeInTheDocument();
  await waitFor(() =>
    expect(within(drawer).getByTestId("space-member-table-wrap")).not.toHaveClass(
      "tenant-list-transition--refreshing",
    ),
  );
});
