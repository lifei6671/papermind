import { expect, test } from "vitest";
import type { ReactElement } from "react";
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
    menuRoles: [],
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
    menuRoles: [],
    spaceMemberRoles: ["space_admin"],
  });
  expect(routeVisibleForRole(spaceMemberRoute!, "teacher", profileSpaces)).toBe(true);
  expect(routeVisibleForRole(spaceMemberRoute!, "teacher", [])).toBe(false);
  expect(routeVisibleForRole(spaceMemberRoute!, "tenant_admin", [])).toBe(false);
});
test("概览入口对管理端角色显示在左侧菜单", () => {
  const route = buildAdminRoutes({
    displayName: "租户管理员",
    role: "tenant_admin",
    tenantID: 10,
    userID: 2,
  }).find((item) => item.path === "/")!;
  const teacherSpaces: ProfileSpaceAuthorization[] = [{
    id: 1,
    tenantID: 10,
    spaceID: 301,
    role: "teacher",
    status: "enabled",
  }];

  expect(route.group).toBe("overview");
  expect(routeVisibleForRole(route, "platform_admin")).toBe(true);
  expect(routeVisibleForRole(route, "tenant_admin")).toBe(true);
  expect(routeVisibleForRole(route, "teacher", teacherSpaces, { tenantID: 10, spaceID: 301 })).toBe(true);
  expect(routeVisibleForRole(route, "student")).toBe(false);
});

test("空间管理员授权可看到空间考试业务入口", () => {
  const routes = buildAdminRoutes({
    displayName: "空间管理员",
    role: "student",
    tenantID: 10,
    userID: 3,
  }, 301);
  const profileSpaces: ProfileSpaceAuthorization[] = [{
    id: 1,
    tenantID: 10,
    spaceID: 301,
    role: "space_admin",
    status: "enabled",
  }];

  for (const path of ["/questions", "/imports", "/papers", "/exams", "/grading", "/results"]) {
    const route = routes.find((item) => item.path === path);
    expect(routeVisibleForRole(route!, "student", profileSpaces)).toBe(true);
    expect(routeSpaceID(routes, path)).toBe(301);
  }
});

test("考试业务路由会收到当前选择的空间范围", () => {
  const routes = buildAdminRoutes({
    displayName: "教师",
    role: "teacher",
    tenantID: 10,
    userID: 3,
  }, 301, [], 42);

  expect(routeSpaceID(routes, "/questions")).toBe(301);
  expect(routeSpaceID(routes, "/grading")).toBe(301);
  expect(routeSpaceID(routes, "/results")).toBe(301);
  expect(routeExamID(routes, "/grading")).toBe(42);
  expect(routeExamID(routes, "/results")).toBe(42);
});

test("空间管理员页面 actorRole 来自当前选中空间授权", () => {
  const routes = buildAdminRoutes({
    displayName: "空间管理员",
    role: "teacher",
    tenantID: 10,
    userID: 3,
  }, 301, [{
    id: 1,
    tenantID: 10,
    spaceID: 301,
    role: "space_admin",
    status: "enabled",
  }]);

  expect(routeActorRole(routes, "/grading")).toBe("space_admin");
  expect(routeActorRole(routes, "/results")).toBe("space_admin");
});

test("空间授权菜单只对当前选中空间生效", () => {
  const route = buildAdminRoutes({
    displayName: "多空间用户",
    role: "student",
    tenantID: 10,
    userID: 3,
  }, 100).find((item) => item.path === "/questions")!;
  const profileSpaces: ProfileSpaceAuthorization[] = [
    {
      id: 1,
      tenantID: 10,
      spaceID: 100,
      role: "student",
      status: "enabled",
    },
    {
      id: 2,
      tenantID: 10,
      spaceID: 200,
      role: "teacher",
      status: "enabled",
    },
  ];

  expect(routeVisibleForRole(route, "student", profileSpaces, { tenantID: 10, spaceID: 100 })).toBe(false);
  expect(routeVisibleForRole(route, "student", profileSpaces, { tenantID: 10, spaceID: 200 })).toBe(true);
});

function routeSpaceID(routes: AdminRoute[], path: string) {
  const element = routes.find((route) => route.path === path)?.element as ReactElement<{ spaceID?: number }> | undefined;
  return element?.props.spaceID;
}

function routeActorRole(routes: AdminRoute[], path: string) {
  const element = routes.find((route) => route.path === path)?.element as ReactElement<{ actorRole?: string }> | undefined;
  return element?.props.actorRole;
}

function routeExamID(routes: AdminRoute[], path: string) {
  const element = routes.find((route) => route.path === path)?.element as ReactElement<{ examID?: number }> | undefined;
  return element?.props.examID;
}
