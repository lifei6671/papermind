import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { Pagination } from "./Pagination";

test("分页组件展示总数和当前页", () => {
  render(<Pagination page={2} pageSize={20} total={95} onPageChange={vi.fn()} />);

  expect(screen.getByText("共 95 条")).toBeInTheDocument();
  expect(screen.getByText("第 2 / 5 页")).toBeInTheDocument();
});

test("分页组件在边界页禁用不可用动作", () => {
  render(<Pagination page={1} pageSize={20} total={0} onPageChange={vi.fn()} />);

  expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();
});

test("点击下一页时只请求合法页码", async () => {
  const user = userEvent.setup();
  const onPageChange = vi.fn();

  render(<Pagination page={2} pageSize={20} total={95} onPageChange={onPageChange} />);

  await user.click(screen.getByRole("button", { name: "下一页" }));

  expect(onPageChange).toHaveBeenCalledWith(3);
});
