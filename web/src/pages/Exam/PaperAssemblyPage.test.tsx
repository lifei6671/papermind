import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { expect, test, vi } from "vitest";
import type { PaperAssemblyAPI } from "../../api/papers";
import type { QuestionBankAPI } from "../../api/questions";
import { PaperAssemblyPage } from "./PaperAssemblyPage";

test("组卷页通过真实 API 展示试卷列表和大题结构", async () => {
  render(<PaperAssemblyPage api={createPaperApiDouble()} questionApi={createQuestionApiDouble()} />);

  expect(screen.getAllByRole("tab")).toHaveLength(3);
  expect(screen.getByRole("tab", { name: "试卷" })).toHaveAttribute("aria-selected", "true");
  expect(screen.getByRole("tab", { name: "题目" })).toHaveAttribute("aria-selected", "false");
  expect(screen.getByRole("tab", { name: "组卷规则" })).toHaveAttribute("aria-selected", "false");
  expect(screen.queryByRole("heading", { name: "试卷" })).not.toBeInTheDocument();
  expect(screen.getByRole("button", { name: "新增大题" })).toHaveClass("tenant-create-button");
  expect(screen.queryByRole("button", { name: "生成固定规则试卷" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "保存 rule_live 规则" })).not.toBeInTheDocument();
  expect(screen.getByLabelText("搜索试卷")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "刷新试卷列表" })).toBeInTheDocument();
  expect(screen.queryByLabelText("大题名称")).not.toBeInTheDocument();
  expect(await screen.findByText("高一语文月考试卷")).toBeInTheDocument();
  expect(await screen.findByText("一、现代文阅读")).toBeInTheDocument();
  expect(screen.getByText("rule_fixed")).toBeInTheDocument();
  expect(screen.queryByText("现代文阅读主旨题")).not.toBeInTheDocument();
});

test("教师可以创建大题并手动选题", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  render(<PaperAssemblyPage api={api} questionApi={createQuestionApiDouble()} />);

  await screen.findByText("一、现代文阅读");
  await user.click(screen.getByRole("button", { name: "新增大题" }));

  expect(screen.getByRole("dialog", { name: "新增大题弹窗" })).toBeInTheDocument();

  await user.type(screen.getByLabelText("大题名称"), "三、函数综合");
  await user.type(screen.getByLabelText("大题分值"), "24");
  await user.click(screen.getByRole("button", { name: "确认新增" }));

  expect(await screen.findByText("三、函数综合")).toBeInTheDocument();
  expect(api.createSection).toHaveBeenCalledWith(expect.objectContaining({
    tenantID: 10,
    paperID: 100,
    name: "三、函数综合",
  }));

  await user.click(screen.getByRole("tab", { name: "题目" }));
  await user.click(within(screen.getByRole("row", { name: /现代文阅读主旨题/ })).getByRole("button", { name: "加入试卷" }));

  await waitFor(() => {
    expect(api.addManualQuestion).toHaveBeenCalledWith({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      questionID: 101,
      score: "6",
    });
  });
  expect(screen.getByRole("status")).toHaveTextContent("已手动加入 1 道题");
});

test("教师可以搜索和刷新试卷列表", async () => {
  const user = userEvent.setup();
  render(<PaperAssemblyPage api={createPaperApiDouble()} questionApi={createQuestionApiDouble()} />);

  await screen.findByText("高一语文月考试卷");

  await user.type(screen.getByLabelText("搜索试卷"), "不存在");
  await user.click(screen.getByRole("button", { name: "搜索" }));

  expect(screen.queryByText("高一语文月考试卷")).not.toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "刷新试卷列表" }));

  expect(screen.getByText("高一语文月考试卷")).toBeInTheDocument();
});

test("rule_fixed 可以配置、生成、审题并替换题目", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  render(<PaperAssemblyPage api={api} questionApi={createQuestionApiDouble()} />);

  await user.click(screen.getByRole("tab", { name: "组卷规则" }));
  await user.click(screen.getByRole("button", { name: "生成固定规则试卷" }));

  expect(screen.getByRole("dialog", { name: "rule_fixed 弹窗" })).toBeInTheDocument();

  await user.clear(screen.getByLabelText("固定规则题量"));
  await user.type(screen.getByLabelText("固定规则题量"), "6");
  await user.selectOptions(screen.getByLabelText("固定规则标签"), "阅读理解");
  await user.click(screen.getByRole("button", { name: "确认生成" }));

  await waitFor(() => {
    expect(api.createRule).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      tagIDs: [1],
      questionCount: 6,
      scorePerQuestion: "6",
    }));
    expect(api.generateRuleFixed).toHaveBeenCalledWith({ tenantID: 10, paperID: 100 });
  });
  expect(screen.getByRole("status", { name: "rule-fixed-result" })).toHaveTextContent("已按阅读理解生成 6 道题");

  await user.click(screen.getByRole("button", { name: "替换低匹配题" }));

  expect(screen.getByRole("status", { name: "rule-fixed-review" })).toHaveTextContent("已替换 1 道低匹配题");
});

