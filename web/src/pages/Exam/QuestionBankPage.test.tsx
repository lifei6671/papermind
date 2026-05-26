import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import { QuestionBankPage } from "./QuestionBankPage";

test("题库页展示题目列表、标签和解析摘要", () => {
  render(<QuestionBankPage />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "题库" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "题库" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "保存题目" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "新增标签" })).toHaveClass("tenant-create-button");
  expect(screen.getByRole("button", { name: "导入题目" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索题目")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新题目列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("题目标题")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("题目导入文件")).not.toBeInTheDocument();
  expect(screen.getByText("现代文阅读主旨题")).toBeInTheDocument();
  expect(screen.getByText("阅读理解")).toBeInTheDocument();
  expect(screen.getByText("解析：定位中心句并排除以偏概全选项。")).toBeInTheDocument();
});

test("教师可以在线创建选择题并编辑选项和解析", async () => {
  const user = userEvent.setup();
  render(<QuestionBankPage />);

  await user.click(screen.getByRole("button", { name: "保存题目" }));

  expect(screen.getByRole("dialog", { name: "保存题目弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("题目标题"), "函数单调性判断");
  await user.type(screen.getByLabelText("题干"), "下列函数在 R 上单调递增的是哪一项？");
  await user.clear(screen.getByLabelText("选项 A"));
  await user.type(screen.getByLabelText("选项 A"), "y = x");
  await user.clear(screen.getByLabelText("选项 B"));
  await user.type(screen.getByLabelText("选项 B"), "y = -x");
  await user.type(screen.getByLabelText("题目解析"), "一次函数斜率为正时单调递增。");
  await user.type(screen.getByLabelText("题目标签"), "函数");
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  const row = screen.getByRole("row", { name: /函数单调性判断/ });
  expect(within(row).getByText("函数")).toBeInTheDocument();
  expect(within(row).getByText("解析：一次函数斜率为正时单调递增。")).toBeInTheDocument();
});

test("教师可以搜索和刷新题目列表", async () => {
  const user = userEvent.setup();
  render(<QuestionBankPage />);

  await user.type(screen.getByLabelText("搜索题目"), "语言文字");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("现代文阅读主旨题")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新题目列表" }));

  expect(screen.getByText("现代文阅读主旨题")).toBeInTheDocument();
});

test("教师可以新增标签并导入题目文件", async () => {
  const user = userEvent.setup();
  render(<QuestionBankPage />);

  await user.click(screen.getByRole("button", { name: "新增标签" }));

  expect(screen.getByRole("dialog", { name: "新增标签弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("新标签名称"), "函数");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(screen.getByText("函数")).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  expect(screen.getByRole("dialog", { name: "导入题目弹窗" })).toBeInTheDocument();

  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByRole("status")).toHaveTextContent("questions.csv");
  expect(screen.getByRole("status")).toHaveTextContent("已解析 18 道题");
});
