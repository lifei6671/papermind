import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createSpaceAPI } from "./spaces";

describe("space api", () => {
  test("空间列表会映射成员字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 301,
            tenant_id: 10,
            name: "高一一班",
            logo_url: "",
            description: "理科班",
            members: [{
              id: 1,
              user_id: 20,
              name: "李老师",
              role: "space_admin",
              status: "enabled",
            }],
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      })),
    );
    const api = createSpaceAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listSpaces(10)).resolves.toEqual({
      items: [{
        id: 301,
        tenantID: 10,
        name: "高一一班",
        description: "理科班",
        logoFileName: "未上传",
        members: [{
          id: 1,
          name: "李老师",
          role: "space_admin",
          status: "enabled",
        }],
      }],
    });
    expect(fetcher).toHaveBeenCalledWith("/api/v1/spaces?tenant_id=10", expect.objectContaining({ method: "GET" }));
  });

  test("创建空间会把管理员姓名映射为用户 ID", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 302,
          tenant_id: 10,
          name: "高一二班",
          logo_url: "space.png",
          description: "实验班",
          members: [{
            id: 2,
            user_id: 21,
            name: "用户 21",
            role: "space_admin",
            status: "enabled",
          }],
        },
      })),
    );
    const api = createSpaceAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createSpace({
      tenantID: 10,
      name: "高一二班",
      description: "实验班",
      logoFileName: "space.png",
      adminName: "周老师",
    })).resolves.toMatchObject({
      id: 302,
      members: [{ id: 2, name: "周老师", role: "space_admin", status: "enabled" }],
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/spaces",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          name: "高一二班",
          description: "实验班",
          logo_url: "space.png",
          type: "class",
          admin_user_ids: [21],
        }),
      }),
    );
  });
});
