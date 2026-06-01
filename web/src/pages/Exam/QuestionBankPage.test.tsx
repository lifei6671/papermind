import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import type { QuestionAPI } from "../../api/questions";
import { QuestionBankPage } from "./QuestionBankPage";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });

  return { promise, resolve };
}

function createQuestionAPI() {
  return {
    listQuestions: vi.fn().mockResolvedValue({
      items: [
        {
          id: 100,
          tenantID: 10,
          type: "single",
          title: "现代文阅读主旨题",
          stem: "下列选项最能概括文章中心的是哪一项？",
          options: ["把握中心句", "复述细节"],
          analysis: "定位中心句并排除以偏概全选项。",
          difficulty: "medium",
          tag: "阅读理解",
          tags: ["阅读理解"],
          scoreDefault: "4",
          status: "ready",
        },
      ],
    }),
    createQuestion: vi.fn().mockResolvedValue({
      id: 101,
      tenantID: 10,
      type: "single",
      title: "下列函数在 R 上单调递增的是哪一项？",
      stem: "下列函数在 R 上单调递增的是哪一项？",
      options: ["y = x", "y = -x"],
      analysis: "一次函数斜率为正时单调递增。",
      difficulty: "hard",
      tag: "函数",
      tags: ["阅读理解", "函数"],
      scoreDefault: "6",
      status: "ready",
    }),
    importQuestions: vi.fn().mockResolvedValue({
      successCount: 2,
      errors: [],
    }),
  };
}

test("题库页展示题目列表并隐藏本地标签管理入口", async () => {
  const api = createQuestionAPI();
  render(<QuestionBankPage api={api} tenantID={10} />);

  expect(screen.getAllByRole("tab")).toHaveLength(1);
  expect(screen.getByRole("tab", { name: "题库" })).toHaveAttribute("aria-selected", "true");
  expect(screen.queryByRole("heading", { name: "题库" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "新增题目" })).toHaveAttribute("href", "/questions/new");
  expect(screen.queryByRole("button", { name: "新增标签" })).not.toBeInTheDocument();
  expect(screen.queryByLabelText("题目标签列表")).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "导入题目" })).toHaveClass("tenant-create-button");
  expect(screen.getByLabelText("搜索题目")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新题目列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("题目导入文件")).not.toBeInTheDocument();
  expect(await screen.findByText("现代文阅读主旨题")).toBeInTheDocument();
  expect(screen.getByRole("cell", { name: "阅读理解" })).toBeInTheDocument();
  expect(screen.getByText("解析：定位中心句并排除以偏概全选项。")).toBeInTheDocument();
  expect(api.listQuestions).toHaveBeenCalledWith({ tenantID: 10 });
});

test("教师可以搜索和刷新题目列表", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  render(<QuestionBankPage api={api} tenantID={10} />);

  await screen.findByText("现代文阅读主旨题");
  const refreshResult = deferred<Awaited<ReturnType<QuestionAPI["listQuestions"]>>>();
  vi.mocked(api.listQuestions).mockReturnValueOnce(refreshResult.promise);

  await user.type(screen.getByLabelText("搜索题目"), "语言文字");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("现代文阅读主旨题")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新题目列表" }));
  expect(screen.getByRole("button", { name: "刷新题目列表" }).querySelector("svg")).toHaveClass(
    "tenant-refresh-icon--spinning",
  );

  refreshResult.resolve({
    items: [{
      id: 100,
      tenantID: 10,
      type: "single",
      title: "现代文阅读主旨题",
      stem: "下列选项最能概括文章中心的是哪一项？",
      options: ["把握中心句", "复述细节"],
      analysis: "定位中心句并排除以偏概全选项。",
      difficulty: "medium",
      tag: "阅读理解",
      tags: ["阅读理解"],
      scoreDefault: "4",
      status: "ready",
    }],
  });

  expect(await screen.findByText("现代文阅读主旨题")).toBeInTheDocument();
});

test("教师可以在题库右侧抽屉导入题目", async () => {
  const user = userEvent.setup();
  const api = createQuestionAPI();
  render(<QuestionBankPage api={api} tenantID={10} spaceID={301} />);

  await screen.findByText("现代文阅读主旨题");

  await user.click(screen.getByRole("button", { name: "导入题目" }));

  const drawer = screen.getByRole("dialog", { name: "题目导入抽屉" });
  expect(drawer).toHaveClass("tenant-resource-drawer--open");

  const file = new File(["type,title"], "questions.csv", { type: "text/csv" });
  await user.upload(within(drawer).getByLabelText("题目导入文件"), file);
  await user.click(within(drawer).getByRole("button", { name: "确认导入" }));

  expect(api.importQuestions).toHaveBeenCalledWith({ tenantID: 10, spaceID: 301, file });
  expect(await within(drawer).findByRole("status")).toHaveTextContent("导入成功 2 条");
  expect(within(drawer).getAllByText("questions.csv").length).toBeGreaterThan(0);
});
