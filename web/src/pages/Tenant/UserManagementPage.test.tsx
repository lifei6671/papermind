import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import type { ReactElement } from "react";
import { ApiError } from "../../api/client";
import type { UserManagementAPI } from "../../api/users";
import { FeedbackProvider } from "../../app/feedback";
import { UserManagementPage } from "./UserManagementPage";
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

function createUserAPI() {
  return {
    listUsers: vi.fn().mockResolvedValue({
      items: [
        {
          id: 20,
          tenantID: 10,
          name: "李老师",
          username: "li.teacher",
          role: "teacher",
          avatarFileName: "li.png",
          status: "enabled",
        },
        {
          id: 21,
          tenantID: 10,
          name: "张同学",
          username: "zhang.student",
          role: "student",
          avatarFileName: "zhang.png",
          status: "disabled",
        },
        {
          id: 22,
          tenantID: 10,
          name: "赵老师",
          username: "zhao.teacher",
          role: "teacher",
          avatarFileName: "zhao.png",
          status: "enabled",
        },
        {
          id: 23,
          tenantID: 10,
          name: "王同学",
          username: "wang.student",
          role: "student",
          avatarFileName: "wang.png",
          status: "enabled",
        },
        {
          id: 24,
          tenantID: 10,
          name: "刘老师",
          username: "liu.teacher",
          role: "teacher",
          avatarFileName: "liu.png",
          status: "enabled",
        },
        {
          id: 25,
          tenantID: 10,
          name: "孙同学",
          username: "sun.student",
          role: "student",
          avatarFileName: "sun.png",
          status: "enabled",
        },
        {
          id: 26,
          tenantID: 10,
          name: "周老师",
          username: "zhou.teacher",
          role: "teacher",
          avatarFileName: "zhou.png",
          status: "enabled",
        },
      ],
    }),
    createUser: vi.fn().mockResolvedValue({
      id: 27,
      tenantID: 10,
      name: "钱同学",
      username: "qian.student",
      role: "student",
      avatarFileName: "未上传",
      status: "enabled",
      forcePasswordChange: true,
    }),
    disableUser: vi.fn().mockResolvedValue({
      id: 20,
      tenantID: 10,
      name: "李老师",
      username: "li.teacher",
      role: "teacher",
      avatarFileName: "li.png",
      status: "disabled",
    }),
    enableUser: vi.fn().mockResolvedValue({
      id: 21,
      tenantID: 10,
      name: "张同学",
      username: "zhang.student",
      role: "student",
      avatarFileName: "zhang.png",
      status: "enabled",
    }),
  };
}

function createSpaceAPI() {
  return {
    listSpaces: vi.fn().mockResolvedValue({ items: [] }),
    createSpaceMember: vi.fn().mockResolvedValue({
      id: 1001,
      userID: 27,
      name: "钱同学",
      role: "student",
      status: "enabled",
    }),
    updateSpaceMember: vi.fn().mockResolvedValue({
      id: 1001,
      userID: 20,
      name: "李老师",
      role: "student",
      status: "enabled",
    }),
    removeSpaceMember: vi.fn().mockResolvedValue(undefined),
  };
}

test("用户管理页展示用户列表和角色状态", async () => {
  const api = createUserAPI();
  renderWithFeedback(<UserManagementPage api={api} spaceApi={createSpaceAPI()} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "用户管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "用户管理" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建用户" })).toHaveClass("tenant-create-button");
  expect(screen.queryByRole("button", { name: "导入用户" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("搜索用户")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新用户列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("姓名")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("用户导入文件")).not.toBeInTheDocument();
  const teacherRow = await screen.findByRole("row", { name: /李老师/ });
  expect(within(teacherRow).getByText("教师")).toBeInTheDocument();
  expect(within(teacherRow).getByText("启用")).toBeInTheDocument();
  expect(api.listUsers).toHaveBeenCalledWith(10);
  expect(within(teacherRow).getByRole("button", { name: "禁用用户" })).toHaveClass(
    "tenant-action-button",
    "tenant-action--close",
  );
});

