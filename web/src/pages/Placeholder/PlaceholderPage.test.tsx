import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { PlaceholderPage } from "./PlaceholderPage";

test("占位页使用租户管理同款右侧区域骨架", () => {
  render(<PlaceholderPage title="考试" description="考试草稿、发布范围、邀请码和时间窗口将在这里形成闭环。" />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "考试" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "考试" })).not.toBeInTheDocument();
  expect(screen.getByText("页面骨架已预留")).toBeInTheDocument();
});