test("rule_live 配置和组卷预检查展示风险提示", async () => {
  const user = userEvent.setup();
  const api = createPaperApiDouble();
  render(<PaperAssemblyPage api={api} questionApi={createQuestionApiDouble()} />);

  await user.click(screen.getByRole("tab", { name: "组卷规则" }));
  await user.click(screen.getByRole("button", { name: "保存 rule_live 规则" }));

  expect(screen.getByRole("dialog", { name: "rule_live 弹窗" })).toBeInTheDocument();

  await user.clear(screen.getByLabelText("动态规则题量"));
  await user.type(screen.getByLabelText("动态规则题量"), "3");
  await user.selectOptions(screen.getByLabelText("动态规则标签"), "语言文字");
  await user.click(screen.getByRole("button", { name: "确认保存" }));

  await waitFor(() => {
    expect(api.createRule).toHaveBeenCalledWith(expect.objectContaining({
      tenantID: 10,
      paperID: 100,
      sectionID: 1,
      tagIDs: [2],
      questionCount: 3,
      scorePerQuestion: "6",
    }));
  });
  expect(screen.getByRole("status", { name: "rule-live-result" })).toHaveTextContent("语言文字");

  await user.click(screen.getByRole("button", { name: "运行组卷预检查" }));

  await waitFor(() => {
    expect(api.precheckRuleLive).toHaveBeenCalledWith({ tenantID: 10, paperID: 100 });
  });
  expect(screen.getByRole("alert")).toHaveTextContent("预检查通过");
  expect(screen.getByRole("alert")).toHaveTextContent("候选题池 2 道题");
});

function createPaperApiDouble(): PaperAssemblyAPI {
  return {
    listPapers: vi.fn(async () => ({
      items: [{
        id: 100,
        tenantID: 10,
        name: "高一语文月考试卷",
        totalScore: "0",
        buildMode: "rule_fixed",
        status: "draft" as const,
      }],
    })),
    createPaper: vi.fn(async (input) => ({
      id: 101,
      tenantID: input.tenantID,
      ...(input.spaceID === undefined ? {} : { spaceID: input.spaceID }),
      name: input.name,
      description: input.description ?? "",
      totalScore: "0",
      buildMode: "manual",
      status: "draft" as const,
    })),
    deletePaper: vi.fn(async () => undefined),
    listSections: vi.fn(async () => ({
      items: [
        {
          id: 1,
          tenantID: 10,
          paperID: 100,
          sortOrder: 1,
          name: "一、现代文阅读",
          questionType: "single",
          totalScore: "30",
          questionCount: 5,
        },
        {
          id: 2,
          tenantID: 10,
          paperID: 100,
          sortOrder: 2,
          name: "二、语言文字运用",
          questionType: "single",
          totalScore: "20",
          questionCount: 5,
        },
      ],
    })),
    createSection: vi.fn(async (input) => ({
      id: 3,
      tenantID: input.tenantID,
      paperID: input.paperID,
      sortOrder: 3,
      name: input.name,
      questionType: input.questionType,
      totalScore: "0",
      questionCount: 0,
    })),
    addManualQuestion: vi.fn(async (input) => ({
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      questionID: input.questionID,
      sortOrder: 1,
      score: input.score,
    })),
    listRules: vi.fn(async () => ({ items: [] })),
    createRule: vi.fn(async (input) => ({
      id: 301,
      tenantID: input.tenantID,
      paperID: input.paperID,
      sectionID: input.sectionID,
      sortOrder: input.sortOrder,
      difficulty: input.difficulty,
      tagIDs: input.tagIDs,
      questionCount: input.questionCount,
      scorePerQuestion: input.scorePerQuestion,
      shuffleOptions: input.shuffleOptions,
    })),
    generateRuleFixed: vi.fn(async (input) => ({
      paperID: input.paperID,
      generated: true,
    })),
    precheckRuleLive: vi.fn(async () => ({
      candidateQuestionIDs: [101, 102],
      candidateCount: 2,
    })),
  };
}

function createQuestionApiDouble(): QuestionBankAPI {
  return {
    listQuestions: vi.fn(async () => ({
      items: [{
        id: 101,
        tenantID: 10,
        title: "现代文阅读主旨题",
        stem: "阅读文本后选择最准确的主旨。",
        options: ["把握中心句", "复述细节"],
        analysis: "定位中心句并排除以偏概全选项。",
        tag: "阅读理解",
        scoreDefault: "6",
        status: "ready" as const,
      }],
    })),
    createQuestion: vi.fn(),
  };
}
