import { render, screen } from "@testing-library/react";
import { test } from "vitest";
import { DashboardPage } from "./DashboardPage";

test("租户管理员看到租户空间概览", () => {
  render(<DashboardPage
    profileSpaces={[{
      id: 1,
      tenantID: 10,
      tenantName: "明德学校",
      spaceID: 301,
      spaceName: "高一 1 班",
      role: "space_admin",
      status: "enabled",
    }]}
    selectedSpaceID={301}
    user={{ displayName: "租户管理员", role: "tenant_admin", tenantID: 10, userID: 2 }}
  />);

  expect(screen.getByRole("heading", { name: "高一 1 班概览" })).toBeInTheDocument();
  expect(screen.getByText("租户管理员视角")).toBeInTheDocument();
  expect(screen.getByText("本空间题库与考试发布")).toBeInTheDocument();
});

test("教师看到当前授权空间的教学概览", () => {
  render(<DashboardPage
    profileSpaces={[{
      id: 1,
      tenantID: 10,
      tenantName: "明德学校",
      spaceID: 301,
      spaceName: "高一 1 班",
      role: "teacher",
      status: "enabled",
    }]}
    selectedSpaceID={301}
    user={{ displayName: "阅卷教师", role: "teacher", tenantID: 10, userID: 3 }}
  />);

  expect(screen.getByRole("heading", { name: "高一 1 班教学概览" })).toBeInTheDocument();
  expect(screen.getByText("教师视角")).toBeInTheDocument();
  expect(screen.getByText("我负责的待阅卷答卷")).toBeInTheDocument();
});
