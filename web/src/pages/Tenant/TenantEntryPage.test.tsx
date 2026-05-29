import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import type { AuthAPI, ProfileSpaceMembership } from "../../api/auth";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import { SessionProvider } from "../../auth/session";
import { TenantEntryPage } from "./TenantEntryPage";

afterEach(() => {
  window.localStorage.clear();
});

test("租户管理员按租户展示后台入口且不要求选择空间", async () => {
  const user = userEvent.setup();
  const memberships: ProfileSpaceMembership[] = [{
    id: 1,
    tenantID: 10,
    tenantName: "明德学校",
    spaceID: 301,
    spaceName: "高一空间",
    role: "tenant_admin",
    status: "enabled",
  }, {
    id: 2,
    tenantID: 10,
    tenantName: "明德学校",
    spaceID: 302,
    spaceName: "高二空间",
    role: "tenant_admin",
    status: "enabled",
  }];
  const selectTenantSpace = vi.fn(async (input) => {
    expect(input).toEqual({ tenantID: 10 });
    return {
      accessToken: "tenant-access-token",
      refreshToken: "tenant-refresh-token",
      user: { displayName: "租户管理员", role: "tenant_admin" as const, tenantID: 10, userID: 2 },
    };
  });
  const api = createTenantEntryAPI({
    listProfileSpaces: vi.fn(async () => ({ items: memberships })),
    selectTenantSpace,
  });

  renderTenantEntry(api);

  expect(await screen.findByRole("button", { name: /明德学校/ })).toBeInTheDocument();
  expect(screen.getByText("租户管理后台")).toBeInTheDocument();
  expect(screen.queryByText("高一空间")).not.toBeInTheDocument();
  expect(screen.queryByText("高二空间")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: /明德学校/ }));

  expect(selectTenantSpace).toHaveBeenCalledTimes(1);
  expect(await screen.findByText("租户后台首页")).toBeInTheDocument();
});

test("教师和学生按空间展示不同入口并保留已选空间", async () => {
  const user = userEvent.setup();
  const memberships: ProfileSpaceMembership[] = [{
    id: 3,
    tenantID: 10,
    tenantName: "明德学校",
    spaceID: 301,
    spaceName: "高一空间",
    role: "teacher",
    status: "enabled",
  }, {
    id: 4,
    tenantID: 10,
    tenantName: "明德学校",
    spaceID: 302,
    spaceName: "高二考试空间",
    role: "student",
    status: "enabled",
  }];
  const api = createTenantEntryAPI({
    listProfileSpaces: vi.fn(async () => ({ items: memberships })),
    selectTenantSpace: vi.fn(async () => ({
      accessToken: "tenant-access-token",
      refreshToken: "tenant-refresh-token",
      user: { displayName: "教师", role: "teacher" as const, tenantID: 10, userID: 3 },
    })),
  });

  renderTenantEntry(api);

  expect(await screen.findByRole("button", { name: /高一空间/ })).toHaveTextContent("教学业务入口");
  expect(screen.getByRole("button", { name: /高二考试空间/ })).toHaveTextContent("考试入口");

  await user.click(screen.getByRole("button", { name: /高一空间/ }));

  const storedSession = JSON.parse(window.localStorage.getItem(SESSION_STORAGE_KEY) ?? "{}");
  expect(storedSession.selectedSpaceID).toBe(301);
});

function renderTenantEntry(api: AuthAPI) {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      accessToken: "tenant-access-token",
      refreshToken: "tenant-refresh-token",
      user: { displayName: "租户用户", role: "tenant_user", userID: 2 },
    }),
  );

  render(
    <SessionProvider>
      <MemoryRouter initialEntries={["/tenant-entry"]}>
        <Routes>
          <Route path="/tenant-entry" element={<TenantEntryPage api={api} />} />
          <Route path="/" element={<div>租户后台首页</div>} />
          <Route path="/exam-entry" element={<div>考试入口页</div>} />
        </Routes>
      </MemoryRouter>
    </SessionProvider>,
  );
}

function createTenantEntryAPI(overrides: Pick<AuthAPI, "listProfileSpaces" | "selectTenantSpace">): AuthAPI {
  return {
    listProfileSpaces: overrides.listProfileSpaces,
    platformLogin: vi.fn(),
    selectTenantSpace: overrides.selectTenantSpace,
    tenantLogin: vi.fn(),
    tenantRegister: vi.fn(),
  };
}
