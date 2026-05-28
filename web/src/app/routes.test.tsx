import { expect, test } from "vitest";
import { buildAdminRoutes, routeVisibleForRole } from "./routes";
import type { AdminRoute } from "./routes";
import type { ProfileSpaceAuthorization, SessionUser } from "../auth/session-context";
import { School } from "lucide-react";

test("空间管理员菜单可见性来自授权空间列表而不是 session 角色", () => {
  const user: SessionUser = {
    displayName: "空间管理员",
    role: "teacher",
    tenantID: 10,
    userID: 3,
  };
  const spaceMemberRoute: AdminRoute = {
    description: "管理当前授权空间的成员",
    element: <div>空间成员</div>,
    group: "tenant",
    icon: School,
    label: "空间成员",
    menuRoles: ["tenant_admin"],
    path: "/space-members",
    spaceMemberRoles: ["space_admin"],
  };
  const profileSpaces: ProfileSpaceAuthorization[] = [{
    id: 1,
    tenantID: 10,
    spaceID: 100,
    role: "space_admin",
    status: "enabled",
  }];

  // session 仍然只保存租户级 teacher，空间管理员身份只从启用的空间授权列表推导菜单。
  expect(routeVisibleForRole(spaceMemberRoute, user.role, profileSpaces)).toBe(true);
});

test("真实空间成员入口由授权空间列表驱动", () => {
  const routes = buildAdminRoutes({
    displayName: "空间管理员",
    role: "teacher",
    tenantID: 10,
    userID: 3,
  });
  const spaceMemberRoute = routes.find((route) => route.path === "/space-members");
  const profileSpaces: ProfileSpaceAuthorization[] = [{
    id: 1,
    tenantID: 10,
    spaceID: 100,
    role: "space_admin",
    status: "enabled",
  }];

  expect(spaceMemberRoute).toMatchObject({
    group: "tenant",
    menuRoles: ["tenant_admin"],
    spaceMemberRoles: ["space_admin"],
  });
  expect(routeVisibleForRole(spaceMemberRoute!, "teacher", profileSpaces)).toBe(true);
  expect(routeVisibleForRole(spaceMemberRoute!, "teacher", [])).toBe(false);
});
