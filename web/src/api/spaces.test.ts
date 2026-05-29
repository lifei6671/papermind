import { describe, expect, test, vi } from "vitest";
import { createApiClient } from "./client";
import { createSpaceAPI } from "./spaces";

function okResponse(data: unknown) {
  return new Response(JSON.stringify({ code: 0, message: "ok", data }));
}

describe("space api", () => {
  test("空间列表会映射成员字段", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      okResponse({
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
      }),
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
          userID: 20,
          name: "李老师",
          role: "space_admin",
          status: "enabled",
        }],
      }],
    });
    expect(fetcher).toHaveBeenCalledWith("/api/v1/tenant/spaces?tenant_id=10", expect.objectContaining({ method: "GET" }));
  });

  test("创建空间会提交真实管理员用户 ID", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      okResponse({
        id: 302,
        tenant_id: 10,
        name: "高一二班",
        logo_url: "space.png",
        description: "实验班",
        members: [{
          id: 2,
          user_id: 21,
          name: "周老师",
          role: "space_admin",
          status: "enabled",
        }],
      }),
    );
    const api = createSpaceAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createSpace({
      tenantID: 10,
      name: "高一二班",
      description: "实验班",
      logoFileName: "space.png",
      adminUserID: 21,
    })).resolves.toMatchObject({
      id: 302,
      members: [{ id: 2, name: "周老师", role: "space_admin", status: "enabled" }],
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/spaces",
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

  test("空间成员列表会按授权空间接口读取", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      okResponse([{
          id: 8,
          user_id: 55,
          name: "阅卷教师",
          role: "space_admin",
          status: "enabled",
        }]),
    );
    const api = createSpaceAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.listSpaceMembers({ tenantID: 10, spaceID: 301 })).resolves.toEqual({
      items: [{
        id: 8,
        userID: 55,
        name: "阅卷教师",
        role: "space_admin",
        status: "enabled",
      }],
    });
    expect(fetcher).toHaveBeenCalledWith(
      "/api/v1/tenant/spaces/301/members?tenant_id=10",
      expect.objectContaining({ method: "GET" }),
    );
  });

  test("空间成员增删改会调用租户空间成员接口", async () => {
    const fetcher = vi.fn().mockImplementation(() => Promise.resolve(okResponse({
      id: 8,
      user_id: 55,
      name: "阅卷教师",
      role: "teacher",
      status: "enabled",
    })));
    const api = createSpaceAPI(createApiClient({ baseUrl: "", fetcher }));

    await expect(api.createSpaceMember({
      tenantID: 10,
      spaceID: 301,
      userID: 55,
      role: "teacher",
    })).resolves.toMatchObject({ userID: 55, role: "teacher" });
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenant/spaces/301/members",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ tenant_id: 10, user_id: 55, role: "teacher" }),
      }),
    );

    await expect(api.updateSpaceMember({
      tenantID: 10,
      spaceID: 301,
      userID: 55,
      role: "space_admin",
      status: "enabled",
    })).resolves.toMatchObject({ userID: 55 });
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenant/spaces/301/members/55",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ tenant_id: 10, role: "space_admin", status: "enabled" }),
      }),
    );

    await expect(api.removeSpaceMember({
      tenantID: 10,
      spaceID: 301,
      userID: 55,
    })).resolves.toBeUndefined();
    expect(fetcher).toHaveBeenLastCalledWith(
      "/api/v1/tenant/spaces/301/members/55?tenant_id=10",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
