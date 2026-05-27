import { render, screen, waitFor, within } from "@testing-library/react";
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
      logoFileName: "/uploads/tenant-logos/xinghai.png",
      allowRegister: true,
    }),
    updateTenantProfile: vi.fn().mockResolvedValue({
      id: 1,
      name: "青藤实验中学",
      description: "服务端保存后的租户描述",
      code: "PM-QT01",
      logoFileName: "/uploads/tenant-logos/qingteng-new.webp",
      allowRegister: true,
    }),
    resetTenantCode: vi.fn().mockResolvedValue({
      id: 1,
      name: "青藤实验中学",
      description: "服务端保存后的租户描述",
      code: "PM-QT88",
      logoFileName: "/uploads/tenant-logos/qingteng-new.webp",
      allowRegister: true,
    }),
    disableTenantRegistration: vi.fn().mockResolvedValue({
      id: 1,
      name: "青藤实验中学",
      description: "服务端保存后的租户描述",
      code: "PM-QT01",
      logoFileName: "/uploads/tenant-logos/qingteng-new.webp",
      allowRegister: false,
      registerClosedLabel: "暂停注册",
    }),
    enableTenantRegistration: vi.fn().mockResolvedValue({
      id: 2,
      name: "知行培训",
      description: "企业知识课堂和阶段测评",
      code: "PM-ZX01",
      logoFileName: "zhixing.png",
      allowRegister: true,
    }),
  };
}

