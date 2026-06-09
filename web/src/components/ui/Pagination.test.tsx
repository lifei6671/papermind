import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { Pagination } from "./Pagination";

test("分页组件展示总数和当前页", () => {
  render(<Pagination page={2} pageSize={20} total={95} onPageChange={vi.fn()} />);

  expect(screen.getByRole("navigation", { name: "分页" })).toHaveClass("pagination--right");
  expect(screen.getByText("共 95 条")).toBeInTheDocument();
  expect(screen.getByTitle("2")).toHaveClass("ant-pagination-item-active");
  expect(document.querySelector(".ant-pagination")).toBeInTheDocument();
});

test("分页组件在边界页禁用不可用动作", () => {
  render(<Pagination page={1} pageSize={20} total={0} onPageChange={vi.fn()} />);

  expect(document.querySelector(".ant-pagination-prev")).toHaveClass("ant-pagination-disabled");
  expect(document.querySelector(".ant-pagination-next")).toHaveClass("ant-pagination-disabled");
});

test("点击分页动作时只请求合法页码", async () => {
  const user = userEvent.setup();
  const onPageChange = vi.fn();

  render(<Pagination page={2} pageSize={20} total={95} onPageChange={onPageChange} />);

  await user.click(screen.getByTitle("1"));
  await user.click(screen.getByTitle("3"));
  await user.click(screen.getByTitle("5"));

  expect(onPageChange).toHaveBeenCalledWith(1);
  expect(onPageChange).toHaveBeenCalledWith(3);
  expect(onPageChange).toHaveBeenCalledWith(5);
});
