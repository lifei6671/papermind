import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { UserManagementPage } from "./UserManagementPage";

test("用户管理页展示用户列表和角色状态", () => {
  render(<UserManagementPage />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "用户管理" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "用户管理" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建用户" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "导入用户" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索用户")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新用户列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("姓名")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("用户导入文件")).not.toBeInTheDocument();
  expect(screen.getByText("李老师")).toBeInTheDocument();
  expect(screen.getByText("教师")).toBeInTheDocument();
  expect(screen.getByText("启用")).toBeInTheDocument();
  expect(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "禁用用户" })).toHaveClass(
    "tenant-action-button",
    "tenant-action--close",
  );
});

test("租户管理员可以创建用户并上传头像", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage />);

  await user.click(screen.getByRole("button", { name: "创建用户" }));

  expect(screen.getByRole("dialog", { name: "创建用户弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("姓名"), "王同学");
  await user.type(screen.getByLabelText("账号"), "wang.student");
  await user.selectOptions(screen.getByLabelText("角色"), "student");
  await user.upload(screen.getByLabelText("用户头像"), new File(["avatar"], "wang.png", { type: "image/png" }));
  await user.click(screen.getByRole("button", { name: "确认创建" }));

  expect(screen.getByText("王同学")).toBeInTheDocument();
  expect(screen.getByText("wang.student")).toBeInTheDocument();
  expect(screen.getByText("wang.png")).toBeInTheDocument();
});

test("租户管理员可以搜索和刷新用户列表", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage />);

  await user.type(screen.getByLabelText("搜索用户"), "zhang.student");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("李老师")).not.toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新用户列表" }));

  expect(screen.getByText("李老师")).toBeInTheDocument();
  expect(screen.getByText("张同学")).toBeInTheDocument();
});

test("租户管理员可以导入用户文件", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage />);

  await user.click(screen.getByRole("button", { name: "导入用户" }));

  expect(screen.getByRole("dialog", { name: "导入用户弹窗" })).toBeInTheDocument();

  await user.upload(screen.getByLabelText("用户导入文件"), new File(["name,role"], "users.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByRole("status")).toHaveTextContent("users.csv");
  expect(screen.getByRole("status")).toHaveTextContent("已解析 24 个用户");
});

test("禁用用户前展示影响范围并确认禁用", async () => {
  const user = userEvent.setup();
  render(<UserManagementPage />);

  await user.click(within(screen.getByRole("row", { name: /李老师/ })).getByRole("button", { name: "禁用用户" }));

  expect(screen.getByRole("dialog", { name: "禁用用户提示" })).toHaveTextContent("将失去登录能力");
  expect(screen.getByRole("dialog", { name: "禁用用户提示" })).toHaveTextContent("阅卷和考试发布权限会被收回");

  await user.click(screen.getByRole("button", { name: "确认禁用" }));

  expect(within(screen.getByRole("row", { name: /李老师/ })).getByText("禁用")).toBeInTheDocument();
});
