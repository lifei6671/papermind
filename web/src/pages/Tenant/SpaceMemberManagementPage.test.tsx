import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, test, vi } from "vitest";
import type { SpaceMember, SpaceMemberAPI } from "../../api/spaces";
import { SESSION_STORAGE_KEY } from "../../auth/session-context";
import { SessionProvider } from "../../auth/session";
import { SpaceMemberManagementPage } from "./SpaceMemberManagementPage";

afterEach(() => {
  window.localStorage.clear();
});

test("空间成员页面对接成员增删改 API", async () => {
  const user = userEvent.setup();
  let members: SpaceMember[] = [{
    id: 1,
    userID: 21,
    name: "李老师",
    role: "teacher",
    status: "enabled",
  }];
  const api: SpaceMemberAPI = {
    listSpaceMembers: vi.fn(async () => ({ items: members })),
    createSpaceMember: vi.fn(async (input) => {
      const created: SpaceMember = {
        id: 2,
        userID: input.userID,
        name: `用户 ${input.userID}`,
        role: input.role,
        status: "enabled",
      };
      members = [...members, created];
      return created;
    }),
    updateSpaceMember: vi.fn(async (input) => {
      members = members.map((member) =>
        member.userID === input.userID
          ? { ...member, role: input.role ?? member.role, status: input.status ?? member.status }
          : member,
      );
      return members.find((member) => member.userID === input.userID)!;
    }),
    removeSpaceMember: vi.fn(async (input) => {
      members = members.filter((member) => member.userID !== input.userID);
    }),
  };

  renderSpaceMembers(api);

  expect(await screen.findByText(/李老师/)).toBeInTheDocument();

  await user.type(screen.getByLabelText("用户 ID"), "55");
  await user.selectOptions(screen.getByLabelText("空间身份"), "student");
  await user.click(screen.getByRole("button", { name: "添加成员" }));

  expect(api.createSpaceMember).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 301,
    userID: 55,
    role: "student",
  });
  expect(await screen.findByText("成员已添加")).toBeInTheDocument();
  expect(await screen.findByText(/用户 55/)).toBeInTheDocument();

  await user.selectOptions(screen.getByLabelText("修改 李老师 的空间身份"), "space_admin");
  expect(api.updateSpaceMember).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 301,
    userID: 21,
    role: "space_admin",
  });

  const teacherRow = screen.getByText(/李老师/).closest("tr");
  expect(teacherRow).not.toBeNull();
  await user.click(within(teacherRow!).getByRole("button", { name: "禁用" }));
  expect(api.updateSpaceMember).toHaveBeenLastCalledWith({
    tenantID: 10,
    spaceID: 301,
    userID: 21,
    status: "disabled",
  });

  await user.click(within(teacherRow!).getByRole("button", { name: "移除" }));
  expect(api.removeSpaceMember).toHaveBeenCalledWith({
    tenantID: 10,
    spaceID: 301,
    userID: 21,
  });
});

function renderSpaceMembers(api: SpaceMemberAPI) {
  window.localStorage.setItem(
    SESSION_STORAGE_KEY,
    JSON.stringify({
      profileSpaces: [{
        id: 1,
        tenantID: 10,
        tenantName: "明德学校",
        spaceID: 301,
        spaceName: "高一空间",
        role: "space_admin",
        status: "enabled",
      }],
      user: { displayName: "空间管理员", role: "teacher", tenantID: 10, userID: 20 },
    }),
  );

  render(
    <SessionProvider>
      <SpaceMemberManagementPage api={api} tenantID={10} />
    </SessionProvider>,
  );
}
