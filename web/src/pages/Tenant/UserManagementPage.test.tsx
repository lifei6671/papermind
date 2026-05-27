import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { UserManagementPage } from "./UserManagementPage";

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
      ],
    }),
    createUser: vi.fn().mockResolvedValue({
      id: 22,
      tenantID: 10,
      name: "王同学",
      username: "wang.student",
      role: "student",
      avatarFileName: "wang.png",
      status: "enabled",
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
  };
}

test("用户管理页展示用户列表和角色状态", async () => {
  const api = createUserAPI();
  render(<UserManagementPage api={api} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "用户管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "用户管理" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建用户" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "导入用户" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索用户")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新用户列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("姓名")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("用户导入文件")).not.toBeInTheDocument();
  expect(await screen.findByText("李老师")).toBeInTheDocument();
  expect(screen.getByText("教师")).toBeInTheDocument();
  expect(screen.getByText("启用")).toBeInTheDocument();
  expect(api.listUsers).toHaveBeenCalledWith(10);
  expect(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "禁用用户" })).toHaveClass(
    "tenant-action-button",
    "tenant-action--close",
  );
});

test("租户管理员可以创建用户并上传头像", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  render(<UserManagementPage api={api} tenantID={10} />);

  await screen.findByText("李老师");

  await user.click(screen.getByRole("button", { name: "创建用户" }));

  expect(screen.getByRole("dialog", { name: "创建用户弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("姓名"), "王同学");
  await user.type(screen.getByLabelText("账号"), "wang.student");
  await user.type(screen.getByLabelText("初始密码"), "student-secure-123");
  await user.selectOptions(screen.getByLabelText("角色"), "student");
  await user.upload(screen.getByLabelText("用户头像"), new File(["avatar"], "wang.png", { type: "image/png" }));
  await user.click(screen.getByRole("button", { name: "确认创建" }));

  expect(api.createUser).toHaveBeenCalledWith({
    tenantID: 10,
    name: "王同学",
    username: "wang.student",
    password: "student-secure-123",
    role: "student",
    avatarFileName: "wang.png",
  });
  expect(await screen.findByText("王同学")).toBeInTheDocument();
  expect(screen.getByText("wang.student")).toBeInTheDocument();
  expect(screen.getByText("wang.png")).toBeInTheDocument();
});

test("租户管理员可以搜索和刷新用户列表", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage api={createUserAPI()} tenantID={10} />);

  await screen.findByText("李老师");

  await user.type(screen.getByLabelText("搜索用户"), "zhang.student");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("李老师")).not.toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新用户列表" }));

  expect(screen.getByText("李老师")).toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();
});

test("用户列表查询不到数据时显示无记录", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage api={createUserAPI()} tenantID={10} />);

  await screen.findByText("李老师");

  await user.type(screen.getByLabelText("搜索用户"), "不存在的用户");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.getByText("无记录")).toBeInTheDocument();
  expect(screen.queryByText("李老师")).not.toBeInTheDocument();
  expect(screen.queryByText("张同学")).not.toBeInTheDocument();
});

test("租户管理员可以导入用户文件", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage api={createUserAPI()} tenantID={10} />);

  await screen.findByText("李老师");

  await user.click(screen.getByRole("button", { name: "导入用户" }));

  expect(screen.getByRole("dialog", { name: "导入用户弹窗" })).toBeInTheDocument();

  await user.upload(screen.getByLabelText("用户导入文件"), new File(["name,role"], "users.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByRole("status")).toHaveTextContent("users.csv");
  expect(screen.getByRole("status")).toHaveTextContent("已解析 24 个用户");
});

test("禁用用户前展示影响范围并确认禁用", async () => {
  const user = userEvent.setup();
  const api = createUserAPI();
  render(<UserManagementPage api={api} tenantID={10} actorID={99} />);

  await screen.findByText("李老师");

  await user.click(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "禁用用户" }));

  expect(screen.getByRole("dialog", { name: "禁用用户提示" })).toHaveTextContent("将失去登录能力");
  expect(screen.getByRole("dialog", { name: "禁用用户提示" })).toHaveTextContent("阅卷和考试发布权限会被收回");

  await user.click(screen.getByRole("button", { name: "确认禁用" }));

  expect(api.disableUser).toHaveBeenCalledWith({ tenantID: 10, actorID: 99, userID: 20 });
  expect(await within(screen.getByRole("row", { name: /李老师/ })).findByText("禁用")).toBeInTheDocument();
});
