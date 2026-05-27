import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test } from "vitest";
import type { QuestionImportAPI } from "../../api/questions";
import { QuestionImportPage } from "./QuestionImportPage";

test("题目导入页可以上传文件并展示解析结果", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async (input) => {
      expect(input.tenantID).toBe(10);
      expect(input.file.name).toBe("questions.csv");
      return {
        successCount: 1,
        errors: [{ rowNumber: 3, reason: "choice question needs correct answer" }],
      };
    },
  };
  render(<QuestionImportPage api={api} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "题目导入" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "题目导入" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "导入题目" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索导入记录")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新导入记录" })).toBeInTheDocument();
  expect(screen.queryByLabelText("题目导入文件")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  expect(screen.getByRole("dialog", { name: "导入题目弹窗" })).toBeInTheDocument();

  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByRole("status")).toHaveTextContent("questions.csv");
  expect(screen.getByRole("status")).toHaveTextContent("已导入 1 道题，1 行失败");
  expect(screen.getByRole("status")).toHaveTextContent("第 3 行");
});

test("题目导入页可以搜索和刷新导入记录", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async () => ({ successCount: 1, errors: [] }),
  };
  render(<QuestionImportPage api={api} />);

  expect(screen.queryByText("sample-questions.csv")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByText("questions.csv")).toBeInTheDocument();

  await user.type(screen.getByLabelText("搜索导入记录"), "不存在");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("questions.csv")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新导入记录" }));

  expect(screen.getByText("questions.csv")).toBeInTheDocument();
});

test("取消导入后不会保留上一次选择的文件", async () => {
  const user = userEvent.setup();
  const api: QuestionImportAPI = {
    importQuestions: async () => {
      throw new Error("不应该提交已取消的文件");
    },
  };
  render(<QuestionImportPage api={api} tenantID={10} />);

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.upload(screen.getByLabelText("题目导入文件"), new File(["title,type"], "questions.csv", { type: "text/csv" }));
  await user.click(screen.getByRole("button", { name: "取消" }));

  await user.click(screen.getByRole("button", { name: "导入题目" }));
  await user.click(screen.getByRole("button", { name: "确认导入" }));

  expect(screen.getByRole("status")).toHaveTextContent("请选择题目导入文件");
});