test("用户管理页展示教师空间分配状态", async () => {
  const spaceApi = createSpaceAPI();
  vi.mocked(spaceApi.listSpaces).mockResolvedValue({
      items: [{
        id: 301,
        tenantID: 10,
        name: "高一一班",
        description: "月考空间",
        logoFileName: "class.png",
        members: [{
          id: 1,
          userID: 20,
          name: "李老师",
          role: "teacher",
          status: "enabled",
        }],
      }],
    });

  renderWithFeedback(<UserManagementPage api={createUserAPI()} spaceApi={spaceApi} tenantID={10} />);

  const teacherRow = await screen.findByRole("row", { name: /李老师/ });
  expect(within(teacherRow).getByText("已加入 1 个空间")).toBeInTheDocument();
  expect(screen.getByRole("row", { name: /张同学/ })).toHaveTextContent("暂未分配空间");
  expect(spaceApi.listSpaces).toHaveBeenCalledWith(10);
});

test("用户详情展示教师空间分配状态", async () => {
  const user = userEvent.setup();
  const spaceApi = createSpaceAPI();
  vi.mocked(spaceApi.listSpaces).mockResolvedValue({
      items: [{
        id: 301,
        tenantID: 10,
        name: "高一一班",
        description: "月考空间",
        logoFileName: "class.png",
        members: [{
          id: 1,
          userID: 20,
          name: "李老师",
          role: "teacher",
          status: "enabled",
        }],
      }],
    });
  renderWithFeedback(<UserManagementPage api={createUserAPI()} spaceApi={spaceApi} tenantID={10} />);

  const teacherRow = await screen.findByRole("row", { name: /李老师/ });
  await user.click(within(teacherRow).getByRole("button", { name: "查看详情" }));

  const dialog = screen.getByRole("dialog", { name: "用户详情" });
  expect(dialog.closest(".ant-drawer")).toBeInTheDocument();
  expect(dialog.querySelector(".ant-descriptions")).toBeInTheDocument();
  expect(dialog).toHaveTextContent("李老师");
  expect(dialog).toHaveTextContent("空间分配");
  expect(dialog).toHaveTextContent("已加入 1 个空间");
  await user.click(within(dialog).getByRole("button", { name: "关闭抽屉" }));
  expect(screen.queryByRole("dialog", { name: "用户详情" })).not.toBeInTheDocument();
});

test("缺少租户上下文时不请求用户接口", () => {
  const api = createUserAPI();

  renderWithFeedback(<UserManagementPage api={api} spaceApi={createSpaceAPI()} />);

  expect(api.listUsers).not.toHaveBeenCalled();
  expect(screen.getByRole("alert")).toHaveTextContent("当前账号没有租户上下文");
});

test("租户管理员可以创建用户", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  const spaceApi = createSpaceAPI();
  vi.mocked(spaceApi.listSpaces).mockResolvedValue({
    items: [
      {
        id: 301,
        tenantID: 10,
        name: "高一一班",
        description: "月考空间",
        logoFileName: "class-a.png",
        members: [],
      },
      {
        id: 302,
        tenantID: 10,
        name: "高一二班",
        description: "周考空间",
        logoFileName: "class-b.png",
        members: [],
      },
    ],
  });
  renderWithFeedback(<UserManagementPage api={api} spaceApi={spaceApi} tenantID={10} />);

  await screen.findByText("李老师");

  await user.click(screen.getByRole("button", { name: "创建用户" }));

  const drawer = screen.getByRole("dialog", { name: "创建用户抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(within(drawer).getAllByText("*")).toHaveLength(4);
  for (const marker of within(drawer).getAllByText("*")) {
    expect(marker).toHaveClass("required-marker");
  }
  expect(screen.queryByLabelText("用户头像")).not.toBeInTheDocument();
  expect(within(drawer).getByLabelText("首次登录必须修改密码")).toBeChecked();

  await user.type(within(drawer).getByLabelText("姓名"), "钱同学");
  await user.type(within(drawer).getByLabelText("账号"), "qian.student");
  await user.type(within(drawer).getByLabelText("初始密码"), "student-secure-123");
  await user.click(within(drawer).getByLabelText("高一一班"));
  await user.click(within(drawer).getByLabelText("高一二班"));
  await user.click(within(drawer).getByRole("button", { name: "确认创建" }));

  expect(api.createUser).toHaveBeenCalledWith({
    tenantID: 10,
    name: "钱同学",
    username: "qian.student",
    password: "student-secure-123",
    role: "student",
    avatarFileName: "未上传",
    forcePasswordChange: true,
  });
  expect(spaceApi.createSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, userID: 27, role: "student" });
  expect(spaceApi.createSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 302, userID: 27, role: "student" });
  expect(await screen.findByText("钱同学")).toBeInTheDocument();
  expect(screen.getAllByRole("row")[1]).toHaveTextContent("钱同学");
  expect(screen.getByText("qian.student")).toBeInTheDocument();
  expect(screen.getByText("未上传")).toBeInTheDocument();
});

