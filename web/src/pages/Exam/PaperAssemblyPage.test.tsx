import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { PaperAssemblyPage } from "./PaperAssemblyPage";

test("组卷页展示试卷列表和大题结构", () => {
  render(<PaperAssemblyPage />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "试卷" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "试卷" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "新增大题" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "生成 rule_fixed 试卷" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "保存 rule_live 规则" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索试卷")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新试卷列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("大题名称")).not.toBeInTheDocument();
  expect(screen.getByText("高一语文月考试卷")).toBeInTheDocument();
  expect(screen.getByText("一、现代文阅读")).toBeInTheDocument();
  expect(screen.getByText("rule_fixed")).toBeInTheDocument();
});

test("教师可以创建大题并手动选题", async () => {
  const user = userEvent.setup();
  render(<PaperAssemblyPage />);

  await user.click(screen.getByRole("button", { name: "新增大题" }));

  expect(screen.getByRole("dialog", { name: "新增大题弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("大题名称"), "三、函数综合");
  await user.type(screen.getByLabelText("大题分值"), "24");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(screen.getByText("三、函数综合")).toBeInTheDocument();

  await user.click(within(screen.getByRole("row", { name: /现代文阅读主旨题/ })).getByRole("button", { name: "加入试卷" }));

  expect(screen.getByRole("status")).toHaveTextContent("已手动加入 1 道题");
});

test("教师可以搜索和刷新试卷列表", async () => {
  const user = userEvent.setup();
  render(<PaperAssemblyPage />);

  await user.type(screen.getByLabelText("搜索试卷"), "不存在");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("高一语文月考试卷")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新试卷列表" }));

  expect(screen.getByText("高一语文月考试卷")).toBeInTheDocument();
});

test("rule_fixed 可以配置、生成、审题并替换题目", async () => {
  const user = userEvent.setup();
  render(<PaperAssemblyPage />);

  await user.click(screen.getByRole("button", { name: "生成 rule_fixed 试卷" }));

  expect(screen.getByRole("dialog", { name: "rule_fixed 弹窗" })).toBeInTheDocument();

  await user.clear(screen.getByLabelText("固定规则题量"));
  await user.type(screen.getByLabelText("固定规则题量"), "6");
  await user.selectOptions(screen.getByLabelText("固定规则标签"), "阅读理解");
  await user.click(screen.getByRole("button", { name: "确认生成" }));

  expect(screen.getByRole("status", { name: "rule-fixed-result" })).toHaveTextContent("已按阅读理解生成 6 道题");

  await user.click(screen.getByRole("button", { name: "替换低匹配题" }));

  expect(screen.getByRole("status", { name: "rule-fixed-review" })).toHaveTextContent("已替换 1 道低匹配题");
});

test("rule_live 配置和组卷预检查展示风险提示", async () => {
  const user = userEvent.setup();
  render(<PaperAssemblyPage />);

  await user.click(screen.getByRole("button", { name: "保存 rule_live 规则" }));

  expect(screen.getByRole("dialog", { name: "rule_live 弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("动态规则说明"), "优先补齐近三次错题标签");
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  expect(screen.getByRole("status", { name: "rule-live-result" })).toHaveTextContent("优先补齐近三次错题标签");

  await user.click(screen.getByRole("button", { name: "运行组卷预检查" }));

  expect(screen.getByRole("alert")).toHaveTextContent("预检查通过");
  expect(screen.getByRole("alert")).toHaveTextContent("已覆盖 2 个大题");
});
