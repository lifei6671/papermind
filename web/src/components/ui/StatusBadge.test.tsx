import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { StatusBadge } from "./StatusBadge";

test("状态标签使用 Ant Design Tag 作为底层组件", () => {
  render(<StatusBadge tone="success">已启用</StatusBadge>);

  const badge = screen.getByText("已启用");

  expect(badge).toHaveClass("ant-tag");
  expect(badge).toHaveClass("status-badge");
  expect(badge).toHaveClass("status-badge--success");
});