test("租户管理员编辑用户时只能维护空间归属不能变更角色", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  const spaceApi = createSpaceAPI();
  vi.mocked(spaceApi.listSpaces).mockResolvedValue({
    items: [
      {
        id: 301,
        tenantID: 10,
        name: "高一一班",
        description: "月考空间",
        logoFileName: "class-a.png",
        members: [{ id: 1, userID: 20, name: "李老师", role: "teacher", status: "enabled" }],
      },
      {
        id: 302,
        tenantID: 10,
        name: "高一二班",
        description: "周考空间",
        logoFileName: "class-b.png",
        members: [],
      },
    ],
  });
  renderWithFeedback(<UserManagementPage api={api} spaceApi={spaceApi} tenantID={10} />);

  const teacherRow = await screen.findByRole("row", { name: /李老师/ });
  expect(within(teacherRow).getByText("已加入 1 个空间")).toBeInTheDocument();
  await user.click(within(teacherRow).getByRole("button", { name: "编辑用户" }));

  const drawer = screen.getByRole("dialog", { name: "编辑用户抽屉" });
  expect(drawer.closest(".ant-drawer")).toBeInTheDocument();
  expect(within(drawer).getByLabelText("角色")).toHaveValue("教师");
  expect(within(drawer).getByLabelText("角色")).toHaveAttribute("readonly");
  expect(within(drawer).getByLabelText("高一一班")).toBeChecked();
  expect(within(drawer).getByLabelText("高一二班")).not.toBeChecked();

  await user.click(within(drawer).getByLabelText("高一二班"));
  await user.click(within(drawer).getByRole("button", { name: "保存用户" }));

  expect(spaceApi.updateSpaceMember).not.toHaveBeenCalled();
  expect(spaceApi.createSpaceMember).toHaveBeenCalledWith({ tenantID: 10, spaceID: 302, userID: 20, role: "teacher" });
  expect(await within(teacherRow).findByText("教师")).toBeInTheDocument();
  expect(within(teacherRow).getByText("已加入 2 个空间")).toBeInTheDocument();
});

test("编辑用户空间归属时会展示并保护空间管理员归属", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  const spaceApi = createSpaceAPI();
  vi.mocked(spaceApi.listSpaces).mockResolvedValue({
    items: [{
      id: 301,
      tenantID: 10,
      name: "高一一班",
      description: "月考空间",
      logoFileName: "class-a.png",
      members: [{ id: 1, userID: 20, name: "李老师", role: "space_admin", status: "enabled" }],
    }],
  });
  renderWithFeedback(<UserManagementPage api={api} spaceApi={spaceApi} tenantID={10} />);

  const teacherRow = await screen.findByRole("row", { name: /李老师/ });
  expect(within(teacherRow).getByText("已加入 1 个空间")).toBeInTheDocument();
  await user.click(within(teacherRow).getByRole("button", { name: "编辑用户" }));
  const drawer = screen.getByRole("dialog", { name: "编辑用户抽屉" });
  expect(within(drawer).getByLabelText("高一一班")).toBeChecked();
  expect(within(drawer).getByLabelText("高一一班")).toBeDisabled();
  await user.click(within(drawer).getByRole("button", { name: "保存用户" }));

  expect(spaceApi.updateSpaceMember).not.toHaveBeenCalledWith(expect.objectContaining({
    spaceID: 301,
    userID: 20,
    role: "teacher",
  }));
  expect(spaceApi.removeSpaceMember).not.toHaveBeenCalledWith(expect.objectContaining({
    spaceID: 301,
    userID: 20,
  }));
});

