import { readFileSync } from "node:fs";
import { resolve } from "node:path";
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

test("分页组件支持在右侧分页前切换每页条数", async () => {
  const user = userEvent.setup();
  const onPageSizeChange = vi.fn();

  render(
    <Pagination
      onPageChange={vi.fn()}
      onPageSizeChange={onPageSizeChange}
      page={2}
      pageSize={20}
      pageSizeOptions={[10, 20, 50]}
      total={95}
    />,
  );

  const pagination = screen.getByRole("navigation", { name: "分页" });
  const pageSize = screen.getByRole("combobox", { name: "每页条数" });
  const antPagination = pagination.querySelector(".ant-pagination");

  expect(pageSize.closest(".pagination__page-size")).toBeInTheDocument();
  expect(antPagination).not.toBeNull();
  expect(pageSize.compareDocumentPosition(antPagination as Element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();

  await user.click(pageSize);
  await user.click(screen.getByRole("option", { name: "50 条 / 页" }));

  expect(onPageSizeChange).toHaveBeenCalledWith(50);
});

test("分页每页条数选择器保持普通输入框高度", () => {
  const stylesheet = readFileSync(resolve(__dirname, "../../styles/global.css"), "utf8");
  const selectRootRule = stylesheet.match(/\.pagination__page-size-select \.ui-select-trigger\.ant-select\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const selectorRule = stylesheet.match(/\.pagination__page-size-select \.ui-select-trigger \.ant-select-selector\s*\{[\s\S]*?\}/)?.[0] ?? "";
  const selectionRule = stylesheet.match(/\.pagination__page-size-select \.ui-select-trigger \.ant-select-selection-item,[\s\S]*?\.pagination__page-size-select \.ui-select-trigger \.ant-select-selection-placeholder\s*\{[\s\S]*?\}/)?.[0] ?? "";

  expect(selectRootRule).toContain("height: 38px");
  expect(selectRootRule).toContain("min-height: 38px");
  expect(selectorRule).toContain("height: 38px !important");
  expect(selectorRule).toContain("min-height: 38px !important");
  expect(selectionRule).toContain("line-height: 36px !important");
});
