import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createUserAPI } from "./users";

describe("user api", () => {
  test("用户列表会映射租户用户字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          items: [{
            id: 20,
            tenant_id: 10,
            username: "teacher01",
            real_name: "李老师",
            avatar_url: "",
            role: "teacher",
            status: "enabled",
          }],
          page: 1,
          page_size: 20,
          total: 1,
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listUsers(10)).resolves.toEqual({
      items: [{
        id: 20,
        tenantID: 10,
        name: "李老师",
        username: "teacher01",
        role: "teacher",
        avatarFileName: "未上传",
        status: "enabled",
      }],
    });
    expect(fetcher).toHaveBeenCalledWith("/api/v1/tenant/users?tenant_id=10", expect.objectContaining({ method: "GET" }));
  });

  test("创建用户会提交租户和角色字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 21,
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          role: "student",
          status: "enabled",
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createUser({
      tenantID: 10,
      name: "张三",
      username: "student01",
      password: "student-secure-123",
      role: "student",
      avatarFileName: "avatar.png",
    })).resolves.toMatchObject({
      id: 21,
      tenantID: 10,
      name: "张三",
      role: "student",
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          password: "student-secure-123",
          role: "student",
        }),
      }),
    );
  });


  test("启用用户会携带操作者 ID", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 21,
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          role: "student",
          status: "enabled",
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.enableUser({ tenantID: 10, actorID: 99, userID: 21 })).resolves.toMatchObject({
      id: 21,
      status: "enabled",
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users/21/enable",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          actor_id: 99,
        }),
      }),
    );
  });

  test("禁用用户会携带操作者 ID", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({
        code: 0,
        message: "ok",
        data: {
          id: 21,
          tenant_id: 10,
          username: "student01",
          real_name: "张三",
          avatar_url: "avatar.png",
          role: "student",
          status: "disabled",
        },
      })),
    );
    const api = createUserAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.disableUser({ tenantID: 10, actorID: 99, userID: 21 })).resolves.toMatchObject({
      id: 21,
      status: "disabled",
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/users/21/disable",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          tenant_id: 10,
          actor_id: 99,
        }),
      }),
    );
  });
});