test("编辑用户空间成员保存失败后刷新后端真实状态", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  const spaceApi = createSpaceAPI();
  vi.mocked(api.listUsers).mockResolvedValue({
    items: [{
      id: 20,
      tenantID: 10,
      name: "李老师",
      username: "li.teacher",
      role: "teacher",
      avatarFileName: "li.png",
      status: "enabled",
    }],
  });
  vi.mocked(spaceApi.listSpaces).mockResolvedValue({
    items: [{
      id: 301,
      tenantID: 10,
      name: "高一一班",
      description: "月考空间",
      logoFileName: "class-a.png",
      members: [{ id: 1, userID: 20, name: "李老师", role: "teacher", status: "enabled" }],
    }, {
      id: 302,
      tenantID: 10,
      name: "高一二班",
      description: "周考空间",
      logoFileName: "class-b.png",
      members: [],
    }],
  });
  vi.mocked(spaceApi.createSpaceMember).mockRejectedValue(new ApiError("空间成员保存失败", 40001, 200, {
    code: 40001,
    message: "空间成员保存失败",
    data: null,
  }));
  renderWithFeedback(<UserManagementPage api={api} spaceApi={spaceApi} tenantID={10} />);

  await screen.findByRole("row", { name: /李老师/ });
  await user.click(screen.getByRole("button", { name: "编辑用户" }));
  const drawer = screen.getByRole("dialog", { name: "编辑用户抽屉" });
  await user.click(within(drawer).getByLabelText("高一二班"));
  await user.click(within(drawer).getByRole("button", { name: "保存用户" }));

  expect(await screen.findByRole("alert")).toHaveTextContent("空间成员保存失败");
  await waitFor(() => {
    expect(api.listUsers).toHaveBeenCalledTimes(2);
  });
  expect(screen.getByRole("row", { name: /李老师/ })).toHaveTextContent("教师");
});

test("用户列表支持分页并在搜索后回到第一页", async () => {
  const user = userEvent.setup();
  renderWithFeedback(<UserManagementPage api={createUserAPI()} spaceApi={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("李老师");

  const pagination = screen.getByRole("navigation", { name: "分页" });
  expect(pagination).toHaveClass("pagination--right");
  expect(pagination.querySelector(".ant-pagination")).toBeInTheDocument();
  expect(screen.getByText("共 7 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 2 页")).toBeInTheDocument();
  expect(screen.getByText("刘老师")).toBeInTheDocument();
  expect(screen.queryByText("孙同学")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "下一页" }));

  expect(screen.getByText("第 2 / 2 页")).toBeInTheDocument();
  expect(screen.getByText("孙同学")).toBeInTheDocument();
  expect(screen.getByText("周老师")).toBeInTheDocument();
  expect(screen.queryByText("李老师")).not.toBeInTheDocument();

  await user.type(screen.getByLabelText("搜索用户"), "zhang.student");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.getByText("共 1 条")).toBeInTheDocument();
  expect(screen.getByText("第 1 / 1 页")).toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();
  expect(screen.queryByText("周老师")).not.toBeInTheDocument();
});

