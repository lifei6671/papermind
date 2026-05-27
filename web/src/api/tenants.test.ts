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
        }),
      }),
    );
  });
});
