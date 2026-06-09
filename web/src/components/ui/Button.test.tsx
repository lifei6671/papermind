import { render, screen } from "@testing-library/react";
import { expect, test } from "vitest";
import { Button } from "./Button";

test("Button 按语义 variant 输出统一样式类", () => {
  render(
    <>
      <Button variant="primary">保存</Button>
      <Button variant="toolbarPrimary">创建</Button>
      <Button aria-label="搜索" variant="icon">S</Button>
      <Button variant="actionClose">禁用</Button>
    </>,
  );

  expect(screen.getByRole("button", { name: "保存" })).toHaveClass("primary-button");
  expect(screen.getByRole("button", { name: "创建" })).toHaveClass("primary-button", "tenant-create-button");
  expect(screen.getByRole("button", { name: "搜索" })).toHaveClass("tenant-icon-button");
  expect(screen.getByRole("button", { name: "禁用" })).toHaveClass("tenant-action-button", "tenant-action--close");
  expect(screen.getByRole("button", { name: "保存" })).toHaveClass("ant-btn");
});

test("图标按钮使用 Ant Design icon 插槽承载图标", () => {
  render(
    <Button aria-label="刷新" variant="icon">
      <svg className="tenant-refresh-icon" role="img" />
    </Button>,
  );

  const button = screen.getByRole("button", { name: "刷新" });

  expect(button.querySelector(".ant-btn-icon .tenant-refresh-icon")).toBeInTheDocument();
});

test("Button 默认使用 button 类型并允许追加页面专用样式", () => {
  render(
    <Button className="student-exam-submit" variant="custom">
      提交
    </Button>,
  );

  expect(screen.getByRole("button", { name: "提交" })).toHaveAttribute("type", "button");
  expect(screen.getByRole("button", { name: "提交" })).toHaveClass("student-exam-submit");
});