test("租户管理员可以搜索和刷新用户列表", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  renderWithFeedback(<UserManagementPage api={api} spaceApi={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("李老师");
  const refreshResult = deferred<Awaited<ReturnType<UserManagementAPI["listUsers"]>>>();
  vi.mocked(api.listUsers).mockReturnValueOnce(refreshResult.promise);

  await user.type(screen.getByLabelText("搜索用户"), "zhang.student");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("李老师")).not.toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新用户列表" }));
  expect(screen.getByRole("button", { name: "刷新用户列表" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );

  refreshResult.resolve({
    items: [
      {
        id: 20,
        tenantID: 10,
        name: "李老师",
        username: "li.teacher",
        role: "teacher",
        avatarFileName: "li.png",
        status: "enabled",
      },
      {
        id: 21,
        tenantID: 10,
        name: "张同学",
        username: "zhang.student",
        role: "student",
        avatarFileName: "zhang.png",
        status: "disabled",
      },
    ],
  });

  expect(await screen.findByText("李老师")).toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();
});

test("禁用状态用户的操作按钮改为启用用户并真实调用接口", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  renderWithFeedback(<UserManagementPage api={api} spaceApi={createSpaceAPI()} tenantID={10} actorID={99} />);

  const disabledUserRow = await screen.findByRole("row", { name: /张同学/ });

  expect(within(disabledUserRow).queryByRole("button", { name: "禁用用户" })).not.toBeInTheDocument();
  await user.click(within(disabledUserRow).getByRole("button", { name: "启用用户" }));

  expect(api.enableUser).toHaveBeenCalledWith({ tenantID: 10, actorID: 99, userID: 21 });
  expect(await within(disabledUserRow).findByText("启用")).toBeInTheDocument();
});

test("当前登录用户不能禁用自己", async () => {
  renderWithFeedback(<UserManagementPage api={createUserAPI()} spaceApi={createSpaceAPI()} tenantID={10} actorID={20} />);

  const currentUserRow = await screen.findByRole("row", { name: /李老师/ });

  expect(within(currentUserRow).getByRole("button", { name: "禁用用户" })).toBeDisabled();
});

test("禁用用户失败时展示 toast 提示", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  api.disableUser.mockRejectedValue(new ApiError("cannot disable self", 40001, 200, {
    code: 40001,
    message: "cannot disable self",
    data: null,
  }));
  renderWithFeedback(<UserManagementPage api={api} spaceApi={createSpaceAPI()} tenantID={10} actorID={99} />);

  await screen.findByText("李老师");
  await user.click(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "禁用用户" }));
  await user.click(screen.getByRole("button", { name: "确认禁用" }));

  expect(await screen.findByRole("alert")).toHaveTextContent("cannot disable self");
  expect(api.disableUser).toHaveBeenCalledWith({ tenantID: 10, actorID: 99, userID: 20 });
});


test("用户列表查询不到数据时显示无记录", async () => {
  const user = userEvent.setup();
  renderWithFeedback(<UserManagementPage api={createUserAPI()} spaceApi={createSpaceAPI()} tenantID={10} />);

  await screen.findByText("李老师");

  await user.type(screen.getByLabelText("搜索用户"), "不存在的用户");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.getByText("无记录")).toBeInTheDocument();
  expect(screen.queryByText("李老师")).not.toBeInTheDocument();
  expect(screen.queryByText("张同学")).not.toBeInTheDocument();
});


test("禁用用户前展示影响范围并确认禁用", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  renderWithFeedback(<UserManagementPage api={api} spaceApi={createSpaceAPI()} tenantID={10} actorID={99} />);

  await screen.findByText("李老师");

  await user.click(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "禁用用户" }));

  expect(screen.getByRole("dialog", { name: "禁用用户提示" })).toHaveTextContent("将失去登录能力");
  expect(screen.getByRole("dialog", { name: "禁用用户提示" })).toHaveTextContent("阅卷和考试发布权限会被收回");

  await user.click(screen.getByRole("button", { name: "确认禁用" }));

  expect(api.disableUser).toHaveBeenCalledWith({ tenantID: 10, actorID: 99, userID: 20 });
  expect(await within(screen.getByRole("row", { name: /李老师/ })).findByText("禁用")).toBeInTheDocument();
});
