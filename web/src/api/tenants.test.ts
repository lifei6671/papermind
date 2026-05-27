import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createTenantAPI } from "./tenants";

describe("tenant api", () => {
  test("租户列表会映射服务端字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 10,
            name: "第一中学",
            logo_url: "",
            description: "演示租户",
            tenant_code: "NO1",
            allow_register: false,
            status: "enabled",
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      })),
    );
    const api = createTenantAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listTenants()).resolves.toEqual({
      items: [{
        id: 10,
        name: "第一中学",
        description: "演示租户",
        code: "NO1",
        logoFileName: "未上传",
        allowRegister: false,
        registerClosedLabel: "暂停注册",
      }],
    });
    expect(fetcher).toHaveBeenCalledWith("/api/v1/tenants", expect.objectContaining({ method: "GET" }));
  });

  test("租户列表会把关键字提交给服务端检索", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [],
          page: 1,
          page_size: 20,
          total: 0,
        },
      })),
    );
    const api = createTenantAPI(createApiClient({ baseUrl: "", fetcher }));

    await api.listTenants({ keyword: " 青藤 " });

    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenants?keyword=%E9%9D%92%E8%97%A4",
      expect.objectContaining({ method: "GET" }),
    );
  });

  test("创建租户会提交服务端字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 11,
          name: "第二中学",
          logo_url: "tenant.png",
          description: "新租户",
          tenant_code: "NO2",
          allow_register: true,
          status: "enabled",
        },
      })),
    );
    const api = createTenantAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createTenant({
      name: "第二中学",
      description: "新租户",
      logoFileName: "tenant.png",
      allowRegister: false,
    })).resolves.toMatchObject({
      id: 11,
      code: "NO2",
      logoFileName: "tenant.png",
      allowRegister: true,
      registerClosedLabel: undefined,
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenants",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          name: "第二中学",
          description: "新租户",
          logo_url: "tenant.png",
          allow_register: false,
        }),
      }),
    );
  });

  test("租户操作会提交后端接口并映射返回租户", async () => {
    const fetcher = vi.fn().mockImplementation(() =>
      Promise.resolve(new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 11,
          name: "第二中学",
          logo_url: "tenant.png",
          description: "服务端保存后的描述",
          tenant_code: "NO3",
          allow_register: false,
          status: "enabled",
        },
      }))),
    );
    const api = createTenantAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.updateTenantProfile({
      tenantID: 11,
      name: "第二实验中学",
      description: "服务端保存后的描述",
      logoFileName: "tenant-new.webp",
    })).resolves.toMatchObject({
      id: 11,
      description: "服务端保存后的描述",
    });
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenants/11/profile",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          name: "第二实验中学",
          description: "服务端保存后的描述",
          logo_url: "tenant-new.webp",
        }),
      }),
    );

    await expect(api.resetTenantCode(11)).resolves.toMatchObject({ code: "NO3" });
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenants/11/reset-code",
      expect.objectContaining({ method: "POST" }),
    );

    await expect(api.disableTenantRegistration(11)).resolves.toMatchObject({
      allowRegister: false,
      registerClosedLabel: "暂停注册",
    });
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenants/11/register-setting",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ allow_register: false }),
      }),
    );

    await api.enableTenantRegistration(11);
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenants/11/register-setting",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ allow_register: true }),
      }),
    );
  });
});
