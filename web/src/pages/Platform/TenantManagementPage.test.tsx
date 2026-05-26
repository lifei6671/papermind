import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { TenantManagementPage } from "./TenantManagementPage";

function createTenantAPI() {
  return {
    listTenants: vi.fn().mockResolvedValue({
      items: [
        {
          id: 1,
          name: "青藤一中",
          description: "统一管理月考、联考和补测",
          code: "PM-QT01",
          logoFileName: "qingteng.png",
          allowRegister: true,
        },
        {
          id: 2,
          name: "知行培训",
          description: "企业知识课堂和阶段测评",
          code: "PM-ZX01",
          logoFileName: "zhixing.png",
          allowRegister: false,
          registerClosedLabel: "暂停注册",
        },
      ],
    }),
    createTenant: vi.fn().mockResolvedValue({
      id: 3,
      name: "星海大学",
      description: "面向公共课和企业培训的考试空间",
      code: "PM-XH01",
      logoFileName: "xinghai.png",
      allowRegister: true,
    }),
  };
}

test("租户管理页展示租户列表和租户码", async () => {
  const api = createTenantAPI();
  render(<TenantManagementPage api={api} />);

  const platformMenu = screen.getByRole("tablist", { name: "平台运营菜单" });
  expect(within(platformMenu).getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "租户管理" })).toHaveAttribute("aria-selected", "true");
  expect(within(platformMenu).queryByRole("tab", { name: "概览" })).not.toBeInTheDocument();
  expect(within(platformMenu).queryByRole("tab", { name: "平台配置" })).not.toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "租户列表" })).not.toBeInTheDocument();
  expect(screen.queryByText("租户码用于注册链接和手动注册归属。")).not.toBeInTheDocument();
  expect(await screen.findByText("青藤一中")).toBeInTheDocument();
  expect(screen.getByText("PM-QT01")).toBeInTheDocument();
  expect(screen.getByText("允许注册")).toBeInTheDocument();
  expect(api.listTenants).toHaveBeenCalled();
});

test("平台管理员可以创建租户并上传 logo", async () => {
  const user = userEvent.setup();
  const api = createTenantAPI();
  render(<TenantManagementPage api={api} />);

  await screen.findByText("青藤一中");

  await user.click(screen.getByRole("button", { name: "创建租户" }));

  expect(screen.getByRole("dialog", { name: "创建租户弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("租户名称"), "星海大学");
  await user.type(screen.getByLabelText("租户描述"), "面向公共课和企业培训的考试空间");
  await user.upload(screen.getByLabelText("租户 Logo"), new File(["logo"], "xinghai.png", { type: "image/png" }));
  await user.click(screen.getByRole("button", { name: "确认创建" }));

  expect(api.createTenant).toHaveBeenCalledWith({
    description: "面向公共课和企业培训的考试空间",
    logoFileName: "xinghai.png",
    name: "星海大学",
  });
  expect(await screen.findByText("星海大学")).toBeInTheDocument();
  expect(screen.getByText("xinghai.png")).toBeInTheDocument();
  expect(screen.getByText("PM-XH01")).toBeInTheDocument();
});

test("租户列表上方提供搜索输入和创建操作区", async () => {
  const user = userEvent.setup();
  render(<TenantManagementPage api={createTenantAPI()} />);

  await screen.findByText("青藤一中");

  expect(screen.getByLabelText("搜索租户")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建租户" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "搜索" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新租户列表" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("搜索租户"), "知行");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.getByText("知行培训")).toBeInTheDocument();
  expect(screen.queryByText("青藤一中")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新租户列表" }));

  expect(screen.getByText("青藤一中")).toBeInTheDocument();
  expect(screen.getByText("知行培训")).toBeInTheDocument();
});

test("平台管理员可以编辑描述、重置租户码并切换注册开关", async () => {
  const user = userEvent.setup();
  render(<TenantManagementPage api={createTenantAPI()} />);

  await screen.findByText("青藤一中");

  const row = screen.getByRole("row", { name: /青藤一中/ });
  const editButton = within(row).getByRole("button", { name: "编辑描述" });
  const resetButton = within(row).getByRole("button", { name: "重置租户码" });
  const closeRegisterButton = within(row).getByRole("button", { name: "关闭注册" });

  expect(editButton).toHaveClass("tenant-action--edit");
  expect(resetButton).toHaveClass("tenant-action--reset");
  expect(closeRegisterButton).toHaveClass("tenant-action--close");

  await user.click(editButton);
  await user.clear(screen.getByLabelText("编辑租户描述"));
  await user.type(screen.getByLabelText("编辑租户描述"), "统一管理月考、联考和补测");
  await user.click(screen.getByRole("button", { name: "保存描述" }));

  expect(screen.getByText("统一管理月考、联考和补测")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /青藤一中/ })).getByRole("button", { name: "重置租户码" }));
  expect(screen.getByText("PM-QT02")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /青藤一中/ })).getByRole("button", { name: "关闭注册" }));
  expect(screen.getByText("禁止注册")).toBeInTheDocument();
});