function createUploadAPI() {
  return {
    uploadFile: vi.fn().mockResolvedValue({
      key: "tenant-logos/20260527/xinghai.png",
      url: "/uploads/tenant-logos/xinghai.png",
      fileName: "xinghai.png",
      contentType: "image/png",
      size: 4,
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
  expect(screen.getByRole("img", { name: "青藤一中 Logo" })).toHaveAttribute("src", "qingteng.png");
  expect(screen.getByText("允许注册")).toBeInTheDocument();
  expect(api.listTenants).toHaveBeenCalled();
});

test("平台管理员可以创建租户并上传 logo", async () => {
  const user = userEvent.setup();
  const api = createTenantAPI();
  const uploadAPI = createUploadAPI();
  render(<TenantManagementPage api={api} uploadAPI={uploadAPI} />);

  await screen.findByText("青藤一中");

  await user.click(screen.getByRole("button", { name: "创建租户" }));

  expect(screen.getByRole("dialog", { name: "创建租户弹窗" })).toBeInTheDocument();
  expect(screen.getAllByText("*", { selector: ".required-marker" })).toHaveLength(2);
  expect(screen.getByLabelText("是否开放注册")).toBeChecked();
  expect(screen.getByLabelText("租户 Logo上传区域")).toBeInTheDocument();
  expect(screen.getByLabelText("租户 Logo预览")).toBeInTheDocument();

  await user.type(screen.getByLabelText("租户名称"), "星海大学");
  await user.type(screen.getByLabelText("租户描述"), "面向公共课和企业培训的考试空间");
  const logo = new File(["logo"], "xinghai.png", { type: "image/png" });
  await user.upload(screen.getByLabelText("租户 Logo"), logo);
  await waitFor(() => {
    expect(uploadAPI.uploadFile).toHaveBeenCalledWith({ category: "tenant-logos", file: logo });
  });
  await user.click(screen.getByLabelText("是否开放注册"));
  await user.click(screen.getByRole("button", { name: "确认创建" }));

  expect(api.createTenant).toHaveBeenCalledWith({
    description: "面向公共课和企业培训的考试空间",
    logoFileName: "/uploads/tenant-logos/xinghai.png",
    name: "星海大学",
    allowRegister: false,
  });
  expect(await screen.findByText("星海大学")).toBeInTheDocument();
  const tableRows = screen.getAllByRole("row");
  expect(within(tableRows[1]).getByText("星海大学")).toBeInTheDocument();
  expect(screen.queryByText("/uploads/tenant-logos/xinghai.png")).not.toBeInTheDocument();
  expect(screen.getByRole("img", { name: "星海大学 Logo" })).toHaveAttribute(
    "src",
    "/uploads/tenant-logos/xinghai.png",
  );
  expect(screen.getByText("PM-XH01")).toBeInTheDocument();
});

test("租户列表上方提供搜索输入和创建操作区", async () => {
  const user = userEvent.setup();
  const api = createTenantAPI();
  api.listTenants
    .mockResolvedValueOnce({
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
    })
    .mockResolvedValueOnce({
      items: [{
        id: 2,
        name: "知行培训",
        description: "企业知识课堂和阶段测评",
        code: "PM-ZX01",
        logoFileName: "zhixing.png",
        allowRegister: false,
        registerClosedLabel: "暂停注册",
      }],
    })
    .mockResolvedValueOnce({
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
    });
  render(<TenantManagementPage api={api} />);

  await screen.findByText("青藤一中");

  expect(screen.getByLabelText("搜索租户")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "创建租户" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "搜索" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新租户列表" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("搜索租户"), "知行");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(api.listTenants).toHaveBeenLastCalledWith({ keyword: "知行" });
  expect(screen.getByText("知行培训")).toBeInTheDocument();
  expect(screen.queryByText("青藤一中")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新租户列表" }));

  expect(api.listTenants).toHaveBeenLastCalledWith();
  expect(screen.getByText("青藤一中")).toBeInTheDocument();
  expect(screen.getByText("知行培训")).toBeInTheDocument();
});

test("搜索输入框支持按回车提交服务端检索", async () => {
  const user = userEvent.setup();
  const api = createTenantAPI();
  api.listTenants
    .mockResolvedValueOnce({
      items: [{
        id: 1,
        name: "青藤一中",
        description: "统一管理月考、联考和补测",
        code: "PM-QT01",
        logoFileName: "qingteng.png",
        allowRegister: true,
      }],
    })
    .mockResolvedValueOnce({
      items: [{
        id: 2,
        name: "知行培训",
        description: "企业知识课堂和阶段测评",
        code: "PM-ZX01",
        logoFileName: "zhixing.png",
        allowRegister: false,
        registerClosedLabel: "暂停注册",
      }],
    });
  render(<TenantManagementPage api={api} />);

  await screen.findByText("青藤一中");

  await user.type(screen.getByLabelText("搜索租户"), "知行{Enter}");

  expect(api.listTenants).toHaveBeenLastCalledWith({ keyword: "知行" });
  expect(await screen.findByText("知行培训")).toBeInTheDocument();
  expect(screen.queryByText("青藤一中")).not.toBeInTheDocument();
});

test("平台管理员可以通过后端接口编辑描述、重置租户码并切换注册开关", async () => {
  const user = userEvent.setup();
  const api = createTenantAPI();
  const uploadAPI = createUploadAPI();
  uploadAPI.uploadFile.mockResolvedValueOnce({
    key: "tenant-logos/20260527/qingteng-new.webp",
    url: "/uploads/tenant-logos/qingteng-new.webp",
    fileName: "qingteng-new.webp",
    contentType: "image/webp",
    size: 4,
  });
  render(<TenantManagementPage api={api} uploadAPI={uploadAPI} />);

  await screen.findByText("青藤一中");

  const row = screen.getByRole("row", { name: /青藤一中/ });
  const editButton = within(row).getByRole("button", { name: "编辑资料" });
  const resetButton = within(row).getByRole("button", { name: "重置租户码" });
  const closeRegisterButton = within(row).getByRole("button", { name: "关闭注册" });

  expect(editButton).toHaveClass("tenant-action--edit");
  expect(resetButton).toHaveClass("tenant-action--reset");
  expect(closeRegisterButton).toHaveClass("tenant-action--close");

  await user.click(editButton);
  await user.clear(screen.getByLabelText("编辑租户名称"));
  await user.type(screen.getByLabelText("编辑租户名称"), "青藤实验中学");
  await user.clear(screen.getByLabelText("编辑租户描述"));
  await user.type(screen.getByLabelText("编辑租户描述"), "服务端保存后的租户描述");
  await user.upload(screen.getByLabelText("编辑租户 Logo"), new File(["logo"], "qingteng-new.png", { type: "image/png" }));
  await waitFor(() => {
    expect(uploadAPI.uploadFile).toHaveBeenCalledWith({
      category: "tenant-logos",
      file: expect.objectContaining({ name: "qingteng-new.png" }),
    });
  });
  await user.click(screen.getByRole("button", { name: "保存资料" }));

  expect(api.updateTenantProfile).toHaveBeenCalledWith({
    tenantID: 1,
    name: "青藤实验中学",
    description: "服务端保存后的租户描述",
    logoFileName: "/uploads/tenant-logos/qingteng-new.webp",
  });
  expect(await screen.findByText("青藤实验中学")).toBeInTheDocument();
  expect(await screen.findByText("服务端保存后的租户描述")).toBeInTheDocument();
  expect(screen.getByRole("img", { name: "青藤实验中学 Logo" })).toHaveAttribute(
    "src",
    "/uploads/tenant-logos/qingteng-new.webp",
  );

  await user.click(within(screen.getByRole("row", { name: /青藤实验中学/ })).getByRole("button", { name: "重置租户码" }));
  expect(api.resetTenantCode).toHaveBeenCalledWith(1);
  expect(await screen.findByText("PM-QT88")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /青藤实验中学/ })).getByRole("button", { name: "关闭注册" }));
  expect(api.disableTenantRegistration).toHaveBeenCalledWith(1);
  expect(await within(screen.getByRole("row", { name: /青藤实验中学/ })).findByText("暂停注册")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /知行培训/ })).getByRole("button", { name: "开启注册" }));
  expect(api.enableTenantRegistration).toHaveBeenCalledWith(2);
  expect(await within(screen.getByRole("row", { name: /知行培训/ })).findByText("允许注册")).toBeInTheDocument();
});
