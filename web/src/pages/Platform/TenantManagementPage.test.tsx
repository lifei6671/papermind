import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { TenantManagementPage } from "./TenantManagementPage";

test("租户管理页展示租户列表和租户码", () => {
  render(<TenantManagementPage />);

  expect(screen.getByRole("heading", { name: "租户管理" })).toBeInTheDocument();
  expect(screen.getByText("青藤一中")).toBeInTheDocument();
  expect(screen.getByText("PM-QT01")).toBeInTheDocument();
  expect(screen.getByText("允许注册")).toBeInTheDocument();
});

test("平台管理员可以创建租户并上传 logo", async () => {
  const user = userEvent.setup();
  render(<TenantManagementPage />);

  await user.type(screen.getByLabelText("租户名称"), "星海大学");
  await user.type(screen.getByLabelText("租户描述"), "面向公共课和企业培训的考试空间");
  await user.upload(screen.getByLabelText("租户 Logo"), new File(["logo"], "xinghai.png", { type: "image/png" }));
  await user.click(screen.getByRole("button", { name: "创建租户" }));

  expect(screen.getByText("星海大学")).toBeInTheDocument();
  expect(screen.getByText("xinghai.png")).toBeInTheDocument();
  expect(screen.getByText(/PM-XH/)).toBeInTheDocument();
});

test("平台管理员可以编辑描述、重置租户码并切换注册开关", async () => {
  const user = userEvent.setup();
  render(<TenantManagementPage />);

  const row = screen.getByRole("row", { name: /青藤一中/ });

  await user.click(within(row).getByRole("button", { name: "编辑描述" }));
  await user.clear(screen.getByLabelText("编辑租户描述"));
  await user.type(screen.getByLabelText("编辑租户描述"), "统一管理月考、联考和补测");
  await user.click(screen.getByRole("button", { name: "保存描述" }));

  expect(screen.getByText("统一管理月考、联考和补测")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /青藤一中/ })).getByRole("button", { name: "重置租户码" }));
  expect(screen.getByText("PM-QT02")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /青藤一中/ })).getByRole("button", { name: "关闭注册" }));
  expect(screen.getByText("禁止注册")).toBeInTheDocument();
});
