import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import { Select } from "./Select";

test("下拉选择使用 Ant Design Select 作为底层组件", () => {
  render(
    <Select
      ariaLabel="每页条数"
      onChange={vi.fn()}
      options={[
        { value: "20", label: "20 条 / 页" },
        { value: "50", label: "50 条 / 页" },
      ]}
      value="20"
    />,
  );

  const select = screen.getByRole("combobox", { name: "每页条数" });

  expect(select.closest(".ant-select")).toBeInTheDocument();
  expect(select.closest(".ui-select-trigger")).toBeInTheDocument();
});

test("选择选项后会收起下拉面板", async () => {
  const user = userEvent.setup();
  const handleChange = vi.fn();

  render(
    <Select
      ariaLabel="每页条数"
      onChange={handleChange}
      options={[
        { value: "20", label: "20 条 / 页" },
        { value: "50", label: "50 条 / 页" },
      ]}
      value="20"
    />,
  );

  await user.click(screen.getByRole("combobox", { name: "每页条数" }));
  expect(screen.getByText("50 条 / 页")).toBeInTheDocument();

  await user.click(screen.getByText("50 条 / 页"));

  expect(handleChange).toHaveBeenCalledWith("50");
  await waitFor(() => {
    expect(document.querySelector(".ui-select-content")).toHaveClass("ant-select-dropdown-hidden");
  });
});

test("自定义下拉样式不会覆盖 Ant Design 的隐藏状态", () => {
  const stylesheet = readFileSync(resolve(__dirname, "../../styles/global.css"), "utf8");

  expect(stylesheet).toContain(".ui-select-content.ant-select-dropdown-hidden");
  expect(stylesheet).toContain("display: none");
});
